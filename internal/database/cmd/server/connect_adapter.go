package main

import (
	"context"

	"connectrpc.com/connect"
)

// connectAdapter bridges the gRPC-style databaseServer methods to the
// Connect handler interface so both protocols share one implementation.
type connectAdapter struct {
	inner *databaseServer
}

func (a *connectAdapter) Health(ctx context.Context, req *connect.Request[HealthRequest]) (*connect.Response[HealthResponse], error) {
	resp, err := a.inner.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) ListDatabases(ctx context.Context, req *connect.Request[ListDatabasesRequest]) (*connect.Response[ListDatabasesResponse], error) {
	resp, err := a.inner.ListDatabases(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) CreateDatabase(ctx context.Context, req *connect.Request[CreateDatabaseRequest]) (*connect.Response[CreateDatabaseResponse], error) {
	resp, err := a.inner.CreateDatabase(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) DeleteDatabase(ctx context.Context, req *connect.Request[DeleteDatabaseRequest]) (*connect.Response[DeleteDatabaseResponse], error) {
	resp, err := a.inner.DeleteDatabase(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) GetDatabase(ctx context.Context, req *connect.Request[GetDatabaseRequest]) (*connect.Response[GetDatabaseResponse], error) {
	resp, err := a.inner.GetDatabase(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) GetConnectionString(ctx context.Context, req *connect.Request[GetConnectionStringRequest]) (*connect.Response[GetConnectionStringResponse], error) {
	resp, err := a.inner.GetConnectionString(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) GetOperation(ctx context.Context, req *connect.Request[GetOperationRequest]) (*connect.Response[GetOperationResponse], error) {
	resp, err := a.inner.GetOperation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) ListSchemas(ctx context.Context, req *connect.Request[ListSchemasRequest]) (*connect.Response[ListSchemasResponse], error) {
	resp, err := a.inner.ListSchemas(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) CreateSchema(ctx context.Context, req *connect.Request[CreateSchemaRequest]) (*connect.Response[CreateSchemaResponse], error) {
	resp, err := a.inner.CreateSchema(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) DeleteSchema(ctx context.Context, req *connect.Request[DeleteSchemaRequest]) (*connect.Response[DeleteSchemaResponse], error) {
	resp, err := a.inner.DeleteSchema(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
