package keys

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const AlgorithmEdDSA = "EdDSA"

type KeyPair struct {
	KID         string
	Algorithm   string
	Private     ed25519.PrivateKey
	Public      ed25519.PublicKey
	ActivatedAt time.Time
}

type Signer struct {
	db      *sql.DB
	mu      sync.RWMutex
	current *KeyPair
	all     []KeyPair
}

func NewSigner(db *sql.DB) *Signer {
	return &Signer{db: db}
}

func (s *Signer) EnsureActive(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	pairs, err := loadNonRevoked(ctx, s.db)
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		pair, err := generateAndStore(ctx, s.db)
		if err != nil {
			return err
		}
		pairs = []KeyPair{pair}
	}
	s.all = pairs
	s.current = &pairs[len(pairs)-1]
	return nil
}

func (s *Signer) Current() *KeyPair {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *Signer) PublicKeys() []KeyPair {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]KeyPair, len(s.all))
	copy(out, s.all)
	return out
}

func generateAndStore(ctx context.Context, db *sql.DB) (KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return KeyPair{}, err
	}
	kid, err := randomKID()
	if err != nil {
		return KeyPair{}, err
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return KeyPair{}, err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return KeyPair{}, err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := db.ExecContext(ctx, `
INSERT INTO identity_signing_keys (kid, algorithm, private_key_pem, public_key_pem, created_at, activated_at)
VALUES ($1, $2, $3, $4, $5, $5)
`, kid, AlgorithmEdDSA, string(privPEM), string(pubPEM), now); err != nil {
		return KeyPair{}, err
	}
	activated, _ := time.Parse(time.RFC3339Nano, now)
	return KeyPair{
		KID:         kid,
		Algorithm:   AlgorithmEdDSA,
		Private:     priv,
		Public:      pub,
		ActivatedAt: activated,
	}, nil
}

func loadNonRevoked(ctx context.Context, db *sql.DB) ([]KeyPair, error) {
	rows, err := db.QueryContext(ctx, `
SELECT kid, algorithm, private_key_pem, public_key_pem, activated_at
FROM identity_signing_keys
WHERE revoked_at IS NULL
ORDER BY activated_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KeyPair
	for rows.Next() {
		var (
			kid, alg, privPEM, pubPEM, activatedAt string
		)
		if err := rows.Scan(&kid, &alg, &privPEM, &pubPEM, &activatedAt); err != nil {
			return nil, err
		}
		priv, err := parseEd25519Private(privPEM)
		if err != nil {
			return nil, fmt.Errorf("kid=%s: %w", kid, err)
		}
		pub, err := parseEd25519Public(pubPEM)
		if err != nil {
			return nil, fmt.Errorf("kid=%s: %w", kid, err)
		}
		activated, _ := time.Parse(time.RFC3339Nano, activatedAt)
		out = append(out, KeyPair{
			KID:         kid,
			Algorithm:   alg,
			Private:     priv,
			Public:      pub,
			ActivatedAt: activated,
		})
	}
	return out, rows.Err()
}

func parseEd25519Private(pemStr string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("not an ed25519 private key")
	}
	return priv, nil
}

func parseEd25519Public(pemStr string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("invalid PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("not an ed25519 public key")
	}
	return pub, nil
}

func randomKID() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

// JWKS builds an RFC 7517 JSON Web Key Set from the non-revoked keys.
type jwk struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	X   string `json:"x"`
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

func (s *Signer) JWKS() jwkSet {
	pairs := s.PublicKeys()
	keys := make([]jwk, 0, len(pairs))
	for _, p := range pairs {
		keys = append(keys, jwk{
			Kty: "OKP",
			Crv: "Ed25519",
			Kid: p.KID,
			Alg: p.Algorithm,
			Use: "sig",
			X:   base64.RawURLEncoding.EncodeToString(p.Public),
		})
	}
	return jwkSet{Keys: keys}
}

// HandleJWKS serves the JWKS document as application/json.
func (s *Signer) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	set := s.JWKS()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_ = json.NewEncoder(w).Encode(set)
}
