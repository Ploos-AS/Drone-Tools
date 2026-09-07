package geo

type Bounds struct {
	MinLat float64 `json:"min_lat"`
	MinLon float64 `json:"min_lon"`
	MaxLat float64 `json:"max_lat"`
	MaxLon float64 `json:"max_lon"`
}

type Summary struct {
	Format      string  `json:"format"`
	Features    int     `json:"features"`
	Points      int     `json:"points"`
	LineStrings int     `json:"line_strings"`
	Polygons    int     `json:"polygons"`
	Coordinates int     `json:"coordinates"`
	Bounds      *Bounds `json:"bounds,omitempty"`
}

type accumulator struct {
	summary Summary
	set     bool
}

func (a *accumulator) addCoord(lon, lat float64) {
	a.summary.Coordinates++
	if !a.set {
		a.summary.Bounds = &Bounds{MinLat: lat, MinLon: lon, MaxLat: lat, MaxLon: lon}
		a.set = true
		return
	}
	b := a.summary.Bounds
	if lat < b.MinLat {
		b.MinLat = lat
	}
	if lat > b.MaxLat {
		b.MaxLat = lat
	}
	if lon < b.MinLon {
		b.MinLon = lon
	}
	if lon > b.MaxLon {
		b.MaxLon = lon
	}
}
