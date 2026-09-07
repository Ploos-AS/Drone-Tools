package health

import "math"

type GPSInput struct {
	Samples                 int
	FixQualitySamples       int
	SatelliteQualitySamples int
	LowFixSamples           int
	LowSatelliteSamples     int
	GapEvents               int
	MaxGapSeconds           float64
}

type BatteryInput struct {
	Samples        int
	StartRemaining *float64
	EndRemaining   *float64
	MinVoltageV    *float64
	MaxCurrentA    *float64
}

type DataInput struct {
	TrackSamples       int
	TimedSamples       int
	NonMonotonicTimes  int
	InvalidCoordinates int
}

type Input struct {
	GPS     GPSInput
	Battery BatteryInput
	Data    DataInput
}

type Component struct {
	Score    int      `json:"score"`
	Status   string   `json:"status"`
	Findings []string `json:"findings,omitempty"`
}

type Result struct {
	Score     int       `json:"score"`
	Status    string    `json:"status"`
	GPS       Component `json:"gps"`
	Battery   Component `json:"battery"`
	Data      Component `json:"data_quality"`
	Findings  []string  `json:"findings,omitempty"`
	Algorithm string    `json:"algorithm"`
}

func Evaluate(in Input) Result {
	gps := evaluateGPS(in.GPS)
	battery := evaluateBattery(in.Battery)
	data := evaluateData(in.Data)

	// M3 weights remain stable: GPS 40%, data quality 35%, battery 25%.
	score := int(math.Round(float64(gps.Score)*0.4 + float64(data.Score)*0.35 + float64(battery.Score)*0.25))
	result := Result{
		Score:     clamp(score),
		Status:    statusForScore(score),
		GPS:       gps,
		Battery:   battery,
		Data:      data,
		Algorithm: "m3.3-deterministic-v2",
	}
	result.Findings = append(result.Findings, gps.Findings...)
	result.Findings = append(result.Findings, battery.Findings...)
	result.Findings = append(result.Findings, data.Findings...)
	return result
}

func evaluateGPS(in GPSInput) Component {
	c := Component{Score: 100}
	if in.Samples == 0 {
		c.Score = 40
		c.Findings = append(c.Findings, "no GPS samples available")
		c.Status = statusForScore(c.Score)
		return c
	}
	if in.LowFixSamples > 0 {
		denominator := in.FixQualitySamples
		if denominator == 0 {
			denominator = in.Samples
		}
		ratio := float64(in.LowFixSamples) / float64(denominator)
		c.Score -= penaltyByRatio(ratio, 10, 25, 45)
		c.Findings = append(c.Findings, "GPS includes samples below 3D fix")
	}
	if in.LowSatelliteSamples > 0 {
		denominator := in.SatelliteQualitySamples
		if denominator == 0 {
			denominator = in.Samples
		}
		ratio := float64(in.LowSatelliteSamples) / float64(denominator)
		c.Score -= penaltyByRatio(ratio, 5, 15, 30)
		c.Findings = append(c.Findings, "GPS includes samples with fewer than 6 satellites")
	}
	if in.GapEvents > 0 {
		switch {
		case in.GapEvents >= 4:
			c.Score -= 35
		case in.GapEvents >= 2:
			c.Score -= 20
		default:
			c.Score -= 10
		}
		c.Findings = append(c.Findings, "GPS timestamp gaps suggest telemetry dropouts")
	}
	if in.MaxGapSeconds >= 60 {
		c.Score -= 10
		c.Findings = append(c.Findings, "GPS includes a gap of at least 60 seconds")
	}
	c.Score = clamp(c.Score)
	c.Status = statusForScore(c.Score)
	return c
}

func evaluateBattery(in BatteryInput) Component {
	c := Component{Score: 100}
	if in.Samples == 0 {
		c.Score = 70
		c.Findings = append(c.Findings, "no battery telemetry available")
		c.Status = statusForScore(c.Score)
		return c
	}
	if in.EndRemaining != nil {
		remaining := *in.EndRemaining
		switch {
		case remaining < 0.10:
			c.Score -= 35
			c.Findings = append(c.Findings, "battery remaining ended below 10%")
		case remaining < 0.20:
			c.Score -= 20
			c.Findings = append(c.Findings, "battery remaining ended below 20%")
		case remaining < 0.30:
			c.Score -= 8
			c.Findings = append(c.Findings, "battery remaining ended below 30%")
		}
	}
	if in.StartRemaining != nil && in.EndRemaining != nil && *in.EndRemaining > *in.StartRemaining+0.05 {
		c.Score -= 15
		c.Findings = append(c.Findings, "battery remaining increased unexpectedly during flight")
	}
	c.Score = clamp(c.Score)
	c.Status = statusForScore(c.Score)
	return c
}

func evaluateData(in DataInput) Component {
	c := Component{Score: 100}
	if in.TrackSamples == 0 {
		c.Score = 30
		c.Findings = append(c.Findings, "no track samples available")
		c.Status = statusForScore(c.Score)
		return c
	}
	if in.TimedSamples < 2 {
		c.Score -= 30
		c.Findings = append(c.Findings, "insufficient timestamps for time-based analysis")
	} else if in.TimedSamples < in.TrackSamples/2 {
		c.Score -= 15
		c.Findings = append(c.Findings, "less than half of track samples have timestamps")
	}
	if in.NonMonotonicTimes > 0 {
		c.Score -= min(30, 10+in.NonMonotonicTimes*5)
		c.Findings = append(c.Findings, "non-monotonic timestamps detected")
	}
	if in.InvalidCoordinates > 0 {
		c.Score -= min(35, 10+in.InvalidCoordinates*5)
		c.Findings = append(c.Findings, "invalid GPS coordinates detected")
	}
	c.Score = clamp(c.Score)
	c.Status = statusForScore(c.Score)
	return c
}

func penaltyByRatio(ratio float64, low, medium, high int) int {
	switch {
	case ratio >= 0.5:
		return high
	case ratio >= 0.1:
		return medium
	default:
		return low
	}
}

func statusForScore(score int) string {
	switch {
	case score >= 85:
		return "good"
	case score >= 65:
		return "attention"
	default:
		return "poor"
	}
}

func clamp(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
