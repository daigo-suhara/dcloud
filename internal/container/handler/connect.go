package handler

import (
	"context"

	"connectrpc.com/connect"
	"github.com/daigo-suhara/dcloud/internal/container/service"
	containerpb "github.com/daigo-suhara/dcloud/internal/pb/containerpb"
)

// Connect implements containerpbconnect.ContainerServiceHandler.
type Connect struct {
	svc *service.Server
}

func NewConnect(svc *service.Server) *Connect { return &Connect{svc: svc} }

func (h *Connect) Health(ctx context.Context, req *connect.Request[containerpb.HealthRequest]) (*connect.Response[containerpb.HealthResponse], error) {
	resp, err := h.svc.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) ListServices(ctx context.Context, req *connect.Request[containerpb.ListServicesRequest]) (*connect.Response[containerpb.ListServicesResponse], error) {
	resp, err := h.svc.ListServices(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) DeployService(ctx context.Context, req *connect.Request[containerpb.DeployServiceRequest]) (*connect.Response[containerpb.DeployServiceResponse], error) {
	resp, err := h.svc.DeployService(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) DeleteService(ctx context.Context, req *connect.Request[containerpb.DeleteServiceRequest]) (*connect.Response[containerpb.DeleteServiceResponse], error) {
	resp, err := h.svc.DeleteService(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) GetOperation(ctx context.Context, req *connect.Request[containerpb.GetOperationRequest]) (*connect.Response[containerpb.GetOperationResponse], error) {
	resp, err := h.svc.GetOperation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) SetServiceDomain(ctx context.Context, req *connect.Request[containerpb.SetServiceDomainRequest]) (*connect.Response[containerpb.SetServiceDomainResponse], error) {
	resp, err := h.svc.SetServiceDomain(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) GetServiceLogs(ctx context.Context, req *connect.Request[containerpb.GetServiceLogsRequest], stream *connect.ServerStream[containerpb.LogLine]) error {
	return h.svc.StreamServiceLogs(ctx, req.Msg, stream.Send)
}
