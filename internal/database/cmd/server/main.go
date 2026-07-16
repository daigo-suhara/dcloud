// Command database-server is the DatabaseService binary. Listens on
// gRPC :8086 and, on h2c :8096, serves ConnectRPC and REST /api/v1/database
// routes with JWT auth; a background reconciler tracks KubeBlocks
// Cluster deletions.
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
	"github.com/daigo-suhara/dcloud/internal/database/handler"
	"github.com/daigo-suhara/dcloud/internal/database/service"
	databasepb "github.com/daigo-suhara/dcloud/internal/pb/databasepb"
	"github.com/daigo-suhara/dcloud/internal/pb/databasepb/databasepbconnect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	grpcAddr := env("DCLD_DATABASE_ADDR", ":8086")
	httpAddr := env("DCLD_DATABASE_HTTP_ADDR", ":8096")
	jwksURL := env("DCLD_IDENTITY_JWKS_URL", "http://identity:8093/.well-known/jwks.json")
	namespace := env("DCLD_TARGET_NAMESPACE", "dcloud-system")

	svc, err := service.New(namespace)
	if err != nil {
		logger.Error("failed to initialize database server", "error", err)
		os.Exit(1)
	}
	defer svc.Close()

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("failed to listen", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer()
	databasepb.RegisterDatabaseServiceServer(grpcServer, handler.NewGRPC(svc))

	verifier := jwtverify.New(jwksURL)
	mux := http.NewServeMux()
	connectPath, connectHandler := databasepbconnect.NewDatabaseServiceHandler(
		handler.NewConnect(svc),
		connect.WithInterceptors(verifier.ConnectInterceptor()),
	)
	mux.Handle(connectPath, connectHandler)
	handler.RegisterRESTRoutes(mux, svc, verifier)
	httpServer := &http.Server{Addr: httpAddr, Handler: h2c.NewHandler(mux, &http2.Server{})}

	errc := make(chan error, 2)
	go func() {
		logger.Info("database grpc listening", "addr", grpcAddr)
		errc <- grpcServer.Serve(lis)
	}()
	go func() {
		logger.Info("database http listening", "addr", httpAddr)
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
