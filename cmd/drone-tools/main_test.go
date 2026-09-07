package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	var response healthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" || response.Service != "drone-tools" {
		t.Fatalf("unexpected health response: %+v", response)
	}
}

func TestInfo(t *testing.T) {
	dataDir := t.TempDir()
	handler, err := newHandler(dataDir)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["name"] != "Drone-Tools" {
		t.Fatalf("name = %v, want Drone-Tools", response["name"])
	}
	if response["stage"] != "M0" {
		t.Fatalf("stage = %v, want M0", response["stage"])
	}
	if response["data_dir"] != dataDir {
		t.Fatalf("data_dir = %v, want %s", response["data_dir"], dataDir)
	}
}

func TestIndex(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Drone-Tools") {
		t.Fatal("index response does not contain Drone-Tools")
	}
}

func TestEnvOrDefault(t *testing.T) {
	const name = "DRONE_TOOLS_TEST_VALUE"
	t.Setenv(name, "configured")
	if got := envOrDefault(name, "fallback"); got != "configured" {
		t.Fatalf("envOrDefault() = %q, want configured", got)
	}

	const missing = "DRONE_TOOLS_TEST_MISSING"
	t.Setenv(missing, "")
	if got := envOrDefault(missing, "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault() = %q, want fallback", got)
	}
}
