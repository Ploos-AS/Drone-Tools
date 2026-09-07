package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/Ploos-AS/Drone-Tools/internal/dataflash"
	"github.com/Ploos-AS/Drone-Tools/internal/health"
	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
	"github.com/Ploos-AS/Drone-Tools/internal/tlog"
	"github.com/Ploos-AS/Drone-Tools/internal/ulog"
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
		summary, err := tlog.Inspect(file)
		if err != nil {
			http.Error(w, "invalid MAVLink TLOG file", http.StatusBadRequest)
			return
		}
		input = healthInputFromTLOG(summary.Telemetry)
	default:
		http.Error(w, "flight health currently supports .ulg, .bin and .tlog", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(health.Evaluate(input))
}

func healthInputFromULog(telemetry ulog.Telemetry) health.Input {
	in := health.Input{}
	for i, sample := range telemetry.GPS {
		in.GPS.Samples++
		in.Data.TrackSamples++
		if sample.FixType > 0 && sample.FixType < 3 {
			in.GPS.LowFixSamples++
		}
		if sample.SatellitesUsed > 0 && sample.SatellitesUsed < 6 {
			in.GPS.LowSatelliteSamples++
		}
		accumulateTrackQuality(&in.Data, sample.TimestampUS, sample.Latitude, sample.Longitude, previousULogTimestamp(telemetry.GPS, i))
	}
	accumulateULogBattery(&in.Battery, telemetry.Battery)
	return in
}

func healthInputFromDataFlash(telemetry dataflash.Telemetry) health.Input {
	in := health.Input{}
	for i, sample := range telemetry.GPS {
		in.GPS.Samples++
		in.Data.TrackSamples++
		if sample.Status > 0 && sample.Status < 3 {
			in.GPS.LowFixSamples++
		}
		if sample.Satellites > 0 && sample.Satellites < 6 {
			in.GPS.LowSatelliteSamples++
		}
		accumulateTrackQuality(&in.Data, sample.TimestampUS, sample.Latitude, sample.Longitude, previousDataFlashTimestamp(telemetry.GPS, i))
	}
	accumulateDataFlashBattery(&in.Battery, telemetry.Battery)
	return in
}

func healthInputFromTLOG(telemetry tlog.Telemetry) health.Input {
	in := health.Input{}
	for i, sample := range telemetry.GPS {
		in.GPS.Samples++
		in.Data.TrackSamples++
		if sample.FixType > 0 && sample.FixType < 3 {
			in.GPS.LowFixSamples++
		}
		if sample.Satellites > 0 && sample.Satellites < 6 {
			in.GPS.LowSatelliteSamples++
		}
		accumulateTrackQuality(&in.Data, sample.TimestampUS, sample.Latitude, sample.Longitude, previousTLOGTimestamp(telemetry.GPS, i))
	}
	accumulateTLOGBattery(&in.Battery, telemetry.Battery)
	return in
}

func accumulateTrackQuality(data *health.DataInput, timestamp uint64, lat, lon float64, previous uint64) {
	if timestamp > 0 {
		data.TimedSamples++
		if previous > 0 && timestamp <= previous {
			data.NonMonotonicTimes++
		}
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
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
