package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
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
	"google.golang.org/grpc/status"
)

type Empty = containerpb.Empty
type HealthRequest = containerpb.HealthRequest
type HealthResponse = containerpb.HealthResponse
type Service = containerpb.Service
type ListServicesRequest = containerpb.ListServicesRequest
type ListServicesResponse = containerpb.ListServicesResponse
type DeployServiceRequest = containerpb.DeployServiceRequest
type DeployServiceResponse = containerpb.DeployServiceResponse
type DeleteServiceRequest = containerpb.DeleteServiceRequest
type DeleteServiceResponse = containerpb.DeleteServiceResponse
type GetOperationRequest = containerpb.GetOperationRequest
type GetOperationResponse = containerpb.GetOperationResponse
type ContainerServer = containerpb.ContainerServiceServer

func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "container-op-" + hex.EncodeToString(buf), nil
}

type projectScope struct {
	UserID    string
	ProjectID string
}

type deployRequest struct {
	Name     string
	Image    string
	Port     int32
	MinScale int32
	MaxScale int32
}

type deployedService struct {
	Name         string
	Image        string
	URL          string
	ResourceName string
	Ready        bool
	Reason       string
	CreatedAt    string
	UpdatedAt    string
	Namespace    string
	ProjectID    string
	Generation   int64
}

type containerServer struct {
	containerpb.UnimplementedContainerServiceServer
	namespace string
	db        *sql.DB
	q         *dbsqlc.Queries
	knative   *knativeServiceManager
}

func newContainerServer(namespace string) (*containerServer, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	knative, err := newKnativeServiceManager(namespace, publicServiceDomain())
	if err != nil {
		return nil, err
	}
	return &containerServer{namespace: namespace, db: database, q: dbsqlc.New(database), knative: knative}, nil
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
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
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
	records, err := s.knative.list(ctx, projectScope{UserID: userID, ProjectID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query containers")
	}
	items := make([]*Service, 0, len(records))
	for _, record := range records {
		items = append(items, &Service{
			Name:       record.Name,
			Image:      record.Image,
			Url:        record.URL,
			Ready:      record.Ready,
			Reason:     record.Reason,
			CreatedAt:  record.CreatedAt,
			UpdatedAt:  record.UpdatedAt,
			Namespace:  record.Namespace,
			ProjectId:  record.ProjectID,
			Generation: record.Generation,
		})
	}
	return &ListServicesResponse{UserId: userID, ProjectId: projectID, Namespace: s.namespace, Containers: items}, nil
}

func (s *containerServer) DeployService(ctx context.Context, req *DeployServiceRequest) (*DeployServiceResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
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

	created, err := s.knative.deploy(ctx, projectScope{UserID: userID, ProjectID: projectID}, deployRequest{
		Name:     name,
		Image:    image,
		Port:     req.Port,
		MinScale: req.MinScale,
		MaxScale: req.MaxScale,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to deploy service")
	}
	createdAt := created.CreatedAt
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	updatedAt := created.UpdatedAt
	if updatedAt == "" {
		updatedAt = createdAt
	}
	if _, err := s.q.UpsertContainer(ctx, dbsqlc.UpsertContainerParams{
		ProjectID: projectID,
		Name:      name,
		Image:     created.Image,
		Url:       created.URL,
		Reason:    sql.NullString{},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Namespace: created.Namespace,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to persist service")
	}
	svc := Service{
		Name:       created.Name,
		Image:      created.Image,
		Url:        created.URL,
		Ready:      created.Ready,
		Reason:     created.Reason,
		CreatedAt:  created.CreatedAt,
		UpdatedAt:  created.UpdatedAt,
		Namespace:  created.Namespace,
		ProjectId:  created.ProjectID,
		Generation: created.Generation,
	}
	return &DeployServiceResponse{Service: &svc}, nil
}

func (s *containerServer) DeleteService(ctx context.Context, req *DeleteServiceRequest) (*DeleteServiceResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
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
	opID, err := newOperationID()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.q.CreateOperation(ctx, dbsqlc.CreateOperationParams{
		ID:           opID,
		ResourceType: sql.NullString{String: "container", Valid: true},
		ResourceName: sql.NullString{String: name, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	go func() {
		bgCtx := context.Background()
		errMsg := sql.NullString{}
		newStatus := "done"
		if err := s.knative.delete(bgCtx, projectScope{UserID: userID, ProjectID: projectID}, name); err != nil {
			newStatus = "error"
			errMsg = sql.NullString{String: err.Error(), Valid: true}
		} else {
			_, _ = s.q.DeleteContainer(bgCtx, dbsqlc.DeleteContainerParams{ProjectID: projectID, Name: name})
		}
		_ = s.q.UpdateOperation(bgCtx, dbsqlc.UpdateOperationParams{
			ID:        opID,
			Status:    newStatus,
			Error:     errMsg,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}()
	return &DeleteServiceResponse{OperationId: opID}, nil
}

func (s *containerServer) GetOperation(ctx context.Context, req *GetOperationRequest) (*GetOperationResponse, error) {
	opID := strings.TrimSpace(req.OperationId)
	if opID == "" {
		return nil, status.Error(codes.InvalidArgument, "operationId is required")
	}
	op, err := s.q.GetOperation(ctx, opID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "operation not found")
		}
		return nil, status.Error(codes.Internal, "failed to get operation")
	}
	errStr := ""
	if op.Error.Valid {
		errStr = op.Error.String
	}
	return &GetOperationResponse{OperationId: op.ID, Status: op.Status, Error: errStr}, nil
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

	grpcServer := grpc.NewServer()
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

func publicServiceDomain() string {
	return env("DCLD_PUBLIC_SERVICE_DOMAIN", "drkatana.com")
}
