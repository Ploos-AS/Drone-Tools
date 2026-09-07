package geo

import (
	"strings"
	"testing"
)

func TestParseKML(t *testing.T) {
	input := `<kml><Document><Placemark><Point><coordinates>7,58,10</coordinates></Point></Placemark><Placemark><LineString><coordinates>7,58,10 8,59,20</coordinates></LineString></Placemark></Document></kml>`
	summary, err := ParseKML(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Features != 2 || summary.Points != 1 || summary.LineStrings != 1 || summary.Coordinates != 3 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Bounds == nil || summary.Bounds.MinLon != 7 || summary.Bounds.MaxLat != 59 {
		t.Fatalf("unexpected bounds: %+v", summary.Bounds)
	}
}

func TestParseGeoJSON(t *testing.T) {
	input := `{"type":"FeatureCollection","features":[{"type":"Feature","geometry":{"type":"Point","coordinates":[7,58]}},{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[7,58],[8,58],[8,59],[7,58]]]}}]}`
	summary, err := ParseGeoJSON(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if summary.Features != 2 || summary.Points != 1 || summary.Polygons != 1 || summary.Coordinates != 5 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Bounds == nil || summary.Bounds.MinLat != 58 || summary.Bounds.MaxLon != 8 {
		t.Fatalf("unexpected bounds: %+v", summary.Bounds)
	}
}
