package handler

import (
	"net/http"
	"strings"

	"github.com/daigo-suhara/dcloud/internal/apihelp"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
	"github.com/daigo-suhara/dcloud/internal/storage/service"
)

type REST struct {
	svc      *service.Server
	verifier *jwtverify.Verifier
	cookie   string
}

func NewREST(svc *service.Server, verifier *jwtverify.Verifier) *REST {
	return &REST{svc: svc, verifier: verifier, cookie: apihelp.EnvOr("DCLD_SESSION_COOKIE_NAME", "dcloud_session")}
}

func (h *REST) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/storage", h.list)
	mux.HandleFunc("POST /api/v1/storage", h.create)
	mux.HandleFunc("DELETE /api/v1/storage/{name}", h.del)
	mux.HandleFunc("GET /api/v1/storage/{name}/credentials", h.creds)
	mux.HandleFunc("GET /api/v1/operations/{id}", h.operation)
}

func (h *REST) auth(w http.ResponseWriter, r *http.Request) (*jwtverify.Claims, string) {
	claims, _, err := h.verifier.VerifyCookieOrBearer(r, h.cookie)
	if err != nil {
		http.Error(w, "ログインが必要です", http.StatusUnauthorized)
		return nil, ""
	}
	projectID := strings.TrimSpace(r.Header.Get("X-DCP-Project"))
	if projectID == "" {
		http.Error(w, "プロジェクトを選択してください", http.StatusBadRequest)
		return nil, ""
	}
	return claims, projectID
}

func (h *REST) list(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	resp, err := h.svc.ListBuckets(r.Context(), &storagepb.ListBucketsRequest{UserId: claims.Subject, ProjectId: projectID})
	if err != nil {
		apihelp.WriteRPCError(w, err, "バケット一覧を取得できません")
		return
	}
	out := make([]map[string]any, 0, len(resp.Buckets))
	for _, b := range resp.Buckets {
		out = append(out, bucketDict(b))
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{"user": claims.Subject, "projectId": projectID, "buckets": out})
}

func (h *REST) create(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	body, err := apihelp.ReadJSONBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	name := apihelp.StrVal(body["name"])
	if name == "" {
		http.Error(w, "バケット名は必須です", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.CreateBucket(r.Context(), &storagepb.CreateBucketRequest{UserId: claims.Subject, ProjectId: projectID, Name: name})
	if err != nil {
		apihelp.WriteRPCError(w, err, "バケットを作成できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, bucketDict(resp.Bucket))
}

func (h *REST) del(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	resp, err := h.svc.DeleteBucket(r.Context(), &storagepb.DeleteBucketRequest{UserId: claims.Subject, ProjectId: projectID, Name: r.PathValue("name")})
	if err != nil {
		apihelp.WriteRPCError(w, err, "バケットを削除できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleting", "operationId": resp.OperationId})
}

func (h *REST) creds(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	resp, err := h.svc.GetBucketCredentials(r.Context(), &storagepb.GetBucketCredentialsRequest{UserId: claims.Subject, ProjectId: projectID, Name: r.PathValue("name")})
	if err != nil {
		apihelp.WriteRPCError(w, err, "認証情報を取得できません")
		return
	}
	c := resp.GetCredentials()
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{
		"endpoint": c.GetEndpoint(), "bucketName": c.GetBucketName(),
		"accessKeyId": c.GetAccessKeyId(), "secretAccessKey": c.GetSecretAccessKey(),
	})
}

func (h *REST) operation(w http.ResponseWriter, r *http.Request) {
	if _, _, err := h.verifier.VerifyCookieOrBearer(r, h.cookie); err != nil {
		http.Error(w, "ログインが必要です", http.StatusUnauthorized)
		return
	}
	resp, err := h.svc.GetOperation(r.Context(), &storagepb.GetOperationRequest{OperationId: r.PathValue("id")})
	if err != nil {
		apihelp.WriteRPCError(w, err, "オペレーションが見つかりません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{
		"operationId": resp.OperationId, "status": resp.Status, "error": resp.Error,
	})
}

func bucketDict(b *storagepb.Bucket) map[string]any {
	return map[string]any{
		"name": b.GetName(), "endpoint": b.GetEndpoint(),
		"ready": b.GetReady(), "status": b.GetStatus(),
		"createdAt": b.GetCreatedAt(), "projectId": b.GetProjectId(),
	}
}
