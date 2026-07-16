package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	"github.com/daigo-suhara/dcloud/internal/identity/service"
	identitypb "github.com/daigo-suhara/dcloud/internal/pb/identitypb"
	"google.golang.org/grpc/status"
)

// RegisterRESTRoutes mirrors the /api/v1/auth/* routes.
func RegisterRESTRoutes(mux *http.ServeMux, server *service.Server, verifier *jwtverify.Verifier) {
	cookie := envDef("DCLD_SESSION_COOKIE_NAME", "dcloud_session")
	secure := isCookieSecure()

	mux.HandleFunc("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		claims, _, err := verifier.VerifyCookieOrBearer(r, cookie)
		if err != nil {
			http.Error(w, "ログインが必要です", http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":        claims.Subject,
			"username":  first(claims.Username, claims.Email),
			"email":     nullable(claims.Email),
			"name":      nullable(claims.Name),
			"createdAt": "",
			"updatedAt": "",
		})
	})

	mux.HandleFunc("GET /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	mux.HandleFunc("GET /api/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	})
	mux.HandleFunc("GET /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusFound)
	})

	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		body, err := readBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		email := strOr(body["email"], body["username"])
		password := strv(body["password"])
		resp, err := server.Login(r.Context(), &identitypb.LoginRequest{Email: email, Password: password})
		if err != nil {
			writeRPCError(w, err, "メールアドレスまたはパスワードが違います")
			return
		}
		setSessionCookie(w, cookie, resp.Session.Jwt, secure)
		writeJSON(w, http.StatusOK, map[string]any{"user": userDict(resp.User)})
	})

	mux.HandleFunc("POST /api/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		body, err := readBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		email := strOr(body["email"], body["username"])
		password := strv(body["password"])
		resp, err := server.Register(r.Context(), &identitypb.RegisterRequest{Email: email, Password: password})
		if err != nil {
			writeRPCError(w, err, "アカウントを作成できませんでした")
			return
		}
		setSessionCookie(w, cookie, resp.Session.Jwt, secure)
		writeJSON(w, http.StatusOK, map[string]any{"user": userDict(resp.User)})
	})

	mux.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		clearSessionCookie(w, cookie)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func setSessionCookie(w http.ResponseWriter, name, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,
	})
}

func clearSessionCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1})
}

func isCookieSecure() bool {
	v := os.Getenv("DCLD_COOKIE_SECURE")
	switch v {
	case "0", "false", "no", "off", "":
		return false
	}
	return true
}

func userDict(u *identitypb.User) map[string]any {
	return map[string]any{
		"id":        u.GetId(),
		"username":  u.GetUsername(),
		"email":     nullable(u.GetEmail()),
		"name":      nullable(u.GetName()),
		"createdAt": u.GetCreatedAt(),
		"updatedAt": u.GetUpdatedAt(),
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeRPCError(w http.ResponseWriter, err error, fallback string) {
	if st, ok := status.FromError(err); ok {
		httpCode := grpcCodeToHTTP(st.Code().String())
		msg := st.Message()
		if msg == "" {
			msg = fallback
		}
		http.Error(w, msg, httpCode)
		return
	}
	http.Error(w, fallback, http.StatusInternalServerError)
}

func grpcCodeToHTTP(code string) int {
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

func readBody(r *http.Request) (map[string]any, error) {
	if r.Body == nil {
		return map[string]any{}, nil
	}
	defer r.Body.Close()
	body := map[string]any{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, http.ErrBodyReadAfterClose) {
		if err.Error() == "EOF" {
			return map[string]any{}, nil
		}
		return nil, err
	}
	return body, nil
}

func strv(v any) string {
	s, _ := v.(string)
	return s
}

func strOr(a, b any) string {
	if s, ok := a.(string); ok && s != "" {
		return s
	}
	s, _ := b.(string)
	return s
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func envDef(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// suppress unused warning for time import (used in Cookie MaxAge implicitly)
var _ = time.Hour
