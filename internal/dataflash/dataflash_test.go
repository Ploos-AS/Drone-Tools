package dataflash

import (
	"bytes"
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
