package handler

import (
	"context"

	projectpb "github.com/daigo-suhara/dcloud/internal/pb/projectpb"
	"github.com/daigo-suhara/dcloud/internal/project/service"
)

type GRPC struct {
	projectpb.UnimplementedProjectServiceServer
	svc *service.Server
}

func NewGRPC(svc *service.Server) *GRPC { return &GRPC{svc: svc} }

func (h *GRPC) Health(ctx context.Context, req *projectpb.HealthRequest) (*projectpb.HealthResponse, error) {
	return h.svc.Health(ctx, req)
}
func (h *GRPC) Platform(ctx context.Context, req *projectpb.PlatformRequest) (*projectpb.PlatformResponse, error) {
	return h.svc.Platform(ctx, req)
}
func (h *GRPC) ListProjects(ctx context.Context, req *projectpb.ListProjectsRequest) (*projectpb.ListProjectsResponse, error) {
	return h.svc.ListProjects(ctx, req)
}
func (h *GRPC) CreateProject(ctx context.Context, req *projectpb.CreateProjectRequest) (*projectpb.CreateProjectResponse, error) {
	return h.svc.CreateProject(ctx, req)
}
func (h *GRPC) DeleteProject(ctx context.Context, req *projectpb.DeleteProjectRequest) (*projectpb.DeleteProjectResponse, error) {
	return h.svc.DeleteProject(ctx, req)
}
func (h *GRPC) IsProjectDeleting(ctx context.Context, req *projectpb.IsProjectDeletingRequest) (*projectpb.IsProjectDeletingResponse, error) {
	return h.svc.IsProjectDeleting(ctx, req)
}
func (h *GRPC) ProjectExists(ctx context.Context, req *projectpb.ProjectExistsRequest) (*projectpb.ProjectExistsResponse, error) {
	return h.svc.ProjectExists(ctx, req)
}
func (h *GRPC) GetProjectRepository(ctx context.Context, req *projectpb.GetProjectRepositoryRequest) (*projectpb.GetProjectRepositoryResponse, error) {
	return h.svc.GetProjectRepository(ctx, req)
}
func (h *GRPC) UpsertProjectRepository(ctx context.Context, req *projectpb.UpsertProjectRepositoryRequest) (*projectpb.UpsertProjectRepositoryResponse, error) {
	return h.svc.UpsertProjectRepository(ctx, req)
}
func (h *GRPC) CreateProjectDeleteOperation(ctx context.Context, req *projectpb.CreateProjectDeleteOperationRequest) (*projectpb.CreateProjectDeleteOperationResponse, error) {
	return h.svc.CreateProjectDeleteOperation(ctx, req)
}
