package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sort"

	"github.com/Ploos-AS/Drone-Tools/internal/dataflash"
	"github.com/Ploos-AS/Drone-Tools/internal/health"
	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
	"github.com/Ploos-AS/Drone-Tools/internal/ulog"
)

const (
	maxHDOP                = 2.5
	maxVDOP                = 3.5
	maxHorizontalAccuracyM = 5.0
	maxVerticalAccuracyM   = 8.0
)

func flightHealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, inspector.MaxUploadBytes+(1<<20))
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "expected multipart field named file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	var input health.Input
	switch filepath.Ext(header.Filename) {
	case ".ulg":
		summary, err := ulog.Inspect(file)
		if err != nil {
			http.Error(w, "invalid PX4 ULog file", http.StatusBadRequest)
			return
		}
		input = healthInputFromULog(summary.Telemetry)
	case ".bin":
		summary, err := dataflash.Inspect(file)
		if err != nil {
			http.Error(w, "invalid ArduPilot DataFlash log", http.StatusBadRequest)
			return
		}
		input = healthInputFromDataFlash(summary.Telemetry)
	case ".tlog":
		summary, err := tlog.InspectDetailed(file)
		if err != nil {
			http.Error(w, "invalid MAVLink TLOG file", http.StatusBadRequest)
			return
		}
		input = healthInputFromTLOGWithRoles(summary.Telemetry, summary.EndpointRoles)
	default:
		http.Error(w, "flight health currently supports .ulg, .bin and .tlog", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(health.Evaluate(input))
}

func healthInputFromULog(telemetry ulog.Telemetry) health.Input {
	in := health.Input{}
	timestamps := make([]uint64, 0, len(telemetry.GPS))
	for i, sample := range telemetry.GPS {
		in.GPS.Samples++
		in.GPS.FixQualitySamples++
		in.GPS.SatelliteQualitySamples++
		in.Data.TrackSamples++
		if sample.FixType < 3 {
			in.GPS.LowFixSamples++
		}
		if sample.SatellitesUsed < 6 {
			in.GPS.LowSatelliteSamples++
		}
		if sample.HorizontalAccuracyM != nil || sample.VerticalAccuracyM != nil {
			in.GPS.PrecisionQualitySamples++
			poor := sample.HorizontalAccuracyM != nil && *sample.HorizontalAccuracyM > maxHorizontalAccuracyM
			poor = poor || sample.VerticalAccuracyM != nil && *sample.VerticalAccuracyM > maxVerticalAccuracyM
			if poor {
				in.GPS.PoorPrecisionSamples++
			}
		}
		accumulateTrackQuality(&in.Data, sample.TimestampUS, sample.Latitude, sample.Longitude, previousULogTimestamp(telemetry.GPS, i))
		if sample.TimestampUS > 0 {
			timestamps = append(timestamps, sample.TimestampUS)
		}
	}
	accumulateGPSGaps(&in.GPS, timestamps)
	accumulateULogBattery(&in.Battery, telemetry.Battery)
	return in
}

func healthInputFromDataFlash(telemetry dataflash.Telemetry) health.Input {
	in := health.Input{}
	timestamps := make([]uint64, 0, len(telemetry.GPS))
	for i, sample := range telemetry.GPS {
		in.GPS.Samples++
		in.GPS.FixQualitySamples++
		in.GPS.SatelliteQualitySamples++
		in.Data.TrackSamples++
		if sample.Status < 3 {
			in.GPS.LowFixSamples++
		}
		if sample.Satellites < 6 {
			in.GPS.LowSatelliteSamples++
		}
		if sample.HDOP != nil {
			in.GPS.PrecisionQualitySamples++
			if *sample.HDOP > maxHDOP {
				in.GPS.PoorPrecisionSamples++
			}
		}
		accumulateTrackQuality(&in.Data, sample.TimestampUS, sample.Latitude, sample.Longitude, previousDataFlashTimestamp(telemetry.GPS, i))
		if sample.TimestampUS > 0 {
			timestamps = append(timestamps, sample.TimestampUS)
		}
	}
	accumulateGPSGaps(&in.GPS, timestamps)
	accumulateDataFlashBattery(&in.Battery, telemetry.Battery)
	return in
}

func healthInputFromTLOG(telemetry tlog.Telemetry) health.Input {
	return healthInputFromTLOGWithRoles(telemetry, nil)
}

func healthInputFromTLOGWithRoles(telemetry tlog.Telemetry, roles []tlog.EndpointRole) health.Input {
	in := health.Input{}
	track := tlogTrackSamplesWithRoles(telemetry.GPS, roles)
	endpoint, haveEndpoint := tlogTrackEndpoint(track)

	qualitySamples := telemetry.GPS
	if haveEndpoint {
		qualitySamples = filterTLOGGPSByEndpoint(telemetry.GPS, endpoint)
	}

	gapTimestamps := make([]uint64, 0, len(qualitySamples))
	for _, sample := range qualitySamples {
		in.GPS.Samples++
		if sample.Source != "GPS_RAW_INT" {
			continue
		}
		in.GPS.FixQualitySamples++
		if sample.FixType < 3 {
			in.GPS.LowFixSamples++
		}
		if sample.Satellites != 0xff {
			in.GPS.SatelliteQualitySamples++
			if sample.Satellites < 6 {
				in.GPS.LowSatelliteSamples++
			}
		}
		if sample.HDOP != nil || sample.VDOP != nil {
			in.GPS.PrecisionQualitySamples++
			poor := sample.HDOP != nil && *sample.HDOP > maxHDOP
			poor = poor || sample.VDOP != nil && *sample.VDOP > maxVDOP
			if poor {
				in.GPS.PoorPrecisionSamples++
			}
		}
		if sample.TimestampUS > 0 {
			gapTimestamps = append(gapTimestamps, sample.TimestampUS)
		}
	}

	for i, sample := range track {
		in.Data.TrackSamples++
		accumulateTrackQuality(&in.Data, sample.TimestampUS, sample.Latitude, sample.Longitude, previousTLOGTimestamp(track, i))
	}
	if len(gapTimestamps) == 0 {
		for _, sample := range track {
			if sample.TimestampUS > 0 {
				gapTimestamps = append(gapTimestamps, sample.TimestampUS)
			}
		}
	}
	accumulateGPSGaps(&in.GPS, gapTimestamps)

	battery := telemetry.Battery
	if haveEndpoint {
		if matching := filterTLOGBatteryByEndpoint(telemetry.Battery, endpoint); len(matching) > 0 {
			battery = matching
		}
	}
	accumulateTLOGBattery(&in.Battery, battery)
	return in
}

func tlogTrackEndpoint(track []tlog.GPSSample) (tlogEndpointKey, bool) {
	if len(track) == 0 {
		return tlogEndpointKey{}, false
	}
	return tlogEndpointKey{systemID: track[0].SystemID, componentID: track[0].ComponentID}, true
}

func filterTLOGGPSByEndpoint(samples []tlog.GPSSample, endpoint tlogEndpointKey) []tlog.GPSSample {
	filtered := make([]tlog.GPSSample, 0, len(samples))
	for _, sample := range samples {
		if sample.SystemID == endpoint.systemID && sample.ComponentID == endpoint.componentID {
			filtered = append(filtered, sample)
		}
	}
	return filtered
}

func filterTLOGBatteryByEndpoint(samples []tlog.BatterySample, endpoint tlogEndpointKey) []tlog.BatterySample {
	filtered := make([]tlog.BatterySample, 0, len(samples))
	for _, sample := range samples {
		if sample.SystemID == endpoint.systemID && sample.ComponentID == endpoint.componentID {
			filtered = append(filtered, sample)
		}
	}
	return filtered
}

func accumulateGPSGaps(gps *health.GPSInput, timestamps []uint64) {
	if len(timestamps) < 3 {
		return
	}
	deltas := make([]uint64, 0, len(timestamps)-1)
	for i := 1; i < len(timestamps); i++ {
		if timestamps[i] > timestamps[i-1] {
			deltas = append(deltas, timestamps[i]-timestamps[i-1])
		}
	}
	if len(deltas) < 2 {
		return
	}
	baselineValues := append([]uint64(nil), deltas...)
	sort.Slice(baselineValues, func(i, j int) bool { return baselineValues[i] < baselineValues[j] })
	baseline := baselineValues[len(baselineValues)/2]
	threshold := baseline * 5
	const minimumGapUS = uint64(5_000_000)
	if threshold < minimumGapUS {
		threshold = minimumGapUS
	}
	for _, delta := range deltas {
		if delta <= threshold {
			continue
		}
		gps.GapEvents++
		seconds := float64(delta) / 1e6
		if seconds > gps.MaxGapSeconds {
			gps.MaxGapSeconds = seconds
		}
	}
}

func accumulateTrackQuality(data *health.DataInput, timestamp uint64, lat, lon float64, previous uint64) {
	if timestamp > 0 {
		data.TimedSamples++
		if previous > 0 && timestamp <= previous {
			data.NonMonotonicTimes++
		}
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 || (lat == 0 && lon == 0) {
		data.InvalidCoordinates++
	}
}

func previousULogTimestamp(samples []ulog.GPSSample, i int) uint64 {
	if i == 0 {
		return 0
	}
	return samples[i-1].TimestampUS
}

func previousDataFlashTimestamp(samples []dataflash.GPSSample, i int) uint64 {
	if i == 0 {
		return 0
	}
	return samples[i-1].TimestampUS
}

func previousTLOGTimestamp(samples []tlog.GPSSample, i int) uint64 {
	if i == 0 {
		return 0
	}
	return samples[i-1].TimestampUS
}

func accumulateULogBattery(out *health.BatteryInput, samples []ulog.BatterySample) {
	for _, sample := range samples {
		accumulateBattery(out, sample.VoltageV, sample.CurrentA, sample.Remaining)
	}
}

func accumulateDataFlashBattery(out *health.BatteryInput, samples []dataflash.BatterySample) {
	for _, sample := range samples {
		accumulateBattery(out, sample.VoltageV, sample.CurrentA, sample.Remaining)
	}
}

func accumulateTLOGBattery(out *health.BatteryInput, samples []tlog.BatterySample) {
	for _, sample := range samples {
		accumulateBattery(out, sample.VoltageV, sample.CurrentA, sample.Remaining)
	}
}

func accumulateBattery(out *health.BatteryInput, voltage, current, remaining float64) {
	out.Samples++
	if voltage > 0 && (out.MinVoltageV == nil || voltage < *out.MinVoltageV) {
		v := voltage
		out.MinVoltageV = &v
	}
	if current > 0 && (out.MaxCurrentA == nil || current > *out.MaxCurrentA) {
		v := current
		out.MaxCurrentA = &v
	}
	if remaining > 0 && remaining <= 1 {
		if out.StartRemaining == nil {
			v := remaining
			out.StartRemaining = &v
		}
		v := remaining
		out.EndRemaining = &v
	}
}
