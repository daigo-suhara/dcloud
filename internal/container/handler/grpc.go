// Package handler adapts the transport-neutral service layer to the two
// protocols the container service speaks: gRPC and ConnectRPC.
package handler

import (
	"context"

	"github.com/daigo-suhara/dcloud/internal/container/service"
	containerpb "github.com/daigo-suhara/dcloud/internal/pb/containerpb"
)

// GRPC implements containerpb.ContainerServiceServer by forwarding every
// RPC to the shared *service.Server.
type GRPC struct {
	containerpb.UnimplementedContainerServiceServer
	svc *service.Server
}

func NewGRPC(svc *service.Server) *GRPC { return &GRPC{svc: svc} }

func (h *GRPC) Health(ctx context.Context, req *containerpb.HealthRequest) (*containerpb.HealthResponse, error) {
	return h.svc.Health(ctx, req)
}

func (h *GRPC) ListServices(ctx context.Context, req *containerpb.ListServicesRequest) (*containerpb.ListServicesResponse, error) {
	return h.svc.ListServices(ctx, req)
}

func (h *GRPC) DeployService(ctx context.Context, req *containerpb.DeployServiceRequest) (*containerpb.DeployServiceResponse, error) {
	return h.svc.DeployService(ctx, req)
}

func (h *GRPC) DeleteService(ctx context.Context, req *containerpb.DeleteServiceRequest) (*containerpb.DeleteServiceResponse, error) {
	return h.svc.DeleteService(ctx, req)
}

func (h *GRPC) GetOperation(ctx context.Context, req *containerpb.GetOperationRequest) (*containerpb.GetOperationResponse, error) {
	return h.svc.GetOperation(ctx, req)
}

func (h *GRPC) SetServiceDomain(ctx context.Context, req *containerpb.SetServiceDomainRequest) (*containerpb.SetServiceDomainResponse, error) {
	return h.svc.SetServiceDomain(ctx, req)
}

func (h *GRPC) GetServiceLogs(req *containerpb.GetServiceLogsRequest, stream containerpb.ContainerService_GetServiceLogsServer) error {
	return h.svc.StreamServiceLogs(stream.Context(), req, stream.Send)
}
