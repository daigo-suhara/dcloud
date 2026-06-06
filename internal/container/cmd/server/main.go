package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	containerpb "github.com/daigo-suhara/dcloud/internal/pb/containerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

type jsonCodec struct{}

func (jsonCodec) Name() string                       { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func init() { encoding.RegisterCodec(jsonCodec{}) }

type Empty = containerpb.Empty
type HealthRequest = containerpb.HealthRequest
type HealthResponse = containerpb.HealthResponse
type Service = containerpb.Service
type ListServicesRequest = containerpb.ListServicesRequest
type ListServicesResponse = containerpb.ListServicesResponse
type DeployServiceRequest = containerpb.DeployServiceRequest
type DeleteServiceRequest = containerpb.DeleteServiceRequest
type ContainerServer = containerpb.ContainerServiceServer

type containerServer struct {
	namespace string
	db        *sql.DB
	q         *dbsqlc.Queries
}

func newContainerServer(namespace string) (*containerServer, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	return &containerServer{namespace: namespace, db: database, q: dbsqlc.New(database)}, nil
}

func (s *containerServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *containerServer) Health(context.Context, *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "container", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *containerServer) projectExists(ctx context.Context, userID, projectID string) (bool, error) {
	return s.q.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
}

func (s *containerServer) ListServices(ctx context.Context, req *ListServicesRequest) (*ListServicesResponse, error) {
	userID := strings.TrimSpace(req.UserID)
	projectID := strings.TrimSpace(req.ProjectID)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	records, err := s.q.ListContainers(ctx, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query containers")
	}
	items := make([]Service, 0, len(records))
	for _, record := range records {
		items = append(items, Service{
			Name:       record.Name,
			Image:      record.Image,
			URL:        record.Url,
			Ready:      record.Ready,
			Reason:     nullStringValue(record.Reason),
			CreatedAt:  record.CreatedAt,
			UpdatedAt:  record.UpdatedAt,
			Namespace:  record.Namespace,
			ProjectID:  record.ProjectID,
			Generation: record.Generation,
		})
	}
	return &ListServicesResponse{UserID: userID, ProjectID: projectID, Namespace: s.namespace, Containers: items}, nil
}

func (s *containerServer) DeployService(ctx context.Context, req *DeployServiceRequest) (*Service, error) {
	userID := strings.TrimSpace(req.UserID)
	projectID := strings.TrimSpace(req.ProjectID)
	name := strings.TrimSpace(req.Name)
	image := strings.TrimSpace(req.Image)
	if userID == "" || projectID == "" || name == "" || image == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, name, and image are required")
	}
	if req.Port < 1 || req.Port > 65535 {
		return nil, status.Error(codes.InvalidArgument, "port must be between 1 and 65535")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	url := fmt.Sprintf("https://%s.%s", name, publicServiceDomain())
	created, err := s.q.UpsertContainer(ctx, dbsqlc.UpsertContainerParams{
		ProjectID: projectID,
		Name:      name,
		Image:     image,
		Url:       url,
		Reason:    sql.NullString{},
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
		Namespace: s.namespace,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to persist service")
	}
	svc := Service{
		Name:       created.Name,
		Image:      created.Image,
		URL:        created.Url,
		Ready:      created.Ready,
		Reason:     nullStringValue(created.Reason),
		CreatedAt:  created.CreatedAt,
		UpdatedAt:  created.UpdatedAt,
		Namespace:  created.Namespace,
		ProjectID:  created.ProjectID,
		Generation: created.Generation,
	}
	return &svc, nil
}

func (s *containerServer) DeleteService(ctx context.Context, req *DeleteServiceRequest) (*Empty, error) {
	userID := strings.TrimSpace(req.UserID)
	projectID := strings.TrimSpace(req.ProjectID)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	rowsAffected, err := s.q.DeleteContainer(ctx, dbsqlc.DeleteContainerParams{ProjectID: projectID, Name: name})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete service")
	}
	if rowsAffected == 0 {
		return nil, status.Error(codes.NotFound, "service not found")
	}
	return &Empty{}, nil
}

func RegisterContainerServer(server *grpc.Server, impl ContainerServer) {
	containerpb.RegisterContainerServiceServer(server, impl)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCP_VM_ADDR", ":8082")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}
	server, err := newContainerServer(env("DCLD_TARGET_NAMESPACE", "dcloud-system"))
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	grpcServer := grpc.NewServer(grpc.ForceServerCodec(jsonCodec{}))
	RegisterContainerServer(grpcServer, server)
	errc := make(chan error, 1)
	go func() {
		logger.Info("container grpc listening", "addr", addr)
		errc <- grpcServer.Serve(lis)
	}()
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigc:
		grpcServer.GracefulStop()
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

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func publicServiceDomain() string {
	return env("DCLD_PUBLIC_SERVICE_DOMAIN", "drkatana.com")
}
