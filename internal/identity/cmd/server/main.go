package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
	identitypb "github.com/daigo-suhara/dcloud/internal/pb/identitypb"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const sessionDuration = 30 * 24 * time.Hour

type Empty = identitypb.Empty
type HealthRequest = identitypb.HealthRequest
type HealthResponse = identitypb.HealthResponse
type User = identitypb.User
type Session = identitypb.Session
type AuthResponse = identitypb.AuthResponse
type RegisterRequest = identitypb.RegisterRequest
type LoginRequest = identitypb.LoginRequest
type MeRequest = identitypb.MeRequest
type MeResponse = identitypb.MeResponse
type LogoutRequest = identitypb.LogoutRequest
type LogoutResponse = identitypb.LogoutResponse
type IdentityServer = identitypb.IdentityServiceServer

type identityServer struct {
	identitypb.UnimplementedIdentityServiceServer
	db *sql.DB
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

func newIdentityServer() (*identityServer, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	return &identityServer{db: database}, nil
}

func (s *identityServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *identityServer) Health(context.Context, *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "identity", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *identityServer) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	email := strings.TrimSpace(req.Email)
	name := strings.TrimSpace(req.Name)
	if username == "" || password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
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
		Username:     username,
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

	tx, err := s.db.BeginTx(ctx, nil)
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
		return nil, status.Error(codes.Internal, "failed to create session")
	}
	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "failed to persist user")
	}
	return &AuthResponse{User: userToProto(created), Session: sessionToProto(session)}, nil
}

func (s *identityServer) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return nil, status.Error(codes.InvalidArgument, "username and password are required")
	}
	user, err := s.getUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.Unauthenticated, "invalid username or password")
		}
		return nil, status.Error(codes.Internal, "failed to query user")
	}
	if err := verifyPassword(user.PasswordHash, password); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid username or password")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()
	session, err := createSession(ctx, tx, user.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create session")
	}
	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, "failed to persist session")
	}
	return &AuthResponse{User: userToProto(user), Session: sessionToProto(session)}, nil
}

func (s *identityServer) Me(ctx context.Context, req *MeRequest) (*MeResponse, error) {
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
	return &MeResponse{User: userToProto(user)}, nil
}

func (s *identityServer) Logout(ctx context.Context, req *LogoutRequest) (*LogoutResponse, error) {
	sessionToken := strings.TrimSpace(req.SessionToken)
	if sessionToken == "" {
		return &LogoutResponse{}, nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM identity_sessions WHERE token_hash = $1`, sessionHash(sessionToken))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to delete session")
	}
	return &LogoutResponse{}, nil
}

func (s *identityServer) getUserByUsername(ctx context.Context, username string) (userRecord, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, username, password_hash, email, name, created_at, updated_at
FROM identity_users
WHERE username = $1
`, username)
	return scanUser(row)
}

func (s *identityServer) getUserBySession(ctx context.Context, sessionToken string) (userRecord, error) {
	row := s.db.QueryRowContext(ctx, `
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

func userToProto(user userRecord) *User {
	proto := &User{
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

func sessionToProto(session sessionRecord) *Session {
	return &Session{Token: session.Token, ExpiresAt: session.ExpiresAt}
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

func RegisterIdentityServer(server *grpc.Server, impl IdentityServer) {
	identitypb.RegisterIdentityServiceServer(server, impl)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCLD_IDENTITY_ADDR", ":8083")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}
	server, err := newIdentityServer()
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	grpcServer := grpc.NewServer()
	RegisterIdentityServer(grpcServer, server)
	errC := make(chan error, 1)
	go func() {
		logger.Info("identity grpc listening", "addr", addr)
		errC <- grpcServer.Serve(lis)
	}()
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigC:
		grpcServer.GracefulStop()
	case err := <-errC:
		if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
