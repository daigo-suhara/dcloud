package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const projectsConfigMapName = "dcp-projects"

type memoryProjectManager struct {
	mu       sync.Mutex
	projects []project
}

func newMemoryProjectManager() projectManager {
	return &memoryProjectManager{}
}

func (m *memoryProjectManager) List(_ context.Context, userID string) ([]project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureDefaultLocked(userID)
	return filterProjectsByOwner(m.projects, userID), nil
}

func (m *memoryProjectManager) Create(_ context.Context, userID string, name string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := newProject(userID, name, m.projects)
	m.projects = append(m.projects, p)
	return p, nil
}

func (m *memoryProjectManager) Ensure(_ context.Context, userID string, projectID string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ensureDefaultLocked(userID)
	for _, p := range m.projects {
		if p.Owner == userID && p.ID == projectID {
			return p, nil
		}
	}
	return project{}, errProjectNotFound
}

func (m *memoryProjectManager) Default(_ context.Context, userID string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.ensureDefaultLocked(userID), nil
}

func (m *memoryProjectManager) ensureDefaultLocked(userID string) project {
	for _, p := range m.projects {
		if p.Owner == userID && p.ID == defaultProjectID(userID) {
			return p
		}
	}
	p := project{
		ID:        defaultProjectID(userID),
		Name:      "default",
		Owner:     userID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	m.projects = append(m.projects, p)
	return p
}

type kubeProjectManager struct {
	namespace string
	client    *http.Client
	baseURL   string
	token     string
	mu        sync.Mutex
}

func newProjectManager(namespace string) (projectManager, error) {
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
	return &kubeProjectManager{
		namespace: namespace,
		baseURL:   baseURL,
		token:     string(tokenBytes),
		client:    client,
	}, nil
}

func (m *kubeProjectManager) List(ctx context.Context, userID string) ([]project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, resourceVersion, err := m.load(ctx)
	if err != nil {
		return nil, err
	}
	projects, _, err = ensureDefaultProject(projects, userID)
	if err != nil {
		return nil, err
	}
	if err := m.save(ctx, projects, resourceVersion); err != nil {
		return nil, err
	}
	return filterProjectsByOwner(projects, userID), nil
}

func (m *kubeProjectManager) Create(ctx context.Context, userID string, name string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, resourceVersion, err := m.load(ctx)
	if err != nil {
		return project{}, err
	}
	p := newProject(userID, name, projects)
	projects = append(projects, p)
	if err := m.save(ctx, projects, resourceVersion); err != nil {
		return project{}, err
	}
	return p, nil
}

func (m *kubeProjectManager) Ensure(ctx context.Context, userID string, projectID string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, resourceVersion, err := m.load(ctx)
	if err != nil {
		return project{}, err
	}
	projects, _, err = ensureDefaultProject(projects, userID)
	if err != nil {
		return project{}, err
	}
	if err := m.save(ctx, projects, resourceVersion); err != nil {
		return project{}, err
	}
	for _, p := range projects {
		if p.Owner == userID && p.ID == projectID {
			return p, nil
		}
	}
	return project{}, errProjectNotFound
}

func (m *kubeProjectManager) Default(ctx context.Context, userID string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, resourceVersion, err := m.load(ctx)
	if err != nil {
		return project{}, err
	}
	projects, p, err := ensureDefaultProject(projects, userID)
	if err != nil {
		return project{}, err
	}
	if err := m.save(ctx, projects, resourceVersion); err != nil {
		return project{}, err
	}
	return p, nil
}

func (m *kubeProjectManager) load(ctx context.Context) ([]project, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps/%s", m.baseURL, m.namespace, projectsConfigMapName), nil)
	if err != nil {
		return nil, "", err
	}
	m.authorize(req)

	res, err := m.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}
	if res.StatusCode >= 300 {
		return nil, "", decodeAPIError(res)
	}

	var payload struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, "", err
	}
	raw := payload.Data["projects.json"]
	if raw == "" {
		return nil, payload.Metadata.ResourceVersion, nil
	}
	var projects []project
	if err := json.Unmarshal([]byte(raw), &projects); err != nil {
		return nil, "", err
	}
	return projects, payload.Metadata.ResourceVersion, nil
}

func (m *kubeProjectManager) save(ctx context.Context, projects []project, resourceVersion string) error {
	raw, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	method := http.MethodPut
	url := fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps/%s", m.baseURL, m.namespace, projectsConfigMapName)
	if resourceVersion == "" {
		method = http.MethodPost
		url = fmt.Sprintf("%s/api/v1/namespaces/%s/configmaps", m.baseURL, m.namespace)
	}
	body := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]string{
			"name": projectsConfigMapName,
		},
		"data": map[string]string{
			"projects.json": string(raw),
		},
	}
	if resourceVersion != "" {
		body["metadata"] = map[string]string{
			"name":            projectsConfigMapName,
			"resourceVersion": resourceVersion,
		}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	m.authorize(req)
	req.Header.Set("Content-Type", "application/json")

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

func (m *kubeProjectManager) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+m.token)
	req.Header.Set("Accept", "application/json")
}

func newProject(userID string, name string, existing []project) project {
	now := time.Now().UTC()
	id := sanitizeDNSLabel(name)
	if id == "" || id == "default" {
		id = "project"
	}
	baseID := id
	for i := 0; projectExists(existing, userID, id); i++ {
		id = fmt.Sprintf("%s-%s", baseID, strconv36(now.UnixNano()+int64(i)))
		if len(id) > 63 {
			id = id[:63]
			id = strings.Trim(id, "-")
		}
	}
	return project{
		ID:        id,
		Name:      strings.TrimSpace(name),
		Owner:     userID,
		CreatedAt: now.Format(time.RFC3339),
	}
}

func ensureDefaultProject(projects []project, userID string) ([]project, project, error) {
	id := defaultProjectID(userID)
	for _, p := range projects {
		if p.Owner == userID && p.ID == id {
			return projects, p, nil
		}
	}
	p := project{
		ID:        id,
		Name:      "default",
		Owner:     userID,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return append(projects, p), p, nil
}

func filterProjectsByOwner(projects []project, userID string) []project {
	out := make([]project, 0, len(projects))
	for _, p := range projects {
		if p.Owner == userID {
			out = append(out, p)
		}
	}
	return out
}

func projectExists(projects []project, userID string, id string) bool {
	for _, p := range projects {
		if p.Owner == userID && p.ID == id {
			return true
		}
	}
	return false
}

func defaultProjectID(userID string) string {
	id := sanitizeDNSLabel(userID)
	if id == "" {
		id = "default-user"
	}
	return "default-" + id
}

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 50 {
		out = strings.Trim(out[:50], "-")
	}
	return out
}

func strconv36(value int64) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	if value < 0 {
		value = -value
	}
	if value == 0 {
		return "0"
	}
	var out [16]byte
	i := len(out)
	for value > 0 {
		i--
		out[i] = alphabet[value%36]
		value /= 36
	}
	return string(out[i:])
}
