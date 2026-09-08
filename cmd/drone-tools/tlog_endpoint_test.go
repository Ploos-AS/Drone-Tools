package main

import (
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
)

func TestTLOGTrackSelectsSingleEndpoint(t *testing.T) {
	samples := []tlog.GPSSample{
		{Source: "GLOBAL_POSITION_INT", SystemID: 2, ComponentID: 1, TimestampUS: 1, Latitude: 58.0, Longitude: 7.0},
		{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1, Latitude: 60.0, Longitude: 8.0},
		{Source: "GLOBAL_POSITION_INT", SystemID: 2, ComponentID: 1, TimestampUS: 2, Latitude: 58.1, Longitude: 7.1},
	}
	track := tlogTrackSamples(samples)
	if len(track) != 2 {
		t.Fatalf("track len = %d, want 2", len(track))
	}
	for _, sample := range track {
		if sample.SystemID != 2 || sample.ComponentID != 1 {
			t.Fatalf("mixed endpoint in track: %+v", track)
		}
	}
}

func TestTLOGTrackEndpointTieBreakIsDeterministic(t *testing.T) {
	samples := []tlog.GPSSample{
		{Source: "GPS_RAW_INT", SystemID: 3, ComponentID: 2, TimestampUS: 2, Latitude: 58.2, Longitude: 7.2},
		{Source: "GPS_RAW_INT", SystemID: 2, ComponentID: 9, TimestampUS: 1, Latitude: 58.1, Longitude: 7.1},
	}
	track := tlogTrackSamples(samples)
	if len(track) != 1 {
		t.Fatalf("track len = %d, want 1", len(track))
	}
	if track[0].SystemID != 2 || track[0].ComponentID != 9 {
		t.Fatalf("tie-break endpoint = %d/%d, want 2/9", track[0].SystemID, track[0].ComponentID)
	}
}

func TestTLOGAnalysisUsesSelectedEndpointForQualityAndBattery(t *testing.T) {
	telemetry := tlog.Telemetry{
		GPS: []tlog.GPSSample{
			{Source: "GPS_RAW_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1, Latitude: 58.0, Longitude: 7.0, AltitudeMeters: 100, FixType: 3, Satellites: 12},
			{Source: "GPS_RAW_INT", SystemID: 2, ComponentID: 1, TimestampUS: 2, Latitude: 60.0, Longitude: 8.0, AltitudeMeters: 200, FixType: 1, Satellites: 2},
			{Source: "GPS_RAW_INT", SystemID: 1, ComponentID: 1, TimestampUS: 3, Latitude: 58.1, Longitude: 7.1, AltitudeMeters: 101, FixType: 3, Satellites: 11},
		},
		Battery: []tlog.BatterySample{
			{Source: "BATTERY_STATUS", SystemID: 1, ComponentID: 1, TimestampUS: 3, VoltageV: 15.2, Remaining: 0.8},
			{Source: "BATTERY_STATUS", SystemID: 2, ComponentID: 1, TimestampUS: 4, VoltageV: 9.0, Remaining: 0.1},
		},
	}
	analysis := analyzeTLOG(telemetry)
	if analysis.Quality != "good" {
		t.Fatalf("foreign endpoint contaminated quality: %+v", analysis)
	}
	if analysis.TrackPoints != 2 {
		t.Fatalf("track points = %d, want 2", analysis.TrackPoints)
	}
	if analysis.BatteryVoltageV == nil || *analysis.BatteryVoltageV != 15.2 {
		t.Fatalf("foreign endpoint contaminated battery: %+v", analysis)
	}
}
