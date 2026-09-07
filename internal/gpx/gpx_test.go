package gpx

import (
	"math"
	"strings"
	"testing"
)

func TestParseSummary(t *testing.T) {
	input := `<?xml version="1.0"?><gpx version="1.1"><wpt lat="58.0" lon="7.0"/><trk><trkseg><trkpt lat="58.0000" lon="7.0000"><ele>10</ele><time>2026-09-07T08:00:00Z</time></trkpt><trkpt lat="58.0010" lon="7.0020"><ele>30</ele><time>2026-09-07T08:02:00Z</time></trkpt></trkseg></trk></gpx>`
	s, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if s.TrackPoints != 2 || s.Waypoints != 1 {
		t.Fatalf("unexpected counts: %+v", s)
	}
	if s.MinElevation == nil || *s.MinElevation != 10 || s.MaxElevation == nil || *s.MaxElevation != 30 {
		t.Fatalf("unexpected elevations: %+v", s)
	}
	if s.DurationSeconds == nil || *s.DurationSeconds != 120 {
		t.Fatalf("unexpected duration: %+v", s.DurationSeconds)
	}
	if s.DistanceMeters <= 0 || math.IsNaN(s.DistanceMeters) {
		t.Fatalf("unexpected distance: %v", s.DistanceMeters)
	}
	if s.Bounds.MinLat != 58 || s.Bounds.MaxLat != 58.001 || s.Bounds.MinLon != 7 || s.Bounds.MaxLon != 7.002 {
		t.Fatalf("unexpected bounds: %+v", s.Bounds)
	}
}

func TestParseNoTrackPoints(t *testing.T) {
	_, err := Parse(strings.NewReader(`<gpx version="1.1"></gpx>`))
	if err != ErrNoTrackPoints {
		t.Fatalf("err = %v, want ErrNoTrackPoints", err)
	}
}
