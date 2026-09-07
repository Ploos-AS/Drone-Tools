# Drone-Tools

Self-hosted, privacy-first web tools for drone hobbyists.

Drone-Tools is intended to be a lightweight "IT-Tools for drones": inspect flight-related files, analyze telemetry, work with coordinates and maps, and keep useful drone data local.

## M0 scope

M0 establishes the project foundation:

- Go web service
- minimal embedded web UI
- health endpoint
- OCI container image
- Docker Compose example
- Podman Quadlet example
- non-root runtime
- amd64 and arm64 CI build coverage
- persistent `/data` directory
- MIT license
- roadmap and architecture documentation

## Planned capabilities

Initial functional milestones will focus on:

- GPX, KML and GeoJSON inspection
- ArduPilot BIN/TLOG support
- PX4 ULog support
- flight maps and replay
- telemetry and battery analysis
- coordinate tools
- local flight logbook
- photo/EXIF geolocation tools

Drone-Tools is not a flight-control application and does not replace a ground-control station.

## Run with Docker Compose

```sh
docker compose up --build
```

Then open <http://localhost:8080>.

## Run directly

Requires Go 1.25 or newer.

```sh
go run ./cmd/drone-tools
```

Environment variables:

- `DRONE_TOOLS_ADDR` - listen address, default `:8080`
- `DRONE_TOOLS_DATA_DIR` - persistent data directory, default `/data`

## Health check

```sh
curl http://localhost:8080/healthz
```

## License

MIT. See [LICENSE](LICENSE).
