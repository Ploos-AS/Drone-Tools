package ulog

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrInvalidHeader = errors.New("invalid ULog header")
	ErrTruncated     = errors.New("truncated ULog message")
)

var magic = [7]byte{'U', 'L', 'o', 'g', 0x01, 0x12, 0x35}

type Subscription struct {
	MessageID uint16 `json:"message_id"`
	MultiID   uint8  `json:"multi_id"`
	Name      string `json:"name"`
}

type GPSSample struct {
	TimestampUS         uint64   `json:"timestamp_us,omitempty"`
	Latitude            float64  `json:"latitude"`
	Longitude           float64  `json:"longitude"`
	AltitudeMeters      float64  `json:"altitude_m,omitempty"`
	VelocityMPS         float64  `json:"velocity_mps,omitempty"`
	FixType             uint8    `json:"fix_type,omitempty"`
	SatellitesUsed      uint8    `json:"satellites_used,omitempty"`
	HorizontalAccuracyM *float64 `json:"horizontal_accuracy_m,omitempty"`
	VerticalAccuracyM   *float64 `json:"vertical_accuracy_m,omitempty"`
}

type LocalPositionSample struct {
	TimestampUS uint64  `json:"timestamp_us,omitempty"`
	X           float64 `json:"x_m,omitempty"`
	Y           float64 `json:"y_m,omitempty"`
	Z           float64 `json:"z_m,omitempty"`
	VX          float64 `json:"vx_mps,omitempty"`
	VY          float64 `json:"vy_mps,omitempty"`
	VZ          float64 `json:"vz_mps,omitempty"`
}

type BatterySample struct {
	TimestampUS uint64  `json:"timestamp_us,omitempty"`
	VoltageV    float64 `json:"voltage_v,omitempty"`
	CurrentA    float64 `json:"current_a,omitempty"`
	Remaining   float64 `json:"remaining,omitempty"`
}

type Telemetry struct {
	GPS           []GPSSample           `json:"gps,omitempty"`
	LocalPosition []LocalPositionSample `json:"local_position,omitempty"`
	Battery       []BatterySample       `json:"battery,omitempty"`
}

type Summary struct {
	Format          string         `json:"format"`
	Version         uint8          `json:"version"`
	StartTimestamp  uint64         `json:"start_timestamp_us"`
	Messages        int            `json:"messages"`
	MessageTypes    map[string]int `json:"message_types"`
	FormatNames     []string       `json:"format_names,omitempty"`
	Subscriptions   []Subscription `json:"subscriptions,omitempty"`
	LoggedDataCount int            `json:"logged_data_messages"`
	Telemetry       Telemetry      `json:"telemetry"`
}

type fieldDef struct {
	Type   string
	Name   string
	Count  int
	Offset int
	Size   int
}

type formatDef struct {
	Name   string
	Fields []fieldDef
	Size   int
}

func Inspect(r io.Reader) (Summary, error) {
	br := bufio.NewReader(r)
	header := make([]byte, 16)
	if _, err := io.ReadFull(br, header); err != nil {
		return Summary{}, ErrInvalidHeader
	}
	for i := range magic {
		if header[i] != magic[i] {
			return Summary{}, ErrInvalidHeader
		}
	}

	s := Summary{
		Format:         "px4-ulog",
		Version:        header[7],
		StartTimestamp: binary.LittleEndian.Uint64(header[8:16]),
		MessageTypes:   map[string]int{},
	}
	formats := map[string]formatDef{}
	subscriptions := map[uint16]Subscription{}

	for {
		msgHeader := make([]byte, 3)
		_, err := io.ReadFull(br, msgHeader)
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return Summary{}, ErrTruncated
		}
		if err != nil {
			return Summary{}, err
		}

		size := int(binary.LittleEndian.Uint16(msgHeader[:2]))
		msgType := msgHeader[2]
		payload := make([]byte, size)
		if _, err := io.ReadFull(br, payload); err != nil {
			return Summary{}, ErrTruncated
		}

		s.Messages++
		s.MessageTypes[string([]byte{msgType})]++
		switch msgType {
		case 'F':
			def, ok := parseFormat(string(payload))
			if ok {
				formats[def.Name] = def
			}
		case 'A':
			if len(payload) < 3 {
				return Summary{}, fmt.Errorf("%w: short subscription", ErrTruncated)
			}
			sub := Subscription{
				MultiID:   payload[0],
				MessageID: binary.LittleEndian.Uint16(payload[1:3]),
				Name:      strings.TrimSpace(string(payload[3:])),
			}
			subscriptions[sub.MessageID] = sub
		case 'D':
			s.LoggedDataCount++
			if len(payload) < 2 {
				return Summary{}, fmt.Errorf("%w: short data message", ErrTruncated)
			}
			id := binary.LittleEndian.Uint16(payload[:2])
			sub, ok := subscriptions[id]
			if !ok {
				continue
			}
			def, ok := formats[sub.Name]
			if !ok {
				continue
			}
			decodeCoreTelemetry(&s.Telemetry, sub.Name, def, payload[2:])
		}
	}

	for name := range formats {
		s.FormatNames = append(s.FormatNames, name)
	}
	sort.Strings(s.FormatNames)
	for _, sub := range subscriptions {
		s.Subscriptions = append(s.Subscriptions, sub)
	}
	sort.Slice(s.Subscriptions, func(i, j int) bool { return s.Subscriptions[i].MessageID < s.Subscriptions[j].MessageID })
	return s, nil
}

func parseFormat(raw string) (formatDef, bool) {
	name, body, ok := strings.Cut(strings.TrimSpace(raw), ":")
	if !ok || strings.TrimSpace(name) == "" {
		return formatDef{}, false
	}
	def := formatDef{Name: strings.TrimSpace(name)}
	for _, part := range strings.Split(body, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		pieces := strings.Fields(part)
		if len(pieces) != 2 {
			return formatDef{}, false
		}
		typeName, count := parseArray(pieces[0])
		size := primitiveSize(typeName)
		if size == 0 {
			return formatDef{}, false
		}
		f := fieldDef{Type: typeName, Name: pieces[1], Count: count, Offset: def.Size, Size: size}
		def.Fields = append(def.Fields, f)
		def.Size += size * count
	}
	return def, true
}

func parseArray(t string) (string, int) {
	open := strings.IndexByte(t, '[')
	if open < 0 || !strings.HasSuffix(t, "]") {
		return t, 1
	}
	n, err := strconv.Atoi(t[open+1 : len(t)-1])
	if err != nil || n < 1 {
		return t, 1
	}
	return t[:open], n
}

func primitiveSize(t string) int {
	switch t {
	case "int8_t", "uint8_t", "bool", "char":
		return 1
	case "int16_t", "uint16_t":
		return 2
	case "int32_t", "uint32_t", "float":
		return 4
	case "int64_t", "uint64_t", "double":
		return 8
	default:
		return 0
	}
}

func decodeCoreTelemetry(out *Telemetry, name string, def formatDef, data []byte) {
	if len(data) < def.Size {
		return
	}
	switch name {
	case "vehicle_gps_position":
		lat, latOK := number(def, data, "lat")
		lon, lonOK := number(def, data, "lon")
		if !latOK || !lonOK {
			return
		}
		s := GPSSample{Latitude: scaledCoordinate(lat), Longitude: scaledCoordinate(lon)}
		s.TimestampUS = uint64Value(def, data, "timestamp")
		if alt, ok := number(def, data, "alt"); ok {
			// PX4 vehicle_gps_position.alt is millimetres above MSL.
			s.AltitudeMeters = alt / 1000
		}
		if vel, ok := number(def, data, "vel_m_s"); ok {
			s.VelocityMPS = vel
		}
		s.FixType = uint8(uint64Value(def, data, "fix_type"))
		s.SatellitesUsed = uint8(uint64Value(def, data, "satellites_used"))
		if eph, ok := number(def, data, "eph"); ok && eph >= 0 && !math.IsNaN(eph) && !math.IsInf(eph, 0) {
			v := eph
			s.HorizontalAccuracyM = &v
		}
		if epv, ok := number(def, data, "epv"); ok && epv >= 0 && !math.IsNaN(epv) && !math.IsInf(epv, 0) {
			v := epv
			s.VerticalAccuracyM = &v
		}
		out.GPS = append(out.GPS, s)
	case "vehicle_local_position":
		s := LocalPositionSample{TimestampUS: uint64Value(def, data, "timestamp")}
		s.X, _ = number(def, data, "x")
		s.Y, _ = number(def, data, "y")
		s.Z, _ = number(def, data, "z")
		s.VX, _ = number(def, data, "vx")
		s.VY, _ = number(def, data, "vy")
		s.VZ, _ = number(def, data, "vz")
		out.LocalPosition = append(out.LocalPosition, s)
	case "battery_status":
		s := BatterySample{TimestampUS: uint64Value(def, data, "timestamp")}
		s.VoltageV, _ = firstNumber(def, data, "voltage_v", "voltage_filtered_v")
		s.CurrentA, _ = firstNumber(def, data, "current_a", "current_filtered_a")
		s.Remaining, _ = number(def, data, "remaining")
		out.Battery = append(out.Battery, s)
	}
}

func firstNumber(def formatDef, data []byte, names ...string) (float64, bool) {
	for _, name := range names {
		if v, ok := number(def, data, name); ok {
			return v, true
		}
	}
	return 0, false
}

func scaledCoordinate(v float64) float64 {
	if math.Abs(v) > 180 {
		return v / 1e7
	}
	return v
}

func uint64Value(def formatDef, data []byte, name string) uint64 {
	v, ok := number(def, data, name)
	if !ok || v < 0 {
		return 0
	}
	return uint64(v)
}

func number(def formatDef, data []byte, name string) (float64, bool) {
	for _, f := range def.Fields {
		if f.Name != name || f.Count != 1 || f.Offset+f.Size > len(data) {
			continue
		}
		b := data[f.Offset : f.Offset+f.Size]
		switch f.Type {
		case "int8_t":
			return float64(int8(b[0])), true
		case "uint8_t", "bool", "char":
			return float64(b[0]), true
		case "int16_t":
			return float64(int16(binary.LittleEndian.Uint16(b))), true
		case "uint16_t":
			return float64(binary.LittleEndian.Uint16(b)), true
		case "int32_t":
			return float64(int32(binary.LittleEndian.Uint32(b))), true
		case "uint32_t":
			return float64(binary.LittleEndian.Uint32(b)), true
		case "int64_t":
			return float64(int64(binary.LittleEndian.Uint64(b))), true
		case "uint64_t":
			return float64(binary.LittleEndian.Uint64(b)), true
		case "float":
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(b))), true
		case "double":
			return math.Float64frombits(binary.LittleEndian.Uint64(b)), true
		}
	}
	return 0, false
}
