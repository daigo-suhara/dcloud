package jwtverify

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultJWKSTTL = 5 * time.Minute
	expectedAlg    = "EdDSA"
)

type Claims struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	Audience  string `json:"aud"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	JTI       string `json:"jti"`
	Username  string `json:"username,omitempty"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
}

type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	X   string `json:"x"`
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

type Verifier struct {
	jwksURL       string
	expectedIss   string
	expectedAud   string
	httpClient    *http.Client
	mu            sync.RWMutex
	cache         map[string]ed25519.PublicKey
	cacheFetched  time.Time
	ttl           time.Duration
	skewTolerance time.Duration
}

type Option func(*Verifier)

func WithHTTPClient(c *http.Client) Option {
	return func(v *Verifier) { v.httpClient = c }
}

func WithIssuer(iss string) Option {
	return func(v *Verifier) { v.expectedIss = iss }
}

func WithAudience(aud string) Option {
	return func(v *Verifier) { v.expectedAud = aud }
}

func WithCacheTTL(d time.Duration) Option {
	return func(v *Verifier) { v.ttl = d }
}

func WithClockSkew(d time.Duration) Option {
	return func(v *Verifier) { v.skewTolerance = d }
}

func New(jwksURL string, opts ...Option) *Verifier {
	v := &Verifier{
		jwksURL:       jwksURL,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		cache:         make(map[string]ed25519.PublicKey),
		ttl:           defaultJWKSTTL,
		skewTolerance: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// Verify parses and validates a JWT, returning the claims on success.
func (v *Verifier) Verify(ctx context.Context, token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed jwt")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	if header.Alg != expectedAlg {
		return nil, fmt.Errorf("unexpected alg %q", header.Alg)
	}
	if header.Kid == "" {
		return nil, errors.New("kid missing")
	}
	pub, err := v.publicKey(ctx, header.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return nil, errors.New("signature mismatch")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims Claims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}
	now := time.Now().Unix()
	if claims.ExpiresAt != 0 && now > claims.ExpiresAt+int64(v.skewTolerance.Seconds()) {
		return nil, errors.New("token expired")
	}
	if claims.IssuedAt != 0 && claims.IssuedAt > now+int64(v.skewTolerance.Seconds()) {
		return nil, errors.New("token issued in the future")
	}
	if v.expectedIss != "" && claims.Issuer != v.expectedIss {
		return nil, fmt.Errorf("issuer mismatch: %q", claims.Issuer)
	}
	if v.expectedAud != "" && claims.Audience != v.expectedAud {
		return nil, fmt.Errorf("audience mismatch: %q", claims.Audience)
	}
	return &claims, nil
}

func (v *Verifier) publicKey(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	v.mu.RLock()
	pub, ok := v.cache[kid]
	fresh := time.Since(v.cacheFetched) < v.ttl
	v.mu.RUnlock()
	if ok && fresh {
		return pub, nil
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	v.mu.RLock()
	pub, ok = v.cache[kid]
	v.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("kid %q not in JWKS", kid)
	}
	return pub, nil
}

func (v *Verifier) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var set jwkSet
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("parse JWKS: %w", err)
	}
	cache := make(map[string]ed25519.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "OKP" || k.Crv != "Ed25519" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			continue
		}
		cache[k.Kid] = ed25519.PublicKey(raw)
	}
	v.mu.Lock()
	v.cache = cache
	v.cacheFetched = time.Now()
	v.mu.Unlock()
	return nil
}
