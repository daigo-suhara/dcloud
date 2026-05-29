package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type knativeServiceManager struct {
	namespace string
	client    *http.Client
	baseURL   string
	token     string
}

func newServiceManager(namespace string) (serviceManager, error) {
	baseURL := fmt.Sprintf("https://%s", env("KUBERNETES_SERVICE_HOST", "kubernetes.default.svc"))
	tokenPath := env("DCP_KUBERNETES_TOKEN_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token")
	caPath := env("DCP_KUBERNETES_CA_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")

	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return nil, err
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caBytes, readErr := os.ReadFile(caPath); readErr == nil {
		rootCAs.AppendCertsFromPEM(caBytes)
	}

	return &knativeServiceManager{
		namespace: namespace,
		baseURL:   baseURL,
		token:     string(tokenBytes),
		client: &http.Client{
			Timeout: 20 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: rootCAs,
				},
			},
		},
	}, nil
}

func (m *knativeServiceManager) List(ctx context.Context) ([]deployedService, error) {
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
				Name              string    `json:"name"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
				Generation        int64     `json:"generation"`
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
					Type    string `json:"type"`
					Status  string `json:"status"`
					Reason  string `json:"reason"`
					Message string `json:"message"`
				} `json:"conditions"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]deployedService, 0, len(payload.Items))
	for _, item := range payload.Items {
		service := deployedService{
			Name:       item.Metadata.Name,
			Namespace:  m.namespace,
			Generation: item.Metadata.Generation,
			CreatedAt:  item.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
			URL:        item.Status.URL,
		}
		if len(item.Spec.Template.Spec.Containers) > 0 {
			service.Image = item.Spec.Template.Spec.Containers[0].Image
		}
		for _, cond := range item.Status.Conditions {
			if cond.Type == "Ready" {
				service.Ready = cond.Status == "True"
				service.Reason = cond.Reason
				break
			}
		}
		out = append(out, service)
	}

	return out, nil
}

func (m *knativeServiceManager) Deploy(ctx context.Context, req deployRequest) (deployedService, error) {
	manifest := knativeServiceManifest{
		APIVersion: "serving.knative.dev/v1",
		Kind:       "Service",
	}
	manifest.Metadata.Name = req.Name
	manifest.Metadata.Namespace = m.namespace
	manifest.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":   "dcp",
		"app.kubernetes.io/component":  "cloudrun",
		"app.kubernetes.io/managed-by": "dcp-control-plane",
	}
	manifest.Spec.Template.Metadata.Labels = map[string]string{
		"app.kubernetes.io/instance":  "dcp",
		"app.kubernetes.io/component": "cloudrun",
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

	url := fmt.Sprintf("%s/apis/serving.knative.dev/v1/namespaces/%s/services/%s?fieldManager=dcp-control-plane&force=true", m.baseURL, m.namespace, req.Name)
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

	var payload struct {
		Metadata struct {
			Name              string    `json:"name"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Generation        int64     `json:"generation"`
			Namespace         string    `json:"namespace"`
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
				Type   string `json:"type"`
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"conditions"`
		} `json:"status"`
	}

	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return deployedService{}, err
	}

	service := deployedService{
		Name:       payload.Metadata.Name,
		Namespace:  payload.Metadata.Namespace,
		Generation: payload.Metadata.Generation,
		CreatedAt:  payload.Metadata.CreationTimestamp.UTC().Format(time.RFC3339),
		URL:        payload.Status.URL,
	}
	if len(payload.Spec.Template.Spec.Containers) > 0 {
		service.Image = payload.Spec.Template.Spec.Containers[0].Image
	}
	for _, cond := range payload.Status.Conditions {
		if cond.Type == "Ready" {
			service.Ready = cond.Status == "True"
			service.Reason = cond.Reason
			break
		}
	}

	return service, nil
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
				Labels map[string]string `json:"labels,omitempty"`
			} `json:"metadata"`
			Spec struct {
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
