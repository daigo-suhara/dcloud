// Package apihelp holds small utilities shared by every service's
// rest_adapter.go: JSON marshalling, error translation from gRPC status
// codes to HTTP codes, JSON body decoding.
package apihelp

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"google.golang.org/grpc/status"
)

func WriteJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteRPCError maps a gRPC status error to an HTTP response.
func WriteRPCError(w http.ResponseWriter, err error, fallback string) {
	if st, ok := status.FromError(err); ok {
		msg := st.Message()
		if msg == "" {
			msg = fallback
		}
		http.Error(w, msg, GRPCToHTTP(st.Code().String()))
		return
	}
	http.Error(w, fallback, http.StatusInternalServerError)
}

func GRPCToHTTP(code string) int {
	switch code {
	case "InvalidArgument":
		return http.StatusBadRequest
	case "NotFound":
		return http.StatusNotFound
	case "AlreadyExists":
		return http.StatusConflict
	case "Unauthenticated":
		return http.StatusUnauthorized
	case "PermissionDenied":
		return http.StatusForbidden
	case "FailedPrecondition":
		return http.StatusPreconditionFailed
	case "Unavailable":
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

// ReadJSONBody parses r.Body as a JSON object into a map. Returns an
// empty map for empty bodies.
func ReadJSONBody(r *http.Request) (map[string]any, error) {
	if r.Body == nil {
		return map[string]any{}, nil
	}
	defer r.Body.Close()
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if errors.Is(err, http.ErrBodyReadAfterClose) || err.Error() == "EOF" {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return body, nil
}

func StrVal(v any) string {
	s, _ := v.(string)
	return s
}

func StrOr(a, b any) string {
	if s, ok := a.(string); ok && s != "" {
		return s
	}
	s, _ := b.(string)
	return s
}

func IntVal(v any, fallback int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return fallback
		}
		if i, err := strconv.Atoi(s); err == nil {
			return i
		}
	}
	return fallback
}

func Nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func EnvOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
