package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	userServiceManagerLabel = "dcp-core"
	internalCloudRunName    = "dcp-cloudrun"
	userLabelKey            = "dcp.dev/user"
	projectLabelKey         = "dcp.dev/project"
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
		service := deployedService{
			Name:       item.Metadata.Name,
			Namespace:  m.namespace,
			ProjectID:  item.Metadata.Labels[projectLabelKey],
			Generation: item.Metadata.Generation,
			CreatedAt:  item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			URL:        servicePublicURL(item.Metadata.Labels[projectLabelKey], item.Metadata.Name),
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
	if _, err := m.getUserService(ctx, scope, name); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, m.namespace, name), nil)
	if err != nil {
		return "", err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		return "", decodeAPIError(res)
	}

	var payload struct {
		Status struct {
			URL string `json:"url"`
		} `json:"status"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.Status.URL == "" {
		return "", fmt.Errorf("service %q has no target url", name)
	}
	return payload.Status.URL, nil
}

func (m *knativeServiceManager) PublicTargetURL(ctx context.Context, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, m.namespace, name), nil)
	if err != nil {
		return "", err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return "", errServiceNotFound
	}
	if res.StatusCode >= 300 {
		return "", decodeAPIError(res)
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
		return "", err
	}
	if payload.Metadata.Name == "" || payload.Metadata.Labels["app.kubernetes.io/managed-by"] != userServiceManagerLabel || payload.Metadata.Name == internalCloudRunName {
		return "", errServiceNotFound
	}
	if payload.Status.URL == "" {
		return "", fmt.Errorf("service %q has no target url", name)
	}
	return payload.Status.URL, nil
}

func (m *knativeServiceManager) Deploy(ctx context.Context, scope projectScope, req deployRequest) (deployedService, error) {
	manifest := knativeServiceManifest{
		APIVersion: "serving.knative.dev/v1",
		Kind:       "Service",
	}
	manifest.Metadata.Name = req.Name
	manifest.Metadata.Namespace = m.namespace
	manifest.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":   "dcp",
		"app.kubernetes.io/component":  "cloudrun",
		"app.kubernetes.io/managed-by": "dcp-core",
		userLabelKey:                   scope.UserID,
		projectLabelKey:                scope.ProjectID,
	}
	manifest.Spec.Template.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":  "dcp",
		"app.kubernetes.io/component": "cloudrun",
		userLabelKey:                  scope.UserID,
		projectLabelKey:               scope.ProjectID,
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

	url := fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s?fieldManager=dcp-core&force=true", m.baseURL, m.namespace, req.Name)
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
	if _, err := m.getUserService(ctx, scope, name); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, m.namespace, name), nil)
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

func (m *knativeServiceManager) getUserService(ctx context.Context, scope projectScope, name string) (deployedService, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s", m.baseURL, m.namespace, name), nil)
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

	service, err := decodeService(res, scope)
	if err != nil {
		return deployedService{}, err
	}
	if service.Name == "" {
		return deployedService{}, errServiceNotFound
	}
	return service, nil
}

func isUserService(name string, labels map[string]string, scope projectScope) bool {
	if name == internalCloudRunName {
		return false
	}
	return labels["app.kubernetes.io/managed-by"] == userServiceManagerLabel &&
		labels[userLabelKey] == scope.UserID &&
		labels[projectLabelKey] == scope.ProjectID
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
		Name:       payload.Metadata.Name,
		Namespace:  payload.Metadata.Namespace,
		ProjectID:  payload.Metadata.Labels[projectLabelKey],
		Generation: payload.Metadata.Generation,
		CreatedAt:  payload.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
		URL:        servicePublicURL(payload.Metadata.Labels[projectLabelKey], payload.Metadata.Name),
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

func servicePublicURL(projectID string, name string) string {
	domain := strings.TrimSpace(os.Getenv("DCP_PUBLIC_SERVICE_DOMAIN"))
	if domain != "" {
		return fmt.Sprintf("https://%s.%s/", name, strings.TrimSuffix(domain, "."))
	}
	if projectID == "" {
		return fmt.Sprintf("/services/%s/", name)
	}
	return fmt.Sprintf("/cloudrun/%s/%s/", projectID, name)
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
