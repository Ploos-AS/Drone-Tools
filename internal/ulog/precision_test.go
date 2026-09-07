package ulog

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestDecodePX4GPSAccuracy(t *testing.T) {
	def, ok := parseFormat("vehicle_gps_position:uint64_t timestamp;int32_t lat;int32_t lon;int32_t alt;float vel_m_s;uint8_t fix_type;uint8_t satellites_used;float eph;float epv;")
	if !ok {
		t.Fatal("format did not parse")
	}
	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, uint64(1_000_000))
	_ = binary.Write(&data, binary.LittleEndian, int32(580000000))
	_ = binary.Write(&data, binary.LittleEndian, int32(70000000))
	_ = binary.Write(&data, binary.LittleEndian, int32(100000))
	_ = binary.Write(&data, binary.LittleEndian, float32(10))
	data.WriteByte(3)
	data.WriteByte(12)
	_ = binary.Write(&data, binary.LittleEndian, float32(1.25))
	_ = binary.Write(&data, binary.LittleEndian, float32(2.5))

	var telemetry Telemetry
	decodeCoreTelemetry(&telemetry, "vehicle_gps_position", def, data.Bytes())
	if len(telemetry.GPS) != 1 {
		t.Fatalf("GPS samples = %d, want 1", len(telemetry.GPS))
	}
	sample := telemetry.GPS[0]
	if sample.HorizontalAccuracyM == nil || sample.VerticalAccuracyM == nil {
		t.Fatalf("accuracy missing: %+v", sample)
	}
	if math.Abs(*sample.HorizontalAccuracyM-1.25) > 1e-6 || math.Abs(*sample.VerticalAccuracyM-2.5) > 1e-6 {
		t.Fatalf("unexpected accuracy: h=%v v=%v", *sample.HorizontalAccuracyM, *sample.VerticalAccuracyM)
	}
}
