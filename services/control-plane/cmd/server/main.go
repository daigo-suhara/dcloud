package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
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

type deployRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Port  int    `json:"port"`
	Scale int    `json:"scale"`
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
	Generation int64  `json:"generation,omitempty"`
}

type serviceManager interface {
	List(context.Context) ([]deployedService, error)
	Deploy(context.Context, deployRequest) (deployedService, error)
	Delete(context.Context, string) error
	TargetURL(context.Context, string) (string, error)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCP_CONTROL_PLANE_ADDR", ":8080")
	namespace := env("DCP_TARGET_NAMESPACE", "dcp-system")
	manager, err := newServiceManager(namespace)
	if err != nil {
		logger.Warn("service manager disabled", "error", err)
	}

	mux := http.NewServeMux()
	api := &apiServer{
		logger:    logger,
		services:  manager,
		namespace: namespace,
	}
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz)
	mux.HandleFunc("GET /api/v1/platform", platform)
	mux.HandleFunc("GET /api/v1/services", api.listServices)
	mux.HandleFunc("POST /api/v1/services", api.deployService)
	mux.HandleFunc("DELETE /api/v1/services/", api.deleteService)
	mux.HandleFunc("/cloudrun/", api.proxyService)
	mux.HandleFunc("/services/", api.proxyService)

	server := &http.Server{
		Addr:              addr,
		Handler:           requestLog(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("control-plane listening", "addr", addr)
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
	services  serviceManager
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
		Service:   "control-plane",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ready",
		Service:   "control-plane",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func platform(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, platformResponse{
		Name:        "dcp",
		Description: "A GCP-like cloud platform running on Kubernetes.",
		Components:  []string{"control-plane", "console", "cloudrun"},
	})
}

func (a *apiServer) listServices(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "service manager is unavailable",
		})
		return
	}

	services, err := a.services.List(r.Context())
	if err != nil {
		a.logger.Error("list services failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
		return
	}
	a.setPublicURLs(r, services)

	writeJSON(w, http.StatusOK, map[string]any{
		"namespace": a.namespace,
		"services":  services,
	})
}

func (a *apiServer) deployService(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "service manager is unavailable",
		})
		return
	}

	var req deployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON body",
		})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Image = strings.TrimSpace(req.Image)
	if req.Port == 0 {
		req.Port = 8080
	}
	if err := validateDeployRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	service, err := a.services.Deploy(r.Context(), req)
	if err != nil {
		a.logger.Error("deploy service failed", "error", err, "name", req.Name)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
		return
	}
	a.setPublicURL(r, &service)

	writeJSON(w, http.StatusCreated, service)
}

func (a *apiServer) deleteService(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "service manager is unavailable",
		})
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/v1/services/")
	name = strings.Trim(name, "/")
	if name == "" || !isDNSLabel(name) {
		http.NotFound(w, r)
		return
	}

	if err := a.services.Delete(r.Context(), name); err != nil {
		a.logger.Error("delete service failed", "error", err, "name", name)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *apiServer) proxyService(w http.ResponseWriter, r *http.Request) {
	if a.services == nil {
		http.Error(w, "service manager is unavailable", http.StatusServiceUnavailable)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/cloudrun/")
	if trimmed == r.URL.Path {
		trimmed = strings.TrimPrefix(r.URL.Path, "/services/")
	}
	if trimmed == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.SplitN(trimmed, "/", 2)
	name := parts[0]
	if !isDNSLabel(name) {
		http.NotFound(w, r)
		return
	}

	targetURL, err := a.services.TargetURL(r.Context(), name)
	if err != nil {
		a.logger.Error("resolve service target failed", "error", err, "name", name)
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		a.logger.Error("invalid service target url", "error", err, "name", name, "target", targetURL)
		http.Error(w, "invalid backend url", http.StatusBadGateway)
		return
	}

	remainder := ""
	if len(parts) == 2 {
		remainder = "/" + parts[1]
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, remainder)
		req.URL.RawQuery = r.URL.RawQuery
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", "http")
	}
	proxy.ErrorHandler = func(_ http.ResponseWriter, _ *http.Request, proxyErr error) {
		a.logger.Error("proxy service failed", "error", proxyErr, "name", name)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

func (a *apiServer) setPublicURLs(r *http.Request, services []deployedService) {
	for i := range services {
		a.setPublicURL(r, &services[i])
	}
}

func (a *apiServer) setPublicURL(r *http.Request, service *deployedService) {
	service.URL = fmt.Sprintf("%s/cloudrun/%s/", publicBaseURL(r), service.Name)
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
		return fmt.Errorf("name is required")
	}
	if req.Image == "" {
		return fmt.Errorf("image is required")
	}
	if !isDNSLabel(req.Name) {
		return fmt.Errorf("name must be a DNS label")
	}
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
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
