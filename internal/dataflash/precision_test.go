package dataflash

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodeArduPilotGPSHDOP(t *testing.T) {
	format := Format{Name: "GPS", Format: "QLLffBBf", Columns: "TimeUS,Lat,Lng,Alt,Spd,Status,NSats,HDop"}
	var payload bytes.Buffer
	_ = binary.Write(&payload, binary.LittleEndian, uint64(1_000_000))
	_ = binary.Write(&payload, binary.LittleEndian, int32(580000000))
	_ = binary.Write(&payload, binary.LittleEndian, int32(70000000))
	_ = binary.Write(&payload, binary.LittleEndian, float32(100))
	_ = binary.Write(&payload, binary.LittleEndian, float32(10))
	payload.WriteByte(3)
	payload.WriteByte(12)
	_ = binary.Write(&payload, binary.LittleEndian, float32(1.8))

	var telemetry Telemetry
	decodeCoreTelemetry(&telemetry, format, payload.Bytes())
	if len(telemetry.GPS) != 1 {
		t.Fatalf("GPS samples = %d, want 1", len(telemetry.GPS))
	}
	sample := telemetry.GPS[0]
	if sample.HDOP == nil {
		t.Fatalf("HDOP missing: %+v", sample)
	}
	if math.Abs(*sample.HDOP-1.8) > 1e-6 {
		t.Fatalf("HDOP = %v, want 1.8", *sample.HDOP)
	}
}
