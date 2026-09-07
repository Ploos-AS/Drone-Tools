package ulog

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
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

type Summary struct {
	Format          string         `json:"format"`
	Version         uint8          `json:"version"`
	StartTimestamp  uint64         `json:"start_timestamp_us"`
	Messages        int            `json:"messages"`
	MessageTypes    map[string]int `json:"message_types"`
	FormatNames     []string       `json:"format_names,omitempty"`
	Subscriptions   []Subscription `json:"subscriptions,omitempty"`
	LoggedDataCount int            `json:"logged_data_messages"`
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
	formatSet := map[string]struct{}{}

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
			name := formatName(string(payload))
			if name != "" {
				formatSet[name] = struct{}{}
			}
		case 'A':
			if len(payload) < 3 {
				return Summary{}, fmt.Errorf("%w: short subscription", ErrTruncated)
			}
			s.Subscriptions = append(s.Subscriptions, Subscription{
				MultiID:   payload[0],
				MessageID: binary.LittleEndian.Uint16(payload[1:3]),
				Name:      strings.TrimSpace(string(payload[3:])),
			})
		case 'D':
			s.LoggedDataCount++
		}
	}

	for name := range formatSet {
		s.FormatNames = append(s.FormatNames, name)
	}
	sort.Strings(s.FormatNames)
	return s, nil
}

func formatName(def string) string {
	name, _, ok := strings.Cut(def, ":")
	if !ok {
		return ""
	}
	return strings.TrimSpace(name)
}
