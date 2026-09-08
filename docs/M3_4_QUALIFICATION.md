# M3.4 Qualification — GPS precision metrics

Status: PASS

Qualified baseline: `a6ed2eb18dd73e128cc964725f77861ce738b288`

CI qualification: GitHub Actions run #98 (`34168395686`).

## Scope

M3.4 extends Flight Health with format-aware GPS precision metrics while preserving the existing deterministic score model and keeping incompatible units separate.

### PX4 ULog

- Parses horizontal GPS accuracy in metres from `vehicle_gps_position.eph`.
- Parses vertical GPS accuracy in metres from `vehicle_gps_position.epv`.
- Health thresholds are applied in metres.

### ArduPilot DataFlash

- Parses GPS `HDop` where present.
- Health thresholds are applied as DOP values, not metres.

### MAVLink TLOG

- Parses `GPS_RAW_INT.eph` as HDOP and `GPS_RAW_INT.epv` as VDOP using MAVLink's x100 scaling.
- Treats `UINT16_MAX` as unknown and excludes it from precision scoring.
- Does not infer fix or precision data from `GLOBAL_POSITION_INT` fields that do not carry those values.

## Flight Health behaviour

- Algorithm identifier: `m3.4-deterministic-v3`.
- GPS, data-quality and battery weights remain unchanged.
- Precision quality has its own sample denominator.
- A GPS sample receives at most one precision penalty even when more than one precision metric exceeds its format-specific threshold.
- Satellite-count and precision findings remain distinct.

Current thresholds:

- HDOP > 2.5
- VDOP > 3.5
- PX4 horizontal accuracy > 5 m
- PX4 vertical accuracy > 8 m

These thresholds are policy in the health adapter layer, not parser semantics.

## CI evidence

Run #98 passed:

- gofmt
- go vet
- unit/integration tests
- linux/amd64 build
- linux/arm64 build
- OCI runtime image build
- configured non-root runtime user verification
- container smoke test
- multi-architecture OCI build

## Known limitations / follow-up

- Thresholds are deterministic project policy, not legal or manufacturer flight-safety limits.
- Real-world binary fixtures are still desirable in addition to synthetic parser fixtures.
- MAVLink frame CRC/CRC_EXTRA validation remains a separate hardening item.
- TLOG source selection/interleaving should remain under regression coverage when both `GPS_RAW_INT` and `GLOBAL_POSITION_INT` are present.
- `/api/v1/info` milestone metadata still needs reconciliation from the older M3.1 label to M3.4; this is administrative and does not change the qualified M3.4 scoring behaviour.
