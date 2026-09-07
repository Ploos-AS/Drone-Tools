package dataflash

import (
	"errors"
	"fmt"
	"io"
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

type Summary struct {
	Format        string         `json:"format"`
	Messages      int            `json:"messages"`
	MessageTypes  map[string]int `json:"message_types"`
	Formats       []Format       `json:"formats"`
	UnknownFrames int            `json:"unknown_frames"`
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
			name := formats[msgID].Name
			if name == "" {
				name = fmt.Sprintf("type-%d", msgID)
			}
			s.MessageTypes[name]++
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

func trimCString(b []byte) string {
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	return strings.TrimSpace(string(b))
}
