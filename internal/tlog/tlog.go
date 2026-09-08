package tlog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
)

const (
	mavlinkV1Magic = 0xFE
	mavlinkV2Magic = 0xFD
	signedFlag     = 0x01

	msgSysStatus         = 1
	msgGPSRawInt         = 24
	msgGlobalPositionInt = 33
	msgBatteryStatus     = 147
)

var (
	ErrInvalidLog = errors.New("invalid MAVLink TLOG")
	ErrTruncated  = errors.New("truncated MAVLink TLOG record")
	ErrChecksum   = errors.New("invalid MAVLink checksum")
)

var coreCRCExtra = map[uint32]byte{
	msgSysStatus:         124,
	msgGPSRawInt:         24,
	msgGlobalPositionInt: 104,
	msgBatteryStatus:     154,
}

type Endpoint struct {
	SystemID    uint8 `json:"system_id"`
	ComponentID uint8 `json:"component_id"`
}

type GPSSample struct {
	Source         string   `json:"source"`
	TimestampUS    uint64   `json:"timestamp_us,omitempty"`
	Latitude       float64  `json:"latitude"`
	Longitude      float64  `json:"longitude"`
	AltitudeMeters float64  `json:"altitude_m,omitempty"`
	SpeedMPS       float64  `json:"speed_mps,omitempty"`
	FixType        uint8    `json:"fix_type,omitempty"`
	Satellites     uint8    `json:"satellites,omitempty"`
	HDOP           *float64 `json:"hdop,omitempty"`
	VDOP           *float64 `json:"vdop,omitempty"`
}

type BatterySample struct {
	Source      string  `json:"source"`
	TimestampUS uint64  `json:"timestamp_us,omitempty"`
	VoltageV    float64 `json:"voltage_v,omitempty"`
	CurrentA    float64 `json:"current_a,omitempty"`
	Remaining   float64 `json:"remaining,omitempty"`
}

type Telemetry struct {
	GPS     []GPSSample     `json:"gps,omitempty"`
	Battery []BatterySample `json:"battery,omitempty"`
}

type Summary struct {
	Format                   string         `json:"format"`
	Records                  int            `json:"records"`
	MAVLink1Records          int            `json:"mavlink1_records"`
	MAVLink2Records          int            `json:"mavlink2_records"`
	SignedRecords            int            `json:"signed_records"`
	ChecksumValidatedRecords int            `json:"checksum_validated_records"`
	StartTimestampUS         uint64         `json:"start_timestamp_us,omitempty"`
	EndTimestampUS           uint64         `json:"end_timestamp_us,omitempty"`
	MessageIDs               map[uint32]int `json:"message_ids"`
	Endpoints                []Endpoint     `json:"endpoints,omitempty"`
	Telemetry                Telemetry      `json:"telemetry"`
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
		frame := data[offset : offset+frameLength]
		if extra, ok := coreCRCExtra[msgID]; ok {
			if !validFrameChecksum(frame, version, extra) {
				return Summary{}, fmt.Errorf("%w: message %d", ErrChecksum, msgID)
			}
			s.ChecksumValidatedRecords++
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
		decodeCoreTelemetry(&s.Telemetry, msgID, timestamp, framePayload(frame, version))
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

func validFrameChecksum(frame []byte, version int, extra byte) bool {
	if len(frame) < 8 {
		return false
	}
	payloadLength := int(frame[1])
	headerLength := 6
	if version == 2 {
		headerLength = 10
	}
	checksumOffset := headerLength + payloadLength
	if checksumOffset+2 > len(frame) {
		return false
	}
	crc := uint16(0xffff)
	for _, b := range frame[1:checksumOffset] {
		crc = x25Accumulate(crc, b)
	}
	crc = x25Accumulate(crc, extra)
	return binary.LittleEndian.Uint16(frame[checksumOffset:checksumOffset+2]) == crc
}

func x25Accumulate(crc uint16, b byte) uint16 {
	tmp := b ^ byte(crc&0xff)
	tmp ^= tmp << 4
	return (crc >> 8) ^ (uint16(tmp) << 8) ^ (uint16(tmp) << 3) ^ (uint16(tmp) >> 4)
}

func framePayload(frame []byte, version int) []byte {
	if len(frame) < 2 {
		return nil
	}
	payloadLength := int(frame[1])
	start := 6
	if version == 2 {
		start = 10
	}
	if start+payloadLength > len(frame) {
		return nil
	}
	return frame[start : start+payloadLength]
}

func decodeCoreTelemetry(out *Telemetry, msgID uint32, recordTimestampUS uint64, payload []byte) {
	switch msgID {
	case msgGPSRawInt:
		if len(payload) < 30 {
			return
		}
		timestamp := binary.LittleEndian.Uint64(payload[0:8])
		if timestamp == 0 {
			timestamp = recordTimestampUS
		}
		lat := float64(int32(binary.LittleEndian.Uint32(payload[9:13]))) / 1e7
		lon := float64(int32(binary.LittleEndian.Uint32(payload[13:17]))) / 1e7
		if !validCoordinate(lat, lon) {
			return
		}
		sample := GPSSample{
			Source:         "GPS_RAW_INT",
			TimestampUS:    timestamp,
			Latitude:       lat,
			Longitude:      lon,
			AltitudeMeters: float64(int32(binary.LittleEndian.Uint32(payload[17:21]))) / 1000,
			SpeedMPS:       float64(binary.LittleEndian.Uint16(payload[25:27])) / 100,
			FixType:        payload[8],
			Satellites:     payload[29],
		}
		if eph := binary.LittleEndian.Uint16(payload[21:23]); eph != 0xffff {
			v := float64(eph) / 100
			sample.HDOP = &v
		}
		if epv := binary.LittleEndian.Uint16(payload[23:25]); epv != 0xffff {
			v := float64(epv) / 100
			sample.VDOP = &v
		}
		out.GPS = append(out.GPS, sample)
	case msgGlobalPositionInt:
		if len(payload) < 28 {
			return
		}
		lat := float64(int32(binary.LittleEndian.Uint32(payload[4:8]))) / 1e7
		lon := float64(int32(binary.LittleEndian.Uint32(payload[8:12]))) / 1e7
		if !validCoordinate(lat, lon) {
			return
		}
		vx := float64(int16(binary.LittleEndian.Uint16(payload[20:22]))) / 100
		vy := float64(int16(binary.LittleEndian.Uint16(payload[22:24]))) / 100
		out.GPS = append(out.GPS, GPSSample{
			Source:         "GLOBAL_POSITION_INT",
			TimestampUS:    recordTimestampUS,
			Latitude:       lat,
			Longitude:      lon,
			AltitudeMeters: float64(int32(binary.LittleEndian.Uint32(payload[12:16]))) / 1000,
			SpeedMPS:       math.Hypot(vx, vy),
		})
	case msgSysStatus:
		if len(payload) < 19 {
			return
		}
		voltageMV := binary.LittleEndian.Uint16(payload[14:16])
		currentCA := int16(binary.LittleEndian.Uint16(payload[16:18]))
		remainingPct := int8(payload[18])
		sample := BatterySample{Source: "SYS_STATUS", TimestampUS: recordTimestampUS}
		var have bool
		if voltageMV != 0xffff {
			sample.VoltageV = float64(voltageMV) / 1000
			have = true
		}
		if currentCA >= 0 {
			sample.CurrentA = float64(currentCA) / 100
			have = true
		}
		if remainingPct >= 0 {
			sample.Remaining = float64(remainingPct) / 100
			have = true
		}
		if have {
			out.Battery = append(out.Battery, sample)
		}
	case msgBatteryStatus:
		if len(payload) < 36 {
			return
		}
		sample := BatterySample{Source: "BATTERY_STATUS", TimestampUS: recordTimestampUS}
		var voltageMV uint64
		for i := 0; i < 10; i++ {
			cell := binary.LittleEndian.Uint16(payload[10+i*2 : 12+i*2])
			if cell == 0xffff {
				continue
			}
			voltageMV += uint64(cell)
		}
		var have bool
		if voltageMV > 0 {
			sample.VoltageV = float64(voltageMV) / 1000
			have = true
		}
		currentCA := int16(binary.LittleEndian.Uint16(payload[30:32]))
		if currentCA >= 0 {
			sample.CurrentA = float64(currentCA) / 100
			have = true
		}
		remainingPct := int8(payload[35])
		if remainingPct >= 0 {
			sample.Remaining = float64(remainingPct) / 100
			have = true
		}
		if have {
			out.Battery = append(out.Battery, sample)
		}
	}
}

func validCoordinate(lat, lon float64) bool {
	return lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
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
