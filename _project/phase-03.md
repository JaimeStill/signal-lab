# Phase 3 — Queue Groups + Headers

**Branch:** `phase-03-queues-headers`

## NATS Concepts

- **Queue groups** — Multiple subscribers join a named group; NATS delivers each message to exactly one member (load balancing)
- **Message headers** — HTTP-like key-value metadata attached to messages without polluting the payload

## Objective

Demonstrate horizontal scaling of dispatch workers via queue groups and structured message metadata via NATS headers for priority-based routing.

## Changes

### Queue Groups

- Dispatch telemetry subscriptions join queue group `dispatch-workers`
- Running multiple dispatch instances distributes messages across them
- Each instance only receives a subset of telemetry signals

### Headers

Sensor attaches headers to published signals:
- `Signal-Priority` — `low`, `normal`, `high`, `critical`
- `Signal-Source` — originating service name
- `Signal-Trace-ID` — UUID for request tracing

Dispatch reads headers and routes accordingly:
- `critical` and `high` priority signals logged at warn/error level
- Priority included in SSE stream metadata

## Files to Modify

**`internal/sensor/telemetry.go`**
- Attach NATS headers when publishing (use `nats.Header` on `nats.Msg`)
- Randomize priority distribution (mostly normal, occasional high/critical)

**`internal/dispatch/monitoring.go`**
- Subscribe with queue group: `conn.QueueSubscribe(subject, "dispatch-workers", handler)`
- Extract and log headers from received messages
- Include priority in SSE event data

## Verification

1. Start sensor + two dispatch instances (different ports)
2. `POST /api/telemetry/start` on sensor
3. Observe messages distributed across both dispatch instances (each gets ~50%)
4. Observe header metadata in SSE streams and logs
5. Stop one dispatch instance → remaining instance receives all messages
