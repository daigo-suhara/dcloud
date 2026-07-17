// Package service implements the compute service's business logic.
// Protocol-neutral: transports live in handler/, KubeVirt CRD access
// lives in repository/kubevirt.
package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/daigo-suhara/dcloud/internal/compute/domain"
	"github.com/daigo-suhara/dcloud/internal/compute/repository/kubevirt"
	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	computepb "github.com/daigo-suhara/dcloud/internal/pb/computepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "compute-op-" + hex.EncodeToString(buf), nil
}

// Server owns the runtime dependencies for the compute service and
// implements every RPC in ComputeService. Transport-agnostic.
type Server struct {
	Namespace string
	DB        *sql.DB
	Queries   *dbsqlc.Queries
	KubeVirt  *kubevirt.Client
}

func New(namespace string) (*Server, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	client, err := kubevirt.NewClient()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &Server{Namespace: namespace, DB: database, Queries: dbsqlc.New(database), KubeVirt: client}, nil
}

func (s *Server) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Server) Health(context.Context, *computepb.HealthRequest) (*computepb.HealthResponse, error) {
	return &computepb.HealthResponse{Status: "ok", Service: "compute", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *Server) projectExists(ctx context.Context, userID, projectID string) (bool, error) {
	return s.Queries.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
}

// LookupMachine finds a VM by (user, project, name). Exposed for the
// VM console WebSocket handler, which needs to validate ownership
// before proxying to Kubernetes.
func (s *Server) LookupMachine(ctx context.Context, userID, projectID, name string) (domain.MachineRecord, error) {
	if userID == "" || projectID == "" || name == "" {
		return domain.MachineRecord{}, status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return domain.MachineRecord{}, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return domain.MachineRecord{}, status.Error(codes.NotFound, "project not found")
	}
	records, err := s.KubeVirt.List(ctx, projectID, userID, projectID)
	if err != nil {
		return domain.MachineRecord{}, status.Error(codes.Internal, "failed to query virtual machines")
	}
	for _, r := range records {
		if r.Name == name {
			return r, nil
		}
	}
	return domain.MachineRecord{}, status.Error(codes.NotFound, "virtual machine not found")
}

func (s *Server) ListMachines(ctx context.Context, req *computepb.ListMachinesRequest) (*computepb.ListMachinesResponse, error) {
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
	records, err := s.KubeVirt.List(ctx, projectID, userID, projectID)
	if err != nil {
		if errors.Is(err, kubevirt.ErrUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, kubevirt.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "virtual machines not found")
		}
		if errors.Is(err, kubevirt.ErrInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to query virtual machines")
	}
	items := make([]*computepb.Machine, 0, len(records))
	for _, record := range records {
		items = append(items, &computepb.Machine{
			Name:       record.Name,
			Image:      record.Image,
			Cpu:        record.CPU,
			Memory:     record.Memory,
			Ready:      record.Ready,
			Status:     record.Status,
			Reason:     record.Reason,
			CreatedAt:  record.CreatedAt,
			UpdatedAt:  record.UpdatedAt,
			Namespace:  record.Namespace,
			ProjectId:  record.ProjectID,
			Generation: record.Generation,
		})
	}
	return &computepb.ListMachinesResponse{UserId: userID, ProjectId: projectID, Namespace: projectID, Machines: items}, nil
}

func (s *Server) CreateMachine(ctx context.Context, req *computepb.CreateMachineRequest) (*computepb.CreateMachineResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	image := strings.TrimSpace(req.Image)
	cpu := strings.TrimSpace(req.Cpu)
	memory := strings.TrimSpace(req.Memory)
	if userID == "" || projectID == "" || name == "" || image == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, name, and image are required")
	}
	if !isDNSLabel(name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a DNS label")
	}
	if cpu == "" {
		cpu = "1"
	}
	if memory == "" {
		memory = "1Gi"
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	created, err := s.KubeVirt.Create(ctx, projectID, domain.ProjectScope{UserID: userID, ProjectID: projectID}, domain.CreateRequest{
		Name:   name,
		Image:  image,
		CPU:    cpu,
		Memory: memory,
	})
	if err != nil {
		if errors.Is(err, kubevirt.ErrUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, kubevirt.ErrInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, kubevirt.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to create virtual machine")
	}
	machine := &computepb.Machine{
		Name:       created.Name,
		Image:      created.Image,
		Cpu:        created.CPU,
		Memory:     created.Memory,
		Ready:      created.Ready,
		Status:     created.Status,
		Reason:     created.Reason,
		CreatedAt:  created.CreatedAt,
		UpdatedAt:  created.UpdatedAt,
		Namespace:  created.Namespace,
		ProjectId:  created.ProjectID,
		Generation: created.Generation,
	}
	return &computepb.CreateMachineResponse{Machine: machine}, nil
}

func (s *Server) DeleteMachine(ctx context.Context, req *computepb.DeleteMachineRequest) (*computepb.DeleteMachineResponse, error) {
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
		ResourceType: sql.NullString{String: "vm", Valid: true},
		ResourceName: sql.NullString{String: name, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	go func() {
		bgCtx := context.Background()
		if err := s.KubeVirt.Delete(bgCtx, projectID, domain.ProjectScope{UserID: userID, ProjectID: projectID}, name); err != nil {
			_ = s.Queries.UpdateOperation(bgCtx, dbsqlc.UpdateOperationParams{
				ID:        opID,
				Status:    "error",
				Error:     sql.NullString{String: err.Error(), Valid: true},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}()
	return &computepb.DeleteMachineResponse{OperationId: opID}, nil
}

func (s *Server) GetOperation(ctx context.Context, req *computepb.GetOperationRequest) (*computepb.GetOperationResponse, error) {
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
	return &computepb.GetOperationResponse{OperationId: op.ID, Status: op.Status, Error: errStr}, nil
}

func (s *Server) reconcileDeletions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileResourceType(ctx, "vm", func(op dbsqlc.ListPendingOperationsByResourceTypeRow) bool {
				if !op.UserID.Valid || !op.ProjectID.Valid || !op.ResourceName.Valid {
					return false
				}
				records, err := s.KubeVirt.List(ctx, op.ProjectID.String, op.UserID.String, op.ProjectID.String)
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
			s.reconcileResourceType(ctx, "project", func(op dbsqlc.ListPendingOperationsByResourceTypeRow) bool {
				if !op.UserID.Valid || !op.ProjectID.Valid {
					return false
				}
				records, err := s.KubeVirt.List(ctx, op.ProjectID.String, op.UserID.String, op.ProjectID.String)
				if err != nil {
					return false
				}
				if len(records) > 0 {
					return false
				}
				containers, err := s.Queries.ListContainers(ctx, op.ProjectID.String)
				if err != nil {
					return false
				}
				return len(containers) == 0
			}, func(op dbsqlc.ListPendingOperationsByResourceTypeRow) error {
				if !op.UserID.Valid || !op.ProjectID.Valid {
					return fmt.Errorf("missing user/project on operation %s", op.ID)
				}
				_, err := s.Queries.DeleteProject(ctx, dbsqlc.DeleteProjectParams{
					UserID: op.UserID.String,
					ID:     op.ProjectID.String,
				})
				return err
			})
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

// ReconcileDeletions runs in the background and marks compute-op
// operations as done once the corresponding VirtualMachine has been
// deleted from the cluster. Called from cmd/server/main.
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
