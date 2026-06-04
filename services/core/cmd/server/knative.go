package main

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
	neturl "net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/daigo-suhara/dcp/services/core/internal/userserviceroute"
)

const (
	userServiceManagerLabel   = "dcp-core"
	internalContainerAppsName = "dcp-container-apps"
	userLabelKey              = "dcp.dev/user"
	projectLabelKey           = "dcp.dev/project"
	serviceNameLabel          = "dcp.dev/service-name"
)

var errServiceNotFound = errors.New("service not found")
var errProjectNotFound = errors.New("project not found")

type knativeServiceManager struct {
	namespace string
	client    *http.Client
	baseURL   string
	token     string
}

func newServiceManager(namespace string) (serviceManager, error) {
	baseURL := fmt.Sprintf("https://%s", env("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc"))
	tokenPath := env("DCP_KUBERNETES_TOKEN_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token")

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	client, err := newKubernetesHTTPClient()
	if err != nil {
		return nil, err
	}

	return &knativeServiceManager{
		namespace: namespace,
		baseURL:   baseURL,
		token:     string(tokenBytes),
		client:    client,
	}, nil
}

func newKubernetesHTTPClient() (*http.Client, error) {
	caPath := env("DCP_KUBERNETES_CA_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caBytes, readErr := os.ReadFile(caPath); readErr == nil {
		rootCAs.AppendCertsFromPEM(caBytes)
	}
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: rootCAs,
			},
		},
	}, nil
}

func (m *knativeServiceManager) List(ctx context.Context, scope projectScope) ([]deployedService, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services", m.baseURL, m.namespace), nil)
	if err != nil {
		return nil, err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return nil, decodeAPIError(res)
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				CreationTimestamp time.Time         `json:"creationTimestamp"`
				Generation        int64             `json:"generation"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Template struct {
					Spec struct {
						Containers []struct {
							Image string `json:"image"`
						} `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
			Status struct {
				URL        string `json:"url"`
				Conditions []struct {
					Type               string    `json:"type"`
					Status             string    `json:"status"`
					Reason             string    `json:"reason"`
					Message            string    `json:"message"`
					LastTransitionTime time.Time `json:"lastTransitionTime"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]deployedService, 0, len(payload.Items))
	for _, item := range payload.Items {
		if !isUserService(item.Metadata.Name, item.Metadata.Labels, scope) {
			continue
		}
		displayName := displayServiceName(item.Metadata.Name, item.Metadata.Labels)
		service := deployedService{
			Name:         displayName,
			Namespace:    m.namespace,
			ProjectID:    item.Metadata.Labels[projectLabelKey],
			Generation:   item.Metadata.Generation,
			CreatedAt:    item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			ResourceName: item.Metadata.Name,
			TargetURL:    item.Status.URL,
			URL:          userserviceroute.UserServiceURL("", strings.TrimSpace(os.Getenv("DCP_PUBLIC_SERVICE_DOMAIN")), item.Metadata.Name),
		}
		if len(item.Spec.Template.Spec.Containers) > 0 {
			service.Image = item.Spec.Template.Spec.Containers[0].Image
		}
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" {
				service.Ready = cond.Status == "True"
				service.Reason = cond.Reason
				if !cond.LastTransitionTime.IsZero() {
					service.UpdatedAt = cond.LastTransitionTime.UTC().Format(time.RFC3339)
				}
				break
			}
		}
		if service.UpdatedAt == "" {
			service.UpdatedAt = service.CreatedAt
		}
		out = append(out, service)
	}

	return out, nil
}

func (m *knativeServiceManager) TargetURL(ctx context.Context, scope projectScope, name string) (string, error) {
	service, err := m.getUserService(ctx, scope, name)
	if err != nil {
		return "", err
	}
	if service.TargetURL == "" {
		return "", fmt.Errorf("service %q has no target url", name)
	}
	return service.TargetURL, nil
}

func (m *knativeServiceManager) PublicTargetURL(ctx context.Context, name string) (string, error) {
	service, err := m.getPublicUserService(ctx, name)
	if err != nil {
		return "", err
	}
	if service.TargetURL == "" {
		return "", fmt.Errorf("service %q has no target url", name)
	}
	return service.TargetURL, nil
}

func (m *knativeServiceManager) Logs(ctx context.Context, scope projectScope, name string, tailLines int) (string, error) {
	service, err := m.getUserService(ctx, scope, name)
	if err != nil {
		return "", err
	}

	pods, err := m.listServicePods(ctx, service)
	if err != nil {
		return "", err
	}
	if len(pods) == 0 {
		return "", nil
	}

	sort.Slice(pods, func(i, j int) bool {
		if pods[i].CreationTimestamp.Equal(pods[j].CreationTimestamp) {
			return pods[i].Name > pods[j].Name
		}
		return pods[i].CreationTimestamp.After(pods[j].CreationTimestamp)
	})

	containerName := service.Name
	for _, pod := range pods {
		logs, err := m.readPodLogs(ctx, pod.Name, containerName, tailLines)
		if err == nil {
			return logs, nil
		}
		if errors.Is(err, errServiceNotFound) {
			continue
		}
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "container") {
			continue
		}
		return "", err
	}

	return "", nil
}

func (m *knativeServiceManager) Deploy(ctx context.Context, scope projectScope, req deployRequest) (deployedService, error) {
	resourceName := userServiceResourceName(scope.ProjectID, req.Name)
	manifest := knativeServiceManifest{
		APIVersion: "serving.knative.dev/v1",
		Kind:       "Service",
	}
	manifest.Metadata.Name = resourceName
	manifest.Metadata.Namespace = m.namespace
	manifest.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":   "dcp",
		"app.kubernetes.io/component":  "container-apps",
		"app.kubernetes.io/managed-by": "dcp-core",
		userLabelKey:                   scope.UserID,
		projectLabelKey:                scope.ProjectID,
		serviceNameLabel:               req.Name,
	}
	manifest.Spec.Template.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":  "dcp",
		"app.kubernetes.io/component": "container-apps",
		userLabelKey:                  scope.UserID,
		projectLabelKey:               scope.ProjectID,
		serviceNameLabel:              req.Name,
	}
	if req.MinScale > 0 || req.MaxScale > 0 {
		manifest.Spec.Template.Metadata.Annotations = map[string]string{}
		if req.MinScale > 0 {
			manifest.Spec.Template.Metadata.Annotations["autoscaling.knative.dev/minScale"] = strconv.Itoa(req.MinScale)
		}
		if req.MaxScale > 0 {
			manifest.Spec.Template.Metadata.Annotations["autoscaling.knative.dev/maxScale"] = strconv.Itoa(req.MaxScale)
		}
	}
	manifest.Spec.Template.Spec.Containers = []knativeContainer{
		{
			Name:  req.Name,
			Image: req.Image,
			Ports: []knativeContainerPort{
				{ContainerPort: req.Port},
			},
		},
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return deployedService{}, err
	}

	url := fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s?fieldManager=dcp-core&force=true", m.baseURL, m.namespace, resourceName)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return deployedService{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+m.token)
	httpReq.Header.Set("Content-Type", "application/apply-patch+yaml")
	httpReq.Header.Set("Accept", "application/json")

	res, err := m.client.Do(httpReq)
	if err != nil {
		return deployedService{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return deployedService{}, decodeAPIError(res)
	}

	return decodeService(res, scope)
}

func (m *knativeServiceManager) Delete(ctx context.Context, scope projectScope, name string) error {
	service, err := m.getUserService(ctx, scope, name)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, m.namespace, service.ResourceName), nil)
	if err != nil {
		return err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return decodeAPIError(res)
	}

	return nil
}

func (m *knativeServiceManager) listServicePods(ctx context.Context, service deployedService) ([]knativePodInfo, error) {
	selector := neturl.Values{}
	selector.Set("labelSelector", strings.Join([]string{
		"app.kubernetes.io/managed-by=" + userServiceManagerLabel,
		userLabelKey + "=" + serviceNameLabelValue(service),
		projectLabelKey + "=" + strings.TrimSpace(service.ProjectID),
		serviceNameLabel + "=" + service.Name,
	}, ","))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/namespaces/%s/pods?%s", m.baseURL, m.namespace, selector.Encode()), nil)
	if err != nil {
		return nil, err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return nil, decodeAPIError(res)
	}

	var payload struct {
		Items []knativePodInfo `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func (m *knativeServiceManager) readPodLogs(ctx context.Context, podName string, containerName string, tailLines int) (string, error) {
	reqURL, err := neturl.Parse(fmt.Sprintf("%s/api/v1/namespaces/%s/pods/%s/log", m.baseURL, m.namespace, podName))
	if err != nil {
		return "", err
	}
	query := reqURL.Query()
	if containerName != "" {
		query.Set("container", containerName)
	}
	if tailLines > 0 {
		query.Set("tailLines", strconv.Itoa(tailLines))
	}
	query.Set("timestamps", "true")
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return "", err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusBadRequest {
		return "", errServiceNotFound
	}
	if res.StatusCode >= 300 {
		return "", decodeAPIError(res)
	}

	logs, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(logs), nil
}

func (m *knativeServiceManager) getUserService(ctx context.Context, scope projectScope, name string) (deployedService, error) {
	return m.findUserService(ctx, scope, name)
}

func (m *knativeServiceManager) getPublicUserService(ctx context.Context, resourceName string) (deployedService, error) {
	return m.findPublicUserService(ctx, resourceName)
}

func isUserService(name string, labels map[string]string, scope projectScope) bool {
	if name == internalContainerAppsName {
		return false
	}
	return labels["app.kubernetes.io/managed-by"] == userServiceManagerLabel &&
		labels[userLabelKey] == scope.UserID &&
		labels[projectLabelKey] == scope.ProjectID
}

func displayServiceName(metadataName string, labels map[string]string) string {
	if value := strings.TrimSpace(labels[serviceNameLabel]); value != "" {
		return value
	}
	return metadataName
}

func userServiceResourceName(projectID, name string) string {
	seed := strings.TrimSpace(projectID) + ":" + strings.TrimSpace(name)
	sum := sha256.Sum256([]byte(seed))
	suffix := hex.EncodeToString(sum[:4])
	prefix := sanitizeDNSLabel(name)
	if prefix == "" {
		prefix = "service"
	}
	maxPrefixLen := 63 - 1 - len(suffix)
	if len(prefix) > maxPrefixLen {
		prefix = strings.TrimRight(prefix[:maxPrefixLen], "-")
	}
	if prefix == "" {
		prefix = "service"
	}
	return prefix + "-" + suffix
}

func decodeService(res *http.Response, scope projectScope) (deployedService, error) {
	var payload struct {
		Metadata struct {
			Name              string            `json:"name"`
			CreationTimestamp time.Time         `json:"creationTimestamp"`
			Generation        int64             `json:"generation"`
			Namespace         string            `json:"namespace"`
			Labels            map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Template struct {
				Spec struct {
					Containers []struct {
						Image string `json:"image"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
		Status struct {
			URL        string `json:"url"`
			Conditions []struct {
				Type               string    `json:"type"`
				Status             string    `json:"status"`
				Reason             string    `json:"reason"`
				LastTransitionTime time.Time `json:"lastTransitionTime"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return deployedService{}, err
	}
	if !isUserService(payload.Metadata.Name, payload.Metadata.Labels, scope) {
		return deployedService{}, errServiceNotFound
	}

	service := deployedService{
		Name:         displayServiceName(payload.Metadata.Name, payload.Metadata.Labels),
		Namespace:    payload.Metadata.Namespace,
		ProjectID:    payload.Metadata.Labels[projectLabelKey],
		Generation:   payload.Metadata.Generation,
		CreatedAt:    payload.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
		ResourceName: payload.Metadata.Name,
		TargetURL:    payload.Status.URL,
		URL:          userserviceroute.UserServiceURL("", strings.TrimSpace(os.Getenv("DCP_PUBLIC_SERVICE_DOMAIN")), payload.Metadata.Name),
	}
	if len(payload.Spec.Template.Spec.Containers) > 0 {
		service.Image = payload.Spec.Template.Spec.Containers[0].Image
	}
	for _, cond := range payload.Status.Conditions {
		if cond.Type == "Ready" {
			service.Ready = cond.Status == "True"
			service.Reason = cond.Reason
			if !cond.LastTransitionTime.IsZero() {
				service.UpdatedAt = cond.LastTransitionTime.UTC().Format(time.RFC3339)
			}
			break
		}
	}
	if service.UpdatedAt == "" {
		service.UpdatedAt = service.CreatedAt
	}

	return service, nil
}

type knativePodInfo struct {
	Name              string    `json:"name"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
	Spec              struct {
		Containers []struct {
			Name string `json:"name"`
		} `json:"containers"`
	} `json:"spec"`
}

func serviceNameLabelValue(service deployedService) string {
	if value := strings.TrimSpace(service.Name); value != "" {
		return value
	}
	return strings.TrimSpace(service.ResourceName)
}

func (m *knativeServiceManager) findUserService(ctx context.Context, scope projectScope, name string) (deployedService, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services", m.baseURL, m.namespace), nil)
	if err != nil {
		return deployedService{}, err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return deployedService{}, err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return deployedService{}, decodeAPIError(res)
	}

	var payload struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				CreationTimestamp time.Time         `json:"creationTimestamp"`
				Generation        int64             `json:"generation"`
				Labels            map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Template struct {
					Spec struct {
						Containers []struct {
							Image string `json:"image"`
						} `json:"containers"`
					} `json:"spec"`
				} `json:"template"`
			} `json:"spec"`
			Status struct {
				URL        string `json:"url"`
				Conditions []struct {
					Type               string    `json:"type"`
					Status             string    `json:"status"`
					Reason             string    `json:"reason"`
					LastTransitionTime time.Time `json:"lastTransitionTime"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return deployedService{}, err
	}

	for _, item := range payload.Items {
		if !isUserService(item.Metadata.Name, item.Metadata.Labels, scope) {
			continue
		}
		if displayServiceName(item.Metadata.Name, item.Metadata.Labels) != name {
			continue
		}
		service := deployedService{
			Name:         displayServiceName(item.Metadata.Name, item.Metadata.Labels),
			Namespace:    m.namespace,
			ProjectID:    item.Metadata.Labels[projectLabelKey],
			Generation:   item.Metadata.Generation,
			CreatedAt:    item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			ResourceName: item.Metadata.Name,
			TargetURL:    item.Status.URL,
			URL:          userserviceroute.UserServiceURL("", strings.TrimSpace(os.Getenv("DCP_PUBLIC_SERVICE_DOMAIN")), item.Metadata.Name),
		}
		if len(item.Spec.Template.Spec.Containers) > 0 {
			service.Image = item.Spec.Template.Spec.Containers[0].Image
		}
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" {
				service.Ready = cond.Status == "True"
				service.Reason = cond.Reason
				if !cond.LastTransitionTime.IsZero() {
					service.UpdatedAt = cond.LastTransitionTime.UTC().Format(time.RFC3339)
				}
				break
			}
		}
		if service.UpdatedAt == "" {
			service.UpdatedAt = service.CreatedAt
		}
		return service, nil
	}

	return deployedService{}, errServiceNotFound
}

func (m *knativeServiceManager) findPublicUserService(ctx context.Context, resourceName string) (deployedService, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, m.namespace, resourceName), nil)
	if err != nil {
		return deployedService{}, err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return deployedService{}, err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return deployedService{}, errServiceNotFound
	}
	if res.StatusCode >= 300 {
		return deployedService{}, decodeAPIError(res)
	}

	var payload struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Status struct {
			URL string `json:"url"`
		} `json:"status"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return deployedService{}, err
	}
	if payload.Metadata.Name == "" || payload.Metadata.Labels["app.kubernetes.io/managed-by"] != userServiceManagerLabel || payload.Metadata.Name == internalContainerAppsName {
		return deployedService{}, errServiceNotFound
	}
	return deployedService{
		Name:         displayServiceName(payload.Metadata.Name, payload.Metadata.Labels),
		ResourceName: payload.Metadata.Name,
		TargetURL:    payload.Status.URL,
		URL:          payload.Status.URL,
	}, nil
}

func (m *knativeServiceManager) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+m.token)
	req.Header.Set("Accept", "application/json")
}

func decodeAPIError(res *http.Response) error {
	var payload struct {
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err == nil {
		if payload.Message != "" {
			return fmt.Errorf("%s: %s", res.Status, payload.Message)
		}
		if payload.Reason != "" {
			return fmt.Errorf("%s: %s", res.Status, payload.Reason)
		}
	}
	return fmt.Errorf("kubernetes api returned %s", res.Status)
}

type knativeServiceManifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels,omitempty"`
	} `json:"metadata"`
	Spec struct {
		Template struct {
			Metadata struct {
				Labels      map[string]string `json:"labels,omitempty"`
				Annotations map[string]string `json:"annotations,omitempty"`
			} `json:"metadata"`
			Spec struct {
				Replicas   *int               `json:"replicas,omitempty"`
				Containers []knativeContainer `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type knativeContainer struct {
	Name  string                 `json:"name"`
	Image string                 `json:"image"`
	Ports []knativeContainerPort `json:"ports"`
}

type knativeContainerPort struct {
	ContainerPort int `json:"containerPort"`
}
