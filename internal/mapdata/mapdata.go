package mapdata

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

type Coordinate [2]float64

type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLat float64 `json:"max_lat"`
	MaxLon float64 `json:"max_lon"`
}

type Document struct {
	Format string         `json:"format"`
	Paths  [][]Coordinate `json:"paths"`
	Points []Coordinate   `json:"points"`
	Bounds *Bounds        `json:"bounds,omitempty"`
}

func Parse(filename string, r io.Reader) (Document, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".gpx":
		return parseGPX(r)
	case ".kml":
		return parseKML(r)
	case ".geojson", ".json":
		return parseGeoJSON(r)
	default:
		return Document{}, fmt.Errorf("unsupported map format %q", ext)
	}
}

func parseGPX(r io.Reader) (Document, error) {
	var root struct {
		Waypoints []struct {
			Lat float64 `xml:"lat,attr"`
			Lon float64 `xml:"lon,attr"`
		} `xml:"wpt"`
		Tracks []struct {
			Segments []struct {
				Points []struct {
					Lat float64 `xml:"lat,attr"`
					Lon float64 `xml:"lon,attr"`
				} `xml:"trkpt"`
			} `xml:"trkseg"`
		} `xml:"trk"`
	}
	if err := xml.NewDecoder(r).Decode(&root); err != nil {
		return Document{}, err
	}
	d := Document{Format: "gpx"}
	for _, w := range root.Waypoints {
		d.Points = append(d.Points, Coordinate{w.Lon, w.Lat})
	}
	for _, track := range root.Tracks {
		for _, segment := range track.Segments {
			path := make([]Coordinate, 0, len(segment.Points))
			for _, p := range segment.Points {
				path = append(path, Coordinate{p.Lon, p.Lat})
			}
			if len(path) > 0 {
				d.Paths = append(d.Paths, path)
			}
		}
	}
	return finish(d)
}

func parseKML(r io.Reader) (Document, error) {
	var root struct {
		Points []struct {
			Coordinates string `xml:"coordinates"`
		} `xml:"Document>Placemark>Point"`
		Lines []struct {
			Coordinates string `xml:"coordinates"`
		} `xml:"Document>Placemark>LineString"`
	}
	if err := xml.NewDecoder(r).Decode(&root); err != nil {
		return Document{}, err
	}
	d := Document{Format: "kml"}
	for _, p := range root.Points {
		coords, err := parseKMLCoordinates(p.Coordinates)
		if err != nil {
			return Document{}, err
		}
		if len(coords) > 0 {
			d.Points = append(d.Points, coords[0])
		}
	}
	for _, line := range root.Lines {
		coords, err := parseKMLCoordinates(line.Coordinates)
		if err != nil {
			return Document{}, err
		}
		if len(coords) > 0 {
			d.Paths = append(d.Paths, coords)
		}
	}
	return finish(d)
}

func parseKMLCoordinates(s string) ([]Coordinate, error) {
	var out []Coordinate
	for _, item := range strings.Fields(s) {
		parts := strings.Split(item, ",")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid KML coordinate")
		}
		lon, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return nil, err
		}
		lat, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, err
		}
		out = append(out, Coordinate{lon, lat})
	}
	return out, nil
}

func parseGeoJSON(r io.Reader) (Document, error) {
	var root any
	if err := json.NewDecoder(r).Decode(&root); err != nil {
		return Document{}, err
	}
	d := Document{Format: "geojson"}
	if err := walkGeoJSON(root, &d); err != nil {
		return Document{}, err
	}
	return finish(d)
}

func walkGeoJSON(v any, d *Document) error {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	typeName, _ := m["type"].(string)
	switch typeName {
	case "FeatureCollection":
		features, _ := m["features"].([]any)
		for _, f := range features {
			if err := walkGeoJSON(f, d); err != nil {
				return err
			}
		}
	case "Feature":
		if g, ok := m["geometry"]; ok && g != nil {
			return walkGeoJSON(g, d)
		}
	case "Point":
		c, err := pair(m["coordinates"])
		if err != nil {
			return err
		}
		d.Points = append(d.Points, c)
	case "MultiPoint":
		for _, raw := range asSlice(m["coordinates"]) {
			c, err := pair(raw)
			if err != nil {
				return err
			}
			d.Points = append(d.Points, c)
		}
	case "LineString":
		path, err := pairs(m["coordinates"])
		if err != nil {
			return err
		}
		d.Paths = append(d.Paths, path)
	case "MultiLineString":
		for _, raw := range asSlice(m["coordinates"]) {
			path, err := pairs(raw)
			if err != nil {
				return err
			}
			d.Paths = append(d.Paths, path)
		}
	case "Polygon":
		rings := asSlice(m["coordinates"])
		if len(rings) > 0 {
			path, err := pairs(rings[0])
			if err != nil {
				return err
			}
			d.Paths = append(d.Paths, path)
		}
	case "GeometryCollection":
		for _, g := range asSlice(m["geometries"]) {
			if err := walkGeoJSON(g, d); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported GeoJSON type %q", typeName)
	}
	return nil
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func pair(v any) (Coordinate, error) {
	values := asSlice(v)
	if len(values) < 2 {
		return Coordinate{}, fmt.Errorf("coordinate requires longitude and latitude")
	}
	lon, ok1 := values[0].(float64)
	lat, ok2 := values[1].(float64)
	if !ok1 || !ok2 {
		return Coordinate{}, fmt.Errorf("invalid coordinate")
	}
	return Coordinate{lon, lat}, nil
}

func pairs(v any) ([]Coordinate, error) {
	items := asSlice(v)
	out := make([]Coordinate, 0, len(items))
	for _, item := range items {
		c, err := pair(item)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func finish(d Document) (Document, error) {
	var all []Coordinate
	all = append(all, d.Points...)
	for _, path := range d.Paths {
		all = append(all, path...)
	}
	if len(all) == 0 {
		return Document{}, fmt.Errorf("no map coordinates found")
	}
	b := Bounds{MinLon: all[0][0], MaxLon: all[0][0], MinLat: all[0][1], MaxLat: all[0][1]}
	for _, c := range all[1:] {
		if c[0] < b.MinLon { b.MinLon = c[0] }
		if c[0] > b.MaxLon { b.MaxLon = c[0] }
		if c[1] < b.MinLat { b.MinLat = c[1] }
		if c[1] > b.MaxLat { b.MaxLat = c[1] }
	}
	d.Bounds = &b
	return d, nil
}
