package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "ok" || got.Service != "core" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestPlatform(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform", nil)
	rec := httptest.NewRecorder()

	platform(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got struct {
		Name       string   `json:"name"`
		Components []string `json:"components"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Name != "dcp" {
		t.Fatalf("expected platform name dcp, got %q", got.Name)
	}
	if len(got.Components) != 3 {
		t.Fatalf("expected 3 components, got %d", len(got.Components))
	}
}

func TestListServices(t *testing.T) {
	auth := testAuth()
	api := &apiServer{
		auth: auth,
		services: &fakeServiceManager{
			services: []deployedService{
				{
					Name:      "hello-dcp",
					Image:     "ghcr.io/example/hello-dcp:latest",
					Namespace: "dcp-system",
					ProjectID: defaultProjectID("default-user"),
					Ready:     true,
					UpdatedAt: "2026-05-31T00:00:00Z",
				},
			},
		},
		projects:  newMemoryProjectManager(),
		namespace: "dcp-system",
	}

	req := httptest.NewRequest(http.MethodGet, "http://172.16.100.11:8080/api/v1/services", nil)
	req.AddCookie(testSessionCookie(t, auth, authUser{ID: "default-user", Username: "default-user"}))
	rec := httptest.NewRecorder()

	api.listServices(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got struct {
		Namespace string            `json:"namespace"`
		Services  []deployedService `json:"services"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Namespace != "dcp-system" {
		t.Fatalf("expected namespace dcp-system, got %q", got.Namespace)
	}
	if len(got.Services) != 1 || got.Services[0].Name != "hello-dcp" {
		t.Fatalf("unexpected services payload: %+v", got.Services)
	}
	if got.Services[0].URL != "https://172.16.100.11:8080/cloudrun/default-default-user/hello-dcp/" {
		t.Fatalf("expected public service url, got %q", got.Services[0].URL)
	}
	if got.Services[0].UpdatedAt == "" {
		t.Fatalf("expected updatedAt to be populated")
	}
}

func TestDeployService(t *testing.T) {
	auth := testAuth()
	manager := &fakeServiceManager{}
	api := &apiServer{
		auth:      auth,
		services:  manager,
		projects:  newMemoryProjectManager(),
		namespace: "dcp-system",
	}

	req := httptest.NewRequest(http.MethodPost, "http://172.16.100.11:8080/api/v1/services", strings.NewReader(`{"name":"hello-dcp","image":"ghcr.io/example/hello-dcp:latest","port":8080,"minScale":1,"maxScale":5}`))
	req.AddCookie(testSessionCookie(t, auth, authUser{ID: "default-user", Username: "default-user"}))
	rec := httptest.NewRecorder()

	api.deployService(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if len(manager.deployed) != 1 {
		t.Fatalf("expected 1 deploy call, got %d", len(manager.deployed))
	}

	var got deployedService
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Name != "hello-dcp" || got.Image != "ghcr.io/example/hello-dcp:latest" {
		t.Fatalf("unexpected deploy response: %+v", got)
	}
	if got.URL != "https://172.16.100.11:8080/cloudrun/default-default-user/hello-dcp/" {
		t.Fatalf("expected public service url, got %q", got.URL)
	}
	if got.UpdatedAt == "" {
		t.Fatalf("expected updatedAt to be populated")
	}
	if manager.deployed[0].req.MinScale != 1 || manager.deployed[0].req.MaxScale != 5 {
		t.Fatalf("expected minScale 1 and maxScale 5, got %+v", manager.deployed[0].req)
	}
	if manager.deployed[0].scope.ProjectID != defaultProjectID("default-user") {
		t.Fatalf("expected default project scope, got %+v", manager.deployed[0].scope)
	}
}

func TestDeleteService(t *testing.T) {
	auth := testAuth()
	manager := &fakeServiceManager{}
	api := &apiServer{
		auth:      auth,
		services:  manager,
		projects:  newMemoryProjectManager(),
		namespace: "dcp-system",
	}

	req := httptest.NewRequest(http.MethodDelete, "http://172.16.100.11:8080/api/v1/services/hello-dcp", nil)
	req.AddCookie(testSessionCookie(t, auth, authUser{ID: "default-user", Username: "default-user"}))
	rec := httptest.NewRecorder()

	api.deleteService(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	if len(manager.deleted) != 1 || manager.deleted[0] != "hello-dcp" {
		t.Fatalf("unexpected delete calls: %+v", manager.deleted)
	}
}

func TestIsUserServiceRejectsInternalCloudRun(t *testing.T) {
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "dcp-core",
		userLabelKey:                   "default-user",
		projectLabelKey:                defaultProjectID("default-user"),
	}
	scope := projectScope{UserID: "default-user", ProjectID: defaultProjectID("default-user")}

	if isUserService("dcp-cloudrun", labels, scope) {
		t.Fatalf("expected dcp-cloudrun to be treated as internal")
	}
	if !isUserService("hello-dcp", labels, scope) {
		t.Fatalf("expected labeled user service to be visible")
	}
	if isUserService("hello-dcp", labels, projectScope{UserID: "other-user", ProjectID: defaultProjectID("other-user")}) {
		t.Fatalf("expected service from another user project to be hidden")
	}
}

func TestAuthFlow(t *testing.T) {
	auth := testAuth()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(testSessionCookie(t, auth, authUser{ID: "alice-id", Username: "alice"}))
	got, err := auth.CurrentUser(req)
	if err != nil {
		t.Fatalf("current user: %v", err)
	}
	if got.ID != "alice-id" || got.Username != "alice" {
		t.Fatalf("unexpected current user: %+v", got)
	}
}

func testAuth() *keycloakAuth {
	return &keycloakAuth{sessionSecret: "dcp-session-secret-change-me"}
}

func testSessionCookie(t *testing.T, auth *keycloakAuth, user authUser) *http.Cookie {
	t.Helper()
	value, err := auth.signPayload(sessionEnvelope{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Name:      user.Name,
		ExpiresAt: time.Now().UTC().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign session: %v", err)
	}
	return &http.Cookie{
		Name:  authCookieName,
		Value: value,
		Path:  "/",
	}
}

type fakeServiceManager struct {
	services []deployedService
	deployed []scopedDeploy
	deleted  []string
}

type scopedDeploy struct {
	scope projectScope
	req   deployRequest
}

func (f *fakeServiceManager) List(_ context.Context, scope projectScope) ([]deployedService, error) {
	out := append([]deployedService(nil), f.services...)
	for i := range out {
		if out[i].ProjectID == "" {
			out[i].ProjectID = scope.ProjectID
		}
		if out[i].URL == "" {
			out[i].URL = "/cloudrun/" + out[i].ProjectID + "/" + out[i].Name + "/"
		}
	}
	return out, nil
}

func (f *fakeServiceManager) Deploy(_ context.Context, scope projectScope, req deployRequest) (deployedService, error) {
	f.deployed = append(f.deployed, scopedDeploy{scope: scope, req: req})
	return deployedService{
		Name:      req.Name,
		Image:     req.Image,
		Namespace: "dcp-system",
		ProjectID: scope.ProjectID,
		Ready:     true,
		URL:       "/cloudrun/" + scope.ProjectID + "/" + req.Name + "/",
		UpdatedAt: "2026-05-31T00:00:00Z",
	}, nil
}

func (f *fakeServiceManager) Delete(_ context.Context, _ projectScope, name string) error {
	f.deleted = append(f.deleted, name)
	return nil
}

func (f *fakeServiceManager) TargetURL(_ context.Context, scope projectScope, name string) (string, error) {
	return "http://" + name + "." + scope.ProjectID + ".svc.cluster.local", nil
}
