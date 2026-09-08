package main

import (
	"testing"

	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
)

func TestTLOGBatteryUsesSelectedSystemAcrossComponents(t *testing.T) {
	telemetry := tlog.Telemetry{
		GPS: []tlog.GPSSample{
			{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1, Latitude: 58, Longitude: 7},
			{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 2, Latitude: 58.001, Longitude: 7.001},
		},
		Battery: []tlog.BatterySample{
			{Source: "BATTERY_STATUS", SystemID: 1, ComponentID: 180, TimestampUS: 1, VoltageV: 15.2, Remaining: 0.8},
			{Source: "SYS_STATUS", SystemID: 1, ComponentID: 191, TimestampUS: 2, VoltageV: 15.0, Remaining: 0.7},
			{Source: "BATTERY_STATUS", SystemID: 2, ComponentID: 180, TimestampUS: 3, VoltageV: 9.0, Remaining: 0.1},
		},
	}

	input := healthInputFromTLOG(telemetry)
	if input.Battery.Samples != 2 {
		t.Fatalf("battery samples = %d, want 2", input.Battery.Samples)
	}
	if input.Battery.MinVoltageV == nil || *input.Battery.MinVoltageV != 15.0 {
		t.Fatalf("unexpected minimum voltage: %+v", input.Battery.MinVoltageV)
	}
	if input.Battery.EndRemaining == nil || *input.Battery.EndRemaining != 0.7 {
		t.Fatalf("unexpected remaining: %+v", input.Battery.EndRemaining)
	}
}

func TestTLOGBatteryDoesNotFallbackToForeignSystem(t *testing.T) {
	telemetry := tlog.Telemetry{
		GPS: []tlog.GPSSample{
			{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 1, Latitude: 58, Longitude: 7},
			{Source: "GLOBAL_POSITION_INT", SystemID: 1, ComponentID: 1, TimestampUS: 2, Latitude: 58.001, Longitude: 7.001},
		},
		Battery: []tlog.BatterySample{
			{Source: "BATTERY_STATUS", SystemID: 2, ComponentID: 180, TimestampUS: 3, VoltageV: 9.0, Remaining: 0.1},
		},
	}

	input := healthInputFromTLOG(telemetry)
	if input.Battery.Samples != 0 {
		t.Fatalf("foreign battery contaminated selected vehicle: %+v", input.Battery)
	}
	if input.Battery.MinVoltageV != nil || input.Battery.EndRemaining != nil {
		t.Fatalf("foreign battery metrics leaked into selected vehicle: %+v", input.Battery)
	}
}
