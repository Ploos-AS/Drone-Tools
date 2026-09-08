# Roadmap

## M0 - Foundation

- project skeleton
- MIT license
- Go web service
- embedded UI
- health endpoint
- OCI build
- Docker Compose
- Podman Quadlet
- non-root runtime
- amd64/arm64 CI
- architecture and security baseline

## M1 - File inspector and geospatial basics

- drag-and-drop file inspection
- GPX
- KML/KMZ
- GeoJSON
- metadata and validation
- coordinate conversion
- distance and bearing tools
- 2D flight-track map

## M2 - Flight logs — qualified

Qualification: `docs/M2_QUALIFICATION.md`.

- ArduPilot DataFlash BIN
- MAVLink TLOG
- PX4 ULog
- normalized telemetry model
- charts
- flight summary

## M3 - Flight health — hardening through M3.12 qualified

Qualifications: `docs/M3_5_TO_M3_11_QUALIFICATION.md` and `docs/M3_12_QUALIFICATION.md`.

Completed through M3.12:

- GPS quality and precision metrics
- deterministic Flight Health foundation
- MAVLink CRC and parser correctness hardening
- MAVLink2 truncated-payload handling
- GPS sentinel/validity hardening
- endpoint-aware TLOG telemetry, analysis and health
- HEARTBEAT-based MAVLink endpoint-role awareness
- conservative MAVLink `RADIO_STATUS` Link Health with explicit unavailable semantics
- Flight Health UI for GPS, battery, data and link components

Remaining M3 scope:

- vehicle-aware provenance for MAVLink link telemetry
- battery health indicators beyond generic telemetry availability
- vibration and sensor indicators
- deterministic anomaly rules beyond the current GPS/data/battery/link model
- flight-to-flight comparison

## M4 - Local logbook

- SQLite persistence
- aircraft
- batteries
- flights
- maintenance records
- notes and tags
- import/export

## Later

- photo EXIF and flight-track correlation
- replay
- weather context
- external airspace-data integrations
- WebODM integration
- optional passive Remote ID receiver companion
