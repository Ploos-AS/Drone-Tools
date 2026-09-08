# M3.5–M3.11 qualification

Status: **QUALIFIED**

This qualification covers the MAVLink TLOG correctness and endpoint-selection hardening delivered after M3.4.

## Qualified scope

### M3.5 — MAVLink correctness hardening

- deterministic `GLOBAL_POSITION_INT` track preference with `GPS_RAW_INT` fallback
- `BATTERY_STATUS` preference over `SYS_STATUS`
- X25 CRC validation for the supported core messages
- signed-record accounting without claiming cryptographic signature verification

### M3.6 — parser resilience

- conservative structural resynchronization after garbage
- resync accounting through `resync_events` and `skipped_bytes`
- unsupported MAVLink2 incompatibility flags are surfaced as unsupported protocol semantics

### M3.7 — MAVLink2 truncated payload handling

- trailing MAVLink2 payload fields may be zero-padded only when the fields required by Drone-Tools are present
- MAVLink1 remains strict
- short payloads are not converted into false GPS telemetry

### M3.8 — GPS validity hardening

- `(0,0)` is excluded as a usable flight-track coordinate
- `GPS_RAW_INT.vel == UINT16_MAX` is treated as unknown by flight analysis
- `satellites_visible == UINT8_MAX` is treated as unknown by health scoring

### M3.9 — endpoint-aware telemetry

- normalized GPS and battery samples retain `system_id` and `component_id`
- tracks use one deterministic endpoint rather than mixing multiple MAVLink systems/components

### M3.10 — endpoint-aware analysis and health

- track, GPS quality, warnings and battery metrics use the same selected endpoint
- foreign endpoints cannot contaminate the selected flight's health result
- battery fallback is used only when the selected endpoint has no battery data

### M3.11 — HEARTBEAT role awareness

- `HEARTBEAT` provides endpoint-role inventory using `MAV_TYPE` and `MAV_AUTOPILOT`
- flight-controller endpoints are preferred when HEARTBEAT establishes that role
- GCS and onboard-controller roles are surfaced without deriving roles from component IDs
- map, analysis and Flight Health use the same role-aware endpoint choice
- absent HEARTBEAT data falls back to the deterministic M3.10 selection rule

## CI evidence

Final code baseline: `c74245fc3be8eb85211911f7364c51ebe4df920e`

GitHub Actions CI run: **#131** (`34201053122`)

- format: PASS
- `go vet`: PASS
- tests: PASS
- amd64 build: PASS
- arm64 build: PASS
- runtime smoke image: PASS
- configured non-root runtime user: PASS
- container smoke test: PASS
- multi-architecture OCI build: PASS

## Known limitations

- MAVLink2 signed frames are structurally parsed and counted, but signature authenticity is not verified because Drone-Tools has no signing-key context.
- CRC validation is available for the supported core-message CRC extras; unknown dialect messages remain structural-only.
- synthetic regression fixtures dominate this qualification; real-world TLOG fixture coverage should still be added.
- Flight Health is a deterministic log-derived indicator, not a flight-worthiness, legal-flight, or safety certification.

## Administrative reconciliation

The stage-metadata debt recorded when this qualification was written was resolved in M3.12: the startup log and `/api/v1/info` now report `M3.12`. This later metadata reconciliation does not alter the M3.5–M3.11 qualification baseline or its CI evidence.

## Verdict

M3.5 through M3.11 runtime behavior is qualified on the final code baseline above.
