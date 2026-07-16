package jwtverify_test

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/daigo-suhara/dcloud/internal/auth/jwtverify"
)

// jwksServer returns an httptest.Server that publishes the given
// Ed25519 public key under the given kid, matching the JWK shape that
// keys.Signer.HandleJWKS produces in production.
func jwksServer(t *testing.T, kid string, pub ed25519.PublicKey) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{
					"kty": "OKP",
					"crv": "Ed25519",
					"kid": kid,
					"alg": "EdDSA",
					"use": "sig",
					"x":   base64.RawURLEncoding.EncodeToString(pub),
				},
			},
		})
	}))
}

// mintJWT hand-builds a JWT with the exact format keys.Signer.Sign
// emits: {header}.{payload}.{signature}, EdDSA, kid header, JSON claims.
// Signing this from the test instead of importing the Signer keeps
// jwtverify testable without a live PostgreSQL for the key store.
func mintJWT(t *testing.T, priv ed25519.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]string{"alg": "EdDSA", "kid": kid, "typ": "JWT"})
	if err != nil {
		t.Fatalf("header marshal: %v", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("claims marshal: %v", err)
	}
	head := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := head + "." + payload
	sig := ed25519.Sign(priv, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func defaultClaims() map[string]any {
	now := time.Now().Unix()
	return map[string]any{
		"iss":      "dcloud-identity",
		"sub":      "user-abc",
		"aud":      "dcloud",
		"iat":      now,
		"exp":      now + 3600,
		"jti":      "test-jti",
		"email":    "test@example.com",
		"username": "test-user",
	}
}

func TestVerify_RoundTrip(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv := jwksServer(t, "kid-1", pub)
	defer srv.Close()

	token := mintJWT(t, priv, "kid-1", defaultClaims())
	v := jwtverify.New(srv.URL,
		jwtverify.WithIssuer("dcloud-identity"),
		jwtverify.WithAudience("dcloud"),
	)
	claims, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "user-abc" {
		t.Errorf("subject = %q, want user-abc", claims.Subject)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("email = %q", claims.Email)
	}
	if claims.Username != "test-user" {
		t.Errorf("username = %q", claims.Username)
	}
	if claims.JTI != "test-jti" {
		t.Errorf("jti = %q", claims.JTI)
	}
}

func TestVerify_Expired(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv := jwksServer(t, "kid-1", pub)
	defer srv.Close()

	c := defaultClaims()
	// Ten minutes ago; safely past the default 30s skew.
	c["iat"] = time.Now().Add(-11 * time.Minute).Unix()
	c["exp"] = time.Now().Add(-10 * time.Minute).Unix()

	token := mintJWT(t, priv, "kid-1", c)
	v := jwtverify.New(srv.URL)
	if _, err := v.Verify(context.Background(), token); err == nil {
		t.Fatal("expected expiry error, got nil")
	}
}

func TestVerify_SignatureMismatch(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key 1: %v", err)
	}
	// Sign with an unrelated key, so verification against pub fails.
	_, wrongPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key 2: %v", err)
	}
	srv := jwksServer(t, "kid-1", pub)
	defer srv.Close()

	token := mintJWT(t, wrongPriv, "kid-1", defaultClaims())
	v := jwtverify.New(srv.URL)
	if _, err := v.Verify(context.Background(), token); err == nil {
		t.Fatal("expected signature mismatch, got nil")
	}
}

func TestVerify_UnknownKid(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv := jwksServer(t, "kid-published", pub)
	defer srv.Close()

	token := mintJWT(t, priv, "kid-missing", defaultClaims())
	v := jwtverify.New(srv.URL)
	if _, err := v.Verify(context.Background(), token); err == nil {
		t.Fatal("expected kid-not-in-jwks error, got nil")
	}
}

func TestVerify_IssuerMismatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv := jwksServer(t, "kid-1", pub)
	defer srv.Close()

	c := defaultClaims()
	c["iss"] = "not-us"
	token := mintJWT(t, priv, "kid-1", c)

	v := jwtverify.New(srv.URL, jwtverify.WithIssuer("dcloud-identity"))
	if _, err := v.Verify(context.Background(), token); err == nil {
		t.Fatal("expected issuer mismatch, got nil")
	}
}

func TestVerify_AudienceMismatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv := jwksServer(t, "kid-1", pub)
	defer srv.Close()

	c := defaultClaims()
	c["aud"] = "different"
	token := mintJWT(t, priv, "kid-1", c)

	v := jwtverify.New(srv.URL, jwtverify.WithAudience("dcloud"))
	if _, err := v.Verify(context.Background(), token); err == nil {
		t.Fatal("expected audience mismatch, got nil")
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	srv := jwksServer(t, "kid-1", pub)
	defer srv.Close()

	v := jwtverify.New(srv.URL)
	for _, tok := range []string{
		"",
		"onlyonepart",
		"only.two.parts.here.wrong",
		"not.b64!.oops",
	} {
		if _, err := v.Verify(context.Background(), tok); err == nil {
			t.Errorf("malformed %q: expected error, got nil", tok)
		}
	}
}

func TestVerify_JWKSCache(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{
				{
					"kty": "OKP",
					"crv": "Ed25519",
					"kid": "kid-1",
					"alg": "EdDSA",
					"x":   base64.RawURLEncoding.EncodeToString(pub),
				},
			},
		})
	}))
	defer srv.Close()

	token := mintJWT(t, priv, "kid-1", defaultClaims())
	v := jwtverify.New(srv.URL, jwtverify.WithCacheTTL(time.Hour))
	for i := 0; i < 5; i++ {
		if _, err := v.Verify(context.Background(), token); err != nil {
			t.Fatalf("verify #%d: %v", i, err)
		}
	}
	if calls != 1 {
		t.Errorf("JWKS fetched %d times, expected 1 (cached)", calls)
	}
}
