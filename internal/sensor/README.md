# internal/sensor

Sensor service domain logic. Publishes environment telemetry, responds to discovery pings, and receives control adjustments from dispatch.

## Files

- `api.go` — Module assembly, route registration (Phase 1)
- `discovery.go` — Ping responder, returns service metadata (Phase 1)
- `telemetry.go` — Reading types, simulated publisher (Phase 2)
- `control.go` — Receives adjustment signals, mutates simulated state (Phase 4)
