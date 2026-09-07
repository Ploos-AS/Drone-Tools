# Security Policy

Drone-Tools processes files originating from flight controllers, mobile applications, cameras and other external sources. Imported files must therefore be treated as untrusted input.

## M0 security baseline

- container runs as a non-root user
- no Linux capabilities are required
- no privileged container mode is required
- application data is isolated below `/data`
- core operation does not require outbound network access
- flight-control functionality is out of scope

Please report security issues privately to the project maintainers rather than opening a public issue containing exploit details.
