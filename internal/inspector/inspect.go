package inspector

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

const MaxUploadBytes int64 = 32 << 20

var ErrTooLarge = errors.New("file exceeds upload limit")

type Result struct {
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
	MediaType string `json:"media_type"`
	Kind      string `json:"kind"`
	Supported bool   `json:"supported"`
	SHA256    string `json:"sha256"`
}

func Inspect(filename string, r io.Reader) (Result, error) {
	name := filepath.Base(filename)
	ext := strings.ToLower(filepath.Ext(name))
	limited := io.LimitReader(r, MaxUploadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Result{}, err
	}
	if int64(len(data)) > MaxUploadBytes {
		return Result{}, ErrTooLarge
	}

	mediaType := mime.TypeByExtension(ext)
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	kind, supported := classify(name, ext)
	sum := sha256.Sum256(data)

	return Result{
		Filename:  name,
		Size:      int64(len(data)),
		Extension: ext,
		MediaType: mediaType,
		Kind:      kind,
		Supported: supported,
		SHA256:    hex.EncodeToString(sum[:]),
	}, nil
}

func classify(name, ext string) (string, bool) {
	switch ext {
	case ".gpx":
		return "gpx", true
	case ".kml":
		return "kml", true
	case ".kmz":
		return "kmz", true
	case ".geojson":
		return "geojson", true
	case ".ulg":
		return "px4-ulog", true
	case ".tlog":
		return "mavlink-tlog", true
	case ".bin":
		return "ardupilot-bin", true
	case ".json":
		if strings.Contains(strings.ToLower(name), "geo") {
			return "geojson-candidate", false
		}
	}
	return "unknown", false
}
