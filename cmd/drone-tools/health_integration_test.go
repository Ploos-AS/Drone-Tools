package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/health"
)

func TestFlightHealthULog(t *testing.T) {
	assertFlightHealth(t, "flight.ulg", buildULogFixture(t))
}

func TestFlightHealthDataFlash(t *testing.T) {
	assertFlightHealth(t, "flight.bin", buildDataFlashFixture(t))
}

func TestFlightHealthTLOG(t *testing.T) {
	assertFlightHealth(t, "flight.tlog", buildTLOGWorkspaceFixture())
}

func TestAccumulateGPSGaps(t *testing.T) {
	var gps health.GPSInput
	accumulateGPSGaps(&gps, []uint64{1_000_000, 2_000_000, 3_000_000, 20_000_000, 21_000_000})
	if gps.GapEvents != 1 || gps.MaxGapSeconds != 17 {
		t.Fatalf("unexpected gap detection: %+v", gps)
	}
}

func TestFlightHealthRejectsUnsupportedFormat(t *testing.T) {
	handler, _ := newHandler(t.TempDir())
	rr := postFile(t, handler, "/api/v1/health", "flight.gpx", "<gpx></gpx>")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func assertFlightHealth(t *testing.T, filename, fixture string) {
	t.Helper()
	handler, _ := newHandler(t.TempDir())
	rr := postFile(t, handler, "/api/v1/health", filename, fixture)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var result health.Result
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Algorithm != "m3.3-deterministic-v2" {
		t.Fatalf("algorithm = %q", result.Algorithm)
	}
	if result.Score != 100 || result.Status != "good" {
		t.Fatalf("unexpected health result: %+v", result)
	}
	if result.GPS.Score != 100 || result.Data.Score != 100 || result.Battery.Score != 100 {
		t.Fatalf("unexpected component scores: %+v", result)
	}
}
