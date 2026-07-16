// Command container-server is the ContainerService binary. It wires the
// runtime dependencies from env, exposes gRPC on :8082 and ConnectRPC over
// h2c on :8092, and waits for SIGINT/SIGTERM for graceful shutdown.
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
	"github.com/daigo-suhara/dcloud/internal/container/handler"
	"github.com/daigo-suhara/dcloud/internal/container/service"
	containerpb "github.com/daigo-suhara/dcloud/internal/pb/containerpb"
	"github.com/daigo-suhara/dcloud/internal/pb/containerpb/containerpbconnect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	grpcAddr := env("DCP_VM_ADDR", ":8082")
	httpAddr := env("DCLD_CONTAINER_HTTP_ADDR", ":8092")
	jwksURL := env("DCLD_IDENTITY_JWKS_URL", "http://identity:8093/.well-known/jwks.json")
	namespace := env("DCLD_TARGET_NAMESPACE", "dcloud-system")

	svc, err := service.New(namespace)
	if err != nil {
		logger.Error("failed to initialize container server", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("failed to listen", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	containerpb.RegisterContainerServiceServer(grpcServer, handler.NewGRPC(svc))

	verifier := jwtverify.New(jwksURL)
	mux := http.NewServeMux()
	connectPath, connectHandler := containerpbconnect.NewContainerServiceHandler(
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
		logger.Info("container grpc listening", "addr", grpcAddr)
		errc <- grpcServer.Serve(lis)
	}()
	go func() {
		logger.Info("container http listening", "addr", httpAddr)
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
