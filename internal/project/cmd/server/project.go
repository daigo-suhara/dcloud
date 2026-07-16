package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"net/http"

	"connectrpc.com/connect"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	projectpb "github.com/daigo-suhara/dcloud/internal/pb/projectpb"
	"github.com/daigo-suhara/dcloud/internal/pb/projectpb/projectpbconnect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Empty = projectpb.Empty
type HealthRequest = projectpb.HealthRequest
type HealthResponse = projectpb.HealthResponse
type PlatformRequest = projectpb.PlatformRequest
type PlatformResponse = projectpb.PlatformResponse
type Project = projectpb.Project
type ProjectRepository = projectpb.ProjectRepository
type ListProjectsRequest = projectpb.ListProjectsRequest
type ListProjectsResponse = projectpb.ListProjectsResponse
type CreateProjectRequest = projectpb.CreateProjectRequest
type CreateProjectResponse = projectpb.CreateProjectResponse
type DeleteProjectRequest = projectpb.DeleteProjectRequest
type DeleteProjectResponse = projectpb.DeleteProjectResponse
type IsProjectDeletingRequest = projectpb.IsProjectDeletingRequest
type IsProjectDeletingResponse = projectpb.IsProjectDeletingResponse
type ProjectExistsRequest = projectpb.ProjectExistsRequest
type ProjectExistsResponse = projectpb.ProjectExistsResponse
type GetProjectRepositoryRequest = projectpb.GetProjectRepositoryRequest
type GetProjectRepositoryResponse = projectpb.GetProjectRepositoryResponse
type UpsertProjectRepositoryRequest = projectpb.UpsertProjectRepositoryRequest
type UpsertProjectRepositoryResponse = projectpb.UpsertProjectRepositoryResponse
type CreateProjectDeleteOperationRequest = projectpb.CreateProjectDeleteOperationRequest
type CreateProjectDeleteOperationResponse = projectpb.CreateProjectDeleteOperationResponse
type ProjectServer = projectpb.ProjectServiceServer

type projectServer struct {
	projectpb.UnimplementedProjectServiceServer
	db       *sql.DB
	q        *dbsqlc.Queries
	resource *resourceClients
}

func newProjectServer(resource *resourceClients) (*projectServer, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	return &projectServer{db: database, q: dbsqlc.New(database), resource: resource}, nil
}

func (s *projectServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *projectServer) Health(context.Context, *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "project", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *projectServer) Platform(context.Context, *PlatformRequest) (*PlatformResponse, error) {
	return &PlatformResponse{Name: "dcloud", Description: "Project service", Components: []string{"project", "compute", "database"}}, nil
}

func (s *projectServer) ListProjects(ctx context.Context, req *ListProjectsRequest) (*ListProjectsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}
	records, err := s.q.ListProjectsWithDeleting(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query projects")
	}
	items := make([]*Project, 0, len(records))
	for _, record := range records {
		items = append(items, &Project{
			Id:        record.ID,
			Name:      record.Name,
			Owner:     record.UserID,
			CreatedAt: record.CreatedAt,
			Deleting:  record.Deleting,
		})
	}
	return &ListProjectsResponse{UserId: userID, Projects: items}, nil
}

func (s *projectServer) CreateProject(ctx context.Context, req *CreateProjectRequest) (*CreateProjectResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and name are required")
	}
	project := Project{
		Id:        fmt.Sprintf("%s-%s", sanitizeDNSLabel(name), shortID()),
		Name:      name,
		Owner:     userID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err := s.q.CreateProject(ctx, dbsqlc.CreateProjectParams{
		ID:        project.Id,
		UserID:    userID,
		Name:      name,
		CreatedAt: project.CreatedAt,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.AlreadyExists, "project already exists")
	}
		return nil, status.Error(codes.Internal, "failed to persist project")
	}
	return &CreateProjectResponse{Project: &project}, nil
}

func (s *projectServer) DeleteProject(ctx context.Context, req *DeleteProjectRequest) (*DeleteProjectResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	rowsAffected, err := s.q.DeleteProject(ctx, dbsqlc.DeleteProjectParams{UserID: userID, ID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete project")
	}
	if rowsAffected == 0 {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	return &DeleteProjectResponse{}, nil
}

func (s *projectServer) ProjectExists(ctx context.Context, req *ProjectExistsRequest) (*ProjectExistsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return &ProjectExistsResponse{Exists: false}, nil
	}
	exists, err := s.q.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to check project")
	}
	return &ProjectExistsResponse{Exists: exists}, nil
}

func (s *projectServer) IsProjectDeleting(ctx context.Context, req *IsProjectDeletingRequest) (*IsProjectDeletingResponse, error) {
	projectID := strings.TrimSpace(req.ProjectId)
	if projectID == "" {
		return &IsProjectDeletingResponse{Deleting: false}, nil
	}
	deleting, err := s.q.IsProjectDeleting(ctx, sql.NullString{String: projectID, Valid: true})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query operations")
	}
	return &IsProjectDeletingResponse{Deleting: deleting}, nil
}

func (s *projectServer) GetProjectRepository(ctx context.Context, req *GetProjectRepositoryRequest) (*GetProjectRepositoryResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	record, err := s.q.GetProjectRepository(ctx, dbsqlc.GetProjectRepositoryParams{UserID: userID, ProjectID: projectID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "repository not configured")
		}
		return nil, status.Error(codes.Internal, "failed to query repository")
	}
	return &GetProjectRepositoryResponse{Repository: repositoryToProto(record)}, nil
}

func (s *projectServer) UpsertProjectRepository(ctx context.Context, req *UpsertProjectRepositoryRequest) (*UpsertProjectRepositoryResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	owner := strings.TrimSpace(req.RepositoryOwner)
	name := strings.TrimSpace(req.RepositoryName)
	branch := strings.TrimSpace(req.RepositoryBranch)
	if branch == "" {
		branch = "main"
	}
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	if owner == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "repositoryOwner and repositoryName are required")
	}
	exists, err := s.q.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to check project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	record, err := s.q.UpsertProjectRepository(ctx, dbsqlc.UpsertProjectRepositoryParams{
		ProjectID:        projectID,
		UserID:           userID,
		RepositoryOwner:  owner,
		RepositoryName:   name,
		RepositoryBranch: branch,
		ConnectedAt:      ts,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to persist repository")
	}
	return &UpsertProjectRepositoryResponse{Repository: repositoryToProto(record)}, nil
}

func (s *projectServer) CreateProjectDeleteOperation(ctx context.Context, req *CreateProjectDeleteOperationRequest) (*CreateProjectDeleteOperationResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	// Best-effort fan-out delete of every resource type across the four
	// sibling services. The caller's JWT is forwarded via context so each
	// service's jwtverify interceptor accepts the calls.
	if s.resource != nil {
		s.resource.deleteAllResources(ctx, userID, projectID)
	}
	opID := fmt.Sprintf("project-op-%s", shortID())
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.q.CreateOperation(ctx, dbsqlc.CreateOperationParams{
		ID:           opID,
		ResourceType: sql.NullString{String: "project", Valid: true},
		ResourceName: sql.NullString{String: projectID, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    ts,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	return &CreateProjectDeleteOperationResponse{OperationId: opID}, nil
}

func repositoryToProto(r dbsqlc.ProjectRepository) *ProjectRepository {
	return &ProjectRepository{
		ProjectId:        r.ProjectID,
		UserId:           r.UserID,
		RepositoryOwner:  r.RepositoryOwner,
		RepositoryName:   r.RepositoryName,
		RepositoryBranch: r.RepositoryBranch,
		ConnectedAt:      r.ConnectedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

func RegisterProjectServer(server *grpc.Server, impl ProjectServer) {
	projectpb.RegisterProjectServiceServer(server, impl)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	grpcAddr := env("DCP_PROJECT_ADDR", ":8081")
	httpAddr := env("DCP_PROJECT_HTTP_ADDR", ":8091")
	jwksURL := env("DCLD_IDENTITY_JWKS_URL", "http://identity:8093/.well-known/jwks.json")

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("failed to listen", "addr", grpcAddr, "error", err)
		os.Exit(1)
	}
	server, err := newProjectServer(newResourceClients())
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	grpcServer := grpc.NewServer()
	RegisterProjectServer(grpcServer, server)

	verifier := jwtverify.New(jwksURL)
	adapter := &connectAdapter{inner: server}
	mux := http.NewServeMux()
	path, handler := projectpbconnect.NewProjectServiceHandler(
		adapter,
		connect.WithInterceptors(verifier.ConnectInterceptor()),
	)
	mux.Handle(path, handler)
	registerRESTRoutes(mux, server, verifier)
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

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

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastHyphen := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		default:
			if b.Len() > 0 && !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func shortID() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf[:])
}
