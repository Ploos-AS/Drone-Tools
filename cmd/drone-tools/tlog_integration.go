package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
	"github.com/Ploos-AS/Drone-Tools/internal/mapdata"
	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
)

func tlogInspectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, inspector.MaxUploadBytes+(1<<20))
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "expected multipart field named file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	summary, err := tlog.Inspect(file)
	if err != nil {
		http.Error(w, "invalid MAVLink TLOG file", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func parseTLOGMap(file interface{ Read([]byte) (int, error) }) (mapdata.Document, error) {
	summary, err := tlog.Inspect(file)
	if err != nil {
		return mapdata.Document{}, err
	}
	return tlogMap(summary.Telemetry.GPS)
}

func tlogTrackSamples(samples []tlog.GPSSample) []tlog.GPSSample {
	preferred := make([]tlog.GPSSample, 0, len(samples))
	fallback := make([]tlog.GPSSample, 0, len(samples))
	for _, sample := range samples {
		switch sample.Source {
		case "GLOBAL_POSITION_INT":
			preferred = append(preferred, sample)
		case "GPS_RAW_INT":
			fallback = append(fallback, sample)
		}
	}
	selected := preferred
	if len(selected) == 0 {
		selected = fallback
	}
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].TimestampUS < selected[j].TimestampUS
	})
	return selected
}

func tlogMap(samples []tlog.GPSSample) (mapdata.Document, error) {
	track := tlogTrackSamples(samples)
	if len(track) == 0 {
		return mapdata.Document{}, errors.New("TLOG contains no GPS samples")
	}
	path := make([]mapdata.Coordinate, 0, len(track))
	for _, sample := range track {
		if sample.Latitude < -90 || sample.Latitude > 90 || sample.Longitude < -180 || sample.Longitude > 180 {
			continue
		}
		path = append(path, mapdata.Coordinate{sample.Longitude, sample.Latitude})
	}
	return finishTelemetryMap("mavlink-tlog", path, "TLOG contains no valid GPS coordinates")
}

func parseTLOGAnalysis(file interface{ Read([]byte) (int, error) }) (flightAnalysis, error) {
	summary, err := tlog.Inspect(file)
	if err != nil {
		return flightAnalysis{}, err
	}
	return analyzeTLOG(summary.Telemetry), nil
}

func analyzeTLOG(telemetry tlog.Telemetry) flightAnalysis {
	track := tlogTrackSamples(telemetry.GPS)
	a := flightAnalysis{Quality: "good", TrackPoints: len(track), TimedPoints: len(track), ElevationPoints: len(track)}
	if len(track) == 0 {
		a.Quality = "limited"
		a.Warnings = append(a.Warnings, "no GPS_RAW_INT/GLOBAL_POSITION_INT samples")
		return a
	}

	minElevation := track[0].AltitudeMeters
	maxElevation := minElevation
	var maxSpeed float64
	for i, sample := range track {
		if sample.AltitudeMeters < minElevation {
			minElevation = sample.AltitudeMeters
		}
		if sample.AltitudeMeters > maxElevation {
			maxElevation = sample.AltitudeMeters
		}
		if sample.SpeedMPS > maxSpeed {
			maxSpeed = sample.SpeedMPS
		}
		if i == 0 {
			continue
		}
		previous := track[i-1]
		a.DistanceMeters += haversine(previous.Latitude, previous.Longitude, sample.Latitude, sample.Longitude)
		delta := sample.AltitudeMeters - previous.AltitudeMeters
		if delta > 0 {
			a.ElevationGainMeters += delta
		} else {
			a.ElevationLossMeters -= delta
		}
	}
	a.MinElevation = pointer(minElevation)
	a.MaxElevation = pointer(maxElevation)
	if maxSpeed > 0 {
		a.MaxSegmentSpeedMPS = pointer(maxSpeed)
	}
	setTimedAnalysis(&a, track[0].TimestampUS, track[len(track)-1].TimestampUS)

	for _, sample := range telemetry.GPS {
		if sample.Source == "GPS_RAW_INT" && sample.FixType < 3 {
			a.Quality = "limited"
			a.Warnings = append(a.Warnings, "MAVLink GPS samples include fix type below 3D")
			break
		}
	}
	for _, sample := range telemetry.GPS {
		if sample.Source == "GPS_RAW_INT" && sample.Satellites < 6 {
			a.Quality = "limited"
			a.Warnings = append(a.Warnings, "MAVLink GPS samples include fewer than 6 satellites")
			break
		}
	}

	if latest, ok := preferredTLOGBattery(telemetry.Battery); ok {
		a.BatteryVoltageV = pointer(latest.VoltageV)
		a.BatteryCurrentA = pointer(latest.CurrentA)
		a.BatteryRemaining = pointer(latest.Remaining)
	} else {
		a.Warnings = append(a.Warnings, "no SYS_STATUS/BATTERY_STATUS samples")
	}
	return a
}

func preferredTLOGBattery(samples []tlog.BatterySample) (tlog.BatterySample, bool) {
	var latestStatus *tlog.BatterySample
	var latestFallback *tlog.BatterySample
	for i := range samples {
		sample := &samples[i]
		if latestFallback == nil || sample.TimestampUS >= latestFallback.TimestampUS {
			latestFallback = sample
		}
		if sample.Source == "BATTERY_STATUS" && (latestStatus == nil || sample.TimestampUS >= latestStatus.TimestampUS) {
			latestStatus = sample
		}
	}
	if latestStatus != nil {
		return *latestStatus, true
	}
	if latestFallback != nil {
		return *latestFallback, true
	}
	return tlog.BatterySample{}, false
}
