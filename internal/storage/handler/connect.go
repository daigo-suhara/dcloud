package handler

import (
	"context"

	"connectrpc.com/connect"
	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
	"github.com/daigo-suhara/dcloud/internal/storage/service"
)

type Connect struct {
	svc *service.Server
}

func NewConnect(svc *service.Server) *Connect { return &Connect{svc: svc} }

func (h *Connect) Health(ctx context.Context, req *connect.Request[storagepb.HealthRequest]) (*connect.Response[storagepb.HealthResponse], error) {
	resp, err := h.svc.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) ListBuckets(ctx context.Context, req *connect.Request[storagepb.ListBucketsRequest]) (*connect.Response[storagepb.ListBucketsResponse], error) {
	resp, err := h.svc.ListBuckets(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) CreateBucket(ctx context.Context, req *connect.Request[storagepb.CreateBucketRequest]) (*connect.Response[storagepb.CreateBucketResponse], error) {
	resp, err := h.svc.CreateBucket(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) DeleteBucket(ctx context.Context, req *connect.Request[storagepb.DeleteBucketRequest]) (*connect.Response[storagepb.DeleteBucketResponse], error) {
	resp, err := h.svc.DeleteBucket(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) GetBucketCredentials(ctx context.Context, req *connect.Request[storagepb.GetBucketCredentialsRequest]) (*connect.Response[storagepb.GetBucketCredentialsResponse], error) {
	resp, err := h.svc.GetBucketCredentials(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (h *Connect) GetOperation(ctx context.Context, req *connect.Request[storagepb.GetOperationRequest]) (*connect.Response[storagepb.GetOperationResponse], error) {
	resp, err := h.svc.GetOperation(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
