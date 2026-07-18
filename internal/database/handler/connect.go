package handler

import (
	"context"

	"connectrpc.com/connect"
	"github.com/daigo-suhara/dcloud/internal/database/service"
	databasepb "github.com/daigo-suhara/dcloud/internal/pb/databasepb"
)

// Connect bridges the gRPC-style service.Server methods to the
// Connect handler interface so both protocols share one implementation.
type Connect struct {
	inner *service.Server
}

func NewConnect(svc *service.Server) *Connect { return &Connect{inner: svc} }

func (a *Connect) Health(ctx context.Context, req *connect.Request[databasepb.HealthRequest]) (*connect.Response[databasepb.HealthResponse], error) {
	resp, err := a.inner.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) ListDatabases(ctx context.Context, req *connect.Request[databasepb.ListDatabasesRequest]) (*connect.Response[databasepb.ListDatabasesResponse], error) {
	resp, err := a.inner.ListDatabases(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) CreateDatabase(ctx context.Context, req *connect.Request[databasepb.CreateDatabaseRequest]) (*connect.Response[databasepb.CreateDatabaseResponse], error) {
	resp, err := a.inner.CreateDatabase(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) DeleteDatabase(ctx context.Context, req *connect.Request[databasepb.DeleteDatabaseRequest]) (*connect.Response[databasepb.DeleteDatabaseResponse], error) {
	resp, err := a.inner.DeleteDatabase(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) GetDatabase(ctx context.Context, req *connect.Request[databasepb.GetDatabaseRequest]) (*connect.Response[databasepb.GetDatabaseResponse], error) {
	resp, err := a.inner.GetDatabase(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) GetConnectionString(ctx context.Context, req *connect.Request[databasepb.GetConnectionStringRequest]) (*connect.Response[databasepb.GetConnectionStringResponse], error) {
	resp, err := a.inner.GetConnectionString(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) GetOperation(ctx context.Context, req *connect.Request[databasepb.GetOperationRequest]) (*connect.Response[databasepb.GetOperationResponse], error) {
	resp, err := a.inner.GetOperation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) ListSchemas(ctx context.Context, req *connect.Request[databasepb.ListSchemasRequest]) (*connect.Response[databasepb.ListSchemasResponse], error) {
	resp, err := a.inner.ListSchemas(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) CreateSchema(ctx context.Context, req *connect.Request[databasepb.CreateSchemaRequest]) (*connect.Response[databasepb.CreateSchemaResponse], error) {
	resp, err := a.inner.CreateSchema(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) DeleteSchema(ctx context.Context, req *connect.Request[databasepb.DeleteSchemaRequest]) (*connect.Response[databasepb.DeleteSchemaResponse], error) {
	resp, err := a.inner.DeleteSchema(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) ListBackups(ctx context.Context, req *connect.Request[databasepb.ListBackupsRequest]) (*connect.Response[databasepb.ListBackupsResponse], error) {
	resp, err := a.inner.ListBackups(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) CreateBackup(ctx context.Context, req *connect.Request[databasepb.CreateBackupRequest]) (*connect.Response[databasepb.CreateBackupResponse], error) {
	resp, err := a.inner.CreateBackup(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) DeleteBackup(ctx context.Context, req *connect.Request[databasepb.DeleteBackupRequest]) (*connect.Response[databasepb.DeleteBackupResponse], error) {
	resp, err := a.inner.DeleteBackup(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
