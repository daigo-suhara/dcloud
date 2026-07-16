// Command project-server is the ProjectService binary. Listens on gRPC
// :8081 and, on h2c :8091, serves ConnectRPC and REST /api/v1/projects
// routes with JWT auth.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	projectpb "github.com/daigo-suhara/dcloud/internal/pb/projectpb"
	"github.com/daigo-suhara/dcloud/internal/pb/projectpb/projectpbconnect"
	"github.com/daigo-suhara/dcloud/internal/project/handler"
	"github.com/daigo-suhara/dcloud/internal/project/service"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	grpcAddr := env("DCP_PROJECT_ADDR", ":8081")
	httpAddr := env("DCP_PROJECT_HTTP_ADDR", ":8091")
	jwksURL := env("DCLD_IDENTITY_JWKS_URL", "http://identity:8093/.well-known/jwks.json")

	svc, err := service.New(NewResourceClients())
	if err != nil {
		logger.Error("failed to initialize project server", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("failed to listen", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	projectpb.RegisterProjectServiceServer(grpcServer, handler.NewGRPC(svc))

	verifier := jwtverify.New(jwksURL)
	mux := http.NewServeMux()
	connectPath, connectHandler := projectpbconnect.NewProjectServiceHandler(
		handler.NewConnect(svc),
		connect.WithInterceptors(verifier.ConnectInterceptor()),
	)
	mux.Handle(connectPath, connectHandler)
	handler.RegisterRESTRoutes(mux, svc, verifier)
	httpServer := &http.Server{Addr: httpAddr, Handler: h2c.NewHandler(mux, &http2.Server{})}

	errc := make(chan error, 2)
	go func() {
		logger.Info("project grpc listening", "addr", grpcAddr)
		errc <- grpcServer.Serve(lis)
	}()
	go func() {
		logger.Info("project http listening", "addr", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigc:
		grpcServer.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
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
