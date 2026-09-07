# Architecture

## Principles

Drone-Tools is designed as a self-hosted, privacy-first application for drone hobbyists.

M0 establishes these constraints:

1. One OCI image.
2. Linux amd64 and arm64 are first-class targets.
3. Run as an unprivileged user.
4. Persist application state below `/data`.
5. Work without cloud accounts or API keys for core functionality.
6. Prefer local parsing and analysis.
7. Keep flight control out of scope.
8. Treat imported flight files as untrusted input.

## Initial shape

The M0 application is a single Go executable containing:

- HTTP server
- embedded static web assets
- health endpoint
- versioned API namespace

This deliberately keeps the deployment small while leaving room for later parser and analysis packages.

## Planned packages

Future milestones are expected to introduce packages broadly along these boundaries:

- `internal/importer` - file identification and safe ingestion
- `internal/flight` - normalized flight model
- `internal/formats` - GPX/KML/GeoJSON/ULog/ArduPilot adapters
- `internal/analysis` - deterministic health and telemetry analysis
- `internal/store` - SQLite-backed local persistence
- `internal/geo` - coordinate and geometry utilities

Heavy photogrammetry is intentionally not part of the core process. Integration with external tools such as WebODM can be considered later.
