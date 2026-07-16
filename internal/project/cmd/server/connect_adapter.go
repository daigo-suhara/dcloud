package main

import (
	"context"

	"connectrpc.com/connect"
)

// connectAdapter bridges the gRPC-style projectServer methods to the
// Connect handler interface. It forwards each Connect request/response to
// the underlying gRPC method so both protocols share a single implementation.
type connectAdapter struct {
	inner *projectServer
}

func (a *connectAdapter) Health(ctx context.Context, req *connect.Request[HealthRequest]) (*connect.Response[HealthResponse], error) {
	resp, err := a.inner.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) Platform(ctx context.Context, req *connect.Request[PlatformRequest]) (*connect.Response[PlatformResponse], error) {
	resp, err := a.inner.Platform(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) ListProjects(ctx context.Context, req *connect.Request[ListProjectsRequest]) (*connect.Response[ListProjectsResponse], error) {
	resp, err := a.inner.ListProjects(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) CreateProject(ctx context.Context, req *connect.Request[CreateProjectRequest]) (*connect.Response[CreateProjectResponse], error) {
	resp, err := a.inner.CreateProject(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) DeleteProject(ctx context.Context, req *connect.Request[DeleteProjectRequest]) (*connect.Response[DeleteProjectResponse], error) {
	resp, err := a.inner.DeleteProject(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) ProjectExists(ctx context.Context, req *connect.Request[ProjectExistsRequest]) (*connect.Response[ProjectExistsResponse], error) {
	resp, err := a.inner.ProjectExists(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) IsProjectDeleting(ctx context.Context, req *connect.Request[IsProjectDeletingRequest]) (*connect.Response[IsProjectDeletingResponse], error) {
	resp, err := a.inner.IsProjectDeleting(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) GetProjectRepository(ctx context.Context, req *connect.Request[GetProjectRepositoryRequest]) (*connect.Response[GetProjectRepositoryResponse], error) {
	resp, err := a.inner.GetProjectRepository(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) UpsertProjectRepository(ctx context.Context, req *connect.Request[UpsertProjectRepositoryRequest]) (*connect.Response[UpsertProjectRepositoryResponse], error) {
	resp, err := a.inner.UpsertProjectRepository(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) CreateProjectDeleteOperation(ctx context.Context, req *connect.Request[CreateProjectDeleteOperationRequest]) (*connect.Response[CreateProjectDeleteOperationResponse], error) {
	resp, err := a.inner.CreateProjectDeleteOperation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
