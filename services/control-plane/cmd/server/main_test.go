package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	if got.Status != "ok" || got.Service != "control-plane" {
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
	api := &apiServer{
		services: &fakeServiceManager{
			services: []deployedService{
				{
					Name:      "hello-dcp",
					Image:     "ghcr.io/example/hello-dcp:latest",
					Namespace: "dcp-system",
					Ready:     true,
				},
			},
		},
		namespace: "dcp-system",
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
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
}

func TestDeployService(t *testing.T) {
	manager := &fakeServiceManager{}
	api := &apiServer{
		services:  manager,
		namespace: "dcp-system",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/services", strings.NewReader(`{"name":"hello-dcp","image":"ghcr.io/example/hello-dcp:latest","port":8080}`))
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
}

type fakeServiceManager struct {
	services []deployedService
	deployed []deployRequest
}

func (f *fakeServiceManager) List(context.Context) ([]deployedService, error) {
	return append([]deployedService(nil), f.services...), nil
}

func (f *fakeServiceManager) Deploy(_ context.Context, req deployRequest) (deployedService, error) {
	f.deployed = append(f.deployed, req)
	return deployedService{
		Name:      req.Name,
		Image:     req.Image,
		Namespace: "dcp-system",
		Ready:     true,
	}, nil
}
