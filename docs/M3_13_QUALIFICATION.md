# M3.13 qualification

Status: **QUALIFIED**

M3.13 hardens MAVLink Link Health by associating `RADIO_STATUS` telemetry with the selected flight vehicle.

## Qualified scope

- The selected MAVLink flight is identified by the same role-aware track selection used by TLOG map, analysis and health.
- Link Health filters `RADIO_STATUS` by the selected track's `system_id`.
- `RADIO_STATUS` from another MAVLink system cannot lower or otherwise contaminate the selected vehicle's Link Health.
- Multiple radio components inside the selected `system_id` remain valid inputs; Link Health does not require the radio `component_id` to equal the flight-controller component.
- When no track endpoint can be selected, M3.12 fallback behavior is retained.

## CI evidence

Qualified code baseline: `2300a41a86bbcf1c8a4a3a33723e8267cdc2a8f2`

GitHub Actions CI run: **#156** (`34282612802`)

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

- Battery selection still has older endpoint/fallback semantics and is handled separately after M3.13.
- Link Health still uses only conservative transport-pressure indicators; raw RSSI/noise values are not assigned universal thresholds.
- Real-world multi-system TLOG fixtures remain desirable in addition to synthetic regression fixtures.

## Verdict

M3.13 vehicle-aware Link Health is qualified on the code baseline and CI run above.
