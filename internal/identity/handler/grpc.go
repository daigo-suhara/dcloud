package handler

import (
	"context"

	"github.com/daigo-suhara/dcloud/internal/identity/service"
	identitypb "github.com/daigo-suhara/dcloud/internal/pb/identitypb"
)

type GRPC struct {
	identitypb.UnimplementedIdentityServiceServer
	svc *service.Server
}

func NewGRPC(svc *service.Server) *GRPC { return &GRPC{svc: svc} }

func (h *GRPC) Health(ctx context.Context, req *identitypb.HealthRequest) (*identitypb.HealthResponse, error) {
	return h.svc.Health(ctx, req)
}

func (h *GRPC) Register(ctx context.Context, req *identitypb.RegisterRequest) (*identitypb.RegisterResponse, error) {
	return h.svc.Register(ctx, req)
}

func (h *GRPC) Login(ctx context.Context, req *identitypb.LoginRequest) (*identitypb.LoginResponse, error) {
	return h.svc.Login(ctx, req)
}

func (h *GRPC) Me(ctx context.Context, req *identitypb.MeRequest) (*identitypb.MeResponse, error) {
	return h.svc.Me(ctx, req)
}

func (h *GRPC) Logout(ctx context.Context, req *identitypb.LogoutRequest) (*identitypb.LogoutResponse, error) {
	return h.svc.Logout(ctx, req)
}
