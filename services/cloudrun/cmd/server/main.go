package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

type cloudRunService struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	Region    string `json:"region"`
	Revision  string `json:"revision"`
	Ready     bool   `json:"ready"`
	UpdatedAt string `json:"updatedAt"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCP_CLOUDRUN_ADDR", ":8080")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz)
	mux.HandleFunc("GET /api/v1/cloudrun/services", listServices)

	server := &http.Server{
		Addr:              addr,
		Handler:           requestLog(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		logger.Info("cloudrun listening", "addr", addr)
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
		Service:   "cloudrun",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ready",
		Service:   "cloudrun",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func listServices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]cloudRunService{
		"services": {
			{
				Name:      "hello-dcp",
				Image:     "ghcr.io/daigo-suhara/hello-dcp:latest",
				Region:    "local",
				Revision:  "hello-dcp-00001",
				Ready:     true,
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	})
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
