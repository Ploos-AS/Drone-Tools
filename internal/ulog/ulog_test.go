package ulog

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestInspect(t *testing.T) {
	var b bytes.Buffer
	b.Write([]byte{'U', 'L', 'o', 'g', 0x01, 0x12, 0x35, 1})
	_ = binary.Write(&b, binary.LittleEndian, uint64(123456))
	writeMessage(&b, 'F', []byte("vehicle_gps_position:uint64_t timestamp;int32_t lat;int32_t lon;int32_t alt;float vel_m_s;uint8_t fix_type;uint8_t satellites_used;"))
	writeMessage(&b, 'A', append([]byte{0, 7, 0}, []byte("vehicle_gps_position")...))

	var data bytes.Buffer
	_ = binary.Write(&data, binary.LittleEndian, uint16(7))
	_ = binary.Write(&data, binary.LittleEndian, uint64(2000000))
	_ = binary.Write(&data, binary.LittleEndian, int32(580000000))
	_ = binary.Write(&data, binary.LittleEndian, int32(70000000))
	_ = binary.Write(&data, binary.LittleEndian, int32(123450))
	_ = binary.Write(&data, binary.LittleEndian, float32(12.5))
	data.WriteByte(3)
	data.WriteByte(14)
	writeMessage(&b, 'D', data.Bytes())

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if s.Format != "px4-ulog" || s.Version != 1 || s.StartTimestamp != 123456 {
		t.Fatalf("unexpected header summary: %+v", s)
	}
	if s.Messages != 3 || s.LoggedDataCount != 1 || len(s.FormatNames) != 1 || len(s.Subscriptions) != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if len(s.Telemetry.GPS) != 1 {
		t.Fatalf("expected one GPS sample: %+v", s.Telemetry)
	}
	gps := s.Telemetry.GPS[0]
	if math.Abs(gps.Latitude-58) > 1e-9 || math.Abs(gps.Longitude-7) > 1e-9 {
		t.Fatalf("unexpected coordinates: %+v", gps)
	}
	if math.Abs(gps.AltitudeMeters-123.45) > 1e-6 || math.Abs(gps.VelocityMPS-12.5) > 1e-6 {
		t.Fatalf("unexpected GPS metrics: %+v", gps)
	}
	if gps.FixType != 3 || gps.SatellitesUsed != 14 || gps.TimestampUS != 2000000 {
		t.Fatalf("unexpected GPS metadata: %+v", gps)
	}
}

func TestLocalPositionAndBattery(t *testing.T) {
	var b bytes.Buffer
	b.Write([]byte{'U', 'L', 'o', 'g', 0x01, 0x12, 0x35, 1})
	_ = binary.Write(&b, binary.LittleEndian, uint64(1))
	writeMessage(&b, 'F', []byte("vehicle_local_position:uint64_t timestamp;float x;float y;float z;float vx;float vy;float vz;"))
	writeMessage(&b, 'F', []byte("battery_status:uint64_t timestamp;float voltage_v;float current_a;float remaining;"))
	writeMessage(&b, 'A', append([]byte{0, 10, 0}, []byte("vehicle_local_position")...))
	writeMessage(&b, 'A', append([]byte{0, 11, 0}, []byte("battery_status")...))

	var local bytes.Buffer
	_ = binary.Write(&local, binary.LittleEndian, uint16(10))
	_ = binary.Write(&local, binary.LittleEndian, uint64(3))
	for _, v := range []float32{1, 2, -3, 4, 5, -6} {
		_ = binary.Write(&local, binary.LittleEndian, v)
	}
	writeMessage(&b, 'D', local.Bytes())

	var battery bytes.Buffer
	_ = binary.Write(&battery, binary.LittleEndian, uint16(11))
	_ = binary.Write(&battery, binary.LittleEndian, uint64(4))
	for _, v := range []float32{15.8, 7.2, 0.64} {
		_ = binary.Write(&battery, binary.LittleEndian, v)
	}
	writeMessage(&b, 'D', battery.Bytes())

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Telemetry.LocalPosition) != 1 || len(s.Telemetry.Battery) != 1 {
		t.Fatalf("unexpected telemetry: %+v", s.Telemetry)
	}
	if s.Telemetry.LocalPosition[0].Z != -3 || s.Telemetry.LocalPosition[0].VZ != -6 {
		t.Fatalf("unexpected local position: %+v", s.Telemetry.LocalPosition[0])
	}
	bat := s.Telemetry.Battery[0]
	if math.Abs(bat.VoltageV-15.8) > 1e-5 || math.Abs(bat.CurrentA-7.2) > 1e-5 || math.Abs(bat.Remaining-0.64) > 1e-5 {
		t.Fatalf("unexpected battery sample: %+v", bat)
	}
}

func TestInvalidHeader(t *testing.T) {
	if _, err := Inspect(bytes.NewReader([]byte("not-ulog"))); err == nil {
		t.Fatal("expected invalid header error")
	}
}

func writeMessage(b *bytes.Buffer, typ byte, payload []byte) {
	_ = binary.Write(b, binary.LittleEndian, uint16(len(payload)))
	b.WriteByte(typ)
	b.Write(payload)
}
