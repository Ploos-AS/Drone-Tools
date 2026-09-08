package main

import (
	"encoding/json"
	"errors"
	"math"
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

type tlogEndpointKey struct {
	systemID    uint8
	componentID uint8
}

func tlogTrackSamples(samples []tlog.GPSSample) []tlog.GPSSample {
	preferred := make([]tlog.GPSSample, 0, len(samples))
	fallback := make([]tlog.GPSSample, 0, len(samples))
	for _, sample := range samples {
		if !usableTLOGCoordinate(sample.Latitude, sample.Longitude) {
			continue
		}
		switch sample.Source {
		case "GLOBAL_POSITION_INT":
			preferred = append(preferred, sample)
		case "GPS_RAW_INT":
			fallback = append(fallback, sample)
		}
	}
	candidates := preferred
	if len(candidates) == 0 {
		candidates = fallback
	}
	selected := selectTLOGTrackEndpoint(candidates)
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].TimestampUS < selected[j].TimestampUS
	})
	return selected
}

func selectTLOGTrackEndpoint(samples []tlog.GPSSample) []tlog.GPSSample {
	if len(samples) == 0 {
		return nil
	}
	counts := make(map[tlogEndpointKey]int)
	for _, sample := range samples {
		counts[tlogEndpointKey{systemID: sample.SystemID, componentID: sample.ComponentID}]++
	}
	var best tlogEndpointKey
	bestCount := -1
	for key, count := range counts {
		if count > bestCount || count == bestCount && endpointKeyLess(key, best) {
			best = key
			bestCount = count
		}
	}
	selected := make([]tlog.GPSSample, 0, bestCount)
	for _, sample := range samples {
		if sample.SystemID == best.systemID && sample.ComponentID == best.componentID {
			selected = append(selected, sample)
		}
	}
	return selected
}

func endpointKeyLess(a, b tlogEndpointKey) bool {
	if a.systemID == b.systemID {
		return a.componentID < b.componentID
	}
	return a.systemID < b.systemID
}

func usableTLOGCoordinate(lat, lon float64) bool {
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return false
	}
	return lat != 0 || lon != 0
}

func knownTLOGSpeed(sample tlog.GPSSample) bool {
	return sample.Source != "GPS_RAW_INT" || math.Abs(sample.SpeedMPS-655.35) > 1e-9
}

func tlogMap(samples []tlog.GPSSample) (mapdata.Document, error) {
	track := tlogTrackSamples(samples)
	if len(track) == 0 {
		return mapdata.Document{}, errors.New("TLOG contains no valid GPS samples")
	}
	path := make([]mapdata.Coordinate, 0, len(track))
	for _, sample := range track {
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
		a.Warnings = append(a.Warnings, "no valid GPS_RAW_INT/GLOBAL_POSITION_INT samples")
		return a
	}

	endpoint, _ := tlogTrackEndpoint(track)
	gps := filterTLOGGPSByEndpoint(telemetry.GPS, endpoint)

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
		if knownTLOGSpeed(sample) && sample.SpeedMPS > maxSpeed {
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

	for _, sample := range gps {
		if sample.Source == "GPS_RAW_INT" && sample.FixType < 3 {
			a.Quality = "limited"
			a.Warnings = append(a.Warnings, "MAVLink GPS samples include fix type below 3D")
			break
		}
	}
	for _, sample := range gps {
		if sample.Source == "GPS_RAW_INT" && sample.Satellites != 0xff && sample.Satellites < 6 {
			a.Quality = "limited"
			a.Warnings = append(a.Warnings, "MAVLink GPS samples include fewer than 6 satellites")
			break
		}
	}

	if latest, ok := preferredTLOGBatteryForEndpoint(telemetry.Battery, endpoint); ok {
		a.BatteryVoltageV = pointer(latest.VoltageV)
		a.BatteryCurrentA = pointer(latest.CurrentA)
		a.BatteryRemaining = pointer(latest.Remaining)
	} else {
		a.Warnings = append(a.Warnings, "no SYS_STATUS/BATTERY_STATUS samples")
	}
	return a
}

func preferredTLOGBatteryForEndpoint(samples []tlog.BatterySample, endpoint tlogEndpointKey) (tlog.BatterySample, bool) {
	matching := filterTLOGBatteryByEndpoint(samples, endpoint)
	if len(matching) > 0 {
		return preferredTLOGBattery(matching)
	}
	return preferredTLOGBattery(samples)
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
