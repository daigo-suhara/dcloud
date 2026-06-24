package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/daigo-suhara/dcloud/internal/db"
	dbsqlc "github.com/daigo-suhara/dcloud/internal/db/sqlc"
	databasepb "github.com/daigo-suhara/dcloud/internal/pb/databasepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type HealthRequest = databasepb.HealthRequest
type HealthResponse = databasepb.HealthResponse
type Database = databasepb.Database
type ListDatabasesRequest = databasepb.ListDatabasesRequest
type ListDatabasesResponse = databasepb.ListDatabasesResponse
type CreateDatabaseRequest = databasepb.CreateDatabaseRequest
type CreateDatabaseResponse = databasepb.CreateDatabaseResponse
type DeleteDatabaseRequest = databasepb.DeleteDatabaseRequest
type DeleteDatabaseResponse = databasepb.DeleteDatabaseResponse
type GetDatabaseRequest = databasepb.GetDatabaseRequest
type GetDatabaseResponse = databasepb.GetDatabaseResponse
type GetConnectionStringRequest = databasepb.GetConnectionStringRequest
type GetConnectionStringResponse = databasepb.GetConnectionStringResponse
type GetOperationRequest = databasepb.GetOperationRequest
type GetOperationResponse = databasepb.GetOperationResponse
type DatabaseServer = databasepb.DatabaseServiceServer

// KubeBlocks uses a unified Cluster CRD for all DB types.
// clusterDefinitions maps dcloud's DB type to KubeBlocks ClusterDefinition name.
// componentNames maps dcloud's DB type to the component name inside the Cluster spec.
// defaultVersionRefs maps dcloud's DB type to KubeBlocks ClusterVersion name.
var (
	clusterDefinitions = map[string]string{
		"postgres": "postgresql",
		"mysql":    "mysql",
		"redis":    "redis",
	}
	componentNames = map[string]string{
		"postgres": "postgresql",
		"mysql":    "mysql",
		"redis":    "redis",
	}
	defaultVersionRefs = map[string]string{
		"postgres": "postgresql-16.4.0",
		"mysql":    "mysql-8.0.35",
		"redis":    "redis-7.2.4",
	}
	dbPorts = map[string]string{
		"postgres": "5432",
		"mysql":    "3306",
		"redis":    "6379",
	}
)

func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "database-op-" + hex.EncodeToString(buf), nil
}

type dbRecord struct {
	Name         string
	Type         string
	Version      string
	CPU          string
	Memory       string
	Storage      string
	Ready        bool
	Status       string
	CreatedAt    string
	ProjectID    string
	ResourceName string
}

type databaseServer struct {
	databasepb.UnimplementedDatabaseServiceServer
	namespace    string
	db           *sql.DB
	q            *dbsqlc.Queries
	kube         *kubeClient
	storageClass string
}

func newDatabaseServer(namespace string) (*databaseServer, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	kube, err := newKubeClient()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &databaseServer{
		namespace:    namespace,
		db:           database,
		q:            dbsqlc.New(database),
		kube:         kube,
		storageClass: env("DCLD_DATABASE_STORAGE_CLASS", "ceph-rbd"),
	}, nil
}

func (s *databaseServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *databaseServer) Health(context.Context, *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "database", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *databaseServer) projectExists(ctx context.Context, userID, projectID string) (bool, error) {
	return s.q.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
}

func (s *databaseServer) ListDatabases(ctx context.Context, req *ListDatabasesRequest) (*ListDatabasesResponse, error) {
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
	records, err := s.kube.listDatabases(ctx, s.namespace, userID, projectID)
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to list databases")
	}
	items := make([]*Database, 0, len(records))
	for _, r := range records {
		items = append(items, recordToProto(r))
	}
	return &ListDatabasesResponse{UserId: userID, ProjectId: projectID, Databases: items}, nil
}

func (s *databaseServer) CreateDatabase(ctx context.Context, req *CreateDatabaseRequest) (*CreateDatabaseResponse, error) {
	userID := strings.TrimSpace(req.UserId)
	projectID := strings.TrimSpace(req.ProjectId)
	name := strings.TrimSpace(req.Name)
	dbType := strings.ToLower(strings.TrimSpace(req.Type))
	version := strings.TrimSpace(req.Version)
	cpu := strings.TrimSpace(req.Cpu)
	memory := strings.TrimSpace(req.Memory)
	storageSize := strings.TrimSpace(req.Storage)

	if userID == "" || projectID == "" || name == "" || dbType == "" {
		return nil, status.Error(codes.InvalidArgument, "userId, projectId, name, and type are required")
	}
	if !isDNSLabel(name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a DNS label")
	}
	if _, ok := clusterDefinitions[dbType]; !ok {
		return nil, status.Error(codes.InvalidArgument, "type must be one of: postgres, mysql, redis")
	}
	if version == "" {
		version = defaultVersionRefs[dbType]
	}
	if cpu == "" {
		cpu = "500m"
	}
	if memory == "" {
		memory = "1Gi"
	}
	if storageSize == "" {
		storageSize = "1Gi"
	}
	exists, err := s.projectExists(ctx, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to query project")
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	record, err := s.kube.createDatabase(ctx, s.namespace, userID, projectID, name, dbType, version, cpu, memory, storageSize, s.storageClass)
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, errInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, errAlreadyExists) {
			return nil, status.Error(codes.AlreadyExists, "database already exists")
		}
		return nil, status.Error(codes.Internal, "failed to create database")
	}
	return &CreateDatabaseResponse{Database: recordToProto(record)}, nil
}

func (s *databaseServer) DeleteDatabase(ctx context.Context, req *DeleteDatabaseRequest) (*DeleteDatabaseResponse, error) {
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
	records, err := s.kube.listDatabases(ctx, s.namespace, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to find database")
	}
	var found *dbRecord
	for i := range records {
		if records[i].Name == name {
			found = &records[i]
			break
		}
	}
	if found == nil {
		return nil, status.Error(codes.NotFound, "database not found")
	}
	opID, err := newOperationID()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.q.CreateOperation(ctx, dbsqlc.CreateOperationParams{
		ID:           opID,
		ResourceType: sql.NullString{String: "database", Valid: true},
		ResourceName: sql.NullString{String: name, Valid: true},
		UserID:       sql.NullString{String: userID, Valid: true},
		ProjectID:    sql.NullString{String: projectID, Valid: true},
		CreatedAt:    now,
	}); err != nil {
		return nil, status.Error(codes.Internal, "failed to create operation")
	}
	resourceName := found.ResourceName
	go func() {
		bgCtx := context.Background()
		if err := s.kube.deleteDatabase(bgCtx, s.namespace, resourceName); err != nil {
			_ = s.q.UpdateOperation(bgCtx, dbsqlc.UpdateOperationParams{
				ID:        opID,
				Status:    "error",
				Error:     sql.NullString{String: err.Error(), Valid: true},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}()
	return &DeleteDatabaseResponse{OperationId: opID}, nil
}

func (s *databaseServer) GetDatabase(ctx context.Context, req *GetDatabaseRequest) (*GetDatabaseResponse, error) {
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
	records, err := s.kube.listDatabases(ctx, s.namespace, userID, projectID)
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to get database")
	}
	for _, r := range records {
		if r.Name == name {
			return &GetDatabaseResponse{Database: recordToProto(r)}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "database not found")
}

func (s *databaseServer) GetConnectionString(ctx context.Context, req *GetConnectionStringRequest) (*GetConnectionStringResponse, error) {
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
	records, err := s.kube.listDatabases(ctx, s.namespace, userID, projectID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to find database")
	}
	var found *dbRecord
	for i := range records {
		if records[i].Name == name {
			found = &records[i]
			break
		}
	}
	if found == nil {
		return nil, status.Error(codes.NotFound, "database not found")
	}
	connInfo, err := s.kube.getConnectionString(ctx, s.namespace, found)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return nil, status.Error(codes.FailedPrecondition, "database is not ready yet")
		}
		return nil, status.Error(codes.Internal, "failed to get connection string")
	}
	return connInfo, nil
}

func (s *databaseServer) GetOperation(ctx context.Context, req *GetOperationRequest) (*GetOperationResponse, error) {
	opID := strings.TrimSpace(req.OperationId)
	if opID == "" {
		return nil, status.Error(codes.InvalidArgument, "operationId is required")
	}
	op, err := s.q.GetOperation(ctx, opID)
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
	return &GetOperationResponse{OperationId: op.ID, Status: op.Status, Error: errStr}, nil
}

func (s *databaseServer) reconcileDeletions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileResourceType(ctx, "database", func(op dbsqlc.ListPendingOperationsByResourceTypeRow) bool {
				if !op.UserID.Valid || !op.ProjectID.Valid || !op.ResourceName.Valid {
					return false
				}
				records, err := s.kube.listDatabases(ctx, s.namespace, op.UserID.String, op.ProjectID.String)
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

func (s *databaseServer) reconcileResourceType(ctx context.Context, resourceType string, isDone func(dbsqlc.ListPendingOperationsByResourceTypeRow) bool, onDone func(dbsqlc.ListPendingOperationsByResourceTypeRow) error) {
	ops, err := s.q.ListPendingOperationsByResourceType(ctx, sql.NullString{String: resourceType, Valid: true})
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
			_ = s.q.UpdateOperation(ctx, dbsqlc.UpdateOperationParams{
				ID:        op.ID,
				Status:    "done",
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCLD_DATABASE_ADDR", ":8086")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}
	server, err := newDatabaseServer(env("DCLD_TARGET_NAMESPACE", "dcloud-system"))
	if err != nil {
		logger.Error("failed to open database server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	grpcServer := grpc.NewServer()
	databasepb.RegisterDatabaseServiceServer(grpcServer, server)
	errc := make(chan error, 1)
	go func() {
		logger.Info("database grpc listening", "addr", addr)
		errc <- grpcServer.Serve(lis)
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go server.reconcileDeletions(ctx)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigc:
		cancel()
		grpcServer.GracefulStop()
	case err := <-errc:
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

func dbResourceName(userID, projectID, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID) + ":" + strings.TrimSpace(projectID) + ":" + strings.TrimSpace(name)))
	return "db-" + hex.EncodeToString(sum[:8])
}

func recordToProto(r dbRecord) *Database {
	return &Database{
		Name:      r.Name,
		Type:      r.Type,
		Version:   r.Version,
		Cpu:       r.CPU,
		Memory:    r.Memory,
		Storage:   r.Storage,
		Ready:     r.Ready,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
		ProjectId: r.ProjectID,
	}
}

var (
	errInvalidArgument = errors.New("invalid argument")
	errNotFound        = errors.New("not found")
	errAlreadyExists   = errors.New("already exists")
	errUnavailable     = errors.New("unavailable")
)

type kubeClient struct {
	baseURL string
	client  *http.Client
	token   string
}

func newKubeClient() (*kubeClient, error) {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return nil, err
	}
	caCert, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to load kubernetes ca")
	}
	baseURL := strings.TrimRight(env("DCLD_KUBERNETES_API_URL", "https://kubernetes.default.svc"), "/")
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool},
	}
	return &kubeClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		token: strings.TrimSpace(string(token)),
	}, nil
}

// kubeCluster represents a KubeBlocks Cluster CRD (apps.kubeblocks.io/v1alpha1).
type kubeClusterList struct {
	Items []kubeCluster `json:"items"`
}

type kubeCluster struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		CreationTimestamp string            `json:"creationTimestamp,omitempty"`
	} `json:"metadata"`
	Status struct {
		// Phase: Creating, Running, Updating, Stopping, Stopped, Deleting, Failed
		Phase string `json:"phase"`
	} `json:"status"`
}

type kubeSecret struct {
	Data map[string]string `json:"data"`
}

type kubeStatus struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code    int    `json:"code"`
}

// listDatabases queries KubeBlocks Cluster CRDs with dcloud labels.
// All DB types share the same clusters resource, so a single API call suffices.
func (c *kubeClient) listDatabases(ctx context.Context, namespace, userID, projectID string) ([]dbRecord, error) {
	selector := url.QueryEscape(fmt.Sprintf("dcloud-component=database,dcloud-user-id=%s,dcloud-project-id=%s", userID, projectID))
	var payload kubeClusterList
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/apis/apps.kubeblocks.io/v1alpha1/namespaces/%s/clusters?labelSelector=%s", namespace, selector), nil, &payload); err != nil {
		return nil, err
	}
	records := make([]dbRecord, 0, len(payload.Items))
	for _, item := range payload.Items {
		records = append(records, clusterToRecord(item))
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt == records[j].CreatedAt {
			return records[i].Name < records[j].Name
		}
		return records[i].CreatedAt < records[j].CreatedAt
	})
	return records, nil
}

func (c *kubeClient) createDatabase(ctx context.Context, namespace, userID, projectID, name, dbType, version, cpu, memory, storageSize, storageClass string) (dbRecord, error) {
	resourceName := dbResourceName(userID, projectID, name)
	clusterDef := clusterDefinitions[dbType]
	componentName := componentNames[dbType]
	payload := map[string]any{
		"apiVersion": "apps.kubeblocks.io/v1alpha1",
		"kind":       "Cluster",
		"metadata": map[string]any{
			"name":      resourceName,
			"namespace": namespace,
			"labels": map[string]string{
				"dcloud-component":       "database",
				"dcloud-user-id":         userID,
				"dcloud-project-id":      projectID,
				"dcloud-display-name":    name,
				"dcloud-db-type":         dbType,
				"app.kubernetes.io/name": "dcloud",
			},
			"annotations": map[string]string{
				"dcloud/name":    name,
				"dcloud/type":    dbType,
				"dcloud/version": version,
				"dcloud/cpu":     cpu,
				"dcloud/memory":  memory,
				"dcloud/storage": storageSize,
			},
		},
		"spec": map[string]any{
			"clusterDefinitionRef": clusterDef,
			"clusterVersionRef":    version,
			"terminationPolicy":    "WipeOut",
			"componentSpecs": []map[string]any{
				{
					"name":             componentName,
					"componentDefRef":  componentName,
					"replicas":         1,
					"resources": map[string]any{
						"requests": map[string]string{"cpu": cpu, "memory": memory},
						"limits":   map[string]string{"cpu": cpu, "memory": memory},
					},
					"volumeClaimTemplates": []map[string]any{
						{
							"name": "data",
							"spec": map[string]any{
								"storageClassName": storageClass,
								"accessModes":      []string{"ReadWriteOnce"},
								"resources": map[string]any{
									"requests": map[string]string{"storage": storageSize},
								},
							},
						},
					},
				},
			},
		},
	}
	var created kubeCluster
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/apis/apps.kubeblocks.io/v1alpha1/namespaces/%s/clusters", namespace), payload, &created); err != nil {
		return dbRecord{}, err
	}
	record := clusterToRecord(created)
	record.CPU = cpu
	record.Memory = memory
	record.Storage = storageSize
	return record, nil
}

func (c *kubeClient) deleteDatabase(ctx context.Context, namespace, resourceName string) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/apis/apps.kubeblocks.io/v1alpha1/namespaces/%s/clusters/%s", namespace, resourceName), nil, nil)
}

// getConnectionString reads the KubeBlocks-generated conn-credential Secret.
// KubeBlocks creates "{cluster-name}-conn-credential" with keys: username, password, host, port, endpoint.
func (c *kubeClient) getConnectionString(ctx context.Context, namespace string, r *dbRecord) (*GetConnectionStringResponse, error) {
	secretName := r.ResourceName + "-conn-credential"
	var secret kubeSecret
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", namespace, secretName), nil, &secret); err != nil {
		return nil, err
	}
	username := decodeBase64(secret.Data["username"])
	password := decodeBase64(secret.Data["password"])
	host := decodeBase64(secret.Data["host"])
	port := decodeBase64(secret.Data["port"])
	if host == "" {
		host = fmt.Sprintf("%s-%s.%s.svc.cluster.local", r.ResourceName, componentNames[r.Type], namespace)
	}
	if port == "" {
		port = dbPorts[r.Type]
	}
	dbName := r.ResourceName
	var connStr string
	switch r.Type {
	case "postgres":
		connStr = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port, dbName)
	case "mysql":
		connStr = fmt.Sprintf("mysql://%s:%s@%s:%s/%s", username, password, host, port, dbName)
	case "redis":
		connStr = fmt.Sprintf("redis://:%s@%s:%s", password, host, port)
		username = ""
		dbName = ""
	}
	return &GetConnectionStringResponse{
		ConnectionString: connStr,
		Host:             host,
		Port:             port,
		Username:         username,
		Password:         password,
		DatabaseName:     dbName,
	}, nil
}

func decodeBase64(s string) string {
	if s == "" {
		return ""
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return strings.TrimSpace(string(b))
}

func (c *kubeClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := kubeErrorMessage(raw)
		switch res.StatusCode {
		case http.StatusBadRequest, http.StatusUnprocessableEntity:
			return fmt.Errorf("%w: %s", errInvalidArgument, message)
		case http.StatusConflict:
			return fmt.Errorf("%w: %s", errAlreadyExists, message)
		case http.StatusNotFound:
			if isUnavailableMessage(message) {
				return fmt.Errorf("%w: %s", errUnavailable, message)
			}
			return fmt.Errorf("%w: %s", errNotFound, message)
		default:
			return fmt.Errorf("%s", message)
		}
	}
	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func kubeErrorMessage(raw []byte) string {
	var payload kubeStatus
	if err := json.Unmarshal(raw, &payload); err == nil {
		if payload.Message != "" {
			return payload.Message
		}
		if payload.Reason != "" {
			return payload.Reason
		}
	}
	text := strings.TrimSpace(string(raw))
	if text != "" {
		return text
	}
	return "kubernetes api error"
}

func isUnavailableMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "could not find the requested resource") ||
		strings.Contains(message, "no matches for kind")
}

func clusterToRecord(item kubeCluster) dbRecord {
	annotations := item.Metadata.Annotations
	labels := item.Metadata.Labels
	name := ""
	dbType := ""
	version := ""
	cpu := ""
	memory := ""
	storageSize := ""
	if annotations != nil {
		name = strings.TrimSpace(annotations["dcloud/name"])
		dbType = strings.TrimSpace(annotations["dcloud/type"])
		version = strings.TrimSpace(annotations["dcloud/version"])
		cpu = strings.TrimSpace(annotations["dcloud/cpu"])
		memory = strings.TrimSpace(annotations["dcloud/memory"])
		storageSize = strings.TrimSpace(annotations["dcloud/storage"])
	}
	if name == "" {
		name = item.Metadata.Name
	}
	if dbType == "" && labels != nil {
		dbType = labels["dcloud-db-type"]
	}
	phase := strings.TrimSpace(item.Status.Phase)
	ready := strings.EqualFold(phase, "Running")
	if phase == "" {
		phase = "Creating"
	}
	return dbRecord{
		Name:         name,
		Type:         dbType,
		Version:      version,
		CPU:          cpu,
		Memory:       memory,
		Storage:      storageSize,
		Ready:        ready,
		Status:       phase,
		CreatedAt:    item.Metadata.CreationTimestamp,
		ProjectID:    labels["dcloud-project-id"],
		ResourceName: item.Metadata.Name,
	}
}
