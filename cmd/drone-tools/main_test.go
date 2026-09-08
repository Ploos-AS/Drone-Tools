package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/dataflash"
	geodata "github.com/Ploos-AS/Drone-Tools/internal/geo"
	"github.com/Ploos-AS/Drone-Tools/internal/gpx"
	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
	"github.com/Ploos-AS/Drone-Tools/internal/mapdata"
	"github.com/Ploos-AS/Drone-Tools/internal/ulog"
)

func TestHealthz(t *testing.T) {
	handler, err := newHandler(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
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
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/info", nil))
	var response map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response["stage"] != "M3.11" {
		t.Fatalf("stage = %v, want M3.11", response["stage"])
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
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Drone-Tools") || !strings.Contains(body, "Flight Analysis") || !strings.Contains(body, ".ulg") {
		t.Fatal("index response does not contain flight workspace UI")
	}
}

func TestInspectEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	rr := postFile(t, handler, "/api/v1/inspect", "flight.gpx", "<gpx></gpx>")
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
	handler, _ := newHandler(t.TempDir())
	content := `<gpx version="1.1"><trk><trkseg><trkpt lat="58" lon="7"><ele>10</ele><time>2026-09-07T08:00:00Z</time></trkpt><trkpt lat="58.001" lon="7.002"><ele>20</ele><time>2026-09-07T08:01:00Z</time></trkpt></trkseg></trk></gpx>`
	rr := postFile(t, handler, "/api/v1/gpx/summary", "flight.gpx", content)
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

func TestAnalyzeEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `<gpx><trk><trkseg><trkpt lat="58" lon="7"><ele>10</ele><time>2026-09-07T08:00:00Z</time></trkpt><trkpt lat="58.001" lon="7.002"><ele>25</ele><time>2026-09-07T08:01:00Z</time></trkpt><trkpt lat="58.002" lon="7.004"><ele>20</ele><time>2026-09-07T08:02:00Z</time></trkpt></trkseg></trk></gpx>`
	rr := postFile(t, handler, "/api/v1/analyze", "flight.gpx", content)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var summary gpx.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.AverageSpeedMPS == nil || summary.MaxSegmentSpeedMPS == nil {
		t.Fatalf("missing speed metrics: %+v", summary)
	}
	if summary.ElevationGainMeters != 15 || summary.ElevationLossMeters != 5 {
		t.Fatalf("unexpected elevation metrics: %+v", summary)
	}
	if summary.Quality != "good" {
		t.Fatalf("quality = %q, want good", summary.Quality)
	}
}

func TestULogTelemetryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := buildULogFixture(t)
	rr := postFile(t, handler, "/api/v1/ulog/telemetry", "flight.ulg", content)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var summary ulog.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Telemetry.GPS) != 2 || summary.Telemetry.GPS[0].Latitude != 58 || len(summary.Telemetry.Battery) != 1 {
		t.Fatalf("unexpected ULog telemetry: %+v", summary.Telemetry)
	}
}

func TestULogMapEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	rr := postFile(t, handler, "/api/v1/map", "flight.ulg", buildULogFixture(t))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var doc mapdata.Document
	if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Format != "px4-ulog" || len(doc.Paths) != 1 || len(doc.Paths[0]) != 2 || doc.Bounds == nil {
		t.Fatalf("unexpected ULog map: %+v", doc)
	}
}

func TestULogAnalyzeEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	rr := postFile(t, handler, "/api/v1/analyze", "flight.ulg", buildULogFixture(t))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var analysis flightAnalysis
	if err := json.NewDecoder(rr.Body).Decode(&analysis); err != nil {
		t.Fatal(err)
	}
	if analysis.TrackPoints != 2 || analysis.DistanceMeters <= 0 || analysis.DurationSeconds == nil || analysis.BatteryVoltageV == nil || analysis.BatteryRemaining == nil {
		t.Fatalf("unexpected ULog analysis: %+v", analysis)
	}
	if math.Abs(*analysis.BatteryVoltageV-15.2) > 1e-5 || math.Abs(*analysis.BatteryRemaining-0.75) > 1e-6 {
		t.Fatalf("unexpected battery analysis: %+v", analysis)
	}
	if analysis.ElevationGainMeters != 2 || analysis.ElevationLossMeters != 0 {
		t.Fatalf("unexpected ULog elevation analysis: %+v", analysis)
	}
}

func TestDataFlashTelemetryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	rr := postFile(t, handler, "/api/v1/dataflash/telemetry", "flight.bin", buildDataFlashFixture(t))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var summary dataflash.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Telemetry.GPS) != 2 || len(summary.Telemetry.Battery) != 1 || summary.Telemetry.GPS[0].Latitude != 58 {
		t.Fatalf("unexpected DataFlash telemetry: %+v", summary.Telemetry)
	}
}

func TestDataFlashWorkspaceEndpoints(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	fixture := buildDataFlashFixture(t)
	mapRR := postFile(t, handler, "/api/v1/map", "flight.bin", fixture)
	if mapRR.Code != http.StatusOK {
		t.Fatalf("map status = %d body=%s", mapRR.Code, mapRR.Body.String())
	}
	var doc mapdata.Document
	if err := json.NewDecoder(mapRR.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Format != "ardupilot-dataflash" || len(doc.Paths) != 1 || len(doc.Paths[0]) != 2 {
		t.Fatalf("unexpected DataFlash map: %+v", doc)
	}

	analysisRR := postFile(t, handler, "/api/v1/analyze", "flight.bin", fixture)
	if analysisRR.Code != http.StatusOK {
		t.Fatalf("analysis status = %d body=%s", analysisRR.Code, analysisRR.Body.String())
	}
	var analysis flightAnalysis
	if err := json.NewDecoder(analysisRR.Body).Decode(&analysis); err != nil {
		t.Fatal(err)
	}
	if analysis.TrackPoints != 2 || analysis.DistanceMeters <= 0 || analysis.DurationSeconds == nil || analysis.BatteryVoltageV == nil {
		t.Fatalf("unexpected DataFlash analysis: %+v", analysis)
	}
	if math.Abs(*analysis.BatteryVoltageV-15.2) > 1e-5 || math.Abs(analysis.ElevationGainMeters-2) > 1e-6 {
		t.Fatalf("unexpected DataFlash metrics: %+v", analysis)
	}
}

func TestKMLSummaryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `<kml><Document><Placemark><LineString><coordinates>7,58,10 7.1,58.1,20</coordinates></LineString></Placemark></Document></kml>`
	rr := postFile(t, handler, "/api/v1/kml/summary", "flight.kml", content)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var summary geodata.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.Format != "kml" || summary.LineStrings != 1 || summary.Coordinates != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestGeoJSONSummaryEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"LineString","coordinates":[[7,58],[7.1,58.1]]}}]}`
	rr := postFile(t, handler, "/api/v1/geojson/summary", "flight.geojson", content)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var summary geodata.Summary
	if err := json.NewDecoder(rr.Body).Decode(&summary); err != nil {
		t.Fatal(err)
	}
	if summary.Format != "geojson" || summary.Features != 1 || summary.LineStrings != 1 || summary.Coordinates != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestMapEndpoint(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	content := `<gpx><wpt lat="58" lon="7"/><trk><trkseg><trkpt lat="58" lon="7"/><trkpt lat="58.1" lon="7.2"/></trkseg></trk></gpx>`
	rr := postFile(t, handler, "/api/v1/map", "flight.gpx", content)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var doc mapdata.Document
	if err := json.NewDecoder(rr.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc.Format != "gpx" || len(doc.Paths) != 1 || len(doc.Points) != 1 || doc.Bounds == nil {
		t.Fatalf("unexpected map document: %+v", doc)
	}
}

func TestInspectEndpointMethod(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
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

func buildULogFixture(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	b.Write([]byte{'U', 'L', 'o', 'g', 0x01, 0x12, 0x35, 1})
	_ = binary.Write(&b, binary.LittleEndian, uint64(1))

	writeULogMessage(&b, 'F', []byte("vehicle_gps_position:uint64_t timestamp;int32_t lat;int32_t lon;int32_t alt;float vel_m_s;uint8_t fix_type;uint8_t satellites_used;"))
	writeULogMessage(&b, 'A', append([]byte{0, 7, 0}, []byte("vehicle_gps_position")...))
	writeGPSData(t, &b, 7, 1000000, 580000000, 70000000, 100000, 10, 3, 12)
	writeGPSData(t, &b, 7, 3000000, 580010000, 70020000, 102000, 12, 3, 11)

	writeULogMessage(&b, 'F', []byte("battery_status:uint64_t timestamp;float voltage_v;float current_a;float remaining;"))
	writeULogMessage(&b, 'A', append([]byte{0, 8, 0}, []byte("battery_status")...))
	var battery bytes.Buffer
	_ = binary.Write(&battery, binary.LittleEndian, uint16(8))
	_ = binary.Write(&battery, binary.LittleEndian, uint64(3000000))
	_ = binary.Write(&battery, binary.LittleEndian, float32(15.2))
	_ = binary.Write(&battery, binary.LittleEndian, float32(6.5))
	_ = binary.Write(&battery, binary.LittleEndian, float32(0.75))
	writeULogMessage(&b, 'D', battery.Bytes())
	return b.String()
}

func buildDataFlashFixture(t *testing.T) string {
	t.Helper()
	var b bytes.Buffer
	b.Write(dataFlashFormatFrame(42, 29, "GPS", "QLLffBB", "TimeUS,Lat,Lng,Alt,Spd,Status,NSats"))
	writeDataFlashGPS(t, &b, 42, 1000000, 580000000, 70000000, 100, 10, 3, 12)
	writeDataFlashGPS(t, &b, 42, 3000000, 580010000, 70020000, 102, 12, 3, 11)
	b.Write(dataFlashFormatFrame(43, 20, "BAT", "QffB", "TimeUS,Volt,Curr,RemPct"))
	var bat bytes.Buffer
	bat.Write([]byte{0xA3, 0x95, 43})
	_ = binary.Write(&bat, binary.LittleEndian, uint64(3000000))
	_ = binary.Write(&bat, binary.LittleEndian, float32(15.2))
	_ = binary.Write(&bat, binary.LittleEndian, float32(6.5))
	bat.WriteByte(75)
	b.Write(bat.Bytes())
	return b.String()
}

func dataFlashFormatFrame(typ, length byte, name, format, columns string) []byte {
	frame := make([]byte, 89)
	frame[0], frame[1], frame[2] = 0xA3, 0x95, 128
	frame[3], frame[4] = typ, length
	copy(frame[5:9], name)
	copy(frame[9:25], format)
	copy(frame[25:89], columns)
	return frame
}

func writeDataFlashGPS(t *testing.T, b *bytes.Buffer, typ byte, timestamp uint64, lat, lon int32, alt, speed float32, status, satellites byte) {
	t.Helper()
	b.Write([]byte{0xA3, 0x95, typ})
	_ = binary.Write(b, binary.LittleEndian, timestamp)
	_ = binary.Write(b, binary.LittleEndian, lat)
	_ = binary.Write(b, binary.LittleEndian, lon)
	_ = binary.Write(b, binary.LittleEndian, alt)
	_ = binary.Write(b, binary.LittleEndian, speed)
	b.WriteByte(status)
	b.WriteByte(satellites)
}

func writeGPSData(t *testing.T, b *bytes.Buffer, id uint16, timestamp uint64, lat, lon, alt int32, speed float32, fix, satellites byte) {
	t.Helper()
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, id)
	_ = binary.Write(&data, binary.LittleEndian, timestamp)
	_ = binary.Write(&data, binary.LittleEndian, lat)
	_ = binary.Write(&data, binary.LittleEndian, lon)
	_ = binary.Write(&data, binary.LittleEndian, alt)
	_ = binary.Write(&data, binary.LittleEndian, speed)
	data.WriteByte(fix)
	data.WriteByte(satellites)
	writeULogMessage(b, 'D', data.Bytes())
}

func writeULogMessage(b *bytes.Buffer, typ byte, payload []byte) {
	_ = binary.Write(b, binary.LittleEndian, uint16(len(payload)))
	b.WriteByte(typ)
	b.Write(payload)
}

func postFile(t *testing.T, handler http.Handler, path, filename, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}
