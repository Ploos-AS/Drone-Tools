package main

import (
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
)

func TestTLOGTrackPrefersHeartbeatFlightController(t *testing.T) {
	samples := []tlog.GPSSample{
		{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1, Latitude: 58.0, Longitude: 7.0},
		{Source: "GLOBAL_POSITION_INT", SystemID: 2, ComponentID: 191, TimestampUS: 1, Latitude: 60.0, Longitude: 8.0},
		{Source: "GLOBAL_POSITION_INT", SystemID: 2, ComponentID: 191, TimestampUS: 2, Latitude: 60.1, Longitude: 8.1},
		{Source: "GLOBAL_POSITION_INT", SystemID: 2, ComponentID: 191, TimestampUS: 3, Latitude: 60.2, Longitude: 8.2},
	}
	roles := []tlog.EndpointRole{
		{SystemID: 1, ComponentID: 1, MAVType: 2, Autopilot: 12, Role: "flight-controller"},
		{SystemID: 2, ComponentID: 191, MAVType: 18, Autopilot: 8, Role: "onboard-controller"},
	}
	track := tlogTrackSamplesWithRoles(samples, roles)
	if len(track) != 1 {
		t.Fatalf("track len = %d, want 1", len(track))
	}
	if track[0].SystemID != 1 || track[0].ComponentID != 1 {
		t.Fatalf("selected endpoint = %d/%d, want 1/1", track[0].SystemID, track[0].ComponentID)
	}
}

func TestTLOGTrackFallsBackWithoutHeartbeatRoles(t *testing.T) {
	samples := []tlog.GPSSample{
		{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1, Latitude: 58.0, Longitude: 7.0},
		{Source: "GLOBAL_POSITION_INT", SystemID: 2, ComponentID: 191, TimestampUS: 1, Latitude: 60.0, Longitude: 8.0},
		{Source: "GLOBAL_POSITION_INT", SystemID: 2, ComponentID: 191, TimestampUS: 2, Latitude: 60.1, Longitude: 8.1},
	}
	track := tlogTrackSamplesWithRoles(samples, nil)
	if len(track) != 2 || track[0].SystemID != 2 {
		t.Fatalf("unexpected fallback track: %+v", track)
	}
}
