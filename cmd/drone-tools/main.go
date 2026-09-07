package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	geodata "github.com/Ploos-AS/Drone-Tools/internal/geo"
	"github.com/Ploos-AS/Drone-Tools/internal/gpx"
	"github.com/Ploos-AS/Drone-Tools/internal/inspector"
)

//go:embed web/*
var webFiles embed.FS

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
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

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Drone-Tools M1.2 listening on %s", addr)
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
			"timestamp": time.Now().UTC().Format(time.RFC3339), "stage": "M1.2",
		})
	})
	mux.HandleFunc("/api/v1/inspect", inspectHandler)
	mux.HandleFunc("/api/v1/gpx/summary", gpxSummaryHandler)
	mux.HandleFunc("/api/v1/kml/summary", geoSummaryHandler(geodata.ParseKML, "KML"))
	mux.HandleFunc("/api/v1/geojson/summary", geoSummaryHandler(geodata.ParseGeoJSON, "GeoJSON"))

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

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
