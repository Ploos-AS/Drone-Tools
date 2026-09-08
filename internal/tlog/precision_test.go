package tlog

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodeGPSRawPrecision(t *testing.T) {
	payload := make([]byte, 30)
	binary.LittleEndian.PutUint64(payload[0:8], 1_000_000)
	payload[8] = 3
	binary.LittleEndian.PutUint32(payload[9:13], uint32(int32(580000000)))
	binary.LittleEndian.PutUint32(payload[13:17], uint32(int32(70000000)))
	binary.LittleEndian.PutUint32(payload[17:21], uint32(int32(100000)))
	binary.LittleEndian.PutUint16(payload[21:23], 180)
	binary.LittleEndian.PutUint16(payload[23:25], 320)
	payload[29] = 12

	var telemetry Telemetry
	decodeCoreTelemetry(&telemetry, msgGPSRawInt, 1_000_000, 2, payload)
	if len(telemetry.GPS) != 1 {
		t.Fatalf("GPS samples = %d, want 1", len(telemetry.GPS))
	}
	sample := telemetry.GPS[0]
	if sample.HDOP == nil || sample.VDOP == nil {
		t.Fatalf("precision missing: %+v", sample)
	}
	if math.Abs(*sample.HDOP-1.8) > 1e-9 || math.Abs(*sample.VDOP-3.2) > 1e-9 {
		t.Fatalf("unexpected precision: hdop=%v vdop=%v", *sample.HDOP, *sample.VDOP)
	}
}

func TestDecodeGPSRawUnknownPrecision(t *testing.T) {
	payload := make([]byte, 30)
	payload[8] = 3
	binary.LittleEndian.PutUint32(payload[9:13], uint32(int32(580000000)))
	binary.LittleEndian.PutUint32(payload[13:17], uint32(int32(70000000)))
	binary.LittleEndian.PutUint16(payload[21:23], 0xffff)
	binary.LittleEndian.PutUint16(payload[23:25], 0xffff)

	var telemetry Telemetry
	decodeCoreTelemetry(&telemetry, msgGPSRawInt, 1_000_000, 2, payload)
	if len(telemetry.GPS) != 1 {
		t.Fatalf("GPS samples = %d, want 1", len(telemetry.GPS))
	}
	if telemetry.GPS[0].HDOP != nil || telemetry.GPS[0].VDOP != nil {
		t.Fatalf("unknown precision should be omitted: %+v", telemetry.GPS[0])
	}
}
