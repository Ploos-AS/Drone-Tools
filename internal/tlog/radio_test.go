package tlog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestInspectDetailedDecodesRadioStatus(t *testing.T) {
	payload := make([]byte, 9)
	binary.LittleEndian.PutUint16(payload[0:2], 12)
	binary.LittleEndian.PutUint16(payload[2:4], 7)
	payload[4] = 180
	payload[5] = 170
	payload[6] = 64
	payload[7] = 40
	payload[8] = 45

	var b bytes.Buffer
	writeRecord(&b, 2_000_000, radioStatusV1Frame(3, 68, payload))
	detailed, err := InspectDetailed(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(detailed.Radio) != 1 {
		t.Fatalf("radio samples = %d, want 1", len(detailed.Radio))
	}
	r := detailed.Radio[0]
	if r.SystemID != 3 || r.ComponentID != 68 || r.TimestampUS != 2_000_000 || r.TxBufferPct != 64 || r.RxErrors != 12 || r.FixedPackets != 7 {
		t.Fatalf("unexpected radio sample: %+v", r)
	}
	if r.RSSI == nil || *r.RSSI != 180 || r.RemoteRSSI == nil || *r.RemoteRSSI != 170 || r.Noise == nil || *r.Noise != 40 || r.RemoteNoise == nil || *r.RemoteNoise != 45 {
		t.Fatalf("unexpected radio signal fields: %+v", r)
	}
}

func TestRadioStatusUnknownSignalValuesAreOmitted(t *testing.T) {
	payload := make([]byte, 9)
	payload[4], payload[5], payload[7], payload[8] = 0xff, 0xff, 0xff, 0xff
	payload[6] = 100

	var b bytes.Buffer
	writeRecord(&b, 1_000_000, radioStatusV1Frame(1, 1, payload))
	detailed, err := InspectDetailed(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	r := detailed.Radio[0]
	if r.RSSI != nil || r.RemoteRSSI != nil || r.Noise != nil || r.RemoteNoise != nil {
		t.Fatalf("unknown radio values should be omitted: %+v", r)
	}
}

func TestRadioStatusRejectsInvalidChecksum(t *testing.T) {
	frame := radioStatusV1Frame(1, 1, make([]byte, 9))
	frame[len(frame)-1] ^= 0xff
	var b bytes.Buffer
	writeRecord(&b, 1_000_000, frame)
	if _, err := InspectDetailed(bytes.NewReader(b.Bytes())); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
}

func radioStatusV1Frame(sysID, compID byte, payload []byte) []byte {
	frame := []byte{mavlinkV1Magic, byte(len(payload)), 1, sysID, compID, msgRadioStatus}
	frame = append(frame, payload...)
	crc := uint16(0xffff)
	for _, b := range frame[1:] {
		crc = x25Accumulate(crc, b)
	}
	crc = x25Accumulate(crc, radioStatusCRCExtra)
	return append(frame, byte(crc), byte(crc>>8))
}
