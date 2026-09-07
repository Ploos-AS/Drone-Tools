package main

import (
	"encoding/json"
	"errors"
	"net/http"

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

func tlogMap(samples []tlog.GPSSample) (mapdata.Document, error) {
	if len(samples) == 0 {
		return mapdata.Document{}, errors.New("TLOG contains no GPS samples")
	}
	path := make([]mapdata.Coordinate, 0, len(samples))
	for _, sample := range samples {
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
	a := flightAnalysis{Quality: "good", TrackPoints: len(telemetry.GPS), TimedPoints: len(telemetry.GPS), ElevationPoints: len(telemetry.GPS)}
	if len(telemetry.GPS) == 0 {
		a.Quality = "limited"
		a.Warnings = append(a.Warnings, "no GPS_RAW_INT/GLOBAL_POSITION_INT samples")
		return a
	}

	minElevation := telemetry.GPS[0].AltitudeMeters
	maxElevation := minElevation
	var maxSpeed float64
	for i, sample := range telemetry.GPS {
		if sample.AltitudeMeters < minElevation {
			minElevation = sample.AltitudeMeters
		}
		if sample.AltitudeMeters > maxElevation {
			maxElevation = sample.AltitudeMeters
		}
		if sample.SpeedMPS > maxSpeed {
			maxSpeed = sample.SpeedMPS
		}
		if sample.FixType > 0 && sample.FixType < 3 {
			a.Quality = "limited"
		}
		if sample.Satellites > 0 && sample.Satellites < 6 {
			a.Quality = "limited"
		}
		if i == 0 {
			continue
		}
		previous := telemetry.GPS[i-1]
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
	setTimedAnalysis(&a, telemetry.GPS[0].TimestampUS, telemetry.GPS[len(telemetry.GPS)-1].TimestampUS)

	for _, sample := range telemetry.GPS {
		if sample.FixType > 0 && sample.FixType < 3 {
			a.Warnings = append(a.Warnings, "MAVLink GPS samples include fix type below 3D")
			break
		}
	}
	for _, sample := range telemetry.GPS {
		if sample.Satellites > 0 && sample.Satellites < 6 {
			a.Warnings = append(a.Warnings, "MAVLink GPS samples include fewer than 6 satellites")
			break
		}
	}

	if len(telemetry.Battery) > 0 {
		latest := telemetry.Battery[len(telemetry.Battery)-1]
		a.BatteryVoltageV = pointer(latest.VoltageV)
		a.BatteryCurrentA = pointer(latest.CurrentA)
		a.BatteryRemaining = pointer(latest.Remaining)
	} else {
		a.Warnings = append(a.Warnings, "no SYS_STATUS/BATTERY_STATUS samples")
	}
	return a
}
