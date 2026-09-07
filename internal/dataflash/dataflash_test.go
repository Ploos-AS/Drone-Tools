package dataflash

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

func TestInspect(t *testing.T) {
	var b bytes.Buffer
	b.Write(formatFrame(42, 7, "GPS", "QBLL", "TimeUS,Lat,Lng"))
	b.Write([]byte{head1, head2, 42, 1, 2, 3, 4})

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if s.Format != "ardupilot-dataflash" || s.Messages != 2 || len(s.Formats) != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if s.Formats[0].Type != 42 || s.Formats[0].Name != "GPS" || s.MessageTypes["GPS"] != 1 {
		t.Fatalf("unexpected format/message counts: %+v", s)
	}
}

func TestTelemetryDecode(t *testing.T) {
	var b bytes.Buffer
	b.Write(formatFrame(42, 29, "GPS", "QLLffBB", "TimeUS,Lat,Lng,Alt,Spd,Status,NSats"))
	var gps bytes.Buffer
	gps.Write([]byte{head1, head2, 42})
	_ = binary.Write(&gps, binary.LittleEndian, uint64(2000000))
	_ = binary.Write(&gps, binary.LittleEndian, int32(580000000))
	_ = binary.Write(&gps, binary.LittleEndian, int32(70000000))
	_ = binary.Write(&gps, binary.LittleEndian, float32(123.0))
	_ = binary.Write(&gps, binary.LittleEndian, float32(12.5))
	gps.WriteByte(3)
	gps.WriteByte(11)
	b.Write(gps.Bytes())

	b.Write(formatFrame(43, 20, "BAT", "QffB", "TimeUS,Volt,Curr,RemPct"))
	var bat bytes.Buffer
	bat.Write([]byte{head1, head2, 43})
	_ = binary.Write(&bat, binary.LittleEndian, uint64(2000000))
	_ = binary.Write(&bat, binary.LittleEndian, float32(15.2))
	_ = binary.Write(&bat, binary.LittleEndian, float32(6.5))
	bat.WriteByte(75)
	b.Write(bat.Bytes())

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Telemetry.GPS) != 1 || len(s.Telemetry.Battery) != 1 {
		t.Fatalf("unexpected telemetry: %+v", s.Telemetry)
	}
	g := s.Telemetry.GPS[0]
	if g.Latitude != 58 || g.Longitude != 7 || math.Abs(g.AltitudeMeters-123) > 0.001 || math.Abs(g.SpeedMPS-12.5) > 0.001 || g.Status != 3 || g.Satellites != 11 {
		t.Fatalf("unexpected GPS sample: %+v", g)
	}
	batSample := s.Telemetry.Battery[0]
	if math.Abs(batSample.VoltageV-15.2) > 0.001 || math.Abs(batSample.CurrentA-6.5) > 0.001 || batSample.Remaining != 0.75 {
		t.Fatalf("unexpected battery sample: %+v", batSample)
	}
}

func TestRealisticGPS2BAT2VariantAndResync(t *testing.T) {
	var b bytes.Buffer
	b.Write([]byte{0x00, 0xff, head1, head2, 77, 0x42})
	b.Write(formatFrame(52, 35, "GPS2", "QBBHLLfff", "TimeUS,Status,NSats,HDop,Lat,Lng,Alt,Spd,VZ"))
	b.Write(formatFrame(53, 21, "BAT2", "QBffB", "TimeUS,Inst,VoltR,Curr,RemPct"))

	var gps bytes.Buffer
	gps.Write([]byte{head1, head2, 52})
	_ = binary.Write(&gps, binary.LittleEndian, uint64(9000000))
	gps.WriteByte(3)
	gps.WriteByte(18)
	_ = binary.Write(&gps, binary.LittleEndian, uint16(92))
	_ = binary.Write(&gps, binary.LittleEndian, int32(581234567))
	_ = binary.Write(&gps, binary.LittleEndian, int32(71234567))
	_ = binary.Write(&gps, binary.LittleEndian, float32(245.5))
	_ = binary.Write(&gps, binary.LittleEndian, float32(18.25))
	_ = binary.Write(&gps, binary.LittleEndian, float32(-1.5))
	b.Write(gps.Bytes())

	var bat bytes.Buffer
	bat.Write([]byte{head1, head2, 53})
	_ = binary.Write(&bat, binary.LittleEndian, uint64(9000000))
	bat.WriteByte(1)
	_ = binary.Write(&bat, binary.LittleEndian, float32(22.4))
	_ = binary.Write(&bat, binary.LittleEndian, float32(8.75))
	bat.WriteByte(63)
	b.Write(bat.Bytes())

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if s.UnknownFrames == 0 {
		t.Fatalf("expected resync accounting: %+v", s)
	}
	if len(s.Telemetry.GPS) != 1 || len(s.Telemetry.Battery) != 1 {
		t.Fatalf("unexpected variant telemetry: %+v", s.Telemetry)
	}
	g := s.Telemetry.GPS[0]
	if math.Abs(g.Latitude-58.1234567) > 1e-7 || math.Abs(g.Longitude-7.1234567) > 1e-7 || g.Status != 3 || g.Satellites != 18 || math.Abs(g.SpeedMPS-18.25) > 1e-5 {
		t.Fatalf("unexpected GPS2 sample: %+v", g)
	}
	battery := s.Telemetry.Battery[0]
	if math.Abs(battery.VoltageV-22.4) > 1e-5 || math.Abs(battery.CurrentA-8.75) > 1e-5 || math.Abs(battery.Remaining-0.63) > 1e-9 {
		t.Fatalf("unexpected BAT2 sample: %+v", battery)
	}
}

func TestInvalidLog(t *testing.T) {
	if _, err := Inspect(bytes.NewReader([]byte("not-dataflash"))); err == nil {
		t.Fatal("expected invalid log error")
	}
}

func TestTruncatedFrame(t *testing.T) {
	data := formatFrame(42, 7, "GPS", "QBLL", "TimeUS,Lat,Lng")
	data = append(data, head1, head2, 42, 1)
	if _, err := Inspect(bytes.NewReader(data)); !errorsIs(err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func formatFrame(typ, length byte, name, format, columns string) []byte {
	frame := make([]byte, formatMsgLen)
	frame[0], frame[1], frame[2] = head1, head2, formatMsgID
	frame[3], frame[4] = typ, length
	copy(frame[5:9], name)
	copy(frame[9:25], format)
	copy(frame[25:89], columns)
	return frame
}

func errorsIs(err, target error) bool {
	return err == target
}
