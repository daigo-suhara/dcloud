package main

import (
	"context"
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
	"strings"
	"syscall"
	"time"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

type platformResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Components  []string `json:"components"`
}

type projectScope struct {
	UserID    string
	ProjectID string
}

type project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	CreatedAt string `json:"createdAt"`
}

type projectRequest struct {
	Name string `json:"name"`
}

type deployRequest struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Port     int    `json:"port"`
	MinScale int    `json:"minScale"`
	MaxScale int    `json:"maxScale"`
}

type deployedService struct {
	Name       string `json:"name"`
	Image      string `json:"image"`
	URL        string `json:"url,omitempty"`
	Ready      bool   `json:"ready"`
	Reason     string `json:"reason,omitempty"`
	CreatedAt  string `json:"createdAt,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	Namespace  string `json:"namespace"`
	ProjectID  string `json:"projectId,omitempty"`
	Generation int64  `json:"generation,omitempty"`
}

type serviceManager interface {
	List(context.Context, projectScope) ([]deployedService, error)
	Deploy(context.Context, projectScope, deployRequest) (deployedService, error)
	Delete(context.Context, projectScope, string) error
	TargetURL(context.Context, projectScope, string) (string, error)
	PublicTargetURL(context.Context, string) (string, error)
}

type projectManager interface {
	List(context.Context, string) ([]project, error)
	Create(context.Context, string, string) (project, error)
	Delete(context.Context, string, string) error
	Ensure(context.Context, string, string) (project, error)
	Default(context.Context, string) (project, error)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCP_CORE_ADDR", ":8080")
	namespace := env("DCP_TARGET_NAMESPACE", "dcp-system")
	manager, err := newServiceManager(namespace)
	if err != nil {
		logger.Warn("service manager disabled", "error", err)
	}
	projects, err := newProjectManager(namespace)
	if err != nil {
		logger.Warn("project manager disabled", "error", err)
		projects = newMemoryProjectManager()
	}
	auth, err := newKeycloakAuthFromEnv()
	if err != nil {
		logger.Warn("keycloak auth disabled", "error", err)
	}

	mux := http.NewServeMux()
	api := &apiServer{
		logger:    logger,
		auth:      auth,
		services:  manager,
		projects:  projects,
		namespace: namespace,
	}
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz)
	mux.HandleFunc("GET /api/v1/platform", platform)
	mux.HandleFunc("GET /api/v1/auth/me", api.me)
	mux.HandleFunc("GET /api/v1/auth/login", api.login)
	mux.HandleFunc("GET /api/v1/auth/register", api.register)
	mux.HandleFunc("GET /api/v1/auth/callback", api.callback)
	mux.HandleFunc("GET /api/v1/auth/logout", api.logout)
	mux.HandleFunc("POST /api/v1/auth/logout", api.logout)
	mux.HandleFunc("GET /api/v1/projects", api.listProjects)
	mux.HandleFunc("POST /api/v1/projects", api.createProject)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}", api.deleteProject)
	mux.HandleFunc("/api/v1/projects/", api.projectsByPath)
	mux.HandleFunc("GET /api/v1/services", api.listServices)
	mux.HandleFunc("POST /api/v1/services", api.deployService)
	mux.HandleFunc("DELETE /api/v1/services/", api.deleteService)
	mux.HandleFunc("/cloudrun/", api.proxyService)
	mux.HandleFunc("/services/", api.proxyService)
	mux.HandleFunc("/", api.proxyService)

	server := &http.Server{
		Addr:              addr,
		Handler:           requestLog(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("core listening", "addr", addr)
		errc <- server.ListenAndServe()
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigc:
		logger.Info("shutdown requested", "signal", sig.String())
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

type apiServer struct {
	logger    *slog.Logger
	auth      *keycloakAuth
	services  serviceManager
	projects  projectManager
	namespace string
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Service:   "core",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ready",
		Service:   "core",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func platform(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, platformResponse{
		Name:        "dcp",
		Description: "A GCP-like cloud platform running on Kubernetes.",
		Components:  []string{"core", "console", "cloudrun"},
	})
}

func (a *apiServer) me(w http.ResponseWriter, r *http.Request) {
	user, err := a.currentUser(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ログインしてください"})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *apiServer) register(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "認証機能を利用できません"})
		return
	}
	if err := a.auth.Register(w, r); err != nil {
		a.logger.Error("register redirect failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Keycloak への遷移に失敗しました"})
		return
	}
}

func (a *apiServer) login(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "認証機能を利用できません"})
		return
	}
	if err := a.auth.Login(w, r); err != nil {
		a.logger.Error("login redirect failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "Keycloak への遷移に失敗しました"})
	}
}

func (a *apiServer) callback(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "認証機能を利用できません"})
		return
	}
	if err := a.auth.Callback(w, r); err != nil {
		if errors.Is(err, errStateMismatch) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ログイン情報が一致しません"})
			return
		}
		a.logger.Error("auth callback failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "ログイン処理に失敗しました"})
	}
}

func (a *apiServer) logout(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "認証機能を利用できません"})
		return
	}
	if err := a.auth.Logout(w, r); err != nil {
		a.logger.Error("logout redirect failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "ログアウト処理に失敗しました"})
	}
}

func (a *apiServer) listProjects(w http.ResponseWriter, r *http.Request) {
	userID, err := a.currentUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ログインしてください"})
		return
	}
	projects, err := a.projects.List(r.Context(), userID)
	if err != nil {
		a.logger.Error("list projects failed", "error", err, "user", userID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "プロジェクト一覧の取得に失敗しました"})
		return
	}
	defaultProject, err := a.projects.Default(r.Context(), userID)
	if err != nil {
		a.logger.Error("resolve default project failed", "error", err, "user", userID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "デフォルトプロジェクトの取得に失敗しました"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"user":             userID,
		"projects":         projects,
		"defaultProjectId": defaultProject.ID,
	})
}

func (a *apiServer) createProject(w http.ResponseWriter, r *http.Request) {
	userID, err := a.currentUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ログインしてください"})
		return
	}
	var req projectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "リクエスト本文のJSONが不正です"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "プロジェクト名は必須です"})
		return
	}
	project, err := a.projects.Create(r.Context(), userID, req.Name)
	if err != nil {
		a.logger.Error("create project failed", "error", err, "user", userID)
		if errors.Is(err, errProjectAlreadyExists) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "同じ名前のプロジェクトは作成できません"})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "プロジェクトの作成に失敗しました"})
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (a *apiServer) deleteProject(w http.ResponseWriter, r *http.Request) {
	userID, err := a.currentUserID(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "ログインしてください"})
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		projectID = strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/projects/"))
	}
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "プロジェクトIDが不正です"})
		return
	}
	if err := a.projects.Delete(r.Context(), userID, projectID); err != nil {
		a.logger.Error("delete project failed", "error", err, "user", userID, "project", projectID)
		switch {
		case errors.Is(err, errProjectNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "プロジェクトが見つかりません"})
		case errors.Is(err, errDefaultProjectProtected):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "デフォルトプロジェクトは削除できません"})
		default:
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "プロジェクトの削除に失敗しました"})
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *apiServer) projectsByPath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	a.deleteProject(w, r)
}

func (a *apiServer) listServices(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "サービス管理機能を利用できません",
		})
		return
	}
	scope, err := a.projectScopeFromRequest(r)
	if err != nil {
		writeJSON(w, statusForScopeError(err), map[string]string{"error": err.Error()})
		return
	}

	services, err := a.services.List(r.Context(), scope)
	if err != nil {
		a.logger.Error("list services failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "サービス一覧の取得に失敗しました",
		})
		return
	}
	a.setPublicURLs(r, services)

	writeJSON(w, http.StatusOK, map[string]any{
		"namespace": a.namespace,
		"user":      scope.UserID,
		"projectId": scope.ProjectID,
		"services":  services,
	})
}

func (a *apiServer) deployService(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "サービス管理機能を利用できません",
		})
		return
	}
	scope, err := a.projectScopeFromRequest(r)
	if err != nil {
		writeJSON(w, statusForScopeError(err), map[string]string{"error": err.Error()})
		return
	}

	var req deployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "リクエスト本文のJSONが不正です",
		})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Image = strings.TrimSpace(req.Image)
	if req.Port == 0 {
		req.Port = 8080
	}
	if req.MaxScale == 0 {
		req.MaxScale = 1
	}
	if err := validateDeployRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	service, err := a.services.Deploy(r.Context(), scope, req)
	if err != nil {
		a.logger.Error("deploy service failed", "error", err, "name", req.Name)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "サービスの作成に失敗しました",
		})
		return
	}
	a.setPublicURL(r, &service)

	writeJSON(w, http.StatusCreated, service)
}

func (a *apiServer) deleteService(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "サービス管理機能を利用できません",
		})
		return
	}
	scope, err := a.projectScopeFromRequest(r)
	if err != nil {
		writeJSON(w, statusForScopeError(err), map[string]string{"error": err.Error()})
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	name = strings.Trim(name, "/")
	if name == "" || !isDNSLabel(name) {
		http.NotFound(w, r)
		return
	}

	if err := a.services.Delete(r.Context(), scope, name); err != nil {
		if errors.Is(err, errServiceNotFound) {
			http.NotFound(w, r)
			return
		}
		a.logger.Error("delete service failed", "error", err, "name", name)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": "サービスの削除に失敗しました",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *apiServer) proxyService(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		http.Error(w, "サービス管理機能を利用できません", http.StatusServiceUnavailable)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/cloudrun/")
	if trimmed == r.URL.Path {
		trimmed = strings.TrimPrefix(r.URL.Path, "/services/")
	}
	if trimmed != "" && trimmed != r.URL.Path {
		parts := strings.SplitN(trimmed, "/", 3)
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		userID, err := a.currentUserID(r)
		if err != nil {
			http.Error(w, "ログインしてください", http.StatusUnauthorized)
			return
		}
		scope := projectScope{UserID: userID, ProjectID: parts[0]}
		if !isDNSLabel(scope.ProjectID) {
			http.NotFound(w, r)
			return
		}
		if _, err := a.projects.Ensure(r.Context(), scope.UserID, scope.ProjectID); err != nil {
			http.Error(w, "プロジェクトが見つかりません", http.StatusNotFound)
			return
		}
		name := parts[1]
		if !isDNSLabel(name) {
			http.NotFound(w, r)
			return
		}

		targetURL, err := a.services.TargetURL(r.Context(), scope, name)
		if err != nil {
			a.logger.Error("resolve service target failed", "error", err, "name", name)
			http.Error(w, "サービスが見つかりません", http.StatusNotFound)
			return
		}

		remainder := ""
		if len(parts) == 3 {
			remainder = "/" + parts[2]
		}
		a.proxyToTarget(w, r, targetURL, remainder, name)
		return
	}

	name := a.serviceNameFromHost(r)
	if name == "" {
		http.NotFound(w, r)
		return
	}

	targetURL, err := a.services.PublicTargetURL(r.Context(), name)
	if err != nil {
		a.logger.Error("resolve public service target failed", "error", err, "name", name)
		http.Error(w, "サービスが見つかりません", http.StatusNotFound)
		return
	}

	a.proxyToTarget(w, r, targetURL, r.URL.Path, name)
}

func (a *apiServer) setPublicURLs(r *http.Request, services []deployedService) {
	for i := range services {
		a.setPublicURL(r, &services[i])
	}
}

func (a *apiServer) setPublicURL(r *http.Request, service *deployedService) {
	service.URL = publicServiceURL(r, service.ProjectID, service.Name)
}

func (a *apiServer) proxyToTarget(w http.ResponseWriter, r *http.Request, targetURL string, path string, name string) {
	target, err := url.Parse(targetURL)
	if err != nil {
		a.logger.Error("invalid service target url", "error", err, "name", name, "target", targetURL)
		http.Error(w, "バックエンドURLが不正です", http.StatusBadGateway)
		return
	}

	proxyURL := *target
	proxyURL.Path = singleJoiningSlash(target.Path, path)
	proxyURL.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL.String(), r.Body)
	if err != nil {
		a.logger.Error("proxy request failed", "error", err, "name", name)
		http.Error(w, "接続先サービスを利用できません", http.StatusBadGateway)
		return
	}
	req.Header = cloneHeader(r.Header)
	req.Host = target.Host
	req.Header.Set("X-Forwarded-Host", r.Host)
	if isSecureRequest(r) {
		req.Header.Set("X-Forwarded-Proto", "https")
	} else {
		req.Header.Set("X-Forwarded-Proto", "http")
	}
	removeHopByHopHeaders(req.Header)

	res, err := proxyHTTPClient.Do(req)
	if err != nil {
		a.logger.Error("proxy service failed", "error", err, "name", name)
		http.Error(w, "接続先サービスを利用できません", http.StatusBadGateway)
		return
	}
	defer res.Body.Close()

	copyHeader(w.Header(), res.Header)
	removeHopByHopHeaders(w.Header())
	w.WriteHeader(res.StatusCode)
	if _, err := io.Copy(w, res.Body); err != nil {
		a.logger.Error("proxy response copy failed", "error", err, "name", name)
	}
}

func (a *apiServer) serviceNameFromHost(r *http.Request) string {
	domain := strings.TrimSpace(os.Getenv("DCP_PUBLIC_SERVICE_DOMAIN"))
	if domain == "" {
		return ""
	}

	host := requestHost(r)
	if host == "" {
		return ""
	}
	host = strings.ToLower(host)
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	if host == domain {
		return ""
	}
	if !strings.HasSuffix(host, "."+domain) {
		return ""
	}

	name := strings.TrimSuffix(host, "."+domain)
	if name == "" || strings.Contains(name, ".") || !isDNSLabel(name) {
		return ""
	}
	return name
}

func publicBaseURL(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s", proto, host)
}

func publicServiceURL(r *http.Request, projectID string, name string) string {
	domain := strings.TrimSpace(os.Getenv("DCP_PUBLIC_SERVICE_DOMAIN"))
	if domain != "" {
		domain = strings.TrimSuffix(domain, ".")
		return fmt.Sprintf("https://%s.%s/", name, domain)
	}
	if projectID == "" {
		return fmt.Sprintf("%s/services/%s/", publicBaseURL(r), name)
	}
	return fmt.Sprintf("%s/cloudrun/%s/%s/", publicBaseURL(r), projectID, name)
}

func requestHost(r *http.Request) string {
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}
	if strings.Contains(host, ":") {
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
	}
	return strings.TrimSpace(host)
}

var proxyHTTPClient = &http.Client{Timeout: 30 * time.Second}

func cloneHeader(src http.Header) http.Header {
	dst := make(http.Header, len(src))
	for key, values := range src {
		dst[key] = append([]string(nil), values...)
	}
	return dst
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func removeHopByHopHeaders(header http.Header) {
	for _, key := range []string{
		"Connection",
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	} {
		header.Del(key)
	}
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	default:
		return a + b
	}
}

func validateDeployRequest(req deployRequest) error {
	if req.Name == "" {
		return fmt.Errorf("サービス名は必須です")
	}
	if req.Image == "" {
		return fmt.Errorf("コンテナイメージのURLは必須です")
	}
	if !isDNSLabel(req.Name) {
		return fmt.Errorf("サービス名はDNSラベル形式で指定してください")
	}
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("Portは1から65535の範囲で指定してください")
	}
	if req.MinScale < 0 || req.MaxScale < 1 {
		return fmt.Errorf("最大スケール数は1以上で指定してください")
	}
	if req.MinScale > 0 && req.MaxScale > 0 && req.MaxScale < req.MinScale {
		return fmt.Errorf("最大スケール数は最小スケール数以上で指定してください")
	}
	return nil
}

func (a *apiServer) projectScopeFromRequest(r *http.Request) (projectScope, error) {
	userID, err := a.currentUserID(r)
	if err != nil {
		return projectScope{}, err
	}
	projectID := strings.TrimSpace(r.Header.Get("X-DCP-Project"))
	if projectID == "" {
		projectID = strings.TrimSpace(r.URL.Query().Get("project"))
	}
	var p project
	if projectID == "" {
		p, err = a.projects.Default(r.Context(), userID)
	} else {
		if !isDNSLabel(projectID) {
			return projectScope{}, fmt.Errorf("プロジェクトIDが不正です")
		}
		p, err = a.projects.Ensure(r.Context(), userID, projectID)
	}
	if err != nil {
		if errors.Is(err, errProjectNotFound) {
			return projectScope{}, fmt.Errorf("プロジェクトが見つかりません")
		}
		return projectScope{}, fmt.Errorf("プロジェクトの確認に失敗しました")
	}
	return projectScope{UserID: userID, ProjectID: p.ID}, nil
}

func (a *apiServer) currentUser(r *http.Request) (authUser, error) {
	if a.auth == nil {
		return authUser{}, fmt.Errorf("認証機能を利用できません")
	}
	return a.auth.CurrentUser(r)
}

func (a *apiServer) currentUserID(r *http.Request) (string, error) {
	user, err := a.currentUser(r)
	if err != nil {
		return "", err
	}
	return user.ID, nil
}

func statusForScopeError(err error) int {
	if errors.Is(err, errSessionNotFound) {
		return http.StatusUnauthorized
	}
	if strings.Contains(err.Error(), "見つかりません") {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func isDNSLabel(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	if value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' {
			continue
		}
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}
