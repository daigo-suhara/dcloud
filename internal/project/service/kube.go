package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func projectNamespace(projectID string) string {
	return "proj-" + projectID
}

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
	baseURL := strings.TrimRight(envOr("DCLD_KUBERNETES_API_URL", "https://kubernetes.default.svc"), "/")
	return &kubeClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:   30 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}},
		},
		token: strings.TrimSpace(string(token)),
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c *kubeClient) ensureProjectNamespace(ctx context.Context, projectID, userID string) error {
	payload := map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]any{
			"name": projectNamespace(projectID),
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "dcloud",
				"dcloud/project-id":            projectID,
				"dcloud/user-id":               userID,
			},
		},
	}
	return c.doJSON(ctx, http.MethodPost, "/api/v1/namespaces", payload, nil)
}

func (c *kubeClient) deleteProjectNamespace(ctx context.Context, projectID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/namespaces/"+projectNamespace(projectID), nil, nil)
}

func (c *kubeClient) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
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
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusConflict {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusNotFound && method == http.MethodDelete {
			return nil
		}
		return fmt.Errorf("kube api %s %s: %d %s", method, path, resp.StatusCode, string(buf))
	}
	if out != nil && len(buf) > 0 {
		return json.Unmarshal(buf, out)
	}
	return nil
}
