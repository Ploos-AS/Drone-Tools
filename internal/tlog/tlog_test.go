package tlog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func TestInspectMixedMAVLinkVersions(t *testing.T) {
	var b bytes.Buffer
	writeRecord(&b, 1_000_000, mavlinkV1Frame(1, 1, 0, []byte{1, 2, 3}))
	writeRecord(&b, 2_000_000, mavlinkV2Frame(2, 1, 33, []byte{4, 5}, false))
	writeRecord(&b, 3_000_000, mavlinkV2Frame(2, 1, 24, []byte{6}, true))

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if s.Format != "mavlink-tlog" || s.Records != 3 || s.MAVLink1Records != 1 || s.MAVLink2Records != 2 || s.SignedRecords != 1 {
		t.Fatalf("unexpected summary: %+v", s)
	}
	if s.UnverifiedSignatureRecords != 1 {
		t.Fatalf("unverified signatures = %d, want 1", s.UnverifiedSignatureRecords)
	}
	if s.ChecksumValidatedRecords != 2 {
		t.Fatalf("checksum validated = %d, want 2", s.ChecksumValidatedRecords)
	}
	if s.StartTimestampUS != 1_000_000 || s.EndTimestampUS != 3_000_000 {
		t.Fatalf("unexpected timestamps: %+v", s)
	}
	if s.MessageIDs[0] != 1 || s.MessageIDs[33] != 1 || s.MessageIDs[24] != 1 {
		t.Fatalf("unexpected message counts: %+v", s.MessageIDs)
	}
	if len(s.Endpoints) != 2 || s.Endpoints[0].SystemID != 1 || s.Endpoints[1].SystemID != 2 {
		t.Fatalf("unexpected endpoints: %+v", s.Endpoints)
	}
}

func TestDecodeCoreTelemetry(t *testing.T) {
	var b bytes.Buffer

	gps := make([]byte, 30)
	binary.LittleEndian.PutUint64(gps[0:8], 2_000_000)
	gps[8] = 3
	binary.LittleEndian.PutUint32(gps[9:13], uint32(int32(580000000)))
	binary.LittleEndian.PutUint32(gps[13:17], uint32(int32(70000000)))
	binary.LittleEndian.PutUint32(gps[17:21], uint32(int32(123450)))
	binary.LittleEndian.PutUint16(gps[25:27], 1250)
	gps[29] = 14
	writeRecord(&b, 2_100_000, mavlinkV2Frame(1, 1, msgGPSRawInt, gps, false))

	pos := make([]byte, 28)
	binary.LittleEndian.PutUint32(pos[4:8], uint32(int32(580010000)))
	binary.LittleEndian.PutUint32(pos[8:12], uint32(int32(70020000)))
	binary.LittleEndian.PutUint32(pos[12:16], uint32(int32(125000)))
	binary.LittleEndian.PutUint16(pos[20:22], uint16(int16(300)))
	binary.LittleEndian.PutUint16(pos[22:24], uint16(int16(400)))
	writeRecord(&b, 3_000_000, mavlinkV2Frame(1, 1, msgGlobalPositionInt, pos, false))

	sys := make([]byte, 19)
	binary.LittleEndian.PutUint16(sys[14:16], 15200)
	binary.LittleEndian.PutUint16(sys[16:18], uint16(int16(650)))
	sys[18] = 75
	writeRecord(&b, 3_100_000, mavlinkV1Frame(1, 1, msgSysStatus, sys))

	bat := make([]byte, 36)
	for i := 0; i < 10; i++ {
		binary.LittleEndian.PutUint16(bat[10+i*2:12+i*2], 0xffff)
	}
	for i, mv := range []uint16{3800, 3790, 3810, 3800} {
		binary.LittleEndian.PutUint16(bat[10+i*2:12+i*2], mv)
	}
	binary.LittleEndian.PutUint16(bat[30:32], uint16(int16(700)))
	bat[35] = 70
	writeRecord(&b, 3_200_000, mavlinkV2Frame(1, 1, msgBatteryStatus, bat, false))

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if s.ChecksumValidatedRecords != 4 {
		t.Fatalf("checksum validated = %d, want 4", s.ChecksumValidatedRecords)
	}
	if len(s.Telemetry.GPS) != 2 || len(s.Telemetry.Battery) != 2 {
		t.Fatalf("unexpected telemetry: %+v", s.Telemetry)
	}
	g := s.Telemetry.GPS[0]
	if g.Source != "GPS_RAW_INT" || math.Abs(g.Latitude-58) > 1e-9 || math.Abs(g.Longitude-7) > 1e-9 || math.Abs(g.AltitudeMeters-123.45) > 1e-6 || math.Abs(g.SpeedMPS-12.5) > 1e-6 || g.FixType != 3 || g.Satellites != 14 || g.TimestampUS != 2_000_000 {
		t.Fatalf("unexpected GPS_RAW_INT sample: %+v", g)
	}
	p := s.Telemetry.GPS[1]
	if p.Source != "GLOBAL_POSITION_INT" || math.Abs(p.SpeedMPS-5) > 1e-9 || p.TimestampUS != 3_000_000 {
		t.Fatalf("unexpected GLOBAL_POSITION_INT sample: %+v", p)
	}
	if math.Abs(s.Telemetry.Battery[0].VoltageV-15.2) > 1e-9 || math.Abs(s.Telemetry.Battery[0].CurrentA-6.5) > 1e-9 || s.Telemetry.Battery[0].Remaining != 0.75 {
		t.Fatalf("unexpected SYS_STATUS battery: %+v", s.Telemetry.Battery[0])
	}
	if math.Abs(s.Telemetry.Battery[1].VoltageV-15.2) > 1e-9 || math.Abs(s.Telemetry.Battery[1].CurrentA-7) > 1e-9 || s.Telemetry.Battery[1].Remaining != 0.70 {
		t.Fatalf("unexpected BATTERY_STATUS battery: %+v", s.Telemetry.Battery[1])
	}
}

func TestResyncsAfterStructuralGarbage(t *testing.T) {
	var b bytes.Buffer
	writeRecord(&b, 1_000_000, mavlinkV1Frame(1, 1, 0, []byte{1, 2, 3}))
	b.Write([]byte{0xde, 0xad, 0xbe, 0xef, 0x42})
	writeRecord(&b, 2_000_000, mavlinkV1Frame(1, 1, 0, []byte{4, 5, 6}))

	s, err := Inspect(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if s.Records != 2 || s.ResyncEvents != 1 || s.SkippedBytes != 5 {
		t.Fatalf("unexpected resync summary: %+v", s)
	}
}

func TestRejectsInvalidCoreChecksum(t *testing.T) {
	frame := mavlinkV1Frame(1, 1, msgGPSRawInt, make([]byte, 30))
	frame[len(frame)-1] ^= 0xff
	var b bytes.Buffer
	writeRecord(&b, 1_000_000, frame)
	if _, err := Inspect(bytes.NewReader(b.Bytes())); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
}

func TestDoesNotResyncPastInvalidCoreChecksum(t *testing.T) {
	bad := mavlinkV1Frame(1, 1, msgGPSRawInt, make([]byte, 30))
	bad[len(bad)-1] ^= 0xff
	var b bytes.Buffer
	writeRecord(&b, 1_000_000, bad)
	writeRecord(&b, 2_000_000, mavlinkV1Frame(1, 1, 0, []byte{1, 2, 3}))
	if _, err := Inspect(bytes.NewReader(b.Bytes())); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
}

func TestRejectsUnsupportedMAVLink2IncompatFlags(t *testing.T) {
	frame := mavlinkV2Frame(1, 1, 0, []byte{1, 2, 3}, false)
	frame[2] = 0x02
	var b bytes.Buffer
	writeRecord(&b, 1_000_000, frame)
	if _, err := Inspect(bytes.NewReader(b.Bytes())); !errors.Is(err, ErrUnsupportedIncompatFlag) {
		t.Fatalf("err = %v, want ErrUnsupportedIncompatFlag", err)
	}
}

func TestInvalidMagic(t *testing.T) {
	data := make([]byte, 9)
	binary.BigEndian.PutUint64(data[:8], 1)
	data[8] = 0xAA
	if _, err := Inspect(bytes.NewReader(data)); !errors.Is(err, ErrInvalidLog) {
		t.Fatalf("err = %v, want ErrInvalidLog", err)
	}
}

func TestTruncatedRecord(t *testing.T) {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, uint64(1))
	b.Write([]byte{mavlinkV2Magic, 10, 0, 0, 0, 1, 1, 0, 0, 0})
	if _, err := Inspect(bytes.NewReader(b.Bytes())); !errors.Is(err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}

func writeRecord(b *bytes.Buffer, timestamp uint64, frame []byte) {
	_ = binary.Write(b, binary.BigEndian, timestamp)
	b.Write(frame)
}

func mavlinkV1Frame(sysID, compID byte, msgID uint32, payload []byte) []byte {
	frame := []byte{mavlinkV1Magic, byte(len(payload)), 1, sysID, compID, byte(msgID)}
	frame = append(frame, payload...)
	return appendTestChecksum(frame, msgID)
}

func mavlinkV2Frame(sysID, compID byte, msgID uint32, payload []byte, signed bool) []byte {
	flags := byte(0)
	if signed {
		flags = signedFlag
	}
	frame := []byte{
		mavlinkV2Magic, byte(len(payload)), flags, 0, 1, sysID, compID,
		byte(msgID), byte(msgID >> 8), byte(msgID >> 16),
	}
	frame = append(frame, payload...)
	frame = appendTestChecksum(frame, msgID)
	if signed {
		frame = append(frame, make([]byte, 13)...)
	}
	return frame
}

func appendTestChecksum(frame []byte, msgID uint32) []byte {
	extra, ok := coreCRCExtra[msgID]
	if !ok {
		return append(frame, 0, 0)
	}
	crc := uint16(0xffff)
	for _, b := range frame[1:] {
		crc = x25Accumulate(crc, b)
	}
	crc = x25Accumulate(crc, extra)
	return append(frame, byte(crc), byte(crc>>8))
}
