package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const projectsConfigMapName = "dcp-projects"

var uuidGenerator = uuid.NewString

var errProjectAlreadyExists = errors.New("project already exists")

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

	return filterProjectsByOwner(m.projects, userID), nil
}

func (m *memoryProjectManager) Create(_ context.Context, userID string, name string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if projectNameExists(m.projects, userID, name) {
		return project{}, errProjectAlreadyExists
	}
	p := newProject(userID, name, m.projects)
	m.projects = append(m.projects, p)
	return p, nil
}

func (m *memoryProjectManager) Delete(_ context.Context, userID string, projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, p := range m.projects {
		if p.Owner != userID || (p.ID != projectID && p.Name != projectID) {
			continue
		}
		m.projects = append(m.projects[:i], m.projects[i+1:]...)
		return nil
	}
	return errProjectNotFound
}

func (m *memoryProjectManager) Ensure(_ context.Context, userID string, projectID string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.projects {
		if p.Owner == userID && p.ID == projectID {
			return p, nil
		}
	}
	return project{}, errProjectNotFound
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

	projects, _, err := m.load(ctx)
	if err != nil {
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
	if projectNameExists(projects, userID, name) {
		return project{}, errProjectAlreadyExists
	}
	p := newProject(userID, name, projects)
	projects = append(projects, p)
	if err := m.save(ctx, projects, resourceVersion); err != nil {
		return project{}, err
	}
	return p, nil
}

func (m *kubeProjectManager) Delete(ctx context.Context, userID string, projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, resourceVersion, err := m.load(ctx)
	if err != nil {
		return err
	}
	next := make([]project, 0, len(projects))
	removed := false
	for _, p := range projects {
		if p.Owner == userID && (p.ID == projectID || p.Name == projectID) {
			removed = true
			continue
		}
		next = append(next, p)
	}
	if !removed {
		return errProjectNotFound
	}
	if err := m.save(ctx, next, resourceVersion); err != nil {
		return err
	}
	return nil
}

func (m *kubeProjectManager) Ensure(ctx context.Context, userID string, projectID string) (project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projects, _, err := m.load(ctx)
	if err != nil {
		return project{}, err
	}
	for _, p := range projects {
		if p.Owner == userID && p.ID == projectID {
			return p, nil
		}
	}
	return project{}, errProjectNotFound
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
	baseName := sanitizeDNSLabel(name)
	if baseName == "" {
		baseName = "project"
	}
	id := newProjectID(baseName, existing, now)
	return project{
		ID:        id,
		Name:      strings.TrimSpace(name),
		Owner:     userID,
		CreatedAt: now.Format(time.RFC3339),
	}
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

func projectNameExists(projects []project, userID string, name string) bool {
	normalized := strings.TrimSpace(name)
	for _, p := range projects {
		if p.Owner == userID && p.Name == normalized {
			return true
		}
	}
	return false
}

func projectExists(projects []project, id string) bool {
	for _, p := range projects {
		if p.ID == id {
			return true
		}
	}
	return false
}

func newProjectID(name string, existing []project, now time.Time) string {
	baseName := strings.Trim(name, "-")
	if baseName == "" {
		baseName = "project"
	}

	for i := 0; ; i++ {
		suffix := sanitizeDNSLabel(uuidGenerator())
		if suffix == "" {
			suffix = strconv36(now.UnixNano() + int64(i))
		}
		maxBaseLen := 63 - 1 - len(suffix)
		if maxBaseLen < 1 {
			maxBaseLen = 1
		}
		prefix := baseName
		if len(prefix) > maxBaseLen {
			prefix = strings.Trim(prefix[:maxBaseLen], "-")
		}
		if prefix == "" {
			prefix = "project"
		}
		id := prefix + "-" + suffix
		if !projectExists(existing, id) {
			return id
		}
	}
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
