package tlog

import (
	"bytes"
	"encoding/binary"
	"errors"
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

func mavlinkV1Frame(sysID, compID, msgID byte, payload []byte) []byte {
	frame := []byte{mavlinkV1Magic, byte(len(payload)), 1, sysID, compID, msgID}
	frame = append(frame, payload...)
	return append(frame, 0, 0)
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
	frame = append(frame, 0, 0)
	if signed {
		frame = append(frame, make([]byte, 13)...)
	}
	return frame
}
