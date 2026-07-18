package handler

import (
	"net/http"
	"strings"

	"github.com/daigo-suhara/dcloud/internal/apihelp"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	"github.com/daigo-suhara/dcloud/internal/database/service"
	databasepb "github.com/daigo-suhara/dcloud/internal/pb/databasepb"
)

func RegisterRESTRoutes(mux *http.ServeMux, server *service.Server, verifier *jwtverify.Verifier) {
	cookie := apihelp.EnvOr("DCLD_SESSION_COOKIE_NAME", "dcloud_session")

	auth := func(w http.ResponseWriter, r *http.Request) (*jwtverify.Claims, string) {
		claims, _, err := verifier.VerifyCookieOrBearer(r, cookie)
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

	mux.HandleFunc("GET /api/v1/database", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		resp, err := server.ListDatabases(r.Context(), &databasepb.ListDatabasesRequest{UserId: claims.Subject, ProjectId: projectID})
		if err != nil {
			apihelp.WriteRPCError(w, err, "データベース一覧を取得できません")
			return
		}
		out := make([]map[string]any, 0, len(resp.Databases))
		for _, d := range resp.Databases {
			out = append(out, databaseDict(d))
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]any{"user": claims.Subject, "projectId": projectID, "databases": out})
	})

	mux.HandleFunc("POST /api/v1/database", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		body, err := apihelp.ReadJSONBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := server.CreateDatabase(r.Context(), &databasepb.CreateDatabaseRequest{
			UserId: claims.Subject, ProjectId: projectID,
			Name:    apihelp.StrVal(body["name"]),
			Type:    apihelp.StrVal(body["type"]),
			Version: apihelp.StrVal(body["version"]),
			Cpu:     apihelp.StrVal(body["cpu"]),
			Memory:  apihelp.StrVal(body["memory"]),
			Storage: apihelp.StrVal(body["storage"]),
		})
		if err != nil {
			apihelp.WriteRPCError(w, err, "データベースを作成できません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, databaseDict(resp.Database))
	})

	mux.HandleFunc("DELETE /api/v1/database/{name}", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		resp, err := server.DeleteDatabase(r.Context(), &databasepb.DeleteDatabaseRequest{UserId: claims.Subject, ProjectId: projectID, Name: r.PathValue("name")})
		if err != nil {
			apihelp.WriteRPCError(w, err, "データベースを削除できません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleting", "operationId": resp.OperationId})
	})

	mux.HandleFunc("GET /api/v1/database/{name}/connection", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		resp, err := server.GetConnectionString(r.Context(), &databasepb.GetConnectionStringRequest{
			UserId: claims.Subject, ProjectId: projectID,
			Name: r.PathValue("name"), SchemaName: r.URL.Query().Get("schema"),
		})
		if err != nil {
			apihelp.WriteRPCError(w, err, "接続情報を取得できません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]any{
			"connectionString": resp.ConnectionString, "host": resp.Host, "port": resp.Port,
			"username": resp.Username, "password": resp.Password, "databaseName": resp.DatabaseName,
		})
	})

	mux.HandleFunc("GET /api/v1/database/{name}/schemas", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		resp, err := server.ListSchemas(r.Context(), &databasepb.ListSchemasRequest{UserId: claims.Subject, ProjectId: projectID, Name: r.PathValue("name")})
		if err != nil {
			apihelp.WriteRPCError(w, err, "スキーマ一覧を取得できません")
			return
		}
		schemas := make([]map[string]any, 0, len(resp.Schemas))
		for _, s := range resp.Schemas {
			schemas = append(schemas, map[string]any{"name": s.Name, "charset": s.Charset})
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]any{"schemas": schemas})
	})

	mux.HandleFunc("POST /api/v1/database/{name}/schemas", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		body, err := apihelp.ReadJSONBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := server.CreateSchema(r.Context(), &databasepb.CreateSchemaRequest{
			UserId: claims.Subject, ProjectId: projectID, Name: r.PathValue("name"),
			SchemaName: apihelp.StrVal(body["schemaName"]), Charset: apihelp.StrVal(body["charset"]),
		})
		if err != nil {
			apihelp.WriteRPCError(w, err, "スキーマを作成できません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]any{"name": resp.Schema.Name, "charset": resp.Schema.Charset})
	})

	mux.HandleFunc("DELETE /api/v1/database/{name}/schemas/{schemaName}", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		_, err := server.DeleteSchema(r.Context(), &databasepb.DeleteSchemaRequest{
			UserId: claims.Subject, ProjectId: projectID,
			Name: r.PathValue("name"), SchemaName: r.PathValue("schemaName"),
		})
		if err != nil {
			apihelp.WriteRPCError(w, err, "スキーマを削除できません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("GET /api/v1/database/{name}/backups", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		resp, err := server.ListBackups(r.Context(), &databasepb.ListBackupsRequest{
			UserId: claims.Subject, ProjectId: projectID, Name: r.PathValue("name"),
		})
		if err != nil {
			apihelp.WriteRPCError(w, err, "バックアップを取得できません")
			return
		}
		backups := make([]map[string]any, 0, len(resp.Backups))
		for _, b := range resp.Backups {
			backups = append(backups, map[string]any{
				"name": b.Name, "status": b.Status, "method": b.Method,
				"totalSize": b.TotalSize, "createdAt": b.CreatedAt, "completedAt": b.CompletedAt,
			})
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]any{"backups": backups})
	})

	mux.HandleFunc("POST /api/v1/database/{name}/backups", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		resp, err := server.CreateBackup(r.Context(), &databasepb.CreateBackupRequest{
			UserId: claims.Subject, ProjectId: projectID, Name: r.PathValue("name"),
		})
		if err != nil {
			apihelp.WriteRPCError(w, err, "バックアップを作成できません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]any{
			"name": resp.Backup.Name, "status": resp.Backup.Status,
		})
	})

	mux.HandleFunc("DELETE /api/v1/database/{name}/backups/{backupName}", func(w http.ResponseWriter, r *http.Request) {
		claims, projectID := auth(w, r)
		if claims == nil {
			return
		}
		_, err := server.DeleteBackup(r.Context(), &databasepb.DeleteBackupRequest{
			UserId: claims.Subject, ProjectId: projectID,
			Name: r.PathValue("name"), BackupName: r.PathValue("backupName"),
		})
		if err != nil {
			apihelp.WriteRPCError(w, err, "バックアップを削除できません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	})

	mux.HandleFunc("GET /api/v1/operations/{id}", func(w http.ResponseWriter, r *http.Request) {
		if _, _, err := verifier.VerifyCookieOrBearer(r, cookie); err != nil {
			http.Error(w, "ログインが必要です", http.StatusUnauthorized)
			return
		}
		resp, err := server.GetOperation(r.Context(), &databasepb.GetOperationRequest{OperationId: r.PathValue("id")})
		if err != nil {
			apihelp.WriteRPCError(w, err, "オペレーションが見つかりません")
			return
		}
		apihelp.WriteJSON(w, http.StatusOK, map[string]any{
			"operationId": resp.OperationId, "status": resp.Status, "error": resp.Error,
		})
	})
}

func databaseDict(d *databasepb.Database) map[string]any {
	return map[string]any{
		"name": d.GetName(), "type": d.GetType(), "version": d.GetVersion(),
		"cpu": d.GetCpu(), "memory": d.GetMemory(), "storage": d.GetStorage(),
		"ready": d.GetReady(), "status": d.GetStatus(),
		"createdAt": d.GetCreatedAt(), "projectId": d.GetProjectId(),
	}
}
