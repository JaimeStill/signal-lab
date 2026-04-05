# Phase 2 — Telemetry Pub/Sub + Subject Hierarchies

**Branch:** `phase-02-telemetry`

## NATS Concepts

- **Fire-and-forget pub/sub** — Publisher sends messages without waiting for acknowledgment; messages are lost if no subscriber is listening
- **Subject hierarchies** — Dot-delimited subject names creating a tree structure (e.g., `signal.telemetry.temp.zone-a`)
- **Wildcard subscriptions** — `*` matches exactly one token, `>` matches one or more trailing tokens

## Objective

Sensor publishes simulated environment readings to hierarchical subjects. Dispatch subscribes with wildcards to receive telemetry and exposes it via SSE streaming.

## New Endpoints

```
Sensor:
  POST /api/telemetry/start     → begin publishing readings at configured interval
  POST /api/telemetry/stop      → stop publishing
  GET  /api/telemetry/status    → current publisher state (running, interval, types, zones)

Dispatch:
  GET  /api/monitoring/stream   → SSE stream of received telemetry signals
  GET  /api/monitoring/status   → subscription state, message counts
```

## Files to Create/Modify

**`internal/sensor/telemetry.go`**
- Reading types: temperature, humidity, pressure with zone identifiers
- Simulated publisher: generates randomized readings at a configurable interval
- Publishes to `signal.telemetry.{type}.{zone}` subjects
- HTTP handlers for start/stop/status

**`internal/dispatch/monitoring.go`**
- Subscribes to `signal.telemetry.>` (all telemetry) on startup
- Tracks message counts per subject
- SSE endpoint streams received signals to HTTP clients
- Status endpoint reports subscription state

**`internal/sensor/api.go`** — add telemetry routes
**`internal/dispatch/api.go`** — add monitoring routes
**`internal/config/config.go`** — add telemetry interval, reading types/zones config

## Subject Hierarchy Design

```
signal.telemetry.temp.zone-a       → temperature reading from zone A
signal.telemetry.temp.zone-b       → temperature reading from zone B
signal.telemetry.humidity.zone-a   → humidity reading from zone A
signal.telemetry.pressure.zone-a   → pressure reading from zone A

Subscription patterns:
  signal.telemetry.>               → all telemetry (dispatch default)
  signal.telemetry.temp.*          → all temperature readings
  signal.telemetry.*.zone-a        → all readings from zone A
```

## Verification

1. Start both services
2. `POST /api/telemetry/start` on sensor → publisher begins
3. `GET /api/monitoring/stream` on dispatch → SSE events flow
4. `GET /api/telemetry/status` → shows running state
5. `GET /api/monitoring/status` → shows message counts by subject
6. `POST /api/telemetry/stop` → publisher stops, SSE stream goes quiet
