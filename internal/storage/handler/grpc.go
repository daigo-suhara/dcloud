// Package handler adapts the storage service layer to gRPC and ConnectRPC.
// The S3 object proxy HTTP handlers (list/upload/download/delete) will
// live here too once migrated from the Python api.
package handler

import (
	"context"

	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
	"github.com/daigo-suhara/dcloud/internal/storage/service"
)

type GRPC struct {
	storagepb.UnimplementedObjectStorageServiceServer
	svc *service.Server
}

func NewGRPC(svc *service.Server) *GRPC { return &GRPC{svc: svc} }

func (h *GRPC) Health(ctx context.Context, req *storagepb.HealthRequest) (*storagepb.HealthResponse, error) {
	return h.svc.Health(ctx, req)
}

func (h *GRPC) ListBuckets(ctx context.Context, req *storagepb.ListBucketsRequest) (*storagepb.ListBucketsResponse, error) {
	return h.svc.ListBuckets(ctx, req)
}

func (h *GRPC) CreateBucket(ctx context.Context, req *storagepb.CreateBucketRequest) (*storagepb.CreateBucketResponse, error) {
	return h.svc.CreateBucket(ctx, req)
}

func (h *GRPC) DeleteBucket(ctx context.Context, req *storagepb.DeleteBucketRequest) (*storagepb.DeleteBucketResponse, error) {
	return h.svc.DeleteBucket(ctx, req)
}

func (h *GRPC) GetBucketCredentials(ctx context.Context, req *storagepb.GetBucketCredentialsRequest) (*storagepb.GetBucketCredentialsResponse, error) {
	return h.svc.GetBucketCredentials(ctx, req)
}

func (h *GRPC) GetOperation(ctx context.Context, req *storagepb.GetOperationRequest) (*storagepb.GetOperationResponse, error) {
	return h.svc.GetOperation(ctx, req)
}
