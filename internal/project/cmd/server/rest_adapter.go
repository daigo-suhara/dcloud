package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	projectpb "github.com/daigo-suhara/dcloud/internal/pb/projectpb"
	"google.golang.org/grpc/status"
)

// registerRESTRoutes wires the /api/v1/projects/* routes the console
// currently reaches via api. Auth is via the dcloud_session JWT cookie.
func registerRESTRoutes(mux *http.ServeMux, server *projectServer, verifier *jwtverify.Verifier) {
	cookie := envRest("DCLD_SESSION_COOKIE_NAME", "dcloud_session")

	auth := func(w http.ResponseWriter, r *http.Request) *jwtverify.Claims {
		claims, _, err := verifier.VerifyCookieOrBearer(r, cookie)
		if err != nil {
			http.Error(w, "ログインが必要です", http.StatusUnauthorized)
			return nil
		}
		return claims
	}

	mux.HandleFunc("GET /api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		claims := auth(w, r)
		if claims == nil {
			return
		}
		resp, err := server.ListProjects(r.Context(), &projectpb.ListProjectsRequest{UserId: claims.Subject})
		if err != nil {
			writeRESTError(w, err, "プロジェクト一覧を取得できません")
			return
		}
		items := make([]map[string]any, 0, len(resp.Projects))
		for _, p := range resp.Projects {
			items = append(items, projectDict(p))
		}
		writeRESTJSON(w, http.StatusOK, map[string]any{"user": claims.Subject, "projects": items})
	})

	mux.HandleFunc("POST /api/v1/projects", func(w http.ResponseWriter, r *http.Request) {
		claims := auth(w, r)
		if claims == nil {
			return
		}
		body, err := readRESTBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := strvRest(body["name"])
		if name == "" {
			http.Error(w, "プロジェクト名は必須です", http.StatusBadRequest)
			return
		}
		resp, err := server.CreateProject(r.Context(), &projectpb.CreateProjectRequest{UserId: claims.Subject, Name: name})
		if err != nil {
			writeRESTError(w, err, "プロジェクトを作成できません")
			return
		}
		writeRESTJSON(w, http.StatusOK, projectDict(resp.Project))
	})

	mux.HandleFunc("DELETE /api/v1/projects/{projectId}", func(w http.ResponseWriter, r *http.Request) {
		claims := auth(w, r)
		if claims == nil {
			return
		}
		projectID := r.PathValue("projectId")
		resp, err := server.CreateProjectDeleteOperation(r.Context(), &projectpb.CreateProjectDeleteOperationRequest{UserId: claims.Subject, ProjectId: projectID})
		if err != nil {
			writeRESTError(w, err, "プロジェクトを削除できません")
			return
		}
		writeRESTJSON(w, http.StatusOK, map[string]string{"status": "deleting", "operationId": resp.OperationId})
	})

	mux.HandleFunc("GET /api/v1/projects/{projectId}/repository", func(w http.ResponseWriter, r *http.Request) {
		claims := auth(w, r)
		if claims == nil {
			return
		}
		projectID := r.PathValue("projectId")
		resp, err := server.GetProjectRepository(r.Context(), &projectpb.GetProjectRepositoryRequest{UserId: claims.Subject, ProjectId: projectID})
		if err != nil {
			writeRESTError(w, err, "リポジトリ設定が見つかりません")
			return
		}
		writeRESTJSON(w, http.StatusOK, repoDict(resp.Repository))
	})

	mux.HandleFunc("PUT /api/v1/projects/{projectId}/repository", func(w http.ResponseWriter, r *http.Request) {
		claims := auth(w, r)
		if claims == nil {
			return
		}
		projectID := r.PathValue("projectId")
		body, err := readRESTBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		branch := strvRest(body["repositoryBranch"])
		if branch == "" {
			branch = "main"
		}
		resp, err := server.UpsertProjectRepository(r.Context(), &projectpb.UpsertProjectRepositoryRequest{
			UserId:           claims.Subject,
			ProjectId:        projectID,
			RepositoryOwner:  strvRest(body["repositoryOwner"]),
			RepositoryName:   strvRest(body["repositoryName"]),
			RepositoryBranch: branch,
		})
		if err != nil {
			writeRESTError(w, err, "リポジトリを保存できません")
			return
		}
		writeRESTJSON(w, http.StatusOK, repoDict(resp.Repository))
	})
}

func projectDict(p *projectpb.Project) map[string]any {
	return map[string]any{
		"id":        p.GetId(),
		"name":      p.GetName(),
		"owner":     p.GetOwner(),
		"createdAt": p.GetCreatedAt(),
		"deleting":  p.GetDeleting(),
	}
}

func repoDict(r *projectpb.ProjectRepository) map[string]any {
	if r == nil {
		return nil
	}
	return map[string]any{
		"projectId":        r.GetProjectId(),
		"userId":           r.GetUserId(),
		"repositoryOwner":  r.GetRepositoryOwner(),
		"repositoryName":   r.GetRepositoryName(),
		"repositoryBranch": r.GetRepositoryBranch(),
		"connectedAt":      r.GetConnectedAt(),
		"updatedAt":        r.GetUpdatedAt(),
	}
}

func writeRESTJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeRESTError(w http.ResponseWriter, err error, fallback string) {
	if st, ok := status.FromError(err); ok {
		msg := st.Message()
		if msg == "" {
			msg = fallback
		}
		http.Error(w, msg, restGRPCCode(st.Code().String()))
		return
	}
	http.Error(w, fallback, http.StatusInternalServerError)
}

func restGRPCCode(code string) int {
	switch code {
	case "InvalidArgument":
		return http.StatusBadRequest
	case "NotFound":
		return http.StatusNotFound
	case "AlreadyExists":
		return http.StatusConflict
	case "Unauthenticated":
		return http.StatusUnauthorized
	case "PermissionDenied":
		return http.StatusForbidden
	case "FailedPrecondition":
		return http.StatusPreconditionFailed
	case "Unavailable":
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func readRESTBody(r *http.Request) (map[string]any, error) {
	if r.Body == nil {
		return map[string]any{}, nil
	}
	defer r.Body.Close()
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) || err.Error() == "EOF" {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return body, nil
}

func strvRest(v any) string {
	s, _ := v.(string)
	return s
}

func envRest(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
