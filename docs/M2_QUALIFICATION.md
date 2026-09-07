# M2 Qualification

Status: **QUALIFIED**

Baseline: `9483ff9399d473253a4689e6b763c9aefd3a344c` (`M2.9: add local flight telemetry charts`)

CI run: `#68` / `34145427406`

## Qualified scope

M2 now provides the planned flight-log foundation:

- ArduPilot DataFlash `.bin` inspection and core GPS/battery telemetry decoding.
- PX4 ULog `.ulg` inspection and core GPS/local-position/battery telemetry decoding.
- MAVLink `.tlog` inspection for MAVLink 1 and MAVLink 2, including signed MAVLink 2 frame sizing, plus core GPS and battery telemetry decoding.
- A common Flight Workspace surface for map rendering and flight analysis across GPX, ULog, DataFlash and TLOG.
- Derived flight metrics including distance, duration, average/max speed, elevation range/gain/loss and available battery metrics.
- Local SVG telemetry charts for elevation, speed, battery voltage and battery remaining, with no CDN or external chart service.
- OCI runtime smoke testing and amd64/arm64 builds.

## CI evidence

Run `34145427406` completed successfully:

- format check: PASS
- `go vet ./...`: PASS
- `go test ./...`: PASS
- amd64 build: PASS
- arm64 build: PASS
- runtime smoke image: PASS
- configured runtime user verification: PASS
- container smoke test: PASS
- multi-architecture OCI image build: PASS

## Known limitations / correctness debt

Qualification means the planned M2 capability is present and the automated gate is green. It does **not** mean every real-world log variant is fully covered yet.

Known follow-up items include:

- Validate ULog and DataFlash parsing against additional small real-log regression fixtures.
- Resolve nested ULog format types instead of rejecting or skipping schemas that depend on unsupported nested definitions.
- Decode 64-bit timestamps directly rather than routing integer timestamp values through floating-point helpers where applicable.
- Avoid false derived track jumps when multiple GPS receivers/sources are interleaved; preserve source/instance identity and choose or split tracks deterministically.
- Detect or sort non-monotonic timestamps before duration/distance derivation.
- Filter poor/invalid GPS samples for derived map/distance while retaining raw telemetry.
- Make log-extension dispatch case-insensitive (`.ULG`, `.BIN`, `.TLOG`).
- Fix the older GPX analyzer segment-bridging behavior so separate track segments do not create synthetic distance/elevation/speed jumps.
- Expand MAVLink dialect/message coverage beyond the core M2 subset.

These items are suitable for hardening alongside later milestones and must be revisited before declaring a stable production release.

## Decision

M2 is accepted as complete for roadmap progression. Development may proceed to M3 Flight Health while preserving the correctness-debt list above as release-blocking hardening work.
