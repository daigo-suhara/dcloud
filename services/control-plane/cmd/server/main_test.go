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
