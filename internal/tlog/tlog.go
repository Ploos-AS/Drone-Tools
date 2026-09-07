package tlog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
)

const (
	mavlinkV1Magic = 0xFE
	mavlinkV2Magic = 0xFD
	signedFlag     = 0x01
)

var (
	ErrInvalidLog = errors.New("invalid MAVLink TLOG")
	ErrTruncated  = errors.New("truncated MAVLink TLOG record")
)

type Endpoint struct {
	SystemID    uint8 `json:"system_id"`
	ComponentID uint8 `json:"component_id"`
}

type Summary struct {
	Format           string         `json:"format"`
	Records          int            `json:"records"`
	MAVLink1Records  int            `json:"mavlink1_records"`
	MAVLink2Records  int            `json:"mavlink2_records"`
	SignedRecords    int            `json:"signed_records"`
	StartTimestampUS uint64         `json:"start_timestamp_us,omitempty"`
	EndTimestampUS   uint64         `json:"end_timestamp_us,omitempty"`
	MessageIDs       map[uint32]int `json:"message_ids"`
	Endpoints        []Endpoint     `json:"endpoints,omitempty"`
}

func Inspect(r io.Reader) (Summary, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Summary{}, err
	}
	if len(data) < 9 {
		return Summary{}, ErrInvalidLog
	}

	s := Summary{Format: "mavlink-tlog", MessageIDs: map[uint32]int{}}
	endpoints := map[uint16]Endpoint{}

	for offset := 0; offset < len(data); {
		if len(data)-offset < 9 {
			return Summary{}, ErrTruncated
		}
		timestamp := binary.BigEndian.Uint64(data[offset : offset+8])
		offset += 8

		frameLength, msgID, sysID, compID, signed, version, err := frameInfo(data[offset:])
		if err != nil {
			return Summary{}, err
		}
		if frameLength > len(data)-offset {
			return Summary{}, ErrTruncated
		}

		s.Records++
		if s.Records == 1 {
			s.StartTimestampUS = timestamp
		}
		s.EndTimestampUS = timestamp
		if version == 1 {
			s.MAVLink1Records++
		} else {
			s.MAVLink2Records++
		}
		if signed {
			s.SignedRecords++
		}
		s.MessageIDs[msgID]++
		key := uint16(sysID)<<8 | uint16(compID)
		endpoints[key] = Endpoint{SystemID: sysID, ComponentID: compID}
		offset += frameLength
	}

	if s.Records == 0 {
		return Summary{}, ErrInvalidLog
	}
	for _, endpoint := range endpoints {
		s.Endpoints = append(s.Endpoints, endpoint)
	}
	sort.Slice(s.Endpoints, func(i, j int) bool {
		if s.Endpoints[i].SystemID == s.Endpoints[j].SystemID {
			return s.Endpoints[i].ComponentID < s.Endpoints[j].ComponentID
		}
		return s.Endpoints[i].SystemID < s.Endpoints[j].SystemID
	})
	return s, nil
}

func frameInfo(data []byte) (length int, msgID uint32, sysID, compID uint8, signed bool, version int, err error) {
	if len(data) == 0 {
		return 0, 0, 0, 0, false, 0, ErrTruncated
	}
	switch data[0] {
	case mavlinkV1Magic:
		if len(data) < 2 {
			return 0, 0, 0, 0, false, 0, ErrTruncated
		}
		payloadLength := int(data[1])
		length = 8 + payloadLength
		if len(data) < length || len(data) < 6 {
			return 0, 0, 0, 0, false, 0, ErrTruncated
		}
		return length, uint32(data[5]), data[3], data[4], false, 1, nil
	case mavlinkV2Magic:
		if len(data) < 2 {
			return 0, 0, 0, 0, false, 0, ErrTruncated
		}
		payloadLength := int(data[1])
		if len(data) < 10 {
			return 0, 0, 0, 0, false, 0, ErrTruncated
		}
		signed = data[2]&signedFlag != 0
		length = 12 + payloadLength
		if signed {
			length += 13
		}
		if len(data) < length {
			return 0, 0, 0, 0, false, 0, ErrTruncated
		}
		msgID = uint32(data[7]) | uint32(data[8])<<8 | uint32(data[9])<<16
		return length, msgID, data[5], data[6], signed, 2, nil
	default:
		return 0, 0, 0, 0, false, 0, fmt.Errorf("%w: unknown MAVLink magic 0x%02x", ErrInvalidLog, data[0])
	}
}
