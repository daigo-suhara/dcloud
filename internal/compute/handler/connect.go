package handler

import (
	"context"

	"connectrpc.com/connect"
	"github.com/daigo-suhara/dcloud/internal/compute/service"
	computepb "github.com/daigo-suhara/dcloud/internal/pb/computepb"
)

type Connect struct {
	svc *service.Server
}

func NewConnect(svc *service.Server) *Connect { return &Connect{svc: svc} }

func (h *Connect) Health(ctx context.Context, req *connect.Request[computepb.HealthRequest]) (*connect.Response[computepb.HealthResponse], error) {
	resp, err := h.svc.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) ListMachines(ctx context.Context, req *connect.Request[computepb.ListMachinesRequest]) (*connect.Response[computepb.ListMachinesResponse], error) {
	resp, err := h.svc.ListMachines(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) CreateMachine(ctx context.Context, req *connect.Request[computepb.CreateMachineRequest]) (*connect.Response[computepb.CreateMachineResponse], error) {
	resp, err := h.svc.CreateMachine(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) DeleteMachine(ctx context.Context, req *connect.Request[computepb.DeleteMachineRequest]) (*connect.Response[computepb.DeleteMachineResponse], error) {
	resp, err := h.svc.DeleteMachine(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) GetOperation(ctx context.Context, req *connect.Request[computepb.GetOperationRequest]) (*connect.Response[computepb.GetOperationResponse], error) {
	resp, err := h.svc.GetOperation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
