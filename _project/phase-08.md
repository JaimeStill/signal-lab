# Phase 8 — Subject Inspector (WebSocket Bridge)

**Branch:** `phase-08-inspector`

## NATS Concepts

- **Go-managed WebSocket hub** — Classic "backend-for-frontend" pattern: a browser client connects to the Go service over WebSocket; the Go service maintains the connection lifecycle, receives subscribe/unsubscribe commands from the client, opens corresponding NATS subscriptions on the client's behalf, and fans received messages out over the WebSocket.
- **Hub fan-out** — One NATS subscription per subject, N browser clients — the hub multiplexes. Multiple clients subscribing to the same subject share a single NATS subscription on the server side.
- **Reconnection semantics** — Clients that drop and reconnect re-send their subscribe commands; the hub re-establishes their subject subscriptions without disturbing other clients.
- **Embedded SPA** — The web client is built ahead of time and embedded into the Go binary via `//go:embed`, matching herald's binary-shipped frontend pattern.

## Objective

Alpha hosts a generic NATS subject inspector: a browser UI where the user types a subject pattern, hits Subscribe, and watches messages flow in real time. Multiple concurrent subscriptions per client, with the ability to pause, unsubscribe, and filter. The Go service is the bridge — it holds the WebSocket connection, translates client commands into NATS subscriptions, and streams messages back. Everything is written against the `pkg/ws/` hub infrastructure, which is reusable for any future service that needs a real-time web client.

The inspector is deliberately domain-agnostic: it doesn't know about telemetry, jobs, journal, or any other phase's subjects. It's a general NATS visualization tool that happens to be useful for inspecting prior phases as a side benefit.

## Domain

**Reusable `pkg/ws/` package (new):**
- `Hub` — client registry and fan-out loop; owns the `*nats.Conn` and dynamically manages subject subscriptions based on client requests
- `Client` — per-connection read/write pumps (read client commands, write NATS messages)
- `Protocol` — typed client↔server message structs (subscribe, unsubscribe, signal, subscribed, unsubscribed, error)
- `Bridge` — binds NATS subscriptions to WebSocket fan-out; one NATS subscription per unique subject regardless of how many clients are watching it

**Alpha `inspector` domain (`internal/alpha/inspector/`):**
- `System` interface with `Start(lc) error`, `Handler() *Handler`
- Constructs the hub, wires it to lifecycle (graceful shutdown closes all WebSocket connections before NATS drain)
- `Handler` serves `GET /` (embedded SPA), `GET /dist/*` (SPA assets), `GET /ws` (WebSocket upgrade → hand connection to hub)

**Web client source (`app/inspector/`):**
- Built separately into `app/inspector/dist/` and embedded via `//go:embed`
- Stack TBD — either Lit (matching herald) or vanilla JS for simplicity. Keep it small so it doesn't steal focus from the NATS concepts
- UI surface: subject pattern input, active subscriptions list, message stream pane with filter, pause/resume toggle

## WebSocket Protocol

```
Client → Server:
  {"action": "subscribe",   "subject": "signal.telemetry.>"}
  {"action": "unsubscribe", "subject": "signal.telemetry.>"}

Server → Client:
  {"type": "subscribed",   "subject": "signal.telemetry.>"}
  {"type": "unsubscribed", "subject": "signal.telemetry.>"}
  {"type": "signal",       "subject": "signal.telemetry.temp.zone-a",
                           "headers": {...}, "data": {...}}
  {"type": "error",        "message": "..."}
```

The server-sent `signal` frame includes the NATS subject, decoded headers (if any), and the raw payload. The web client can pretty-print, filter by subject, or inspect raw bytes.

## Dependencies

- `github.com/coder/websocket` — WebSocket library (maintained successor to `nhooyr.io/websocket`)
- Web client toolchain (bundler, template engine) — TBD during implementation

## New Endpoints

```
Alpha:
  GET  /                     → embedded SPA (inspector UI)
  GET  /dist/*               → static SPA assets
  GET  /ws                   → WebSocket upgrade; clients connect here and issue protocol commands
```

No new REST endpoints for the inspector domain — everything client-facing goes through the WebSocket. The existing `/healthz`, `/readyz`, and `/api/*` endpoints from prior phases remain untouched.

## Lifecycle

- **Startup** — hub goroutine starts, ready to accept WebSocket connections
- **Client connect** — upgrade the HTTP connection, register the client with the hub, start read/write pumps
- **Subscribe command** — if no existing NATS subscription for the subject, the hub opens one. Register the client as interested in that subject.
- **NATS message arrives** — fan out to every interested client currently connected
- **Client disconnect** — unregister, decrement interest counts, drop NATS subscriptions that have zero interested clients
- **Shutdown** — close all WebSocket connections gracefully, then allow the bus drain to complete NATS teardown

## Verification

1. Start alpha. Open `http://localhost:3000/` in a browser — the inspector UI loads.
2. Type `signal.telemetry.>` in the subject input, click Subscribe. No messages yet (nothing is publishing).
3. Separately, start beta and `POST /api/telemetry/start`. The inspector begins receiving telemetry signals in real time.
4. Type a second subscription, `signal.jobs.>`, click Subscribe. Also start `POST /api/jobs/start` on alpha. The inspector now shows both streams interleaved.
5. Open a second browser tab, subscribe the same tab to `signal.telemetry.>`. Confirm both tabs receive messages independently (hub fan-out works).
6. Unsubscribe one tab from telemetry — only that tab stops receiving telemetry messages; the other tab continues.
7. Close both tabs — the Go side logs "client disconnected" and drops the now-zero-interest subscriptions.
8. Reconnect — subscriptions are re-established from scratch (the client remembers its own subscription list in localStorage or equivalent).

## Phase Independence (and a note on utility)

The inspector domain has no code-level dependency on telemetry, jobs, journal, settings, or artifacts. It subscribes to subjects the user types. That said, the inspector is genuinely useful for observing *any* phase's traffic — so it acts as a shared debugging tool even though it's not coupled to any prior phase. That's a happy side effect, not a dependency.
