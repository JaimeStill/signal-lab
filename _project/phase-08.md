# Phase 8 — WebSocket Signal Projection

**Branch:** `phase-08-websocket`

## Concepts

- **WebSocket as composable infrastructure** — A reusable `pkg/ws/` package any service can compose into its module assembly when it needs a real-time web client
- **Signal projection** — Curating and filtering the signal topology for human consumption
- **Hub pattern** — Central goroutine managing client connections, NATS subscriptions, and fan-out
- **Reconnection semantics** — Handling WebSocket disconnect/reconnect with JetStream consumer replay

## Objective

Add a reusable WebSocket package and embed a web client into one or both services. The WebSocket connection bridges NATS subjects to the browser, projecting a curated view of discovery, telemetry, and control activity. This follows the same pattern as herald's binary-embedded Lit SPA — the web client is compiled into the service binary via `//go:embed`.

## Architecture

WebSocket is a service-level capability, not a separate gateway. Any service that serves a web client composes the hub into its module assembly alongside its API routes. The same HTTP server handles both REST endpoints and the WebSocket upgrade.

```
pkg/ws/                          # Reusable WebSocket infrastructure
  hub.go                         # Client registry, fan-out loop
  client.go                      # Per-connection read/write pumps
  protocol.go                    # Client ↔ server message types
  bridge.go                      # NATS subscription ↔ WebSocket fan-out binding

internal/{service}/
  web.go                         # Web module: embedded SPA, /ws upgrade, hub ↔ NATS binding

app/                             # Web client source (Lit or vanilla JS — TBD)
  client/                        # SPA source
  dist/                          # Built assets (embedded in binary)
```

### Lifecycle Integration

The hub is a lifecycle participant:
- **Startup** — hub goroutine starts, ready to accept connections
- **Shutdown** — hub gracefully closes all WebSocket connections before NATS drain

### Module Assembly

The web module registers alongside the API module in `cmd/{service}/modules.go`:
- `GET /` — serves the embedded SPA
- `GET /dist/*` — serves static assets
- `GET /ws` — WebSocket upgrade, hands connection to hub

The hub holds the service's `*nats.Conn` and dynamically manages NATS subscriptions based on what connected clients have requested.

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

## Dependencies

- `github.com/coder/websocket` — WebSocket library (maintained successor to nhooyr.io/websocket)
- Web client technology TBD: Lit (matching herald) or vanilla JS for simplicity

## Verification

1. Start all services with telemetry publishing
2. Open web client in browser
3. See live telemetry feed updating in real time
4. Trigger discovery ping → service map updates
5. Adjust thresholds → control loop visualization responds
6. Disconnect/reconnect browser → missed signals replayed
