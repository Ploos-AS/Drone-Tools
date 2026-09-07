package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
)

func TestHealthz(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK) }
	var response healthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil { t.Fatal(err) }
	if response.Status != "ok" || response.Service != "drone-tools" { t.Fatalf("unexpected health response: %+v", response) }
}

func TestInfo(t *testing.T) {
	dataDir := t.TempDir()
	handler, err := newHandler(dataDir)
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil { t.Fatal(err) }
	if response["stage"] != "M1.0" { t.Fatalf("stage = %v, want M1.0", response["stage"]) }
	if response["data_dir"] != dataDir { t.Fatalf("data_dir = %v, want %s", response["data_dir"], dataDir) }
}

func TestIndex(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d", rr.Code) }
	if !strings.Contains(rr.Body.String(), "Drone-Tools") { t.Fatal("index response does not contain Drone-Tools") }
}

func TestInspectEndpoint(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil { t.Fatal(err) }
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "flight.gpx")
	if err != nil { t.Fatal(err) }
	_, _ = part.Write([]byte("<gpx></gpx>"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/inspect", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String()) }
	var result inspector.Result
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil { t.Fatal(err) }
	if result.Kind != "gpx" || !result.Supported { t.Fatalf("unexpected result: %#v", result) }
}

func TestInspectEndpointMethod(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil { t.Fatal(err) }
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/inspect", nil))
	if rr.Code != http.StatusMethodNotAllowed { t.Fatalf("status = %d", rr.Code) }
}

func TestEnvOrDefault(t *testing.T) {
	const name = "DRONE_TOOLS_TEST_VALUE"
	t.Setenv(name, "configured")
	if got := envOrDefault(name, "fallback"); got != "configured" { t.Fatalf("envOrDefault() = %q", got) }
}
