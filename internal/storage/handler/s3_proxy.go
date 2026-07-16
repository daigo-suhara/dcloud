package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
	"github.com/daigo-suhara/dcloud/internal/storage/service"
)

// S3Proxy exposes object-level HTTP operations against Ceph RGW,
// authenticated with per-bucket credentials that GetBucketCredentials
// returns. Registered at /api/v1/storage/{name}/... in main.
type S3Proxy struct {
	svc        *service.Server
	verifier   *jwtverify.Verifier
	cookieName string
}

func NewS3Proxy(svc *service.Server, verifier *jwtverify.Verifier) *S3Proxy {
	return &S3Proxy{
		svc:        svc,
		verifier:   verifier,
		cookieName: envDefault("DCLD_SESSION_COOKIE_NAME", "dcloud_session"),
	}
}

// Register mounts the four object endpoints on mux.
func (p *S3Proxy) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/storage/{name}/objects", http.HandlerFunc(p.listObjects))
	mux.Handle("POST /api/v1/storage/{name}/objects", http.HandlerFunc(p.uploadObject))
	mux.Handle("DELETE /api/v1/storage/{name}/objects", http.HandlerFunc(p.deleteObject))
	mux.Handle("GET /api/v1/storage/{name}/download", http.HandlerFunc(p.downloadObject))
}

func (p *S3Proxy) listObjects(w http.ResponseWriter, r *http.Request) {
	ctx, s3c, bucket, ok := p.prepare(w, r)
	if !ok {
		return
	}
	prefix := r.URL.Query().Get("prefix")
	resp, err := s3c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	objects := make([]map[string]any, 0, len(resp.Contents))
	for _, obj := range resp.Contents {
		if aws.ToString(obj.Key) == prefix {
			continue
		}
		size := int64(0)
		if obj.Size != nil {
			size = *obj.Size
		}
		last := ""
		if obj.LastModified != nil {
			last = obj.LastModified.UTC().Format("2006-01-02T15:04:05Z")
		}
		objects = append(objects, map[string]any{
			"key":          aws.ToString(obj.Key),
			"size":         size,
			"lastModified": last,
		})
	}
	prefixes := make([]string, 0, len(resp.CommonPrefixes))
	for _, cp := range resp.CommonPrefixes {
		prefixes = append(prefixes, aws.ToString(cp.Prefix))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"objects":  objects,
		"prefixes": prefixes,
		"prefix":   prefix,
	})
}

func (p *S3Proxy) uploadObject(w http.ResponseWriter, r *http.Request) {
	ctx, s3c, bucket, ok := p.prepare(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()
	prefix := r.URL.Query().Get("prefix")
	filename := header.Filename
	if filename == "" {
		filename = "upload"
	}
	key := prefix + filename
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, err := s3c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(contentType),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"key": key})
}

func (p *S3Proxy) deleteObject(w http.ResponseWriter, r *http.Request) {
	ctx, s3c, bucket, ok := p.prepare(w, r)
	if !ok {
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}
	if _, err := s3c.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (p *S3Proxy) downloadObject(w http.ResponseWriter, r *http.Request) {
	ctx, s3c, bucket, ok := p.prepare(w, r)
	if !ok {
		return
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}
	resp, err := s3c.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	contentType := aws.ToString(resp.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	filename := key
	if idx := strings.LastIndex(filename, "/"); idx >= 0 {
		filename = filename[idx+1:]
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename)))
	if resp.ContentLength != nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", *resp.ContentLength))
	}
	_, _ = io.Copy(w, resp.Body)
}

// prepare authenticates the caller, fetches per-bucket credentials from
// the storage service, and constructs an S3 client. Returns the derived
// context and s3 handle; on any failure writes an HTTP error and returns
// ok=false.
func (p *S3Proxy) prepare(w http.ResponseWriter, r *http.Request) (context.Context, *s3.Client, string, bool) {
	claims, err := p.authenticate(r)
	if err != nil {
		http.Error(w, "unauthenticated: "+err.Error(), http.StatusUnauthorized)
		return nil, nil, "", false
	}
	bucketName := strings.TrimSpace(r.PathValue("name"))
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if projectID == "" {
		projectID = strings.TrimSpace(r.Header.Get("X-DCP-Project"))
	}
	if bucketName == "" || projectID == "" {
		http.Error(w, "bucket name and projectId are required", http.StatusBadRequest)
		return nil, nil, "", false
	}
	credsResp, err := p.svc.GetBucketCredentials(r.Context(), &storagepb.GetBucketCredentialsRequest{
		UserId:    claims.Subject,
		ProjectId: projectID,
		Name:      bucketName,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return nil, nil, "", false
	}
	creds := credsResp.GetCredentials()
	if creds == nil || creds.GetEndpoint() == "" {
		http.Error(w, "bucket has no credentials", http.StatusFailedDependency)
		return nil, nil, "", false
	}
	s3c := s3.New(s3.Options{
		Region:       "us-east-1",
		BaseEndpoint: aws.String(creds.GetEndpoint()),
		UsePathStyle: true,
		Credentials: credentials.NewStaticCredentialsProvider(
			creds.GetAccessKeyId(), creds.GetSecretAccessKey(), "",
		),
	})
	return r.Context(), s3c, creds.GetBucketName(), true
}

func (p *S3Proxy) authenticate(r *http.Request) (*jwtverify.Claims, error) {
	token := ""
	if cookie, err := r.Cookie(p.cookieName); err == nil {
		token = strings.TrimSpace(cookie.Value)
	}
	if token == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}
	}
	if token == "" {
		return nil, errors.New("missing JWT (cookie or Authorization header)")
	}
	return p.verifier.Verify(r.Context(), token)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func envDefault(_, fallback string) string {
	return fallback
}
