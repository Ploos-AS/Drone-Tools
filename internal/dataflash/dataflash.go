package dataflash

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

const (
	head1        = 0xA3
	head2        = 0x95
	formatMsgID  = 128
	formatMsgLen = 89
)

var (
	ErrInvalidLog = errors.New("invalid ArduPilot DataFlash log")
	ErrTruncated  = errors.New("truncated ArduPilot DataFlash log")
)

type Format struct {
	Type    uint8  `json:"type"`
	Length  uint8  `json:"length"`
	Name    string `json:"name"`
	Format  string `json:"format"`
	Columns string `json:"columns"`
}

type GPSSample struct {
	TimestampUS    uint64  `json:"timestamp_us,omitempty"`
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AltitudeMeters float64 `json:"altitude_m,omitempty"`
	SpeedMPS       float64 `json:"speed_mps,omitempty"`
	Status         uint8   `json:"status,omitempty"`
	Satellites     uint8   `json:"satellites,omitempty"`
}

type BatterySample struct {
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
	Format        string         `json:"format"`
	Messages      int            `json:"messages"`
	MessageTypes  map[string]int `json:"message_types"`
	Formats       []Format       `json:"formats"`
	UnknownFrames int            `json:"unknown_frames"`
	Telemetry     Telemetry      `json:"telemetry"`
}

func Inspect(r io.Reader) (Summary, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Summary{}, err
	}
	if len(data) < 3 {
		return Summary{}, ErrInvalidLog
	}

	s := Summary{Format: "ardupilot-dataflash", MessageTypes: map[string]int{}}
	formats := map[uint8]Format{}

	for offset := 0; offset+3 <= len(data); {
		if data[offset] != head1 || data[offset+1] != head2 {
			offset++
			continue
		}

		msgID := data[offset+2]
		length := 0
		if msgID == formatMsgID {
			length = formatMsgLen
		} else if f, ok := formats[msgID]; ok {
			length = int(f.Length)
		} else {
			s.UnknownFrames++
			offset++
			continue
		}

		if length < 3 {
			return Summary{}, fmt.Errorf("%w: invalid frame length %d", ErrInvalidLog, length)
		}
		if offset+length > len(data) {
			return Summary{}, ErrTruncated
		}

		frame := data[offset : offset+length]
		s.Messages++
		if msgID == formatMsgID {
			f, err := parseFormat(frame)
			if err != nil {
				return Summary{}, err
			}
			formats[f.Type] = f
			s.MessageTypes["FMT"]++
		} else {
			f := formats[msgID]
			name := f.Name
			if name == "" {
				name = fmt.Sprintf("type-%d", msgID)
			}
			s.MessageTypes[name]++
			decodeCoreTelemetry(&s.Telemetry, f, frame[3:])
		}
		offset += length
	}

	if len(formats) == 0 {
		return Summary{}, ErrInvalidLog
	}
	for _, f := range formats {
		s.Formats = append(s.Formats, f)
	}
	sort.Slice(s.Formats, func(i, j int) bool { return s.Formats[i].Type < s.Formats[j].Type })
	return s, nil
}

func parseFormat(frame []byte) (Format, error) {
	if len(frame) != formatMsgLen || frame[0] != head1 || frame[1] != head2 || frame[2] != formatMsgID {
		return Format{}, ErrInvalidLog
	}
	f := Format{
		Type:    frame[3],
		Length:  frame[4],
		Name:    trimCString(frame[5:9]),
		Format:  trimCString(frame[9:25]),
		Columns: trimCString(frame[25:89]),
	}
	if f.Length < 3 || f.Name == "" {
		return Format{}, ErrInvalidLog
	}
	return f, nil
}

func decodeCoreTelemetry(out *Telemetry, f Format, payload []byte) {
	switch f.Name {
	case "GPS", "GPS2":
		lat, latOK := fieldNumber(f, payload, "Lat")
		lon, lonOK := fieldNumber(f, payload, "Lng")
		if !latOK || !lonOK || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return
		}
		sample := GPSSample{Latitude: lat, Longitude: lon}
		sample.TimestampUS = fieldUint64(f, payload, "TimeUS")
		if v, ok := firstFieldNumber(f, payload, "Alt", "RelAlt"); ok {
			sample.AltitudeMeters = v
		}
		if v, ok := firstFieldNumber(f, payload, "Spd", "GSpd"); ok {
			sample.SpeedMPS = v
		}
		if v, ok := firstFieldNumber(f, payload, "Status"); ok && v >= 0 {
			sample.Status = uint8(v)
		}
		if v, ok := firstFieldNumber(f, payload, "NSats", "Sats"); ok && v >= 0 {
			sample.Satellites = uint8(v)
		}
		out.GPS = append(out.GPS, sample)
	case "BAT", "BAT2":
		sample := BatterySample{TimestampUS: fieldUint64(f, payload, "TimeUS")}
		var have bool
		if v, ok := firstFieldNumber(f, payload, "Volt", "VoltR"); ok {
			sample.VoltageV = v
			have = true
		}
		if v, ok := firstFieldNumber(f, payload, "Curr"); ok {
			sample.CurrentA = v
			have = true
		}
		if v, ok := firstFieldNumber(f, payload, "RemPct", "Remaining"); ok {
			if v > 1 {
				v /= 100
			}
			sample.Remaining = v
			have = true
		}
		if have {
			out.Battery = append(out.Battery, sample)
		}
	}
}

func firstFieldNumber(f Format, payload []byte, names ...string) (float64, bool) {
	for _, name := range names {
		if v, ok := fieldNumber(f, payload, name); ok {
			return v, true
		}
	}
	return 0, false
}

func fieldUint64(f Format, payload []byte, name string) uint64 {
	v, ok := fieldNumber(f, payload, name)
	if !ok || v < 0 {
		return 0
	}
	return uint64(v)
}

func fieldNumber(f Format, payload []byte, name string) (float64, bool) {
	columns := strings.Split(f.Columns, ",")
	if len(columns) != len(f.Format) {
		return 0, false
	}
	offset := 0
	for i, code := range []byte(f.Format) {
		size := fieldSize(code)
		if size == 0 || offset+size > len(payload) {
			return 0, false
		}
		if strings.EqualFold(strings.TrimSpace(columns[i]), name) {
			return decodeNumber(code, payload[offset:offset+size])
		}
		offset += size
	}
	return 0, false
}

func fieldSize(code byte) int {
	switch code {
	case 'b', 'B', 'M':
		return 1
	case 'h', 'H', 'c', 'C':
		return 2
	case 'i', 'I', 'f', 'e', 'E', 'L':
		return 4
	case 'q', 'Q', 'd':
		return 8
	case 'n':
		return 4
	case 'N':
		return 16
	case 'Z':
		return 64
	case 'a':
		return 64
	default:
		return 0
	}
}

func decodeNumber(code byte, b []byte) (float64, bool) {
	switch code {
	case 'b':
		return float64(int8(b[0])), true
	case 'B', 'M':
		return float64(b[0]), true
	case 'h':
		return float64(int16(binary.LittleEndian.Uint16(b))), true
	case 'H':
		return float64(binary.LittleEndian.Uint16(b)), true
	case 'i':
		return float64(int32(binary.LittleEndian.Uint32(b))), true
	case 'I':
		return float64(binary.LittleEndian.Uint32(b)), true
	case 'f':
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(b))), true
	case 'd':
		return math.Float64frombits(binary.LittleEndian.Uint64(b)), true
	case 'q':
		return float64(int64(binary.LittleEndian.Uint64(b))), true
	case 'Q':
		return float64(binary.LittleEndian.Uint64(b)), true
	case 'c':
		return float64(int16(binary.LittleEndian.Uint16(b))) / 100, true
	case 'C':
		return float64(binary.LittleEndian.Uint16(b)) / 100, true
	case 'e':
		return float64(int32(binary.LittleEndian.Uint32(b))) / 100, true
	case 'E':
		return float64(binary.LittleEndian.Uint32(b)) / 100, true
	case 'L':
		return float64(int32(binary.LittleEndian.Uint32(b))) / 1e7, true
	default:
		return 0, false
	}
}

func trimCString(b []byte) string {
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}
