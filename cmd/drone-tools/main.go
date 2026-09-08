package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Ploos-AS/Drone-Tools/internal/dataflash"
	geodata "github.com/Ploos-AS/Drone-Tools/internal/geo"
	"github.com/Ploos-AS/Drone-Tools/internal/gpx"
	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
	"github.com/Ploos-AS/Drone-Tools/internal/mapdata"
	"github.com/Ploos-AS/Drone-Tools/internal/ulog"
)

//go:embed web/*
var webFiles embed.FS

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type flightAnalysis struct {
	Quality             string   `json:"quality"`
	DistanceMeters      float64  `json:"distance_meters"`
	DurationSeconds     *float64 `json:"duration_seconds,omitempty"`
	AverageSpeedMPS     *float64 `json:"average_speed_mps,omitempty"`
	MaxSegmentSpeedMPS  *float64 `json:"max_segment_speed_mps,omitempty"`
	MinElevation        *float64 `json:"min_elevation_m,omitempty"`
	MaxElevation        *float64 `json:"max_elevation_m,omitempty"`
	ElevationGainMeters float64  `json:"elevation_gain_m"`
	ElevationLossMeters float64  `json:"elevation_loss_m"`
	TrackPoints         int      `json:"track_points"`
	TimedPoints         int      `json:"timed_points"`
	ElevationPoints     int      `json:"elevation_points"`
	BatteryVoltageV     *float64 `json:"battery_voltage_v,omitempty"`
	BatteryCurrentA     *float64 `json:"battery_current_a,omitempty"`
	BatteryRemaining    *float64 `json:"battery_remaining,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}

func main() {
	addr := envOrDefault("DRONE_TOOLS_ADDR", ":8080")
	dataDir := envOrDefault("DRONE_TOOLS_DATA_DIR", "/data")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create data directory: %v", err)
	}

	handler, err := newHandler(dataDir)
	if err != nil {
		log.Fatalf("prepare HTTP handler: %v", err)
	}

	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("Drone-Tools M3.12 listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(fmt.Errorf("server: %w", err))
	}
}

func newHandler(dataDir string) (http.Handler, error) {
	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		return nil, fmt.Errorf("prepare embedded web assets: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(webRoot)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok", Service: "drone-tools"})
	})
	mux.HandleFunc("/api/v1/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "Drone-Tools", "data_dir": filepath.Clean(dataDir),
			"timestamp": time.Now().UTC().Format(time.RFC3339), "stage": "M3.12",
		})
	})
	mux.HandleFunc("/api/v1/inspect", inspectHandler)
	mux.HandleFunc("/api/v1/gpx/summary", gpxSummaryHandler)
	mux.HandleFunc("/api/v1/analyze", analyzeHandler)
	mux.HandleFunc("/api/v1/health", flightHealthHandler)
	mux.HandleFunc("/api/v1/kml/summary", geoSummaryHandler(geodata.ParseKML, "KML"))
	mux.HandleFunc("/api/v1/geojson/summary", geoSummaryHandler(geodata.ParseGeoJSON, "GeoJSON"))
	mux.HandleFunc("/api/v1/map", mapHandler)
	mux.HandleFunc("/api/v1/ulog/inspect", ulogInspectHandler)
	mux.HandleFunc("/api/v1/ulog/telemetry", ulogInspectHandler)
	mux.HandleFunc("/api/v1/dataflash/inspect", dataflashInspectHandler)
	mux.HandleFunc("/api/v1/dataflash/telemetry", dataflashInspectHandler)
	mux.HandleFunc("/api/v1/tlog/inspect", tlogInspectHandler)
	mux.HandleFunc("/api/v1/tlog/telemetry", tlogInspectHandler)
	return mux, nil
}

func inspectHandler(w http.ResponseWriter, r *http.Request) {
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
	result, err := inspector.Inspect(header.Filename, file)
	if errors.Is(err, inspector.ErrTooLarge) {
		http.Error(w, "file exceeds 32 MiB limit", http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		http.Error(w, "unable to inspect file", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func gpxSummaryHandler(w http.ResponseWriter, r *http.Request) {
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
	summary, err := gpx.Parse(file)
	if err != nil {
		http.Error(w, "invalid GPX file", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func analyzeHandler(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")
	switch filepath.Ext(header.Filename) {
	case ".ulg":
		summary, err := ulog.Inspect(file)
		if err != nil {
			http.Error(w, "invalid PX4 ULog file", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(analyzeULog(summary.Telemetry))
		return
	case ".bin":
		summary, err := dataflash.Inspect(file)
		if err != nil {
			http.Error(w, "invalid ArduPilot DataFlash log", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(analyzeDataFlash(summary.Telemetry))
		return
	case ".tlog":
		analysis, err := parseTLOGAnalysis(file)
		if err != nil {
			http.Error(w, "invalid MAVLink TLOG file", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(analysis)
		return
	}

	summary, err := gpx.Parse(file)
	if err != nil {
		http.Error(w, "invalid GPX file", http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(summary)
}

func geoSummaryHandler(parse func(io.Reader) (geodata.Summary, error), format string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		summary, err := parse(file)
		if err != nil {
			http.Error(w, "invalid "+format+" file", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summary)
	}
}

func mapHandler(w http.ResponseWriter, r *http.Request) {
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

	var doc mapdata.Document
	switch filepath.Ext(header.Filename) {
	case ".ulg":
		summary, err := ulog.Inspect(file)
		if err != nil {
			http.Error(w, "invalid PX4 ULog file", http.StatusBadRequest)
			return
		}
		doc, err = ulogMap(summary.Telemetry.GPS)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case ".bin":
		summary, err := dataflash.Inspect(file)
		if err != nil {
			http.Error(w, "invalid ArduPilot DataFlash log", http.StatusBadRequest)
			return
		}
		doc, err = dataflashMap(summary.Telemetry.GPS)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	case ".tlog":
		doc, err = parseTLOGMap(file)
		if err != nil {
			http.Error(w, "invalid MAVLink TLOG file", http.StatusBadRequest)
			return
		}
	default:
		doc, err = mapdata.Parse(header.Filename, file)
		if err != nil {
			http.Error(w, "unsupported or invalid map file", http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func ulogInspectHandler(w http.ResponseWriter, r *http.Request) {
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
	summary, err := ulog.Inspect(file)
	if err != nil {
		http.Error(w, "invalid PX4 ULog file", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func dataflashInspectHandler(w http.ResponseWriter, r *http.Request) {
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
	summary, err := dataflash.Inspect(file)
	if err != nil {
		http.Error(w, "invalid ArduPilot DataFlash log", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summary)
}

func ulogMap(samples []ulog.GPSSample) (mapdata.Document, error) {
	if len(samples) == 0 {
		return mapdata.Document{}, errors.New("ULog contains no GPS samples")
	}
	path := make([]mapdata.Coordinate, 0, len(samples))
	for _, sample := range samples {
		if sample.Latitude < -90 || sample.Latitude > 90 || sample.Longitude < -180 || sample.Longitude > 180 {
			continue
		}
		path = append(path, mapdata.Coordinate{sample.Longitude, sample.Latitude})
	}
	return finishTelemetryMap("px4-ulog", path, "ULog contains no valid GPS coordinates")
}

func dataflashMap(samples []dataflash.GPSSample) (mapdata.Document, error) {
	if len(samples) == 0 {
		return mapdata.Document{}, errors.New("DataFlash log contains no GPS samples")
	}
	path := make([]mapdata.Coordinate, 0, len(samples))
	for _, sample := range samples {
		if sample.Latitude < -90 || sample.Latitude > 90 || sample.Longitude < -180 || sample.Longitude > 180 {
			continue
		}
		path = append(path, mapdata.Coordinate{sample.Longitude, sample.Latitude})
	}
	return finishTelemetryMap("ardupilot-dataflash", path, "DataFlash log contains no valid GPS coordinates")
}

func finishTelemetryMap(format string, path []mapdata.Coordinate, emptyMessage string) (mapdata.Document, error) {
	if len(path) == 0 {
		return mapdata.Document{}, errors.New(emptyMessage)
	}
	bounds := mapdata.Bounds{MinLon: path[0][0], MaxLon: path[0][0], MinLat: path[0][1], MaxLat: path[0][1]}
	for _, point := range path[1:] {
		bounds.MinLon = math.Min(bounds.MinLon, point[0])
		bounds.MaxLon = math.Max(bounds.MaxLon, point[0])
		bounds.MinLat = math.Min(bounds.MinLat, point[1])
		bounds.MaxLat = math.Max(bounds.MaxLat, point[1])
	}
	return mapdata.Document{Format: format, Paths: [][]mapdata.Coordinate{path}, Bounds: &bounds}, nil
}

func analyzeULog(telemetry ulog.Telemetry) flightAnalysis {
	a := flightAnalysis{Quality: "good", TrackPoints: len(telemetry.GPS), TimedPoints: len(telemetry.GPS), ElevationPoints: len(telemetry.GPS)}
	if len(telemetry.GPS) == 0 {
		a.Quality = "limited"
		a.Warnings = append(a.Warnings, "no vehicle_gps_position samples")
		return a
	}

	minElevation := telemetry.GPS[0].AltitudeMeters
	maxElevation := minElevation
	var maxSpeed float64
	for i, sample := range telemetry.GPS {
		minElevation = math.Min(minElevation, sample.AltitudeMeters)
		maxElevation = math.Max(maxElevation, sample.AltitudeMeters)
		maxSpeed = math.Max(maxSpeed, sample.VelocityMPS)
		if sample.FixType > 0 && sample.FixType < 3 {
			a.Quality = "limited"
		}
		if sample.SatellitesUsed > 0 && sample.SatellitesUsed < 6 {
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
	a.MinElevation = &minElevation
	a.MaxElevation = &maxElevation
	if maxSpeed > 0 {
		a.MaxSegmentSpeedMPS = &maxSpeed
	}

	first := telemetry.GPS[0].TimestampUS
	last := telemetry.GPS[len(telemetry.GPS)-1].TimestampUS
	setTimedAnalysis(&a, first, last)

	for _, sample := range telemetry.GPS {
		if sample.FixType > 0 && sample.FixType < 3 {
			a.Warnings = append(a.Warnings, "GPS samples include fix type below 3D")
			break
		}
	}
	for _, sample := range telemetry.GPS {
		if sample.SatellitesUsed > 0 && sample.SatellitesUsed < 6 {
			a.Warnings = append(a.Warnings, "GPS samples include fewer than 6 satellites")
			break
		}
	}

	if len(telemetry.Battery) > 0 {
		latest := telemetry.Battery[len(telemetry.Battery)-1]
		a.BatteryVoltageV = pointer(latest.VoltageV)
		a.BatteryCurrentA = pointer(latest.CurrentA)
		a.BatteryRemaining = pointer(latest.Remaining)
	} else {
		a.Warnings = append(a.Warnings, "no battery_status samples")
	}
	return a
}

func analyzeDataFlash(telemetry dataflash.Telemetry) flightAnalysis {
	a := flightAnalysis{Quality: "good", TrackPoints: len(telemetry.GPS), TimedPoints: len(telemetry.GPS), ElevationPoints: len(telemetry.GPS)}
	if len(telemetry.GPS) == 0 {
		a.Quality = "limited"
		a.Warnings = append(a.Warnings, "no GPS/GPS2 samples")
		return a
	}

	minElevation := telemetry.GPS[0].AltitudeMeters
	maxElevation := minElevation
	var maxSpeed float64
	for i, sample := range telemetry.GPS {
		minElevation = math.Min(minElevation, sample.AltitudeMeters)
		maxElevation = math.Max(maxElevation, sample.AltitudeMeters)
		maxSpeed = math.Max(maxSpeed, sample.SpeedMPS)
		if sample.Status > 0 && sample.Status < 3 {
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
	a.MinElevation = &minElevation
	a.MaxElevation = &maxElevation
	if maxSpeed > 0 {
		a.MaxSegmentSpeedMPS = &maxSpeed
	}
	setTimedAnalysis(&a, telemetry.GPS[0].TimestampUS, telemetry.GPS[len(telemetry.GPS)-1].TimestampUS)

	for _, sample := range telemetry.GPS {
		if sample.Status > 0 && sample.Status < 3 {
			a.Warnings = append(a.Warnings, "GPS samples include status below 3D fix")
			break
		}
	}
	for _, sample := range telemetry.GPS {
		if sample.Satellites > 0 && sample.Satellites < 6 {
			a.Warnings = append(a.Warnings, "GPS samples include fewer than 6 satellites")
			break
		}
	}

	if len(telemetry.Battery) > 0 {
		latest := telemetry.Battery[len(telemetry.Battery)-1]
		a.BatteryVoltageV = pointer(latest.VoltageV)
		a.BatteryCurrentA = pointer(latest.CurrentA)
		a.BatteryRemaining = pointer(latest.Remaining)
	} else {
		a.Warnings = append(a.Warnings, "no BAT/BAT2 samples")
	}
	return a
}

func setTimedAnalysis(a *flightAnalysis, first, last uint64) {
	if last > first {
		duration := float64(last-first) / 1e6
		a.DurationSeconds = &duration
		average := a.DistanceMeters / duration
		a.AverageSpeedMPS = &average
	} else {
		a.Warnings = append(a.Warnings, "GPS timestamps do not provide a positive flight duration")
	}
}

func haversine(latA, lonA, latB, lonB float64) float64 {
	const earthRadius = 6371000.0
	lat1 := latA * math.Pi / 180
	lat2 := latB * math.Pi / 180
	dLat := (latB - latA) * math.Pi / 180
	dLon := (lonB - lonA) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
}

func pointer(value float64) *float64 {
	return &value
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
