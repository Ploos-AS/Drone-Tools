package ulog

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestInspect(t *testing.T) {
	var b bytes.Buffer
	b.Write([]byte{'U', 'L', 'o', 'g', 0x01, 0x12, 0x35, 1})
	_ = binary.Write(&b, binary.LittleEndian, uint64(123456))
	writeMessage(&b, 'F', []byte("vehicle_gps_position:uint64_t timestamp;double lat;double lon;"))
	writeMessage(&b, 'A', append([]byte{0, 7, 0}, []byte("vehicle_gps_position")...))
	writeMessage(&b, 'D', []byte{7, 0, 1, 2, 3})

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
	if s.Subscriptions[0].MessageID != 7 || s.Subscriptions[0].Name != "vehicle_gps_position" {
		t.Fatalf("unexpected subscription: %+v", s.Subscriptions[0])
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
