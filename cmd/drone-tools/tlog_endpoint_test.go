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
