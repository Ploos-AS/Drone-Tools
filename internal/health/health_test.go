package health

import "testing"

func TestEvaluateHealthyFlight(t *testing.T) {
	start, end := 0.95, 0.42
	result := Evaluate(Input{
		GPS:     GPSInput{Samples: 100},
		Battery: BatteryInput{Samples: 20, StartRemaining: &start, EndRemaining: &end},
		Data:    DataInput{TrackSamples: 100, TimedSamples: 100},
	})
	if result.Score != 100 || result.Status != "good" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Algorithm != "m3.3-deterministic-v2" {
		t.Fatalf("algorithm = %q", result.Algorithm)
	}
}

func TestEvaluateDegradesGPSByRatio(t *testing.T) {
	result := Evaluate(Input{
		GPS:     GPSInput{Samples: 100, LowFixSamples: 60, LowSatelliteSamples: 20},
		Battery: BatteryInput{Samples: 1},
		Data:    DataInput{TrackSamples: 100, TimedSamples: 100},
	})
	if result.GPS.Score != 40 || result.GPS.Status != "poor" {
		t.Fatalf("unexpected GPS component: %+v", result.GPS)
	}
	if len(result.GPS.Findings) != 2 {
		t.Fatalf("unexpected GPS findings: %+v", result.GPS.Findings)
	}
}

func TestEvaluateGPSGapPenalties(t *testing.T) {
	result := Evaluate(Input{
		GPS:     GPSInput{Samples: 100, GapEvents: 2, MaxGapSeconds: 75},
		Battery: BatteryInput{Samples: 1},
		Data:    DataInput{TrackSamples: 100, TimedSamples: 100},
	})
	if result.GPS.Score != 70 || result.GPS.Status != "attention" {
		t.Fatalf("unexpected GPS component: %+v", result.GPS)
	}
	if len(result.GPS.Findings) != 2 {
		t.Fatalf("unexpected GPS findings: %+v", result.GPS.Findings)
	}
}

func TestEvaluateBatteryWarnings(t *testing.T) {
	start, end := 0.50, 0.08
	result := Evaluate(Input{
		GPS:     GPSInput{Samples: 10},
		Battery: BatteryInput{Samples: 10, StartRemaining: &start, EndRemaining: &end},
		Data:    DataInput{TrackSamples: 10, TimedSamples: 10},
	})
	if result.Battery.Score != 65 || result.Battery.Status != "attention" {
		t.Fatalf("unexpected battery component: %+v", result.Battery)
	}
}

func TestEvaluateMissingTelemetryIsExplicit(t *testing.T) {
	result := Evaluate(Input{})
	if result.GPS.Score != 40 || result.Battery.Score != 70 || result.Data.Score != 30 {
		t.Fatalf("unexpected missing-data scores: %+v", result)
	}
	if result.Status != "poor" {
		t.Fatalf("status = %q, want poor", result.Status)
	}
	if len(result.Findings) != 3 {
		t.Fatalf("findings = %+v", result.Findings)
	}
}

func TestEvaluateDataQualityPenalties(t *testing.T) {
	result := Evaluate(Input{
		GPS:     GPSInput{Samples: 10},
		Battery: BatteryInput{Samples: 1},
		Data:    DataInput{TrackSamples: 10, TimedSamples: 1, NonMonotonicTimes: 2, InvalidCoordinates: 3},
	})
	if result.Data.Score != 25 || result.Data.Status != "poor" {
		t.Fatalf("unexpected data component: %+v", result.Data)
	}
}
