# internal/dispatch

Dispatch service domain logic. Monitors sensor telemetry, evaluates thresholds, and sends control adjustments to achieve desired state.

## Files

- `api.go` — Module assembly, route registration (Phase 1)
- `discovery.go` — Ping responder, returns service metadata (Phase 1)
- `monitoring.go` — Subscribes to telemetry, evaluates thresholds (Phase 2)
- `control.go` — Sends adjustment commands to sensor (Phase 4)
