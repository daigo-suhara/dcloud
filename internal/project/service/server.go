package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	projectpb "github.com/daigo-suhara/dcloud/internal/pb/projectpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResourceClients interface {
	DeleteAllResources(ctx context.Context, userID, projectID string)
}

type Server struct {
	DB       *sql.DB
	Queries  *dbsqlc.Queries
	Resource ResourceClients
	Kube     *kubeClient
}

func New(resource ResourceClients) (*Server, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	kube, _ := newKubeClient()
	return &Server{DB: database, Queries: dbsqlc.New(database), Resource: resource, Kube: kube}, nil
}

func (s *Server) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Server) Health(context.Context, *projectpb.HealthRequest) (*projectpb.HealthResponse, error) {
	return &projectpb.HealthResponse{Status: "ok", Service: "project", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Server) Platform(context.Context, *projectpb.PlatformRequest) (*projectpb.PlatformResponse, error) {
	return &projectpb.PlatformResponse{Name: "dcloud", Description: "projectpb.Project service", Components: []string{"project", "compute", "database"}}, nil
}

func (s *Server) ListProjects(ctx context.Context, req *projectpb.ListProjectsRequest) (*projectpb.ListProjectsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId is required")
	}
	records, err := s.Queries.ListProjectsWithDeleting(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query projects")
	}
	items := make([]*projectpb.Project, 0, len(records))
	for _, record := range records {
		items = append(items, &projectpb.Project{
			Id:        record.ID,
			Name:      record.Name,
			Owner:     record.UserID,
			CreatedAt: record.CreatedAt,
			Deleting:  record.Deleting,
		})
	}
	return &projectpb.ListProjectsResponse{UserId: userID, Projects: items}, nil
}

func (s *Server) CreateProject(ctx context.Context, req *projectpb.CreateProjectRequest) (*projectpb.CreateProjectResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and name are required")
	}
	project := projectpb.Project{
		Id:        fmt.Sprintf("%s-%s", sanitizeDNSLabel(name), shortID()),
		Name:      name,
		Owner:     userID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	_, err := s.Queries.CreateProject(ctx, dbsqlc.CreateProjectParams{
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
	if s.Kube != nil {
		if err := s.Kube.ensureProjectNamespace(ctx, project.Id, userID); err != nil {
			fmt.Fprintf(os.Stderr, "project: failed to create namespace for %s: %v\n", project.Id, err)
		}
	}
	return &projectpb.CreateProjectResponse{Project: &project}, nil
}

func (s *Server) DeleteProject(ctx context.Context, req *projectpb.DeleteProjectRequest) (*projectpb.DeleteProjectResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	rowsAffected, err := s.Queries.DeleteProject(ctx, dbsqlc.DeleteProjectParams{UserID: userID, ID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete project")
	}
	if rowsAffected == 0 {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	return &projectpb.DeleteProjectResponse{}, nil
}

func (s *Server) ProjectExists(ctx context.Context, req *projectpb.ProjectExistsRequest) (*projectpb.ProjectExistsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return &projectpb.ProjectExistsResponse{Exists: false}, nil
	}
	exists, err := s.Queries.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to check project")
	}
	return &projectpb.ProjectExistsResponse{Exists: exists}, nil
}

func (s *Server) IsProjectDeleting(ctx context.Context, req *projectpb.IsProjectDeletingRequest) (*projectpb.IsProjectDeletingResponse, error) {
	projectID := strings.TrimSpace(req.ProjectId)
	if projectID == "" {
		return &projectpb.IsProjectDeletingResponse{Deleting: false}, nil
	}
	deleting, err := s.Queries.IsProjectDeleting(ctx, sql.NullString{String: projectID, Valid: true})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query operations")
	}
	return &projectpb.IsProjectDeletingResponse{Deleting: deleting}, nil
}

func (s *Server) GetProjectRepository(ctx context.Context, req *projectpb.GetProjectRepositoryRequest) (*projectpb.GetProjectRepositoryResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	record, err := s.Queries.GetProjectRepository(ctx, dbsqlc.GetProjectRepositoryParams{UserID: userID, ProjectID: projectID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "repository not configured")
		}
		return nil, status.Error(codes.Internal, "failed to query repository")
	}
	return &projectpb.GetProjectRepositoryResponse{Repository: repositoryToProto(record)}, nil
}

func (s *Server) UpsertProjectRepository(ctx context.Context, req *projectpb.UpsertProjectRepositoryRequest) (*projectpb.UpsertProjectRepositoryResponse, error) {
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
	exists, err := s.Queries.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to check project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	record, err := s.Queries.UpsertProjectRepository(ctx, dbsqlc.UpsertProjectRepositoryParams{
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
	return &projectpb.UpsertProjectRepositoryResponse{Repository: repositoryToProto(record)}, nil
}

func (s *Server) CreateProjectDeleteOperation(ctx context.Context, req *projectpb.CreateProjectDeleteOperationRequest) (*projectpb.CreateProjectDeleteOperationResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	if s.Resource != nil {
		s.Resource.DeleteAllResources(ctx, userID, projectID)
	}
	if s.Kube != nil {
		if err := s.Kube.deleteProjectNamespace(ctx, projectID); err != nil {
			fmt.Fprintf(os.Stderr, "project: failed to delete namespace for %s: %v\n", projectID, err)
		}
	}
	opID := fmt.Sprintf("project-op-%s", shortID())
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.Queries.CreateOperation(ctx, dbsqlc.CreateOperationParams{
		ID:           opID,
		ResourceType: sql.NullString{String: "project", Valid: true},
		ResourceName: sql.NullString{String: projectID, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    ts,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	return &projectpb.CreateProjectDeleteOperationResponse{OperationId: opID}, nil
}

func repositoryToProto(r dbsqlc.ProjectRepository) *projectpb.ProjectRepository {
	return &projectpb.ProjectRepository{
		ProjectId:        r.ProjectID,
		UserId:           r.UserID,
		RepositoryOwner:  r.RepositoryOwner,
		RepositoryName:   r.RepositoryName,
		RepositoryBranch: r.RepositoryBranch,
		ConnectedAt:      r.ConnectedAt,
		UpdatedAt:        r.UpdatedAt,
	}
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
