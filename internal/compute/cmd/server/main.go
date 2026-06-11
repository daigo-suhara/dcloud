package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
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
	computepb "github.com/daigo-suhara/dcloud/internal/pb/computepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Empty = computepb.Empty
type HealthRequest = computepb.HealthRequest
type HealthResponse = computepb.HealthResponse
type Machine = computepb.Machine
type ListMachinesRequest = computepb.ListMachinesRequest
type ListMachinesResponse = computepb.ListMachinesResponse
type CreateMachineRequest = computepb.CreateMachineRequest
type CreateMachineResponse = computepb.CreateMachineResponse
type DeleteMachineRequest = computepb.DeleteMachineRequest
type DeleteMachineResponse = computepb.DeleteMachineResponse
type GetOperationRequest = computepb.GetOperationRequest
type GetOperationResponse = computepb.GetOperationResponse
type ComputeServer = computepb.ComputeServiceServer

func newOperationID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "compute-op-" + hex.EncodeToString(buf), nil
}

type projectScope struct {
	UserID    string
	ProjectID string
}

type createRequest struct {
	Name   string
	Image  string
	CPU    string
	Memory string
}

type machineRecord struct {
	Name       string
	Image      string
	CPU        string
	Memory     string
	Ready      bool
	Status     string
	Reason     string
	CreatedAt  string
	UpdatedAt  string
	Namespace  string
	ProjectID  string
	Generation int64
}

type computeServer struct {
	computepb.UnimplementedComputeServiceServer
	namespace string
	db        *sql.DB
	q         *dbsqlc.Queries
	kubevirt  *kubevirtClient
}

func newComputeServer(namespace string) (*computeServer, error) {
	database, err := db.Open()
	if err != nil {
		return nil, err
	}
	kubevirt, err := newKubeVirtClient()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	return &computeServer{namespace: namespace, db: database, q: dbsqlc.New(database), kubevirt: kubevirt}, nil
}

func (s *computeServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *computeServer) Health(context.Context, *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{Status: "ok", Service: "compute", Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func (s *computeServer) projectExists(ctx context.Context, userID, projectID string) (bool, error) {
	return s.q.ProjectExists(ctx, dbsqlc.ProjectExistsParams{UserID: userID, ID: projectID})
}

func (s *computeServer) ListMachines(ctx context.Context, req *ListMachinesRequest) (*ListMachinesResponse, error) {
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
	records, err := s.kubevirt.list(ctx, s.namespace, userID, projectID)
	if err != nil {
		if errors.Is(err, errKubeVirtUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, errNotFound) {
			return nil, status.Error(codes.NotFound, "virtual machines not found")
		}
		if errors.Is(err, errInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to query virtual machines")
	}
	items := make([]*Machine, 0, len(records))
	for _, record := range records {
		items = append(items, &Machine{
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
	return &ListMachinesResponse{UserId: userID, ProjectId: projectID, Namespace: s.namespace, Machines: items}, nil
}

func (s *computeServer) CreateMachine(ctx context.Context, req *CreateMachineRequest) (*CreateMachineResponse, error) {
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
	created, err := s.kubevirt.create(ctx, s.namespace, projectScope{UserID: userID, ProjectID: projectID}, createRequest{
		Name:   name,
		Image:  image,
		CPU:    cpu,
		Memory: memory,
	})
	if err != nil {
		if errors.Is(err, errKubeVirtUnavailable) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		if errors.Is(err, errInvalidArgument) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if errors.Is(err, errNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to create virtual machine")
	}
	machine := &Machine{
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
	return &CreateMachineResponse{Machine: machine}, nil
}

func (s *computeServer) DeleteMachine(ctx context.Context, req *DeleteMachineRequest) (*DeleteMachineResponse, error) {
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
	if _, err := s.q.CreateOperation(ctx, dbsqlc.CreateOperationParams{
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
		if err := s.kubevirt.delete(bgCtx, s.namespace, projectScope{UserID: userID, ProjectID: projectID}, name); err != nil {
			_ = s.q.UpdateOperation(bgCtx, dbsqlc.UpdateOperationParams{
				ID:        opID,
				Status:    "error",
				Error:     sql.NullString{String: err.Error(), Valid: true},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
			})
		}
	}()
	return &DeleteMachineResponse{OperationId: opID}, nil
}

func (s *computeServer) GetOperation(ctx context.Context, req *GetOperationRequest) (*GetOperationResponse, error) {
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

func (s *computeServer) reconcileDeletions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileResourceType(ctx, "vm", func(op dbsqlc.Operation) bool {
				if !op.UserID.Valid || !op.ProjectID.Valid || !op.ResourceName.Valid {
					return false
				}
				records, err := s.kubevirt.list(ctx, s.namespace, op.UserID.String, op.ProjectID.String)
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
			s.reconcileResourceType(ctx, "project", func(op dbsqlc.Operation) bool {
				if !op.UserID.Valid || !op.ProjectID.Valid {
					return false
				}
				records, err := s.kubevirt.list(ctx, s.namespace, op.UserID.String, op.ProjectID.String)
				if err != nil {
					return false
				}
				if len(records) > 0 {
					return false
				}
				containers, err := s.q.ListContainers(ctx, op.ProjectID.String)
				if err != nil {
					return false
				}
				return len(containers) == 0
			}, func(op dbsqlc.Operation) error {
				if !op.UserID.Valid || !op.ProjectID.Valid {
					return fmt.Errorf("missing user/project on operation %s", op.ID)
				}
				_, err := s.q.DeleteProject(ctx, dbsqlc.DeleteProjectParams{
					UserID: op.UserID.String,
					ID:     op.ProjectID.String,
				})
				return err
			})
		}
	}
}

func (s *computeServer) reconcileResourceType(ctx context.Context, resourceType string, isDone func(dbsqlc.Operation) bool, onDone func(dbsqlc.Operation) error) {
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

func RegisterComputeServer(server *grpc.Server, impl ComputeServer) {
	computepb.RegisterComputeServiceServer(server, impl)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("DCP_COMPUTE_ADDR", ":8084")
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}
	server, err := newComputeServer(env("DCLD_TARGET_NAMESPACE", "dcloud-system"))
	if err != nil {
		logger.Error("failed to open compute server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	grpcServer := grpc.NewServer()
	RegisterComputeServer(grpcServer, server)
	errc := make(chan error, 1)
	go func() {
		logger.Info("compute grpc listening", "addr", addr)
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

func machineResourceName(userID, projectID, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID) + ":" + strings.TrimSpace(projectID) + ":" + strings.TrimSpace(name)))
	return "vm-" + hex.EncodeToString(sum[:8])
}

var (
	errInvalidArgument     = errors.New("invalid argument")
	errNotFound            = errors.New("not found")
	errKubeVirtUnavailable = errors.New("kubevirt unavailable")
)

type kubevirtClient struct {
	baseURL string
	client  *http.Client
	token   string
}

func newKubeVirtClient() (*kubevirtClient, error) {
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
	return &kubevirtClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		token: strings.TrimSpace(string(token)),
	}, nil
}

type kubeVMList struct {
	Items []kubeVM `json:"items"`
}

type kubeVM struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		CreationTimestamp string            `json:"creationTimestamp,omitempty"`
		Generation        int64             `json:"generation,omitempty"`
	} `json:"metadata"`
	Spec struct {
		Running  bool `json:"running"`
		Template struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Domain struct {
					Resources struct {
						Requests map[string]string `json:"requests"`
						Limits   map[string]string `json:"limits,omitempty"`
					} `json:"resources"`
					Devices struct {
						Disks []struct {
							Name string `json:"name"`
							Disk struct {
								Bus string `json:"bus,omitempty"`
							} `json:"disk"`
						} `json:"disks"`
						Interfaces []struct {
							Name       string   `json:"name"`
							Masquerade struct{} `json:"masquerade,omitempty"`
						} `json:"interfaces"`
					} `json:"devices"`
				} `json:"domain"`
				Networks []struct {
					Name string   `json:"name"`
					Pod  struct{} `json:"pod,omitempty"`
				} `json:"networks"`
				Volumes []struct {
					Name          string `json:"name"`
					ContainerDisk *struct {
						Image string `json:"image"`
					} `json:"containerDisk,omitempty"`
				} `json:"volumes"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		Ready           bool   `json:"ready"`
		PrintableStatus string `json:"printableStatus"`
	} `json:"status"`
}

type kubeVMCreate struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Metadata   struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace,omitempty"`
		Labels      map[string]string `json:"labels,omitempty"`
		Annotations map[string]string `json:"annotations,omitempty"`
	} `json:"metadata"`
	Spec struct {
		Running  bool `json:"running"`
		Template struct {
			Metadata struct {
				Labels map[string]string `json:"labels,omitempty"`
			} `json:"metadata"`
			Spec struct {
				Domain struct {
					Resources struct {
						Requests map[string]string `json:"requests"`
						Limits   map[string]string `json:"limits,omitempty"`
					} `json:"resources"`
					Devices struct {
						Disks []struct {
							Name string `json:"name"`
							Disk struct {
								Bus string `json:"bus,omitempty"`
							} `json:"disk"`
						} `json:"disks"`
						Interfaces []struct {
							Name       string   `json:"name"`
							Masquerade struct{} `json:"masquerade,omitempty"`
						} `json:"interfaces"`
					} `json:"devices"`
				} `json:"domain"`
				Networks []struct {
					Name string   `json:"name"`
					Pod  struct{} `json:"pod,omitempty"`
				} `json:"networks"`
				Volumes []struct {
					Name          string `json:"name"`
					ContainerDisk *struct {
						Image string `json:"image"`
					} `json:"containerDisk,omitempty"`
				} `json:"volumes"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type kubeStatus struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code    int    `json:"code"`
}

func (c *kubevirtClient) list(ctx context.Context, namespace, userID, projectID string) ([]machineRecord, error) {
	selector := url.QueryEscape(fmt.Sprintf("dcloud-component=compute,dcloud-user-id=%s,dcloud-project-id=%s", userID, projectID))
	var payload kubeVMList
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/apis/kubevirt.io/v1/namespaces/%s/virtualmachines?labelSelector=%s", namespace, selector), nil, &payload); err != nil {
		return nil, err
	}
	records := make([]machineRecord, 0, len(payload.Items))
	for _, item := range payload.Items {
		records = append(records, vmToRecord(item))
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt == records[j].CreatedAt {
			return records[i].Name < records[j].Name
		}
		return records[i].CreatedAt < records[j].CreatedAt
	})
	return records, nil
}

func (c *kubevirtClient) create(ctx context.Context, namespace string, scope projectScope, req createRequest) (machineRecord, error) {
	resourceName := machineResourceName(scope.UserID, scope.ProjectID, req.Name)
	payload := kubeVMCreate{
		APIVersion: "kubevirt.io/v1",
		Kind:       "VirtualMachine",
	}
	payload.Metadata.Name = resourceName
	payload.Metadata.Namespace = namespace
	payload.Metadata.Labels = map[string]string{
		"dcloud-component":       "compute",
		"dcloud-user-id":         scope.UserID,
		"dcloud-project-id":      scope.ProjectID,
		"dcloud-display-name":    req.Name,
		"app.kubernetes.io/name": "dcloud",
	}
	payload.Metadata.Annotations = map[string]string{
		"dcloud/name":   req.Name,
		"dcloud/image":  req.Image,
		"dcloud/cpu":    req.CPU,
		"dcloud/memory": req.Memory,
	}
	payload.Spec.Running = true
	payload.Spec.Template.Metadata.Labels = payload.Metadata.Labels
	payload.Spec.Template.Spec.Domain.Resources.Requests = map[string]string{
		"cpu":                     req.CPU,
		"memory":                  req.Memory,
		"devices.kubevirt.io/kvm": "1",
	}
	payload.Spec.Template.Spec.Domain.Resources.Limits = map[string]string{
		"devices.kubevirt.io/kvm": "1",
	}
	payload.Spec.Template.Spec.Domain.Devices.Disks = []struct {
		Name string `json:"name"`
		Disk struct {
			Bus string `json:"bus,omitempty"`
		} `json:"disk"`
	}{
		{
			Name: "containerdisk",
			Disk: struct {
				Bus string `json:"bus,omitempty"`
			}{Bus: "virtio"},
		},
	}
	payload.Spec.Template.Spec.Domain.Devices.Interfaces = []struct {
		Name       string   `json:"name"`
		Masquerade struct{} `json:"masquerade,omitempty"`
	}{
		{
			Name:       "default",
			Masquerade: struct{}{},
		},
	}
	payload.Spec.Template.Spec.Networks = []struct {
		Name string   `json:"name"`
		Pod  struct{} `json:"pod,omitempty"`
	}{
		{
			Name: "default",
			Pod:  struct{}{},
		},
	}
	payload.Spec.Template.Spec.Volumes = []struct {
		Name          string `json:"name"`
		ContainerDisk *struct {
			Image string `json:"image"`
		} `json:"containerDisk,omitempty"`
	}{
		{
			Name: "containerdisk",
			ContainerDisk: &struct {
				Image string `json:"image"`
			}{Image: req.Image},
		},
	}
	var created kubeVM
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/apis/kubevirt.io/v1/namespaces/%s/virtualmachines", namespace), payload, &created); err != nil {
		return machineRecord{}, err
	}
	return vmToRecord(created), nil
}

func (c *kubevirtClient) delete(ctx context.Context, namespace string, scope projectScope, name string) error {
	resourceName := machineResourceName(scope.UserID, scope.ProjectID, name)
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/apis/kubevirt.io/v1/namespaces/%s/virtualmachines/%s", namespace, resourceName), nil, nil)
}

func (c *kubevirtClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
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
		case http.StatusNotFound:
			if isKubeVirtUnavailableMessage(message) {
				return fmt.Errorf("%w: %s", errKubeVirtUnavailable, message)
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

func isKubeVirtUnavailableMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(message, "could not find the requested resource") ||
		strings.Contains(message, "no matches for kind \"virtualmachine\"") ||
		strings.Contains(message, "no matches for kind virtualmachine")
}

func vmToRecord(item kubeVM) machineRecord {
	annotations := item.Metadata.Annotations
	labels := item.Metadata.Labels
	name := annotationValue(annotations, "dcloud/name")
	if name == "" {
		name = item.Metadata.Name
	}
	image := annotationValue(annotations, "dcloud/image")
	cpu := annotationValue(annotations, "dcloud/cpu")
	memory := annotationValue(annotations, "dcloud/memory")
	status := strings.TrimSpace(item.Status.PrintableStatus)
	ready := item.Status.Ready
	if strings.EqualFold(status, "Running") {
		ready = true
	}
	if status == "" {
		if ready {
			status = "Running"
		} else {
			status = "Provisioning"
		}
	}
	return machineRecord{
		Name:       name,
		Image:      image,
		CPU:        cpu,
		Memory:     memory,
		Ready:      ready,
		Status:     status,
		Reason:     status,
		CreatedAt:  item.Metadata.CreationTimestamp,
		UpdatedAt:  item.Metadata.CreationTimestamp,
		Namespace:  item.Metadata.Namespace,
		ProjectID:  labels["dcloud-project-id"],
		Generation: item.Metadata.Generation,
	}
}

func annotationValue(values map[string]string, key string) string {
	if values == nil {
		return ""
	}
	return strings.TrimSpace(values[key])
}
