package inspector

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestInspectSupportedGPX(t *testing.T) {
	result, err := Inspect("track.gpx", strings.NewReader("<gpx></gpx>"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "gpx" || !result.Supported {
		t.Fatalf("unexpected classification: %#v", result)
	}
	if result.Filename != "track.gpx" {
		t.Fatalf("unexpected filename: %q", result.Filename)
	}
	if len(result.SHA256) != 64 {
		t.Fatalf("unexpected sha256: %q", result.SHA256)
	}
}

func TestInspectSanitizesFilename(t *testing.T) {
	result, err := Inspect("../../flight.ulg", strings.NewReader("ULog"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Filename != "flight.ulg" {
		t.Fatalf("filename was not reduced to basename: %q", result.Filename)
	}
}

func TestInspectUnknown(t *testing.T) {
	result, err := Inspect("notes.txt", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "unknown" || result.Supported {
		t.Fatalf("unexpected classification: %#v", result)
	}
}

func TestInspectRejectsOversizedFile(t *testing.T) {
	_, err := Inspect("big.bin", bytes.NewReader(make([]byte, MaxUploadBytes+1)))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected ErrTooLarge, got %v", err)
	}
}
