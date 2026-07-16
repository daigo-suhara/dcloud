package handler

import (
	"context"

	"connectrpc.com/connect"
	projectpb "github.com/daigo-suhara/dcloud/internal/pb/projectpb"
	"github.com/daigo-suhara/dcloud/internal/project/service"
)

// Connect bridges the gRPC-style service.Server methods to the
// Connect handler interface. It forwards each Connect request/response to
// the underlying gRPC method so both protocols share a single implementation.
type Connect struct {
	inner *service.Server
}

func NewConnect(svc *service.Server) *Connect { return &Connect{inner: svc} }

func (a *Connect) Health(ctx context.Context, req *connect.Request[projectpb.HealthRequest]) (*connect.Response[projectpb.HealthResponse], error) {
	resp, err := a.inner.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) Platform(ctx context.Context, req *connect.Request[projectpb.PlatformRequest]) (*connect.Response[projectpb.PlatformResponse], error) {
	resp, err := a.inner.Platform(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) ListProjects(ctx context.Context, req *connect.Request[projectpb.ListProjectsRequest]) (*connect.Response[projectpb.ListProjectsResponse], error) {
	resp, err := a.inner.ListProjects(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) CreateProject(ctx context.Context, req *connect.Request[projectpb.CreateProjectRequest]) (*connect.Response[projectpb.CreateProjectResponse], error) {
	resp, err := a.inner.CreateProject(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) DeleteProject(ctx context.Context, req *connect.Request[projectpb.DeleteProjectRequest]) (*connect.Response[projectpb.DeleteProjectResponse], error) {
	resp, err := a.inner.DeleteProject(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) ProjectExists(ctx context.Context, req *connect.Request[projectpb.ProjectExistsRequest]) (*connect.Response[projectpb.ProjectExistsResponse], error) {
	resp, err := a.inner.ProjectExists(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) IsProjectDeleting(ctx context.Context, req *connect.Request[projectpb.IsProjectDeletingRequest]) (*connect.Response[projectpb.IsProjectDeletingResponse], error) {
	resp, err := a.inner.IsProjectDeleting(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) GetProjectRepository(ctx context.Context, req *connect.Request[projectpb.GetProjectRepositoryRequest]) (*connect.Response[projectpb.GetProjectRepositoryResponse], error) {
	resp, err := a.inner.GetProjectRepository(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) UpsertProjectRepository(ctx context.Context, req *connect.Request[projectpb.UpsertProjectRepositoryRequest]) (*connect.Response[projectpb.UpsertProjectRepositoryResponse], error) {
	resp, err := a.inner.UpsertProjectRepository(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) CreateProjectDeleteOperation(ctx context.Context, req *connect.Request[projectpb.CreateProjectDeleteOperationRequest]) (*connect.Response[projectpb.CreateProjectDeleteOperationResponse], error) {
	resp, err := a.inner.CreateProjectDeleteOperation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
