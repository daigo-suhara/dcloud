package jwtverify

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
)

type claimsKey struct{}

// FromContext returns the Claims stored in ctx by a middleware/interceptor.
func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(*Claims)
	return claims, ok
}

// WithClaims returns a new context that carries the claims.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

func extractBearer(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", errors.New("missing bearer token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}

// HTTPMiddleware validates the Authorization header and attaches Claims to
// the request context. On failure returns 401.
func (v *Verifier) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := extractBearer(r.Header.Get("Authorization"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		claims, err := v.Verify(r.Context(), token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
	})
}

// ConnectInterceptor validates JWT on every unary and streaming Connect RPC.
func (v *Verifier) ConnectInterceptor() connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			token, err := extractBearer(req.Header().Get("Authorization"))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			claims, err := v.Verify(ctx, token)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			}
			return next(WithClaims(ctx, claims), req)
		})
	})
}
