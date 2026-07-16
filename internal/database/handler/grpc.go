package handler

import (
	"context"

	"github.com/daigo-suhara/dcloud/internal/database/service"
	databasepb "github.com/daigo-suhara/dcloud/internal/pb/databasepb"
)

type GRPC struct {
	databasepb.UnimplementedDatabaseServiceServer
	svc *service.Server
}

func NewGRPC(svc *service.Server) *GRPC { return &GRPC{svc: svc} }

func (h *GRPC) Health(ctx context.Context, req *databasepb.HealthRequest) (*databasepb.HealthResponse, error) {
	return h.svc.Health(ctx, req)
}
func (h *GRPC) ListDatabases(ctx context.Context, req *databasepb.ListDatabasesRequest) (*databasepb.ListDatabasesResponse, error) {
	return h.svc.ListDatabases(ctx, req)
}
func (h *GRPC) CreateDatabase(ctx context.Context, req *databasepb.CreateDatabaseRequest) (*databasepb.CreateDatabaseResponse, error) {
	return h.svc.CreateDatabase(ctx, req)
}
func (h *GRPC) DeleteDatabase(ctx context.Context, req *databasepb.DeleteDatabaseRequest) (*databasepb.DeleteDatabaseResponse, error) {
	return h.svc.DeleteDatabase(ctx, req)
}
func (h *GRPC) GetDatabase(ctx context.Context, req *databasepb.GetDatabaseRequest) (*databasepb.GetDatabaseResponse, error) {
	return h.svc.GetDatabase(ctx, req)
}
func (h *GRPC) GetConnectionString(ctx context.Context, req *databasepb.GetConnectionStringRequest) (*databasepb.GetConnectionStringResponse, error) {
	return h.svc.GetConnectionString(ctx, req)
}
func (h *GRPC) GetOperation(ctx context.Context, req *databasepb.GetOperationRequest) (*databasepb.GetOperationResponse, error) {
	return h.svc.GetOperation(ctx, req)
}
func (h *GRPC) ListSchemas(ctx context.Context, req *databasepb.ListSchemasRequest) (*databasepb.ListSchemasResponse, error) {
	return h.svc.ListSchemas(ctx, req)
}
func (h *GRPC) CreateSchema(ctx context.Context, req *databasepb.CreateSchemaRequest) (*databasepb.CreateSchemaResponse, error) {
	return h.svc.CreateSchema(ctx, req)
}
func (h *GRPC) DeleteSchema(ctx context.Context, req *databasepb.DeleteSchemaRequest) (*databasepb.DeleteSchemaResponse, error) {
	return h.svc.DeleteSchema(ctx, req)
}
