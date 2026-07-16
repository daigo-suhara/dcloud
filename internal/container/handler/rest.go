package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/daigo-suhara/dcloud/internal/apihelp"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	"github.com/daigo-suhara/dcloud/internal/container/service"
	containerpb "github.com/daigo-suhara/dcloud/internal/pb/containerpb"
)

// REST exposes the /api/v1/container[/*] routes the console currently
// reaches via the Python api. It mirrors the Python behaviour: JWT
// auth via cookie, X-DCP-Project header for project scope, JSON I/O.
type REST struct {
	svc      *service.Server
	verifier *jwtverify.Verifier
	cookie   string
}

func NewREST(svc *service.Server, verifier *jwtverify.Verifier) *REST {
	return &REST{svc: svc, verifier: verifier, cookie: apihelp.EnvOr("DCLD_SESSION_COOKIE_NAME", "dcloud_session")}
}

func (h *REST) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/container", h.list)
	mux.HandleFunc("POST /api/v1/container", h.deploy)
	mux.HandleFunc("PUT /api/v1/container/{name}", h.update)
	mux.HandleFunc("DELETE /api/v1/container/{name}", h.del)
	mux.HandleFunc("PUT /api/v1/container/{name}/domain", h.domain)
	mux.HandleFunc("GET /api/v1/container/{name}/logs", h.logs)
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
		projectID = strings.TrimSpace(r.URL.Query().Get("project"))
	}
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
	resp, err := h.svc.ListServices(r.Context(), &containerpb.ListServicesRequest{UserId: claims.Subject, ProjectId: projectID})
	if err != nil {
		apihelp.WriteRPCError(w, err, "サービス一覧を取得できません")
		return
	}
	out := make([]map[string]any, 0, len(resp.Containers))
	for _, s := range resp.Containers {
		out = append(out, serviceDict(s))
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{
		"namespace": resp.Namespace, "user": claims.Subject, "projectId": projectID, "containers": out,
	})
}

func (h *REST) deploy(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	body, err := apihelp.ReadJSONBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req := deployRequestFromBody(claims.Subject, projectID, apihelp.StrVal(body["name"]), body)
	resp, err := h.svc.DeployService(r.Context(), req)
	if err != nil {
		apihelp.WriteRPCError(w, err, "サービスを作成できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, serviceDict(resp.Service))
}

func (h *REST) update(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	name := r.PathValue("name")
	body, err := apihelp.ReadJSONBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req := deployRequestFromBody(claims.Subject, projectID, name, body)
	resp, err := h.svc.DeployService(r.Context(), req)
	if err != nil {
		apihelp.WriteRPCError(w, err, "サービスを更新できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, serviceDict(resp.Service))
}

func (h *REST) del(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	name := r.PathValue("name")
	resp, err := h.svc.DeleteService(r.Context(), &containerpb.DeleteServiceRequest{UserId: claims.Subject, ProjectId: projectID, Name: name})
	if err != nil {
		apihelp.WriteRPCError(w, err, "サービスを削除できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleting", "operationId": resp.OperationId})
}

func (h *REST) domain(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	name := r.PathValue("name")
	body, err := apihelp.ReadJSONBody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := h.svc.SetServiceDomain(r.Context(), &containerpb.SetServiceDomainRequest{
		UserId: claims.Subject, ProjectId: projectID, Name: name,
		CustomDomain: apihelp.StrVal(body["customDomain"]),
	})
	if err != nil {
		apihelp.WriteRPCError(w, err, "ドメインを設定できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, serviceDict(resp.Service))
}

func (h *REST) logs(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	name := r.PathValue("name")
	tail := int32(apihelp.IntVal(r.URL.Query().Get("tail"), 200))
	follow := r.URL.Query().Get("follow") == "1" || r.URL.Query().Get("follow") == "true"
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx := r.Context()
	err := h.svc.StreamServiceLogs(ctx, &containerpb.GetServiceLogsRequest{
		UserId: claims.Subject, ProjectId: projectID, Name: name, TailLines: tail, Follow: follow,
	}, func(line *containerpb.GetServiceLogsResponse) error {
		payload, _ := json.Marshal(map[string]any{"text": line.Text, "timestamp": line.Timestamp})
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if err != nil && ctx.Err() == nil {
		payload, _ := json.Marshal(map[string]any{"detail": err.Error()})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", payload)
		flusher.Flush()
	}
}

func (h *REST) operation(w http.ResponseWriter, r *http.Request) {
	// Only container-op-* IDs belong to us; the nginx layer routes
	// operations by the prefix at the top level. If somehow another id
	// arrives here, delegate to the service and let it 404.
	claims, _, err := h.verifier.VerifyCookieOrBearer(r, h.cookie)
	if err != nil {
		http.Error(w, "ログインが必要です", http.StatusUnauthorized)
		return
	}
	_ = claims
	id := r.PathValue("id")
	resp, err := h.svc.GetOperation(r.Context(), &containerpb.GetOperationRequest{OperationId: id})
	if err != nil {
		apihelp.WriteRPCError(w, err, "オペレーションが見つかりません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{
		"operationId": resp.OperationId, "status": resp.Status, "error": resp.Error,
	})
}

func serviceDict(s *containerpb.Service) map[string]any {
	env := make([]map[string]string, 0, len(s.GetEnv()))
	for _, e := range s.GetEnv() {
		env = append(env, map[string]string{"name": e.GetName(), "value": e.GetValue()})
	}
	var envOut any = env
	if len(env) == 0 {
		envOut = nil
	}
	return map[string]any{
		"name": s.GetName(), "image": s.GetImage(), "url": s.GetUrl(),
		"ready": s.GetReady(), "reason": s.GetReason(),
		"createdAt": s.GetCreatedAt(), "updatedAt": s.GetUpdatedAt(),
		"namespace": s.GetNamespace(), "projectId": s.GetProjectId(),
		"generation":         s.GetGeneration(),
		"customDomain":       apihelp.Nullable(s.GetCustomDomain()),
		"domainStatus":       apihelp.Nullable(s.GetDomainStatus()),
		"domainStatusReason": apihelp.Nullable(s.GetDomainStatusReason()),
		"domainCnameTarget":  apihelp.Nullable(s.GetDomainCnameTarget()),
		"port":               orInt32(s.GetPort(), 8080),
		"minScale":           s.GetMinScale(),
		"maxScale":           s.GetMaxScale(),
		"startupScript":      apihelp.Nullable(s.GetStartupScript()),
		"env":                envOut,
	}
}

func deployRequestFromBody(userID, projectID, name string, body map[string]any) *containerpb.DeployServiceRequest {
	env := []*containerpb.EnvVar{}
	if raw, ok := body["env"].([]any); ok {
		for _, item := range raw {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if n := apihelp.StrVal(m["name"]); n != "" {
				env = append(env, &containerpb.EnvVar{Name: n, Value: apihelp.StrVal(m["value"])})
			}
		}
	}
	return &containerpb.DeployServiceRequest{
		UserId: userID, ProjectId: projectID, Name: name,
		Image:         apihelp.StrVal(body["image"]),
		Port:          int32(apihelp.IntVal(body["port"], 8080)),
		MinScale:      int32(apihelp.IntVal(body["minScale"], 0)),
		MaxScale:      int32(apihelp.IntVal(body["maxScale"], 20)),
		StartupScript: apihelp.StrVal(body["startupScript"]),
		Env:           env,
	}
}

func orInt32(v, fallback int32) int32 {
	if v == 0 {
		return fallback
	}
	return v
}

var _ = context.Background // keep context import stable
