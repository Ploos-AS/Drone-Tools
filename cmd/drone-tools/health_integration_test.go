package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/dataflash"
	"github.com/Ploos-AS/Drone-Tools/internal/health"
	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
	"github.com/Ploos-AS/Drone-Tools/internal/ulog"
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

func TestPrecisionThresholdsAreFormatSpecific(t *testing.T) {
	px4H, px4V := 5.1, 3.0
	px4 := healthInputFromULog(ulog.Telemetry{GPS: []ulog.GPSSample{{
		TimestampUS: 1, Latitude: 58, Longitude: 7, FixType: 3, SatellitesUsed: 12,
		HorizontalAccuracyM: &px4H, VerticalAccuracyM: &px4V,
	}}})
	if px4.GPS.PrecisionQualitySamples != 1 || px4.GPS.PoorPrecisionSamples != 1 {
		t.Fatalf("unexpected PX4 precision input: %+v", px4.GPS)
	}

	hdop := 2.6
	ardupilot := healthInputFromDataFlash(dataflash.Telemetry{GPS: []dataflash.GPSSample{{
		TimestampUS: 1, Latitude: 58, Longitude: 7, Status: 3, Satellites: 12, HDOP: &hdop,
	}}})
	if ardupilot.GPS.PrecisionQualitySamples != 1 || ardupilot.GPS.PoorPrecisionSamples != 1 {
		t.Fatalf("unexpected DataFlash precision input: %+v", ardupilot.GPS)
	}

	tlogHDOP, tlogVDOP := 2.4, 3.6
	mavlink := healthInputFromTLOG(tlog.Telemetry{GPS: []tlog.GPSSample{{
		Source: "GPS_RAW_INT", TimestampUS: 1, Latitude: 58, Longitude: 7, FixType: 3, Satellites: 12,
		HDOP: &tlogHDOP, VDOP: &tlogVDOP,
	}}})
	if mavlink.GPS.PrecisionQualitySamples != 1 || mavlink.GPS.PoorPrecisionSamples != 1 {
		t.Fatalf("unexpected TLOG precision input: %+v", mavlink.GPS)
	}
}

func TestTLOGUnknownSatellitesAreNotScored(t *testing.T) {
	input := healthInputFromTLOG(tlog.Telemetry{GPS: []tlog.GPSSample{{
		Source: "GPS_RAW_INT", TimestampUS: 1, Latitude: 58, Longitude: 7, FixType: 3, Satellites: 0xff,
	}}})
	if input.GPS.SatelliteQualitySamples != 0 || input.GPS.LowSatelliteSamples != 0 {
		t.Fatalf("unexpected satellite quality input: %+v", input.GPS)
	}
}

func TestTLOGHealthUsesSelectedTrackEndpoint(t *testing.T) {
	telemetry := tlog.Telemetry{
		GPS: []tlog.GPSSample{
			{Source: "GPS_RAW_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1, Latitude: 58.0, Longitude: 7.0, FixType: 3, Satellites: 12},
			{Source: "GPS_RAW_INT", SystemID: 2, ComponentID: 1, TimestampUS: 2, Latitude: 60.0, Longitude: 8.0, FixType: 1, Satellites: 2},
			{Source: "GPS_RAW_INT", SystemID: 1, ComponentID: 1, TimestampUS: 3, Latitude: 58.1, Longitude: 7.1, FixType: 3, Satellites: 11},
		},
		Battery: []tlog.BatterySample{
			{Source: "BATTERY_STATUS", SystemID: 1, ComponentID: 1, TimestampUS: 3, VoltageV: 15.2, Remaining: 0.8},
			{Source: "BATTERY_STATUS", SystemID: 2, ComponentID: 1, TimestampUS: 4, VoltageV: 9.0, Remaining: 0.1},
		},
	}
	input := healthInputFromTLOG(telemetry)
	if input.GPS.Samples != 2 || input.GPS.LowFixSamples != 0 || input.GPS.LowSatelliteSamples != 0 {
		t.Fatalf("foreign endpoint contaminated GPS health: %+v", input.GPS)
	}
	if input.Data.TrackSamples != 2 {
		t.Fatalf("track samples = %d, want 2", input.Data.TrackSamples)
	}
	if input.Battery.Samples != 1 || input.Battery.MinVoltageV == nil || *input.Battery.MinVoltageV != 15.2 {
		t.Fatalf("foreign endpoint contaminated battery health: %+v", input.Battery)
	}
}

func TestZeroCoordinateIsInvalidTrackData(t *testing.T) {
	var data health.DataInput
	accumulateTrackQuality(&data, 1, 0, 0, 0)
	if data.InvalidCoordinates != 1 {
		t.Fatalf("invalid coordinates = %d, want 1", data.InvalidCoordinates)
	}
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
	if result.Algorithm != "m3.4-deterministic-v3" {
		t.Fatalf("algorithm = %q", result.Algorithm)
	}
	if result.Score != 100 || result.Status != "good" {
		t.Fatalf("unexpected health result: %+v", result)
	}
	if result.GPS.Score != 100 || result.Data.Score != 100 || result.Battery.Score != 100 {
		t.Fatalf("unexpected component scores: %+v", result)
	}
}
