// Package service implements the identity RPCs. Transport-neutral;
// see handler/ for gRPC/Connect/REST bridges and cmd/server for the
// composition root.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
	"github.com/daigo-suhara/dcloud/internal/identity/keys"
	identitypb "github.com/daigo-suhara/dcloud/internal/pb/identitypb"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const sessionDuration = 30 * 24 * time.Hour

// Server owns the DB connection and JWT signer. Both gRPC and Connect
// handlers wrap the same *Server.
type Server struct {
	DB     *sql.DB
	Signer *keys.Signer
	Logger *slog.Logger
}

type userRecord struct {
	ID           string
	Username     string
	PasswordHash string
	Email        sql.NullString
	Name         sql.NullString
	CreatedAt    string
	UpdatedAt    string
}

func New(logger *slog.Logger) (*Server, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	signer := keys.NewSigner(database)
	if err := signer.EnsureActive(context.Background()); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ensure signing key: %w", err)
	}
	return &Server{DB: database, Signer: signer, Logger: logger}, nil
}

func (s *Server) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Server) Health(context.Context, *identitypb.HealthRequest) (*identitypb.HealthResponse, error) {
	return &identitypb.HealthResponse{Status: "ok", Service: "identity", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Server) Register(ctx context.Context, req *identitypb.RegisterRequest) (*identitypb.RegisterResponse, error) {
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)
	name := strings.TrimSpace(req.Name)
	if email == "" || password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}
	if len(password) < 8 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}
	timestamp := now()
	user := userRecord{
		ID:           fmt.Sprintf("user-%s", shortID()),
		Username:     email,
		PasswordHash: string(hash),
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
	}
	if email != "" {
		user.Email = sql.NullString{String: email, Valid: true}
	}
	if name != "" {
		user.Name = sql.NullString{String: name, Valid: true}
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	created, err := insertUser(ctx, tx, user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.AlreadyExists, "username already exists")
		}
		return nil, status.Error(codes.Internal, "failed to create user")
	}
	session, err := createSession(ctx, tx, created.ID)
	if err != nil {
		s.Logger.Error("createSession failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to create session")
	}
	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "failed to persist user")
	}
	sessionProto, err := s.buildSession(created, session)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to sign session token")
	}
	return &identitypb.RegisterResponse{User: userToProto(created), Session: sessionProto}, nil
}

func (s *Server) Login(ctx context.Context, req *identitypb.LoginRequest) (*identitypb.LoginResponse, error) {
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)
	if email == "" || password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}
	user, err := s.getUserByUsername(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.Unauthenticated, "invalid email or password")
		}
		return nil, status.Error(codes.Internal, "failed to query user")
	}
	if err := verifyPassword(user.PasswordHash, password); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()
	session, err := createSession(ctx, tx, user.ID)
	if err != nil {
		s.Logger.Error("createSession failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to create session")
	}
	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "failed to persist session")
	}
	sessionProto, err := s.buildSession(user, session)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to sign session token")
	}
	return &identitypb.LoginResponse{User: userToProto(user), Session: sessionProto}, nil
}

func (s *Server) Me(ctx context.Context, req *identitypb.MeRequest) (*identitypb.MeResponse, error) {
	sessionToken := strings.TrimSpace(req.SessionToken)
	if sessionToken == "" {
		return nil, status.Error(codes.Unauthenticated, "session token is required")
	}
	user, err := s.getUserBySession(ctx, sessionToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.Unauthenticated, "session not found")
		}
		return nil, status.Error(codes.Internal, "failed to query session")
	}
	return &identitypb.MeResponse{User: userToProto(user)}, nil
}

func (s *Server) Logout(ctx context.Context, req *identitypb.LogoutRequest) (*identitypb.LogoutResponse, error) {
	sessionToken := strings.TrimSpace(req.SessionToken)
	if sessionToken == "" {
		return &identitypb.LogoutResponse{}, nil
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM identity_sessions WHERE token_hash = $1`, sessionHash(sessionToken))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete session")
	}
	return &identitypb.LogoutResponse{}, nil
}

func (s *Server) getUserByUsername(ctx context.Context, username string) (userRecord, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT id, username, password_hash, email, name, created_at, updated_at
FROM identity_users
WHERE username = $1
`, username)
	return scanUser(row)
}

func (s *Server) getUserBySession(ctx context.Context, sessionToken string) (userRecord, error) {
	row := s.DB.QueryRowContext(ctx, `
SELECT u.id, u.username, u.password_hash, u.email, u.name, u.created_at, u.updated_at
FROM identity_sessions s
JOIN identity_users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.expires_at::timestamptz > NOW()
`, sessionHash(sessionToken))
	return scanUser(row)
}

func insertUser(ctx context.Context, tx *sql.Tx, user userRecord) (userRecord, error) {
	row := tx.QueryRowContext(ctx, `
INSERT INTO identity_users (id, username, password_hash, email, name, created_at, updated_at)
VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7)
ON CONFLICT (username) DO NOTHING
RETURNING id, username, password_hash, email, name, created_at, updated_at
`, user.ID, user.Username, user.PasswordHash, nullableString(user.Email), nullableString(user.Name), user.CreatedAt, user.UpdatedAt)
	return scanUser(row)
}

type sessionRecord struct {
	Token     string
	ExpiresAt string
}

func createSession(ctx context.Context, tx *sql.Tx, userID string) (sessionRecord, error) {
	token := randomToken()
	tokenHash := sessionHash(token)
	expiresAt := time.Now().UTC().Add(sessionDuration).Format(time.RFC3339Nano)
	row := tx.QueryRowContext(ctx, `
INSERT INTO identity_sessions (token_hash, user_id, created_at, updated_at, expires_at)
VALUES ($1, $2, $3, $3, $4)
RETURNING token_hash, expires_at
`, tokenHash, userID, now(), expiresAt)
	var created sessionRecord
	if err := row.Scan(&tokenHash, &created.ExpiresAt); err != nil {
		return sessionRecord{}, err
	}
	created.Token = token
	return created, nil
}

func scanUser(row *sql.Row) (userRecord, error) {
	var user userRecord
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return userRecord{}, err
	}
	return user, nil
}

func userToProto(user userRecord) *identitypb.User {
	proto := &identitypb.User{
		Id:        user.ID,
		Username:  user.Username,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
	if user.Email.Valid {
		proto.Email = user.Email.String
	}
	if user.Name.Valid {
		proto.Name = user.Name.String
	}
	return proto
}

func (s *Server) buildSession(user userRecord, session sessionRecord) (*identitypb.Session, error) {
	claims := keys.NewClaims(user.ID, sessionHash(session.Token), sessionDuration)
	claims.Username = user.Username
	if user.Email.Valid {
		claims.Email = user.Email.String
	}
	if user.Name.Valid {
		claims.Name = user.Name.String
	}
	jwt, err := s.Signer.Sign(claims)
	if err != nil {
		return nil, err
	}
	return &identitypb.Session{Token: session.Token, ExpiresAt: session.ExpiresAt, Jwt: jwt}, nil
}

func nullableString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func randomToken() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(buf[:])
}

func sessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func verifyPassword(stored, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)); err != nil {
		return errors.New("password mismatch")
	}
	return nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func shortID() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "00000000"
	}
	return hex.EncodeToString(buf[:])
}

