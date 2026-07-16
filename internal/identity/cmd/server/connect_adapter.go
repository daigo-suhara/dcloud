package main

import (
	"context"

	"connectrpc.com/connect"
)

// connectAdapter bridges the gRPC-style identityServer methods to the
// Connect handler interface so both protocols share one implementation.
type connectAdapter struct {
	inner *identityServer
}

func (a *connectAdapter) Health(ctx context.Context, req *connect.Request[HealthRequest]) (*connect.Response[HealthResponse], error) {
	resp, err := a.inner.Health(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) Register(ctx context.Context, req *connect.Request[RegisterRequest]) (*connect.Response[RegisterResponse], error) {
	resp, err := a.inner.Register(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) Login(ctx context.Context, req *connect.Request[LoginRequest]) (*connect.Response[LoginResponse], error) {
	resp, err := a.inner.Login(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) Me(ctx context.Context, req *connect.Request[MeRequest]) (*connect.Response[MeResponse], error) {
	resp, err := a.inner.Me(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (a *connectAdapter) Logout(ctx context.Context, req *connect.Request[LogoutRequest]) (*connect.Response[LogoutResponse], error) {
	resp, err := a.inner.Logout(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
