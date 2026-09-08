package tlog

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestTelemetryPreservesEndpointMetadata(t *testing.T) {
	gps := make([]byte, 30)
	binary.LittleEndian.PutUint64(gps[0:8], 1_000_000)
	gps[8] = 3
	binary.LittleEndian.PutUint32(gps[9:13], uint32(int32(580000000)))
	binary.LittleEndian.PutUint32(gps[13:17], uint32(int32(70000000)))
	binary.LittleEndian.PutUint32(gps[17:21], uint32(int32(100000)))

	sys := make([]byte, 19)
	binary.LittleEndian.PutUint16(sys[14:16], 15200)
	binary.LittleEndian.PutUint16(sys[16:18], uint16(int16(500)))
	sys[18] = 80

	var b bytes.Buffer
	writeRecord(&b, 1_000_000, mavlinkV2Frame(7, 42, msgGPSRawInt, gps, false))
	writeRecord(&b, 1_100_000, mavlinkV2Frame(7, 1, msgSysStatus, sys, false))

	summary, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Telemetry.GPS) != 1 || len(summary.Telemetry.Battery) != 1 {
		t.Fatalf("unexpected telemetry: %+v", summary.Telemetry)
	}
	gpsSample := summary.Telemetry.GPS[0]
	if gpsSample.SystemID != 7 || gpsSample.ComponentID != 42 {
		t.Fatalf("GPS endpoint = %d/%d, want 7/42", gpsSample.SystemID, gpsSample.ComponentID)
	}
	battery := summary.Telemetry.Battery[0]
	if battery.SystemID != 7 || battery.ComponentID != 1 {
		t.Fatalf("battery endpoint = %d/%d, want 7/1", battery.SystemID, battery.ComponentID)
	}
}
