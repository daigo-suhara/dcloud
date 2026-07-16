package handler

import (
	"net/http"
	"strings"

	"github.com/daigo-suhara/dcloud/internal/apihelp"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	"github.com/daigo-suhara/dcloud/internal/compute/service"
	computepb "github.com/daigo-suhara/dcloud/internal/pb/computepb"
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
	mux.HandleFunc("GET /api/v1/compute", h.list)
	mux.HandleFunc("GET /api/v1/compute/{name}", h.get)
	mux.HandleFunc("POST /api/v1/compute", h.create)
	mux.HandleFunc("DELETE /api/v1/compute/{name}", h.del)
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
	resp, err := h.svc.ListMachines(r.Context(), &computepb.ListMachinesRequest{UserId: claims.Subject, ProjectId: projectID})
	if err != nil {
		apihelp.WriteRPCError(w, err, "仮想マシン一覧を取得できません")
		return
	}
	out := make([]map[string]any, 0, len(resp.Machines))
	for _, m := range resp.Machines {
		out = append(out, machineDict(m))
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{
		"namespace": resp.Namespace, "user": claims.Subject, "projectId": projectID, "machines": out,
	})
}

func (h *REST) get(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	name := r.PathValue("name")
	rec, err := h.svc.LookupMachine(r.Context(), claims.Subject, projectID, name)
	if err != nil {
		apihelp.WriteRPCError(w, err, "仮想マシンが見つかりません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{
		"namespace": rec.Namespace, "user": claims.Subject, "projectId": projectID,
		"machine": map[string]any{
			"name": rec.Name, "image": rec.Image, "cpu": rec.CPU, "memory": rec.Memory,
			"ready": rec.Ready, "status": rec.Status, "reason": rec.Reason,
			"createdAt": rec.CreatedAt, "updatedAt": rec.UpdatedAt,
			"namespace": rec.Namespace, "projectId": rec.ProjectID, "generation": rec.Generation,
		},
	})
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
	resp, err := h.svc.CreateMachine(r.Context(), &computepb.CreateMachineRequest{
		UserId: claims.Subject, ProjectId: projectID,
		Name:   apihelp.StrVal(body["name"]),
		Image:  apihelp.StrVal(body["image"]),
		Cpu:    firstStr(apihelp.StrVal(body["cpu"]), "1"),
		Memory: firstStr(apihelp.StrVal(body["memory"]), "1Gi"),
	})
	if err != nil {
		apihelp.WriteRPCError(w, err, "仮想マシンを作成できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, machineDict(resp.Machine))
}

func (h *REST) del(w http.ResponseWriter, r *http.Request) {
	claims, projectID := h.auth(w, r)
	if claims == nil {
		return
	}
	name := r.PathValue("name")
	resp, err := h.svc.DeleteMachine(r.Context(), &computepb.DeleteMachineRequest{UserId: claims.Subject, ProjectId: projectID, Name: name})
	if err != nil {
		apihelp.WriteRPCError(w, err, "仮想マシンを削除できません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleting", "operationId": resp.OperationId})
}

func (h *REST) operation(w http.ResponseWriter, r *http.Request) {
	claims, _, err := h.verifier.VerifyCookieOrBearer(r, h.cookie)
	if err != nil {
		http.Error(w, "ログインが必要です", http.StatusUnauthorized)
		return
	}
	_ = claims
	resp, err := h.svc.GetOperation(r.Context(), &computepb.GetOperationRequest{OperationId: r.PathValue("id")})
	if err != nil {
		apihelp.WriteRPCError(w, err, "オペレーションが見つかりません")
		return
	}
	apihelp.WriteJSON(w, http.StatusOK, map[string]any{
		"operationId": resp.OperationId, "status": resp.Status, "error": resp.Error,
	})
}

func machineDict(m *computepb.Machine) map[string]any {
	return map[string]any{
		"name": m.GetName(), "image": m.GetImage(), "cpu": m.GetCpu(), "memory": m.GetMemory(),
		"ready": m.GetReady(), "status": m.GetStatus(), "reason": m.GetReason(),
		"createdAt": m.GetCreatedAt(), "updatedAt": m.GetUpdatedAt(),
		"namespace": m.GetNamespace(), "projectId": m.GetProjectId(), "generation": m.GetGeneration(),
	}
}

func firstStr(a, fallback string) string {
	if a == "" {
		return fallback
	}
	return a
}
