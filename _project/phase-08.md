# Phase 8 — WebSocket Signal Projection

**Branch:** `phase-08-websocket`

## Concepts

- **WebSocket messaging boundary** — Bridging NATS subjects to browser clients via persistent WebSocket connections
- **Signal projection** — Curating and filtering the signal topology for human consumption
- **Fan-out** — Broadcasting NATS signals to multiple connected WebSocket clients
- **Reconnection semantics** — Handling WebSocket disconnect/reconnect with JetStream consumer replay

## Objective

Add a web client that connects via WebSocket to visualize real-time signal flow across the system. The service bridges NATS subjects to WebSocket channels, projecting a curated view of discovery, telemetry, and control activity.

## New Endpoints

```
Service (sensor or dispatch, or a dedicated gateway — TBD):
  GET /ws                          → WebSocket upgrade, signal stream
  GET /                            → serve web client SPA
```

## Architecture Decisions (to resolve during implementation)

- **Which service hosts the WebSocket?** Options: sensor, dispatch, or a third lightweight gateway service
- **Web client technology** — Lit (matching herald) or vanilla JS for simplicity
- **Channel design** — one WebSocket with multiplexed subjects, or multiple connections per topic
- **Replay on reconnect** — leverage JetStream consumer cursors to fill gaps

## WebSocket Protocol Design

```
Client → Server:
  {"action": "subscribe", "subject": "signal.telemetry.>"}
  {"action": "unsubscribe", "subject": "signal.telemetry.>"}

Server → Client:
  {"type": "signal", "subject": "signal.telemetry.temp.zone-a", "data": {...}}
  {"type": "discovery", "services": [...]}
  {"type": "control", "adjustment": {...}}
```

## Dashboard Features

- **Service map** — Live discovery status showing connected services
- **Telemetry feed** — Real-time readings with subject filtering
- **Control loop** — Current thresholds, active adjustments, convergence visualization
- **Signal inspector** — Raw signal viewer with headers and metadata

## Files to Create (tentative)

**`pkg/ws/`** (new)
- WebSocket upgrade handler
- Client connection management (hub pattern)
- NATS-to-WebSocket bridge: subscribe to NATS subjects, fan out to WebSocket clients

**Web client** — structure TBD based on technology choice

## Verification

1. Start all services with telemetry publishing
2. Open web client in browser
3. See live telemetry feed updating in real time
4. Trigger discovery ping → service map updates
5. Adjust thresholds → control loop visualization responds
6. Disconnect/reconnect browser → missed signals replayed
