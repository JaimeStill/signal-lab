# Phase 4 — Bidirectional Control

**Branch:** `phase-04-control`

## NATS Concepts

- **Request/reply for command/control** — Using NATS request/reply to send commands and receive acknowledgments
- **Bidirectional signal flow** — Both services act as publishers and subscribers simultaneously
- **Closed-loop coordination** — Continuous feedback cycle: observe → evaluate → adjust → observe

## Objective

Dispatch evaluates incoming telemetry against target thresholds and sends adjustment signals back to sensor. Sensor receives adjustments and mutates its simulated state, creating a closed control loop where sensor readings trend toward dispatch's desired targets.

## New Endpoints

```
Dispatch:
  POST /api/control/adjust      → manually send adjustment to sensor
  GET  /api/control/status      → current target thresholds, active adjustments

Sensor:
  GET  /api/control/state       → current adjustments applied, simulated state
```

## Control Loop Design

1. Sensor emits telemetry reading (e.g., `temp.zone-a = 85°F`)
2. Dispatch receives reading, compares against target (e.g., target = 72°F)
3. Dispatch publishes adjustment to `signal.control.sensor` (e.g., `{"type": "temp", "zone": "zone-a", "delta": -13}`)
4. Sensor receives adjustment, modifies its simulation parameters
5. Sensor's next readings trend toward target (not instant — gradual convergence)

## Files to Create/Modify

**`internal/dispatch/control.go`**
- Target threshold state (in-memory for now, KV store in Phase 6)
- Evaluation logic: compare telemetry against targets
- Publish adjustments to `signal.control.{target-service}`
- HTTP handlers for manual adjust and status

**`internal/sensor/control.go`**
- Subscribe to `signal.control.sensor` on startup
- Apply adjustments to simulation parameters
- Gradual convergence: readings drift toward target over multiple cycles
- HTTP handler for current control state

**`internal/sensor/api.go`** — add control routes
**`internal/dispatch/api.go`** — add control routes

## Verification

1. Start both services, begin telemetry publishing
2. `GET /api/control/status` on dispatch → shows default thresholds
3. `POST /api/control/adjust` on dispatch → sends manual adjustment
4. `GET /api/control/state` on sensor → shows applied adjustment
5. Observe telemetry readings gradually trending toward target values
6. SSE stream shows convergence over time
