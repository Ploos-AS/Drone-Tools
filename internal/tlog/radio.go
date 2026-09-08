package tlog

import (
	"encoding/binary"
	"fmt"
)

const (
	msgRadioStatus      = 109
	radioStatusCRCExtra = 185
)

type RadioSample struct {
	SystemID     uint8  `json:"system_id"`
	ComponentID  uint8  `json:"component_id"`
	TimestampUS  uint64 `json:"timestamp_us,omitempty"`
	RSSI         *uint8 `json:"rssi,omitempty"`
	RemoteRSSI   *uint8 `json:"remote_rssi,omitempty"`
	TxBufferPct  uint8  `json:"tx_buffer_pct"`
	Noise        *uint8 `json:"noise,omitempty"`
	RemoteNoise  *uint8 `json:"remote_noise,omitempty"`
	RxErrors     uint16 `json:"rx_errors"`
	FixedPackets uint16 `json:"fixed_packets"`
}

func inspectRadioStatus(data []byte) ([]RadioSample, error) {
	var out []RadioSample
	for offset := 0; offset < len(data); {
		if len(data)-offset < 9 {
			return nil, ErrTruncated
		}
		timestamp := binary.BigEndian.Uint64(data[offset : offset+8])
		frameStart := offset + 8
		frameLength, msgID, sysID, compID, _, version, err := frameInfo(data[frameStart:])
		if err != nil {
			next, ok := findNextRecord(data, offset+1)
			if !ok {
				return nil, err
			}
			offset = next
			continue
		}
		frame := data[frameStart : frameStart+frameLength]
		if msgID == msgRadioStatus {
			if !validFrameChecksum(frame, version, radioStatusCRCExtra) {
				return nil, fmt.Errorf("%w: message %d", ErrChecksum, msgID)
			}
			payload := framePayload(frame, version)
			if len(payload) >= 9 {
				sample := RadioSample{
					SystemID:     sysID,
					ComponentID:  compID,
					TimestampUS:  timestamp,
					RxErrors:     binary.LittleEndian.Uint16(payload[0:2]),
					FixedPackets: binary.LittleEndian.Uint16(payload[2:4]),
					TxBufferPct:  payload[6],
				}
				if payload[4] != 0xff {
					v := payload[4]
					sample.RSSI = &v
				}
				if payload[5] != 0xff {
					v := payload[5]
					sample.RemoteRSSI = &v
				}
				if payload[7] != 0xff {
					v := payload[7]
					sample.Noise = &v
				}
				if payload[8] != 0xff {
					v := payload[8]
					sample.RemoteNoise = &v
				}
				out = append(out, sample)
			}
		}
		offset = frameStart + frameLength
	}
	return out, nil
}
