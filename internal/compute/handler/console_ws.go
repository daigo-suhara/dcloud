package handler

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/coder/websocket"
	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
	"github.com/daigo-suhara/dcloud/internal/compute/repository/kubevirt"
	"github.com/daigo-suhara/dcloud/internal/compute/service"
)

// ConsoleWSHandler proxies a browser WebSocket to KubeVirt's per-VM
// console subresource on the Kubernetes API. The route is registered at
// /api/v1/compute/{name}/console; the caller must supply a valid JWT
// either as a cookie (dcloud_session) or an Authorization: Bearer token.
type ConsoleWSHandler struct {
	svc          *service.Server
	verifier     *jwtverify.Verifier
	kubeCA       *x509.CertPool
	saToken      string
	kubeBaseURL  string
	cookieName   string
	subprotocols []string
}

func NewConsoleWSHandler(svc *service.Server, verifier *jwtverify.Verifier) (*ConsoleWSHandler, error) {
	tokenPath := envOrDefault("DCLD_KUBERNETES_TOKEN_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token")
	caPath := envOrDefault("DCLD_KUBERNETES_CA_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("read SA token: %w", err)
	}
	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read k8s ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, errors.New("failed to load kubernetes CA")
	}
	return &ConsoleWSHandler{
		svc:         svc,
		verifier:    verifier,
		kubeCA:      pool,
		saToken:     strings.TrimSpace(string(tokenBytes)),
		kubeBaseURL: strings.TrimRight(envOrDefault("DCLD_KUBERNETES_API_URL", "https://kubernetes.default.svc"), "/"),
		cookieName:  envOrDefault("DCLD_SESSION_COOKIE_NAME", "dcloud_session"),
		// KubeVirt's console subresource expects a WebSocket
		// subprotocol; plain.kubevirt.io is the raw bidirectional
		// byte stream (no channel framing). Omitting the negotiation
		// silently ignores the write direction on some virt-api
		// versions.
		subprotocols: []string{"plain.kubevirt.io"},
	}, nil
}

func (h *ConsoleWSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	if name == "" || projectID == "" {
		http.Error(w, "name and projectId are required", http.StatusBadRequest)
		return
	}
	claims, err := h.authenticate(r)
	if err != nil {
		http.Error(w, "unauthenticated: "+err.Error(), http.StatusUnauthorized)
		return
	}
	// Verify the caller owns the VM (project + name).
	if _, err := h.svc.LookupMachine(r.Context(), claims.Subject, projectID, name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	resourceName := kubevirt.MachineResourceName(claims.Subject, projectID, name)
	upstreamURL := fmt.Sprintf(
		"wss://%s/apis/subresources.kubevirt.io/v1/namespaces/%s/virtualmachineinstances/%s/console",
		strings.TrimPrefix(strings.TrimPrefix(h.kubeBaseURL, "https://"), "http://"),
		url.PathEscape(projectID),
		url.PathEscape(resourceName),
	)

	clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: h.subprotocols,
	})
	if err != nil {
		return
	}
	defer clientConn.CloseNow()

	upstreamConn, _, err := websocket.Dial(r.Context(), upstreamURL, &websocket.DialOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: h.kubeCA},
			},
		},
		HTTPHeader:   http.Header{"Authorization": []string{"Bearer " + h.saToken}},
		Subprotocols: h.subprotocols,
	})
	if err != nil {
		_ = clientConn.Close(websocket.StatusInternalError, err.Error())
		return
	}
	defer upstreamConn.CloseNow()

	proxyBidirectional(r.Context(), clientConn, upstreamConn)
}

func (h *ConsoleWSHandler) authenticate(r *http.Request) (*jwtverify.Claims, error) {
	token := ""
	if cookie, err := r.Cookie(h.cookieName); err == nil {
		token = strings.TrimSpace(cookie.Value)
	}
	if token == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		}
	}
	if token == "" {
		return nil, errors.New("missing JWT (cookie or Authorization header)")
	}
	return h.verifier.Verify(r.Context(), token)
}

func proxyBidirectional(ctx context.Context, client, upstream *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer cancel()
		for {
			typ, data, err := client.Read(ctx)
			if err != nil {
				return
			}
			if err := upstream.Write(ctx, typ, data); err != nil {
				return
			}
		}
	}()

	for {
		typ, data, err := upstream.Read(ctx)
		if err != nil {
			return
		}
		if err := client.Write(ctx, typ, data); err != nil {
			return
		}
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
