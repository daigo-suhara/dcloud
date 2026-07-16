package handler

import (
	"context"

	"connectrpc.com/connect"
	"github.com/daigo-suhara/dcloud/internal/identity/service"
	identitypb "github.com/daigo-suhara/dcloud/internal/pb/identitypb"
)

// Connect bridges the gRPC-style service.Server methods to the Connect
// handler interface so both protocols share one implementation.
type Connect struct {
	inner *service.Server
}

func NewConnect(svc *service.Server) *Connect { return &Connect{inner: svc} }

func (a *Connect) Health(ctx context.Context, req *connect.Request[identitypb.HealthRequest]) (*connect.Response[identitypb.HealthResponse], error) {
	resp, err := a.inner.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) Register(ctx context.Context, req *connect.Request[identitypb.RegisterRequest]) (*connect.Response[identitypb.RegisterResponse], error) {
	resp, err := a.inner.Register(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) Login(ctx context.Context, req *connect.Request[identitypb.LoginRequest]) (*connect.Response[identitypb.LoginResponse], error) {
	resp, err := a.inner.Login(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) Me(ctx context.Context, req *connect.Request[identitypb.MeRequest]) (*connect.Response[identitypb.MeResponse], error) {
	resp, err := a.inner.Me(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *Connect) Logout(ctx context.Context, req *connect.Request[identitypb.LogoutRequest]) (*connect.Response[identitypb.LogoutResponse], error) {
	resp, err := a.inner.Logout(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
