// Command compute-server is the ComputeService binary. It listens on
// gRPC :8084, exposes ConnectRPC + the KubeVirt console WebSocket over
// h2c on :8094, and runs a background reconciler for pending VM
// deletions.
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
	"github.com/daigo-suhara/dcloud/internal/compute/handler"
	"github.com/daigo-suhara/dcloud/internal/compute/service"
	computepb "github.com/daigo-suhara/dcloud/internal/pb/computepb"
	"github.com/daigo-suhara/dcloud/internal/pb/computepb/computepbconnect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	grpcAddr := env("DCP_COMPUTE_ADDR", ":8084")
	httpAddr := env("DCLD_COMPUTE_HTTP_ADDR", ":8094")
	jwksURL := env("DCLD_IDENTITY_JWKS_URL", "http://identity:8093/.well-known/jwks.json")
	namespace := env("DCLD_TARGET_NAMESPACE", "dcloud-system")

	svc, err := service.New(namespace)
	if err != nil {
		logger.Error("failed to initialize compute server", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("failed to listen", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	computepb.RegisterComputeServiceServer(grpcServer, handler.NewGRPC(svc))

	verifier := jwtverify.New(jwksURL)
	consoleHandler, err := handler.NewConsoleWSHandler(svc, verifier)
	if err != nil {
		logger.Error("failed to build console handler", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	// KubeVirt VM console WebSocket proxy. Uses path-value routing (Go 1.22+).
	mux.Handle("/api/v1/compute/{name}/console", consoleHandler)
	connectPath, connectHandler := computepbconnect.NewComputeServiceHandler(
		handler.NewConnect(svc),
		connect.WithInterceptors(verifier.ConnectInterceptor()),
	)
	mux.Handle(connectPath, connectHandler)
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	errc := make(chan error, 2)
	go func() {
		logger.Info("compute grpc listening", "addr", grpcAddr)
		errc <- grpcServer.Serve(lis)
	}()
	go func() {
		logger.Info("compute http listening", "addr", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.ReconcileDeletions(ctx)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigc:
		cancel()
		grpcServer.GracefulStop()
		shutdownCtx, cshut := context.WithTimeout(context.Background(), 5*time.Second)
		defer cshut()
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
