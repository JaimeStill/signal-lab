# pkg/bus

Message bus connection management. Wraps the underlying messaging infrastructure (currently NATS) with connection lifecycle, reconnection handling, and health checking.

Named `bus` rather than `natsio` to keep the abstraction implementation-agnostic.

## Files (Phase 1)

- `conn.go` — `Connect()`, `Drain()`, reconnect/disconnect handlers, lifecycle integration
- `health.go` — Connection readiness checks
