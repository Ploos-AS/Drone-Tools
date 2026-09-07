package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/gpx"
	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
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
	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["stage"] != "M1.1" {
		t.Fatalf("stage = %v, want M1.1", response["stage"])
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
		t.Fatalf("status = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Drone-Tools") {
		t.Fatal("index response does not contain Drone-Tools")
	}
}

func TestInspectEndpoint(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "flight.gpx")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("<gpx></gpx>"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/inspect", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var result inspector.Result
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Kind != "gpx" || !result.Supported {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGPXSummaryEndpoint(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "flight.gpx")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(`<gpx version="1.1"><trk><trkseg><trkpt lat="58" lon="7"><ele>10</ele><time>2026-09-07T08:00:00Z</time></trkpt><trkpt lat="58.001" lon="7.002"><ele>20</ele><time>2026-09-07T08:01:00Z</time></trkpt></trkseg></trk></gpx>`))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gpx/summary", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var summary gpx.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.TrackPoints != 2 || summary.DistanceMeters <= 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestInspectEndpointMethod(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/inspect", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestEnvOrDefault(t *testing.T) {
	const name = "DRONE_TOOLS_TEST_VALUE"
	t.Setenv(name, "configured")
	if got := envOrDefault(name, "fallback"); got != "configured" {
		t.Fatalf("envOrDefault() = %q, want configured", got)
	}
}
