package gpx

import (
	"encoding/xml"
	"errors"
	"io"
	"math"
	"time"
)

var ErrNoTrackPoints = errors.New("gpx contains no track points")

type Point struct {
	Lat  float64    `json:"lat"`
	Lon  float64    `json:"lon"`
	Ele  *float64   `json:"ele,omitempty"`
	Time *time.Time `json:"time,omitempty"`
}

type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLat float64 `json:"max_lat"`
	MaxLon float64 `json:"max_lon"`
}

type Summary struct {
	TrackPoints         int        `json:"track_points"`
	Waypoints           int        `json:"waypoints"`
	DistanceMeters      float64    `json:"distance_meters"`
	Bounds              Bounds     `json:"bounds"`
	MinElevation        *float64   `json:"min_elevation_m,omitempty"`
	MaxElevation        *float64   `json:"max_elevation_m,omitempty"`
	ElevationGainMeters float64    `json:"elevation_gain_m"`
	ElevationLossMeters float64    `json:"elevation_loss_m"`
	StartTime           *time.Time `json:"start_time,omitempty"`
	EndTime             *time.Time `json:"end_time,omitempty"`
	DurationSeconds     *float64   `json:"duration_seconds,omitempty"`
	AverageSpeedMPS     *float64   `json:"average_speed_mps,omitempty"`
	MaxSegmentSpeedMPS  *float64   `json:"max_segment_speed_mps,omitempty"`
	TimedPoints         int        `json:"timed_points"`
	ElevationPoints     int        `json:"elevation_points"`
	Quality             string     `json:"quality"`
	Warnings            []string   `json:"warnings,omitempty"`
}

type xmlPoint struct {
	Lat  float64  `xml:"lat,attr"`
	Lon  float64  `xml:"lon,attr"`
	Ele  *float64 `xml:"ele"`
	Time string   `xml:"time"`
}

type document struct {
	Waypoints []xmlPoint `xml:"wpt"`
	Tracks    []struct {
		Segments []struct {
			Points []xmlPoint `xml:"trkpt"`
		} `xml:"trkseg"`
	} `xml:"trk"`
}

func Parse(r io.Reader) (Summary, error) {
	var doc document
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return Summary{}, err
	}

	var points []Point
	for _, track := range doc.Tracks {
		for _, segment := range track.Segments {
			for _, p := range segment.Points {
				points = append(points, convertPoint(p))
			}
		}
	}
	if len(points) == 0 {
		return Summary{}, ErrNoTrackPoints
	}

	s := Summary{TrackPoints: len(points), Waypoints: len(doc.Waypoints), Quality: "good"}
	s.Bounds = Bounds{MinLat: points[0].Lat, MaxLat: points[0].Lat, MinLon: points[0].Lon, MaxLon: points[0].Lon}

	var maxSpeed float64
	var haveMaxSpeed bool
	for i, p := range points {
		if p.Lat < s.Bounds.MinLat {
			s.Bounds.MinLat = p.Lat
		}
		if p.Lat > s.Bounds.MaxLat {
			s.Bounds.MaxLat = p.Lat
		}
		if p.Lon < s.Bounds.MinLon {
			s.Bounds.MinLon = p.Lon
		}
		if p.Lon > s.Bounds.MaxLon {
			s.Bounds.MaxLon = p.Lon
		}
		if p.Ele != nil {
			s.ElevationPoints++
			if s.MinElevation == nil || *p.Ele < *s.MinElevation {
				v := *p.Ele
				s.MinElevation = &v
			}
			if s.MaxElevation == nil || *p.Ele > *s.MaxElevation {
				v := *p.Ele
				s.MaxElevation = &v
			}
			if i > 0 && points[i-1].Ele != nil {
				delta := *p.Ele - *points[i-1].Ele
				if delta > 0 {
					s.ElevationGainMeters += delta
				} else {
					s.ElevationLossMeters += -delta
				}
			}
		}
		if p.Time != nil {
			s.TimedPoints++
			if s.StartTime == nil || p.Time.Before(*s.StartTime) {
				v := *p.Time
				s.StartTime = &v
			}
			if s.EndTime == nil || p.Time.After(*s.EndTime) {
				v := *p.Time
				s.EndTime = &v
			}
		}
		if i > 0 {
			distance := haversine(points[i-1], p)
			s.DistanceMeters += distance
			if points[i-1].Time != nil && p.Time != nil {
				dt := p.Time.Sub(*points[i-1].Time).Seconds()
				if dt > 0 {
					speed := distance / dt
					if !haveMaxSpeed || speed > maxSpeed {
						maxSpeed = speed
						haveMaxSpeed = true
					}
				}
			}
		}
	}
	if s.StartTime != nil && s.EndTime != nil {
		d := s.EndTime.Sub(*s.StartTime).Seconds()
		s.DurationSeconds = &d
		if d > 0 {
			avg := s.DistanceMeters / d
			s.AverageSpeedMPS = &avg
		}
	}
	if haveMaxSpeed {
		s.MaxSegmentSpeedMPS = &maxSpeed
	}

	if s.TimedPoints < 2 {
		s.Warnings = append(s.Warnings, "insufficient timestamps for speed and duration analysis")
	}
	if s.ElevationPoints == 0 {
		s.Warnings = append(s.Warnings, "no elevation samples")
	}
	if len(s.Warnings) > 0 {
		s.Quality = "limited"
	}
	return s, nil
}

func convertPoint(p xmlPoint) Point {
	out := Point{Lat: p.Lat, Lon: p.Lon, Ele: p.Ele}
	if p.Time != "" {
		if t, err := time.Parse(time.RFC3339, p.Time); err == nil {
			out.Time = &t
		}
	}
	return out
}

func haversine(a, b Point) float64 {
	const earthRadius = 6371000.0
	lat1, lat2 := a.Lat*math.Pi/180, b.Lat*math.Pi/180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}
