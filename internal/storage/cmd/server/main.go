package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HealthRequest = storagepb.HealthRequest
type HealthResponse = storagepb.HealthResponse
type Bucket = storagepb.Bucket
type BucketCredentials = storagepb.BucketCredentials
type ListBucketsRequest = storagepb.ListBucketsRequest
type ListBucketsResponse = storagepb.ListBucketsResponse
type CreateBucketRequest = storagepb.CreateBucketRequest
type CreateBucketResponse = storagepb.CreateBucketResponse
type DeleteBucketRequest = storagepb.DeleteBucketRequest
type DeleteBucketResponse = storagepb.DeleteBucketResponse
type GetBucketCredentialsRequest = storagepb.GetBucketCredentialsRequest
type GetBucketCredentialsResponse = storagepb.GetBucketCredentialsResponse
type GetOperationRequest = storagepb.GetOperationRequest
type GetOperationResponse = storagepb.GetOperationResponse
type StorageServer = storagepb.ObjectStorageServiceServer

func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "storage-op-" + hex.EncodeToString(buf), nil
}

type bucketRecord struct {
	Name      string
	Endpoint  string
	Ready     bool
	Status    string
	CreatedAt string
	ProjectID string
	// internal resource name (hashed)
	ResourceName string
}

type storageServer struct {
	storagepb.UnimplementedObjectStorageServiceServer
	namespace  string
	db         *sql.DB
	q          *dbsqlc.Queries
	kube       *kubeClient
	storageClass string
	rgwEndpoint  string
}

func newStorageServer(namespace string) (*storageServer, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	kube, err := newKubeClient()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &storageServer{
		namespace:    namespace,
		db:           database,
		q:            dbsqlc.New(database),
		kube:         kube,
		storageClass: env("DCLD_BUCKET_STORAGE_CLASS", "rook-ceph-delete-bucket"),
		rgwEndpoint:  env("DCLD_RGW_ENDPOINT", ""),
	}, nil
}

func (s *storageServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *storageServer) Health(context.Context, *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "storage", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *storageServer) projectExists(ctx context.Context, userID, projectID string) (bool, error) {
	return s.q.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
}

func (s *storageServer) ListBuckets(ctx context.Context, req *ListBucketsRequest) (*ListBucketsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	records, err := s.kube.listOBCs(ctx, s.namespace, userID, projectID)
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to list buckets")
	}
	items := make([]*Bucket, 0, len(records))
	for _, r := range records {
		items = append(items, &Bucket{
			Name:      r.Name,
			Endpoint:  r.Endpoint,
			Ready:     r.Ready,
			Status:    r.Status,
			CreatedAt: r.CreatedAt,
			ProjectId: r.ProjectID,
		})
	}
	return &ListBucketsResponse{UserId: userID, ProjectId: projectID, Buckets: items}, nil
}

func (s *storageServer) CreateBucket(ctx context.Context, req *CreateBucketRequest) (*CreateBucketResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	if !isDNSLabel(name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a DNS label")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	record, err := s.kube.createOBC(ctx, s.namespace, userID, projectID, name, s.storageClass)
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, errInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, errAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "bucket already exists")
		}
		return nil, status.Error(codes.Internal, "failed to create bucket")
	}
	return &CreateBucketResponse{Bucket: &Bucket{
		Name:      record.Name,
		Endpoint:  s.rgwEndpoint,
		Ready:     record.Ready,
		Status:    record.Status,
		CreatedAt: record.CreatedAt,
		ProjectId: record.ProjectID,
	}}, nil
}

func (s *storageServer) DeleteBucket(ctx context.Context, req *DeleteBucketRequest) (*DeleteBucketResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	opID, err := newOperationID()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.q.CreateOperation(ctx, dbsqlc.CreateOperationParams{
		ID:           opID,
		ResourceType: sql.NullString{String: "bucket", Valid: true},
		ResourceName: sql.NullString{String: name, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	go func() {
		bgCtx := context.Background()
		resourceName := bucketResourceName(userID, projectID, name)
		if err := s.kube.deleteOBC(bgCtx, s.namespace, resourceName); err != nil {
			_ = s.q.UpdateOperation(bgCtx, dbsqlc.UpdateOperationParams{
				ID:        opID,
				Status:    "error",
				Error:     sql.NullString{String: err.Error(), Valid: true},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}()
	return &DeleteBucketResponse{OperationId: opID}, nil
}

func (s *storageServer) GetBucketCredentials(ctx context.Context, req *GetBucketCredentialsRequest) (*GetBucketCredentialsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	resourceName := bucketResourceName(userID, projectID, name)
	creds, err := s.kube.getBucketCredentials(ctx, s.namespace, resourceName, s.rgwEndpoint)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil, status.Error(codes.NotFound, "bucket or credentials not found")
		}
		if errors.Is(err, errUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to get bucket credentials")
	}
	return &GetBucketCredentialsResponse{Credentials: creds}, nil
}

func (s *storageServer) GetOperation(ctx context.Context, req *GetOperationRequest) (*GetOperationResponse, error) {
	opID := strings.TrimSpace(req.OperationId)
	if opID == "" {
		return nil, status.Error(codes.InvalidArgument, "operationId is required")
	}
	op, err := s.q.GetOperation(ctx, opID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "operation not found")
		}
		return nil, status.Error(codes.Internal, "failed to get operation")
	}
	errStr := ""
	if op.Error.Valid {
		errStr = op.Error.String
	}
	return &GetOperationResponse{OperationId: op.ID, Status: op.Status, Error: errStr}, nil
}

func (s *storageServer) reconcileDeletions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileResourceType(ctx, "bucket", func(op dbsqlc.ListPendingOperationsByResourceTypeRow) bool {
				if !op.UserID.Valid || !op.ProjectID.Valid || !op.ResourceName.Valid {
					return false
				}
				records, err := s.kube.listOBCs(ctx, s.namespace, op.UserID.String, op.ProjectID.String)
				if err != nil {
					return false
				}
				for _, r := range records {
					if r.Name == op.ResourceName.String {
						return false
					}
				}
				return true
			}, nil)
		}
	}
}

func (s *storageServer) reconcileResourceType(ctx context.Context, resourceType string, isDone func(dbsqlc.ListPendingOperationsByResourceTypeRow) bool, onDone func(dbsqlc.ListPendingOperationsByResourceTypeRow) error) {
	ops, err := s.q.ListPendingOperationsByResourceType(ctx, sql.NullString{String: resourceType, Valid: true})
	if err != nil || len(ops) == 0 {
		return
	}
	for _, op := range ops {
		if isDone(op) {
			if onDone != nil {
				if err := onDone(op); err != nil {
					continue
				}
			}
			_ = s.q.UpdateOperation(ctx, dbsqlc.UpdateOperationParams{
				ID:        op.ID,
				Status:    "done",
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCLD_STORAGE_ADDR", ":8085")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}
	server, err := newStorageServer(env("DCLD_TARGET_NAMESPACE", "dcloud-system"))
	if err != nil {
		logger.Error("failed to open storage server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	grpcServer := grpc.NewServer()
	storagepb.RegisterObjectStorageServiceServer(grpcServer, server)
	errc := make(chan error, 1)
	go func() {
		logger.Info("storage grpc listening", "addr", addr)
		errc <- grpcServer.Serve(lis)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.reconcileDeletions(ctx)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigc:
		cancel()
		grpcServer.GracefulStop()
	case err := <-errc:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func isDNSLabel(value string) bool {
	if value == "" || len(value) > 63 {
		return false
	}
	if value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			continue
		default:
			return false
		}
	}
	return true
}

func bucketResourceName(userID, projectID, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID) + ":" + strings.TrimSpace(projectID) + ":" + strings.TrimSpace(name)))
	return "obc-" + hex.EncodeToString(sum[:8])
}

var (
	errInvalidArgument = errors.New("invalid argument")
	errNotFound        = errors.New("not found")
	errAlreadyExists   = errors.New("already exists")
	errUnavailable     = errors.New("unavailable")
)

type kubeClient struct {
	baseURL string
	client  *http.Client
	token   string
}

func newKubeClient() (*kubeClient, error) {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, err
	}
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to load kubernetes ca")
	}
	baseURL := strings.TrimRight(env("DCLD_KUBERNETES_API_URL", "https://kubernetes.default.svc"), "/")
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool},
	}
	return &kubeClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		token: strings.TrimSpace(string(token)),
	}, nil
}

type kubeOBCList struct {
	Items []kubeOBC `json:"items"`
}

type kubeOBC struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		CreationTimestamp string            `json:"creationTimestamp,omitempty"`
	} `json:"metadata"`
	Spec struct {
		StorageClassName   string `json:"storageClassName"`
		GenerateBucketName string `json:"generateBucketName,omitempty"`
		BucketName         string `json:"bucketName,omitempty"`
	} `json:"spec"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type kubeSecret struct {
	Data map[string]string `json:"data"`
}

type kubeConfigMap struct {
	Data map[string]string `json:"data"`
}

type kubeStatus struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code    int    `json:"code"`
}

func (c *kubeClient) listOBCs(ctx context.Context, namespace, userID, projectID string) ([]bucketRecord, error) {
	selector := url.QueryEscape(fmt.Sprintf("dcloud-component=storage,dcloud-user-id=%s,dcloud-project-id=%s", userID, projectID))
	var payload kubeOBCList
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/apis/objectbucket.io/v1alpha1/namespaces/%s/objectbucketclaims?labelSelector=%s", namespace, selector), nil, &payload); err != nil {
		return nil, err
	}
	records := make([]bucketRecord, 0, len(payload.Items))
	for _, item := range payload.Items {
		records = append(records, obcToRecord(item))
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt == records[j].CreatedAt {
			return records[i].Name < records[j].Name
		}
		return records[i].CreatedAt < records[j].CreatedAt
	})
	return records, nil
}

func (c *kubeClient) createOBC(ctx context.Context, namespace, userID, projectID, name, storageClass string) (bucketRecord, error) {
	resourceName := bucketResourceName(userID, projectID, name)
	payload := kubeOBC{
		APIVersion: "objectbucket.io/v1alpha1",
		Kind:       "ObjectBucketClaim",
	}
	payload.Metadata.Name = resourceName
	payload.Metadata.Namespace = namespace
	payload.Metadata.Labels = map[string]string{
		"dcloud-component":       "storage",
		"dcloud-user-id":         userID,
		"dcloud-project-id":      projectID,
		"dcloud-display-name":    name,
		"app.kubernetes.io/name": "dcloud",
	}
	payload.Metadata.Annotations = map[string]string{
		"dcloud/name": name,
	}
	payload.Spec.StorageClassName = storageClass
	payload.Spec.GenerateBucketName = resourceName
	var created kubeOBC
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/apis/objectbucket.io/v1alpha1/namespaces/%s/objectbucketclaims", namespace), payload, &created); err != nil {
		return bucketRecord{}, err
	}
	return obcToRecord(created), nil
}

func (c *kubeClient) deleteOBC(ctx context.Context, namespace, resourceName string) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/apis/objectbucket.io/v1alpha1/namespaces/%s/objectbucketclaims/%s", namespace, resourceName), nil, nil)
}

func (c *kubeClient) getBucketCredentials(ctx context.Context, namespace, resourceName, rgwEndpoint string) (*BucketCredentials, error) {
	var secret kubeSecret
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, resourceName), nil, &secret); err != nil {
		return nil, err
	}
	var cm kubeConfigMap
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, resourceName), nil, &cm); err != nil {
		return nil, err
	}
	accessKeyID := decodeBase64(secret.Data["AWS_ACCESS_KEY_ID"])
	secretAccessKey := decodeBase64(secret.Data["AWS_SECRET_ACCESS_KEY"])
	bucketName := cm.Data["BUCKET_NAME"]
	if rgwEndpoint == "" {
		host := cm.Data["BUCKET_HOST"]
		port := cm.Data["BUCKET_PORT"]
		if port != "" && port != "80" && port != "443" {
			rgwEndpoint = "http://" + host + ":" + port
		} else {
			rgwEndpoint = "http://" + host
		}
	}
	return &BucketCredentials{
		Endpoint:        rgwEndpoint,
		BucketName:      bucketName,
		AccessKeyId:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}, nil
}

func decodeBase64(s string) string {
	if s == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return strings.TrimSpace(string(b))
}

func (c *kubeClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := kubeErrorMessage(raw)
		switch res.StatusCode {
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			return fmt.Errorf("%w: %s", errInvalidArgument, message)
		case http.StatusConflict:
			return fmt.Errorf("%w: %s", errAlreadyExists, message)
		case http.StatusNotFound:
			if isUnavailableMessage(message) {
				return fmt.Errorf("%w: %s", errUnavailable, message)
			}
			return fmt.Errorf("%w: %s", errNotFound, message)
		default:
			return fmt.Errorf("%s", message)
		}
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func kubeErrorMessage(raw []byte) string {
	var payload kubeStatus
	if err := json.Unmarshal(raw, &payload); err == nil {
		if payload.Message != "" {
			return payload.Message
		}
		if payload.Reason != "" {
			return payload.Reason
		}
	}
	text := strings.TrimSpace(string(raw))
	if text != "" {
		return text
	}
	return "kubernetes api error"
}

func isUnavailableMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "could not find the requested resource") ||
		strings.Contains(message, "no matches for kind")
}

func obcToRecord(item kubeOBC) bucketRecord {
	name := ""
	if item.Metadata.Annotations != nil {
		name = strings.TrimSpace(item.Metadata.Annotations["dcloud/name"])
	}
	if name == "" {
		name = item.Metadata.Name
	}
	phase := strings.TrimSpace(item.Status.Phase)
	ready := strings.EqualFold(phase, "Bound")
	if phase == "" {
		phase = "Provisioning"
	}
	return bucketRecord{
		Name:         name,
		Ready:        ready,
		Status:       phase,
		CreatedAt:    item.Metadata.CreationTimestamp,
		ProjectID:    item.Metadata.Labels["dcloud-project-id"],
		ResourceName: item.Metadata.Name,
	}
}
