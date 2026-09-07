package geo

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type kmlDoc struct {
	Placemarks []kmlPlacemark `xml:"Document>Placemark"`
	Folders    []kmlFolder    `xml:"Document>Folder"`
}

type kmlFolder struct { Placemarks []kmlPlacemark `xml:"Placemark"` }
type kmlPlacemark struct {
	Point *kmlGeometry `xml:"Point"`
	Line  *kmlGeometry `xml:"LineString"`
	Polygon *struct { Outer struct { Ring kmlGeometry `xml:"LinearRing"` } `xml:"outerBoundaryIs"` } `xml:"Polygon"`
}
type kmlGeometry struct { Coordinates string `xml:"coordinates"` }

func ParseKML(r io.Reader) (Summary, error) {
	var doc kmlDoc
	if err := xml.NewDecoder(r).Decode(&doc); err != nil { return Summary{}, fmt.Errorf("decode KML: %w", err) }
	a := accumulator{summary: Summary{Format: "kml"}}
	placemarks := append([]kmlPlacemark{}, doc.Placemarks...)
	for _, f := range doc.Folders { placemarks = append(placemarks, f.Placemarks...) }
	for _, p := range placemarks {
		a.summary.Features++
		if p.Point != nil { a.summary.Points++; if err := addKMLCoords(&a, p.Point.Coordinates); err != nil { return Summary{}, err } }
		if p.Line != nil { a.summary.LineStrings++; if err := addKMLCoords(&a, p.Line.Coordinates); err != nil { return Summary{}, err } }
		if p.Polygon != nil { a.summary.Polygons++; if err := addKMLCoords(&a, p.Polygon.Outer.Ring.Coordinates); err != nil { return Summary{}, err } }
	}
	return a.summary, nil
}

func addKMLCoords(a *accumulator, raw string) error {
	for _, token := range strings.Fields(raw) {
		parts := strings.Split(token, ",")
		if len(parts) < 2 { return fmt.Errorf("invalid KML coordinate %q", token) }
		lon, err := strconv.ParseFloat(parts[0], 64); if err != nil { return fmt.Errorf("invalid longitude: %w", err) }
		lat, err := strconv.ParseFloat(parts[1], 64); if err != nil { return fmt.Errorf("invalid latitude: %w", err) }
		a.addCoord(lon, lat)
	}
	return nil
}
