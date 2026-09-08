# M3.12 qualification

Status: **QUALIFIED**

M3.12 adds conservative MAVLink link-health telemetry to Flight Health and reconciles the runtime stage metadata.

## Qualified scope

- MAVLink `RADIO_STATUS` telemetry is normalized for Flight Health.
- Link Health is exposed as a separate health component.
- Link scoring is based on transport-pressure indicators available from `RADIO_STATUS`; raw RSSI/noise values are reported but are not assigned universal quality thresholds.
- Missing link telemetry is represented as unavailable and does not reduce the overall Flight Health score.
- The deterministic health algorithm is versioned as `m3.12-deterministic-v4`.
- The Flight Health UI exposes the Link component and explicitly renders `not available` when no supported radio data exists.
- Runtime startup metadata and `/api/v1/info` report stage `M3.12`.

## CI evidence

Qualified code baseline: `f9b2bc0f7cdd04fe7c5b6a56495447e100399942`

GitHub Actions CI run: **#151** (`34245344211`)

- format: PASS
- `go vet`: PASS
- tests: PASS
- amd64 build: PASS
- arm64 build: PASS
- runtime smoke image: PASS
- configured non-root runtime user: PASS
- container smoke test: PASS
- multi-architecture OCI build: PASS

## Safety and interpretation

- Link Health is a deterministic log-derived indicator, not a radio-range prediction or a flight-safety certification.
- RSSI/noise encodings and useful thresholds vary by radio hardware and MAVLink implementation, so M3.12 does not pretend that raw values have a universal meaning.
- Absence of `RADIO_STATUS` is not evidence of a bad link and therefore does not incur a score penalty.

## Known limitations

- Link telemetry is not yet associated with the selected MAVLink vehicle/system; a multi-system TLOG can contain radio records whose provenance is ambiguous.
- Real-world TLOG fixture coverage remains desirable in addition to the synthetic regression fixtures.
- More radio-specific metrics should only be scored after their semantics and provenance are known.

## Verdict

M3.12 is qualified on the code baseline and CI run above. The next link-health hardening step should make radio provenance vehicle-aware before adding more aggressive link scoring.
