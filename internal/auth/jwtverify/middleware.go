package jwtverify

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
)

type claimsKey struct{}
type tokenKey struct{}

// FromContext returns the Claims stored in ctx by a middleware/interceptor.
func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(*Claims)
	return claims, ok
}

// WithClaims returns a new context that carries the claims.
func WithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

// TokenFromContext returns the raw bearer token stored in ctx by a
// middleware/interceptor. Services that need to make outbound calls on
// behalf of the caller (e.g. project's cross-service delete fan-out)
// forward this token on their outgoing Authorization headers.
func TokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(tokenKey{}).(string)
	return token, ok
}

// WithToken returns a new context that carries the raw bearer token.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey{}, token)
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
		ctx := WithClaims(r.Context(), claims)
		ctx = WithToken(ctx, token)
		next.ServeHTTP(w, r.WithContext(ctx))
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
			ctx = WithClaims(ctx, claims)
			ctx = WithToken(ctx, token)
			return next(ctx, req)
		})
	})
}
