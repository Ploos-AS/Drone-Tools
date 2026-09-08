package tlog

import (
	"bytes"
	"testing"
)

func TestInspectDetailedClassifiesHeartbeatRoles(t *testing.T) {
	var b bytes.Buffer
	writeRecord(&b, 1_000_000, heartbeatV1Frame(1, 1, 2, 12))
	writeRecord(&b, 1_100_000, heartbeatV1Frame(1, 190, 6, mavAutopilotInvalid))
	writeRecord(&b, 1_200_000, heartbeatV1Frame(1, 191, 18, mavAutopilotInvalid))

	detailed, err := InspectDetailed(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(detailed.EndpointRoles) != 3 {
		t.Fatalf("roles = %d, want 3: %+v", len(detailed.EndpointRoles), detailed.EndpointRoles)
	}
	want := map[uint8]string{1: "flight-controller", 190: "gcs", 191: "onboard-controller"}
	for _, role := range detailed.EndpointRoles {
		if got := want[role.ComponentID]; got != role.Role {
			t.Fatalf("component %d role = %q, want %q", role.ComponentID, role.Role, got)
		}
	}
	if !IsFlightController(detailed.EndpointRoles, 1, 1) {
		t.Fatal("expected system 1 component 1 to be flight controller")
	}
}

func heartbeatV1Frame(sysID, compID, mavType, autopilot byte) []byte {
	payload := make([]byte, 9)
	payload[4] = mavType
	payload[5] = autopilot
	payload[8] = 3
	frame := []byte{mavlinkV1Magic, byte(len(payload)), 1, sysID, compID, msgHeartbeat}
	frame = append(frame, payload...)
	crc := uint16(0xffff)
	for _, b := range frame[1:] {
		crc = x25Accumulate(crc, b)
	}
	crc = x25Accumulate(crc, heartbeatCRCExtra)
	return append(frame, byte(crc), byte(crc>>8))
}
