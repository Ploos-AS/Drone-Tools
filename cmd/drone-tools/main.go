package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
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

	webRoot, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatalf("prepare embedded web assets: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(webRoot)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:  "ok",
			Service: "drone-tools",
		})
	})
	mux.HandleFunc("/api/v1/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":      "Drone-Tools",
			"data_dir":  filepath.Clean(dataDir),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"stage":     "M0",
		})
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("Drone-Tools M0 listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(fmt.Errorf("server: %w", err))
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
