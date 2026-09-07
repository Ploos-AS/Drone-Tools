package geo

import (
	"encoding/json"
	"fmt"
	"io"
)

type geoJSON struct {
	Type       string          `json:"type"`
	Features   []geoJSONFeature `json:"features"`
	Geometry   *geoJSONGeometry `json:"geometry"`
	Coordinates json.RawMessage `json:"coordinates"`
}

type geoJSONFeature struct { Geometry *geoJSONGeometry `json:"geometry"` }
type geoJSONGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
	Geometries  []geoJSONGeometry `json:"geometries"`
}

func ParseGeoJSON(r io.Reader) (Summary, error) {
	var root geoJSON
	if err := json.NewDecoder(r).Decode(&root); err != nil { return Summary{}, fmt.Errorf("decode GeoJSON: %w", err) }
	a := accumulator{summary: Summary{Format: "geojson"}}
	switch root.Type {
	case "FeatureCollection":
		a.summary.Features = len(root.Features)
		for _, f := range root.Features { if f.Geometry != nil { if err := addGeometry(&a, *f.Geometry); err != nil { return Summary{}, err } } }
	case "Feature":
		a.summary.Features = 1
		if root.Geometry != nil { if err := addGeometry(&a, *root.Geometry); err != nil { return Summary{}, err } }
	default:
		if root.Type == "" { return Summary{}, fmt.Errorf("missing GeoJSON type") }
		if err := addGeometry(&a, geoJSONGeometry{Type: root.Type, Coordinates: root.Coordinates}); err != nil { return Summary{}, err }
	}
	return a.summary, nil
}

func addGeometry(a *accumulator, g geoJSONGeometry) error {
	switch g.Type {
	case "Point":
		a.summary.Points++
		return addCoordinateRaw(a, g.Coordinates, 0)
	case "MultiPoint":
		var coords [][]float64; if err := json.Unmarshal(g.Coordinates, &coords); err != nil { return err }
		a.summary.Points += len(coords); for _, c := range coords { if err := addPair(a, c); err != nil { return err } }
	case "LineString":
		a.summary.LineStrings++
		return addCoordinateRaw(a, g.Coordinates, 1)
	case "MultiLineString":
		var lines [][][]float64; if err := json.Unmarshal(g.Coordinates, &lines); err != nil { return err }
		a.summary.LineStrings += len(lines); for _, line := range lines { for _, c := range line { if err := addPair(a, c); err != nil { return err } } }
	case "Polygon":
		a.summary.Polygons++
		return addCoordinateRaw(a, g.Coordinates, 2)
	case "MultiPolygon":
		var polygons [][][][]float64; if err := json.Unmarshal(g.Coordinates, &polygons); err != nil { return err }
		a.summary.Polygons += len(polygons); for _, p := range polygons { for _, ring := range p { for _, c := range ring { if err := addPair(a, c); err != nil { return err } } } }
	case "GeometryCollection":
		for _, child := range g.Geometries { if err := addGeometry(a, child); err != nil { return err } }
	default:
		return fmt.Errorf("unsupported GeoJSON geometry %q", g.Type)
	}
	return nil
}

func addCoordinateRaw(a *accumulator, raw json.RawMessage, depth int) error {
	switch depth {
	case 0:
		var c []float64; if err := json.Unmarshal(raw, &c); err != nil { return err }; return addPair(a, c)
	case 1:
		var coords [][]float64; if err := json.Unmarshal(raw, &coords); err != nil { return err }; for _, c := range coords { if err := addPair(a, c); err != nil { return err } }
	case 2:
		var rings [][][]float64; if err := json.Unmarshal(raw, &rings); err != nil { return err }; for _, ring := range rings { for _, c := range ring { if err := addPair(a, c); err != nil { return err } } }
	}
	return nil
}

func addPair(a *accumulator, c []float64) error {
	if len(c) < 2 { return fmt.Errorf("coordinate needs longitude and latitude") }
	a.addCoord(c[0], c[1]); return nil
}
