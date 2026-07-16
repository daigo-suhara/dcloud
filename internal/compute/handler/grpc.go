// Package handler adapts the compute service layer to gRPC, ConnectRPC,
// and the KubeVirt VM console WebSocket transport.
package handler

import (
	"context"

	"github.com/daigo-suhara/dcloud/internal/compute/service"
	computepb "github.com/daigo-suhara/dcloud/internal/pb/computepb"
)

type GRPC struct {
	computepb.UnimplementedComputeServiceServer
	svc *service.Server
}

func NewGRPC(svc *service.Server) *GRPC { return &GRPC{svc: svc} }

func (h *GRPC) Health(ctx context.Context, req *computepb.HealthRequest) (*computepb.HealthResponse, error) {
	return h.svc.Health(ctx, req)
}

func (h *GRPC) ListMachines(ctx context.Context, req *computepb.ListMachinesRequest) (*computepb.ListMachinesResponse, error) {
	return h.svc.ListMachines(ctx, req)
}

func (h *GRPC) CreateMachine(ctx context.Context, req *computepb.CreateMachineRequest) (*computepb.CreateMachineResponse, error) {
	return h.svc.CreateMachine(ctx, req)
}

func (h *GRPC) DeleteMachine(ctx context.Context, req *computepb.DeleteMachineRequest) (*computepb.DeleteMachineResponse, error) {
	return h.svc.DeleteMachine(ctx, req)
}

func (h *GRPC) GetOperation(ctx context.Context, req *computepb.GetOperationRequest) (*computepb.GetOperationResponse, error) {
	return h.svc.GetOperation(ctx, req)
}
