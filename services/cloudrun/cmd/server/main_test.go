package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if got.Status != "ok" || got.Service != "cloudrun" {
		t.Fatalf("unexpected response: %+v", got)
	}
}

func TestListServices(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/cloudrun/services", nil)
	rec := httptest.NewRecorder()

	listServices(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var got struct {
		Services []cloudRunService `json:"services"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(got.Services))
	}
	if got.Services[0].Name == "" || got.Services[0].Image == "" {
		t.Fatalf("expected service name and image, got %+v", got.Services[0])
	}
}
