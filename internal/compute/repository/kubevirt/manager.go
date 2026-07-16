package kubevirt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/daigo-suhara/dcloud/internal/compute/domain"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func MachineResourceName(userID, projectID, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID) + ":" + strings.TrimSpace(projectID) + ":" + strings.TrimSpace(name)))
	return "vm-" + hex.EncodeToString(sum[:8])
}


var (
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrNotFound            = errors.New("not found")
	ErrUnavailable = errors.New("kubevirt unavailable")
)

type Client struct {
	baseURL string
	client  *http.Client
	token   string
}

func NewClient() (*Client, error) {
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
	return &Client{
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

func (c *Client) List(ctx context.Context, namespace, userID, projectID string) ([]domain.MachineRecord, error) {
	selector := url.QueryEscape(fmt.Sprintf("dcloud-component=compute,dcloud-user-id=%s,dcloud-project-id=%s", userID, projectID))
	var payload kubeVMList
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/apis/kubevirt.io/v1/namespaces/%s/virtualmachines?labelSelector=%s", namespace, selector), nil, &payload); err != nil {
		return nil, err
	}
	records := make([]domain.MachineRecord, 0, len(payload.Items))
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

func (c *Client) Create(ctx context.Context, namespace string, scope domain.ProjectScope, req domain.CreateRequest) (domain.MachineRecord, error) {
	resourceName := MachineResourceName(scope.UserID, scope.ProjectID, req.Name)
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
		return domain.MachineRecord{}, err
	}
	return vmToRecord(created), nil
}

func (c *Client) Delete(ctx context.Context, namespace string, scope domain.ProjectScope, name string) error {
	resourceName := MachineResourceName(scope.UserID, scope.ProjectID, name)
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/apis/kubevirt.io/v1/namespaces/%s/virtualmachines/%s", namespace, resourceName), nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
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
			return fmt.Errorf("%w: %s", ErrInvalidArgument, message)
		case http.StatusNotFound:
			if isKubeVirtUnavailableMessage(message) {
				return fmt.Errorf("%w: %s", ErrUnavailable, message)
			}
			return fmt.Errorf("%w: %s", ErrNotFound, message)
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

func vmToRecord(item kubeVM) domain.MachineRecord {
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
	return domain.MachineRecord{
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
