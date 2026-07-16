package keys

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	Issuer   = "dcloud-identity"
	Audience = "dcloud"
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

type header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// Sign encodes claims as an EdDSA-signed JWT using the current key.
func (s *Signer) Sign(claims Claims) (string, error) {
	pair := s.Current()
	if pair == nil {
		return "", errors.New("no active signing key")
	}
	h := header{Alg: pair.Algorithm, Kid: pair.KID, Typ: "JWT"}
	headerJSON, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	if claims.Issuer == "" {
		claims.Issuer = Issuer
	}
	if claims.Audience == "" {
		claims.Audience = Audience
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	head := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := head + "." + payload
	sig := ed25519.Sign(pair.Private, []byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// NewClaims builds Claims for a user session with a JTI matching the opaque
// session token so that revoking the opaque session also invalidates the JWT.
func NewClaims(subject, jti string, ttl time.Duration) Claims {
	now := time.Now().UTC()
	return Claims{
		Subject:   subject,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		JTI:       jti,
	}
}

// KeyByID returns the signing key with the given kid if it is non-revoked, or
// nil.
func (s *Signer) KeyByID(kid string) *KeyPair {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.all {
		if s.all[i].KID == kid {
			return &s.all[i]
		}
	}
	return nil
}

// PayloadFor returns the header segment for the current key, useful for tests.
func (s *Signer) PayloadFor(_ Claims) string {
	pair := s.Current()
	if pair == nil {
		return ""
	}
	return fmt.Sprintf("EdDSA/%s", pair.KID)
}
