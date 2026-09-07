package mapdata

import (
	"strings"
	"testing"
)

func TestParseGPX(t *testing.T) {
	content := `<gpx><wpt lat="58" lon="7"/><trk><trkseg><trkpt lat="58" lon="7"/><trkpt lat="58.1" lon="7.2"/></trkseg></trk></gpx>`
	doc, err := Parse("flight.gpx", strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "gpx" || len(doc.Paths) != 1 || len(doc.Paths[0]) != 2 || len(doc.Points) != 1 {
		t.Fatalf("unexpected document: %+v", doc)
	}
	if doc.Bounds == nil || doc.Bounds.MaxLat != 58.1 || doc.Bounds.MaxLon != 7.2 {
		t.Fatalf("unexpected bounds: %+v", doc.Bounds)
	}
}

func TestParseKML(t *testing.T) {
	content := `<kml><Document><Placemark><Point><coordinates>7,58,10</coordinates></Point></Placemark><Placemark><LineString><coordinates>7,58 7.2,58.1</coordinates></LineString></Placemark></Document></kml>`
	doc, err := Parse("flight.kml", strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "kml" || len(doc.Paths) != 1 || len(doc.Points) != 1 {
		t.Fatalf("unexpected document: %+v", doc)
	}
}

func TestParseGeoJSON(t *testing.T) {
	content := `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[7,58]}},{"type":"Feature","geometry":{"type":"LineString","coordinates":[[7,58],[7.2,58.1]]}}]}`
	doc, err := Parse("flight.geojson", strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != "geojson" || len(doc.Paths) != 1 || len(doc.Points) != 1 {
		t.Fatalf("unexpected document: %+v", doc)
	}
}

func TestUnsupportedFormat(t *testing.T) {
	if _, err := Parse("flight.txt", strings.NewReader("x")); err == nil {
		t.Fatal("expected unsupported-format error")
	}
}
