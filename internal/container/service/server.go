// Package service implements the container service's business logic.
// It is protocol-neutral: transport concerns (gRPC/Connect wrapping) live
// in the handler package, and Kubernetes API details live in
// repository/knative.
package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/daigo-suhara/dcloud/internal/container/domain"
	"github.com/daigo-suhara/dcloud/internal/container/repository/knative"
	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	containerpb "github.com/daigo-suhara/dcloud/internal/pb/containerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "container-op-" + hex.EncodeToString(buf), nil
}

// Server holds the runtime dependencies for the container service and
// implements every RPC in ContainerService. It is transport-agnostic:
// both gRPC and ConnectRPC handlers thin-wrap these methods.
type Server struct {
	Namespace string
	DB        *sql.DB
	Queries   *dbsqlc.Queries
	Knative   *knative.Manager
}

// New wires the shared PostgreSQL connection and Knative CRD client for a
// running server.
func New(namespace string) (*Server, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	km, err := knative.NewManager(namespace, publicServiceDomain())
	if err != nil {
		return nil, err
	}
	return &Server{Namespace: namespace, DB: database, Queries: dbsqlc.New(database), Knative: km}, nil
}

func (s *Server) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *Server) Health(context.Context, *containerpb.HealthRequest) (*containerpb.HealthResponse, error) {
	return &containerpb.HealthResponse{Status: "ok", Service: "container", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func publicServiceDomain() string {
	return env("DCLD_PUBLIC_SERVICE_DOMAIN", "drkatana.com")
}

func (s *Server) projectExists(ctx context.Context, userID, projectID string) (bool, error) {
	return s.Queries.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
}

func (s *Server) ListServices(ctx context.Context, req *containerpb.ListServicesRequest) (*containerpb.ListServicesResponse, error) {
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
	records, err := s.Knative.List(ctx, domain.ProjectScope{UserID: userID, ProjectID: projectID})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query containers")
	}
	dbRecords, err := s.Queries.ListContainers(ctx, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query container metadata")
	}
	customDomains := make(map[string]string, len(dbRecords))
	startupScripts := make(map[string]string, len(dbRecords))
	envVars := make(map[string][]domain.EnvVar, len(dbRecords))
	for _, r := range dbRecords {
		if r.CustomDomain.Valid {
			customDomains[r.Name] = r.CustomDomain.String
		}
		if r.StartupScript.Valid {
			startupScripts[r.Name] = r.StartupScript.String
		}
		if vars := unmarshalEnv(r.Env); len(vars) > 0 {
			envVars[r.Name] = vars
		}
	}
	items := make([]*containerpb.Service, 0, len(records))
	for _, record := range records {
		cd := customDomains[record.Name]
		url := record.URL
		domainStatus := ""
		domainStatusReason := ""
		defaultMapping := knative.ServiceResourceName(projectID, record.Name) + "." + s.Knative.PublicDomain
		if cd != "" {
			url = s.Knative.CustomURL(cd)
			domainStatus, domainStatusReason = s.Knative.GetDomainMappingStatus(ctx, projectID, cd, defaultMapping)
		}
		items = append(items, &containerpb.Service{
			Name:               record.Name,
			Image:              record.Image,
			Url:                url,
			Ready:              record.Ready,
			Reason:             record.Reason,
			CreatedAt:          record.CreatedAt,
			UpdatedAt:          record.UpdatedAt,
			Namespace:          record.Namespace,
			ProjectId:          record.ProjectID,
			Generation:         record.Generation,
			CustomDomain:       cd,
			DomainStatus:       domainStatus,
			DomainStatusReason: domainStatusReason,
			DomainCnameTarget:  defaultMapping,
			Port:               record.Port,
			MinScale:           record.MinScale,
			MaxScale:           record.MaxScale,
			StartupScript:      startupScripts[record.Name],
			Env:                internalEnvToProto(envVars[record.Name]),
		})
	}
	return &containerpb.ListServicesResponse{UserId: userID, ProjectId: projectID, Namespace: projectID, Containers: items}, nil
}

func (s *Server) DeployService(ctx context.Context, req *containerpb.DeployServiceRequest) (*containerpb.DeployServiceResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	image := strings.TrimSpace(req.Image)
	if userID == "" || projectID == "" || name == "" || image == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, name, and image are required")
	}
	if req.Port < 1 || req.Port > 65535 {
		return nil, status.Error(codes.InvalidArgument, "port must be between 1 and 65535")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	created, err := s.Knative.Deploy(ctx, domain.ProjectScope{UserID: userID, ProjectID: projectID}, domain.DeployRequest{
		Name:          name,
		Image:         image,
		Port:          req.Port,
		MinScale:      req.MinScale,
		MaxScale:      req.MaxScale,
		StartupScript: strings.TrimSpace(req.StartupScript),
		Env:           protoEnvToInternal(req.Env),
		BucketVolumes: protoBucketsToInternal(req.BucketVolumes),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deploy service: %v", err)
	}
	createdAt := created.CreatedAt
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	updatedAt := created.UpdatedAt
	if updatedAt == "" {
		updatedAt = createdAt
	}
	if _, err := s.Queries.UpsertContainer(ctx, dbsqlc.UpsertContainerParams{
		ProjectID:     projectID,
		Name:          name,
		Image:         created.Image,
		Url:           created.URL,
		Reason:        sql.NullString{},
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		Namespace:     created.Namespace,
		Port:          created.Port,
		MinScale:      created.MinScale,
		MaxScale:      created.MaxScale,
		StartupScript: sql.NullString{String: created.StartupScript, Valid: created.StartupScript != ""},
		Env:           marshalEnv(created.Env),
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to persist service")
	}
	svc := containerpb.Service{
		Name:          created.Name,
		Image:         created.Image,
		Url:           created.URL,
		Ready:         created.Ready,
		Reason:        created.Reason,
		CreatedAt:     created.CreatedAt,
		UpdatedAt:     created.UpdatedAt,
		Namespace:     created.Namespace,
		ProjectId:     created.ProjectID,
		Generation:    created.Generation,
		Port:          created.Port,
		MinScale:      created.MinScale,
		MaxScale:      created.MaxScale,
		StartupScript: created.StartupScript,
		Env:           internalEnvToProto(created.Env),
	}
	return &containerpb.DeployServiceResponse{Service: &svc}, nil
}

func (s *Server) DeleteService(ctx context.Context, req *containerpb.DeleteServiceRequest) (*containerpb.DeleteServiceResponse, error) {
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
		ResourceType: sql.NullString{String: "container", Valid: true},
		ResourceName: sql.NullString{String: name, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	dbRecord, _ := s.Queries.GetContainer(ctx, dbsqlc.GetContainerParams{ProjectID: projectID, Name: name})
	customDomain := ""
	if dbRecord.CustomDomain.Valid {
		customDomain = dbRecord.CustomDomain.String
	}
	go func() {
		bgCtx := context.Background()
		errMsg := sql.NullString{}
		newStatus := "done"
		if err := s.Knative.Delete(bgCtx, domain.ProjectScope{UserID: userID, ProjectID: projectID}, name, customDomain); err != nil {
			newStatus = "error"
			errMsg = sql.NullString{String: err.Error(), Valid: true}
		} else {
			_, _ = s.Queries.DeleteContainer(bgCtx, dbsqlc.DeleteContainerParams{ProjectID: projectID, Name: name})
		}
		_ = s.Queries.UpdateOperation(bgCtx, dbsqlc.UpdateOperationParams{
			ID:        opID,
			Status:    newStatus,
			Error:     errMsg,
			UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}()
	return &containerpb.DeleteServiceResponse{OperationId: opID}, nil
}

func (s *Server) GetOperation(ctx context.Context, req *containerpb.GetOperationRequest) (*containerpb.GetOperationResponse, error) {
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
	return &containerpb.GetOperationResponse{OperationId: op.ID, Status: op.Status, Error: errStr}, nil
}

func (s *Server) SetServiceDomain(ctx context.Context, req *containerpb.SetServiceDomainRequest) (*containerpb.SetServiceDomainResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	customDomain := strings.TrimSpace(req.CustomDomain)
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
	dbRecord, err := s.Queries.GetContainer(ctx, dbsqlc.GetContainerParams{ProjectID: projectID, Name: name})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "container not found")
		}
		return nil, status.Error(codes.Internal, "failed to query container")
	}
	prevCustomDomain := ""
	if dbRecord.CustomDomain.Valid {
		prevCustomDomain = dbRecord.CustomDomain.String
	}
	if customDomain != "" {
		if err := s.Knative.SetCustomDomain(ctx, domain.ProjectScope{UserID: userID, ProjectID: projectID}, name, customDomain); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to apply domain mapping: %v", err)
		}
	} else if prevCustomDomain != "" {
		if err := s.Knative.DeleteDomainMapping(ctx, projectID, prevCustomDomain); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to delete domain mapping: %v", err)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.Queries.UpdateContainerDomain(ctx, dbsqlc.UpdateContainerDomainParams{
		ProjectID:    projectID,
		Name:         name,
		CustomDomain: sql.NullString{String: customDomain, Valid: customDomain != ""},
		UpdatedAt:    now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to update container domain")
	}
	url := s.Knative.PublicURL(knative.ServiceResourceName(projectID, name))
	domainStatus := ""
	domainStatusReason := ""
	defaultMapping := knative.ServiceResourceName(projectID, name) + "." + s.Knative.PublicDomain
	if customDomain != "" {
		url = s.Knative.CustomURL(customDomain)
		domainStatus, domainStatusReason = s.Knative.GetDomainMappingStatus(ctx, projectID, customDomain, defaultMapping)
	}
	svc := &containerpb.Service{
		Name:               name,
		Image:              dbRecord.Image,
		Url:                url,
		Ready:              dbRecord.Ready,
		Reason:             dbRecord.Reason.String,
		CreatedAt:          dbRecord.CreatedAt,
		UpdatedAt:          now,
		Namespace:          dbRecord.Namespace,
		ProjectId:          projectID,
		Generation:         dbRecord.Generation,
		CustomDomain:       customDomain,
		DomainStatus:       domainStatus,
		DomainStatusReason: domainStatusReason,
		DomainCnameTarget:  defaultMapping,
		Port:               dbRecord.Port,
		MinScale:           dbRecord.MinScale,
		MaxScale:           dbRecord.MaxScale,
	}
	return &containerpb.SetServiceDomainResponse{Service: svc}, nil
}

// StreamServiceLogs is protocol-neutral. Both the gRPC streaming handler
// and the ConnectRPC handler forward each LogLine through the send callback,
// so the business logic here stays in one place.
func (s *Server) StreamServiceLogs(ctx context.Context, req *containerpb.GetServiceLogsRequest, send func(*containerpb.GetServiceLogsResponse) error) error {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	if userID == "" || projectID == "" || name == "" {
		return status.Error(codes.InvalidArgument, "userId, projectId, and name are required")
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return status.Error(codes.NotFound, "project not found")
	}
	if _, err := s.Queries.GetContainer(ctx, dbsqlc.GetContainerParams{ProjectID: projectID, Name: name}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return status.Error(codes.NotFound, "container not found")
		}
		return status.Error(codes.Internal, "failed to query container")
	}

	resourceName := knative.ServiceResourceName(projectID, name)
	pods, err := s.Knative.ListServingPods(ctx, projectID, resourceName)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to list pods: %v", err)
	}
	if len(pods) == 0 {
		return status.Error(codes.FailedPrecondition, "no pod is running for this service yet")
	}
	pod := pods[0]
	containerName := knative.PickUserContainerName(pod)
	if containerName == "" {
		return status.Error(codes.Internal, "could not identify user container")
	}

	body, err := s.Knative.StreamPodLogs(ctx, projectID, pod.Name, containerName, req.TailLines, req.Follow)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to open log stream: %v", err)
	}
	defer body.Close()

	emitErr := knative.ForwardLogLines(ctx, body, func(timestamp, text string) error {
		return send(&containerpb.GetServiceLogsResponse{Text: text, Timestamp: timestamp})
	})
	if emitErr != nil && !errors.Is(emitErr, context.Canceled) {
		return status.Errorf(codes.Internal, "log stream interrupted: %v", emitErr)
	}
	return nil
}

func protoEnvToInternal(vars []*containerpb.EnvVar) []domain.EnvVar {
	out := make([]domain.EnvVar, 0, len(vars))
	for _, v := range vars {
		if strings.TrimSpace(v.Name) != "" {
			out = append(out, domain.EnvVar{Name: v.Name, Value: v.Value})
		}
	}
	return out
}

func internalEnvToProto(vars []domain.EnvVar) []*containerpb.EnvVar {
	out := make([]*containerpb.EnvVar, len(vars))
	for i, v := range vars {
		out[i] = &containerpb.EnvVar{Name: v.Name, Value: v.Value}
	}
	return out
}

func protoBucketsToInternal(vs []*containerpb.BucketVolume) []domain.BucketVolume {
	out := make([]domain.BucketVolume, 0, len(vs))
	for _, v := range vs {
		if strings.TrimSpace(v.BucketName) != "" && strings.TrimSpace(v.MountPath) != "" {
			out = append(out, domain.BucketVolume{BucketName: v.BucketName, MountPath: v.MountPath})
		}
	}
	return out
}

func marshalEnv(vars []domain.EnvVar) sql.NullString {
	if len(vars) == 0 {
		return sql.NullString{}
	}
	b, _ := json.Marshal(vars)
	return sql.NullString{String: string(b), Valid: true}
}

func unmarshalEnv(s sql.NullString) []domain.EnvVar {
	if !s.Valid || s.String == "" {
		return nil
	}
	var out []domain.EnvVar
	_ = json.Unmarshal([]byte(s.String), &out)
	return out
}
