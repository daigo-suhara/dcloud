// Package service implements the storage service's business logic.
// Protocol-neutral: transports live in handler/, ObjectBucketClaim CRD
// access via Rook lives in repository/rook.
package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	storagepb "github.com/daigo-suhara/dcloud/internal/pb/storagepb"
	"github.com/daigo-suhara/dcloud/internal/storage/repository/rook"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "storage-op-" + hex.EncodeToString(buf), nil
}

// Server holds runtime dependencies. Transport-agnostic.
type Server struct {
	Namespace    string
	DB           *sql.DB
	Queries      *dbsqlc.Queries
	Rook         *rook.Client
	StorageClass string
	RGWEndpoint  string
}

func New(namespace string) (*Server, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	client, err := rook.NewClient()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &Server{
		Namespace:    namespace,
		DB:           database,
		Queries:      dbsqlc.New(database),
		Rook:         client,
		StorageClass: envOr("DCLD_BUCKET_STORAGE_CLASS", "rook-ceph-delete-bucket"),
		RGWEndpoint:  envOr("DCLD_RGW_ENDPOINT", ""),
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (s *Server) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Server) Health(context.Context, *storagepb.HealthRequest) (*storagepb.HealthResponse, error) {
	return &storagepb.HealthResponse{Status: "ok", Service: "storage", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Server) projectExists(ctx context.Context, userID, projectID string) (bool, error) {
	return s.Queries.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
}

func (s *Server) ListBuckets(ctx context.Context, req *storagepb.ListBucketsRequest) (*storagepb.ListBucketsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	if userID == "" || projectID == "" {
		return nil, status.Error(codes.InvalidArgument, "userId and projectId are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	records, err := s.Rook.ListOBCs(ctx, projectID, userID, projectID)
	if err != nil {
		if errors.Is(err, rook.ErrUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to list buckets")
	}
	items := make([]*storagepb.Bucket, 0, len(records))
	for _, r := range records {
		items = append(items, &storagepb.Bucket{
			Name:      r.Name,
			Endpoint:  r.Endpoint,
			Ready:     r.Ready,
			Status:    r.Status,
			CreatedAt: r.CreatedAt,
			ProjectId: r.ProjectID,
		})
	}
	return &storagepb.ListBucketsResponse{UserId: userID, ProjectId: projectID, Buckets: items}, nil
}

func (s *Server) CreateBucket(ctx context.Context, req *storagepb.CreateBucketRequest) (*storagepb.CreateBucketResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	if !isDNSLabel(name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a DNS label")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	record, err := s.Rook.CreateOBC(ctx, projectID, userID, projectID, name, s.StorageClass)
	if err != nil {
		if errors.Is(err, rook.ErrUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, rook.ErrInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, rook.ErrAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "bucket already exists")
		}
		return nil, status.Error(codes.Internal, "failed to create bucket")
	}
	return &storagepb.CreateBucketResponse{Bucket: &storagepb.Bucket{
		Name:      record.Name,
		Endpoint:  s.RGWEndpoint,
		Ready:     record.Ready,
		Status:    record.Status,
		CreatedAt: record.CreatedAt,
		ProjectId: record.ProjectID,
	}}, nil
}

func (s *Server) DeleteBucket(ctx context.Context, req *storagepb.DeleteBucketRequest) (*storagepb.DeleteBucketResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	opID, err := newOperationID()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.Queries.CreateOperation(ctx, dbsqlc.CreateOperationParams{
		ID:           opID,
		ResourceType: sql.NullString{String: "bucket", Valid: true},
		ResourceName: sql.NullString{String: name, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	go func() {
		bgCtx := context.Background()
		resourceName := rook.BucketResourceName(userID, projectID, name)
		if err := s.Rook.DeleteOBC(bgCtx, projectID, resourceName); err != nil {
			_ = s.Queries.UpdateOperation(bgCtx, dbsqlc.UpdateOperationParams{
				ID:        opID,
				Status:    "error",
				Error:     sql.NullString{String: err.Error(), Valid: true},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}()
	return &storagepb.DeleteBucketResponse{OperationId: opID}, nil
}

func (s *Server) GetBucketCredentials(ctx context.Context, req *storagepb.GetBucketCredentialsRequest) (*storagepb.GetBucketCredentialsResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	resourceName := rook.BucketResourceName(userID, projectID, name)
	creds, err := s.Rook.GetBucketCredentials(ctx, projectID, resourceName, s.RGWEndpoint)
	if err != nil {
		if errors.Is(err, rook.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "bucket or credentials not found")
		}
		if errors.Is(err, rook.ErrUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to get bucket credentials")
	}
	return &storagepb.GetBucketCredentialsResponse{Credentials: creds}, nil
}

func (s *Server) GetOperation(ctx context.Context, req *storagepb.GetOperationRequest) (*storagepb.GetOperationResponse, error) {
	opID := strings.TrimSpace(req.OperationId)
	if opID == "" {
		return nil, status.Error(codes.InvalidArgument, "operationId is required")
	}
	op, err := s.Queries.GetOperation(ctx, opID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "operation not found")
		}
		return nil, status.Error(codes.Internal, "failed to get operation")
	}
	errStr := ""
	if op.Error.Valid {
		errStr = op.Error.String
	}
	return &storagepb.GetOperationResponse{OperationId: op.ID, Status: op.Status, Error: errStr}, nil
}

func (s *Server) reconcileDeletions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileResourceType(ctx, "bucket", func(op dbsqlc.ListPendingOperationsByResourceTypeRow) bool {
				if !op.UserID.Valid || !op.ProjectID.Valid || !op.ResourceName.Valid {
					return false
				}
				records, err := s.Rook.ListOBCs(ctx, op.ProjectID.String, op.UserID.String, op.ProjectID.String)
				if err != nil {
					return false
				}
				for _, r := range records {
					if r.Name == op.ResourceName.String {
						return false
					}
				}
				return true
			}, nil)
		}
	}
}

func (s *Server) reconcileResourceType(ctx context.Context, resourceType string, isDone func(dbsqlc.ListPendingOperationsByResourceTypeRow) bool, onDone func(dbsqlc.ListPendingOperationsByResourceTypeRow) error) {
	ops, err := s.Queries.ListPendingOperationsByResourceType(ctx, sql.NullString{String: resourceType, Valid: true})
	if err != nil || len(ops) == 0 {
		return
	}
	for _, op := range ops {
		if isDone(op) {
			if onDone != nil {
				if err := onDone(op); err != nil {
					continue
				}
			}
			_ = s.Queries.UpdateOperation(ctx, dbsqlc.UpdateOperationParams{
				ID:        op.ID,
				Status:    "done",
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}
}

// ReconcileDeletions runs the background reconciler that marks
// storage-op operations as done once the OBC has been removed from the
// cluster. Called from cmd/server/main.
func (s *Server) ReconcileDeletions(ctx context.Context) {
	s.reconcileDeletions(ctx)
}

func isDNSLabel(value string) bool {
	if value == "" || len(value) > 63 {
		return false
	}
	if value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			continue
		default:
			return false
		}
	}
	return true
}

