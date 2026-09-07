package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	geodata "github.com/Ploos-AS/Drone-Tools/internal/geo"
	"github.com/Ploos-AS/Drone-Tools/internal/gpx"
	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
	"github.com/Ploos-AS/Drone-Tools/internal/mapdata"
)

func TestHealthz(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil { t.Fatal(err) }
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK { t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK) }
	var response healthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil { t.Fatal(err) }
	if response.Status != "ok" || response.Service != "drone-tools" { t.Fatalf("unexpected health response: %+v", response) }
}

func TestInfo(t *testing.T) {
	dataDir := t.TempDir()
	handler, err := newHandler(dataDir)
	if err != nil { t.Fatal(err) }
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/info", nil))
	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil { t.Fatal(err) }
	if response["stage"] != "M1.4" { t.Fatalf("stage = %v, want M1.4", response["stage"]) }
	if response["data_dir"] != dataDir { t.Fatalf("data_dir = %v, want %s", response["data_dir"], dataDir) }
}

func TestIndex(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil { t.Fatal(err) }
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK { t.Fatalf("status = %d", rr.Code) }
	if !strings.Contains(rr.Body.String(), "Drone-Tools") { t.Fatal("index response does not contain Drone-Tools") }
}

func TestInspectEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	rr := postFile(t, handler, "/api/v1/inspect", "flight.gpx", "<gpx></gpx>")
	if rr.Code != http.StatusOK { t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String()) }
	var result inspector.Result
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil { t.Fatal(err) }
	if result.Kind != "gpx" || !result.Supported { t.Fatalf("unexpected result: %#v", result) }
}

func TestGPXSummaryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `<gpx version="1.1"><trk><trkseg><trkpt lat="58" lon="7"><ele>10</ele><time>2026-09-07T08:00:00Z</time></trkpt><trkpt lat="58.001" lon="7.002"><ele>20</ele><time>2026-09-07T08:01:00Z</time></trkpt></trkseg></trk></gpx>`
	rr := postFile(t, handler, "/api/v1/gpx/summary", "flight.gpx", content)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String()) }
	var summary gpx.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil { t.Fatal(err) }
	if summary.TrackPoints != 2 || summary.DistanceMeters <= 0 { t.Fatalf("unexpected summary: %+v", summary) }
}

func TestAnalyzeEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `<gpx><trk><trkseg><trkpt lat="58" lon="7"><ele>10</ele><time>2026-09-07T08:00:00Z</time></trkpt><trkpt lat="58.001" lon="7.002"><ele>25</ele><time>2026-09-07T08:01:00Z</time></trkpt><trkpt lat="58.002" lon="7.004"><ele>20</ele><time>2026-09-07T08:02:00Z</time></trkpt></trkseg></trk></gpx>`
	rr := postFile(t, handler, "/api/v1/analyze", "flight.gpx", content)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String()) }
	var summary gpx.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil { t.Fatal(err) }
	if summary.AverageSpeedMPS == nil || summary.MaxSegmentSpeedMPS == nil { t.Fatalf("missing speed metrics: %+v", summary) }
	if summary.ElevationGainMeters != 15 || summary.ElevationLossMeters != 5 { t.Fatalf("unexpected elevation metrics: %+v", summary) }
	if summary.Quality != "good" { t.Fatalf("quality = %q, want good", summary.Quality) }
}

func TestKMLSummaryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `<kml><Document><Placemark><LineString><coordinates>7,58,10 7.1,58.1,20</coordinates></LineString></Placemark></Document></kml>`
	rr := postFile(t, handler, "/api/v1/kml/summary", "flight.kml", content)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String()) }
	var summary geodata.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil { t.Fatal(err) }
	if summary.Format != "kml" || summary.LineStrings != 1 || summary.Coordinates != 2 { t.Fatalf("unexpected summary: %+v", summary) }
}

func TestGeoJSONSummaryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"LineString","coordinates":[[7,58],[7.1,58.1]]}}]}`
	rr := postFile(t, handler, "/api/v1/geojson/summary", "flight.geojson", content)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String()) }
	var summary geodata.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil { t.Fatal(err) }
	if summary.Format != "geojson" || summary.Features != 1 || summary.LineStrings != 1 || summary.Coordinates != 2 { t.Fatalf("unexpected summary: %+v", summary) }
}

func TestMapEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `<gpx><wpt lat="58" lon="7"/><trk><trkseg><trkpt lat="58" lon="7"/><trkpt lat="58.1" lon="7.2"/></trkseg></trk></gpx>`
	rr := postFile(t, handler, "/api/v1/map", "flight.gpx", content)
	if rr.Code != http.StatusOK { t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String()) }
	var doc mapdata.Document
	if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil { t.Fatal(err) }
	if doc.Format != "gpx" || len(doc.Paths) != 1 || len(doc.Points) != 1 || doc.Bounds == nil { t.Fatalf("unexpected map document: %+v", doc) }
}

func TestInspectEndpointMethod(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/inspect", nil))
	if rr.Code != http.StatusMethodNotAllowed { t.Fatalf("status = %d", rr.Code) }
}

func TestEnvOrDefault(t *testing.T) {
	const name = "DRONE_TOOLS_TEST_VALUE"
	t.Setenv(name, "configured")
	if got := envOrDefault(name, "fallback"); got != "configured" { t.Fatalf("envOrDefault() = %q, want configured", got) }
}

func postFile(t *testing.T, handler http.Handler, path, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil { t.Fatal(err) }
	if _, err := part.Write([]byte(content)); err != nil { t.Fatal(err) }
	if err := writer.Close(); err != nil { t.Fatal(err) }
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}
