# signal-lab

A progressive NATS learning repository built around two Go web services — **sensor** and **dispatch** — that communicate via signal-based coordination through a shared message bus.

## Vision

This repository builds practical familiarity with [NATS](https://nats.io) as a signal layer for event-driven service orchestration. The architecture is inspired by the Organic Intelligence Architecture white paper's concept of domain-aware subsystems coordinating through a shared signaling medium.

Rather than implementing the full OIA vision, signal-lab focuses on exploring NATS capabilities through concrete, runnable demonstrations exposed as HTTP endpoints on real web services.

## Architecture

### Two Services, One Bus

**Sensor** — An environment monitoring service that simulates telemetry readings (temperature, humidity, pressure) and publishes them as signals. Responds to discovery pings with service metadata. Receives control adjustments from dispatch and mutates its simulated state accordingly.

**Dispatch** — A notification and response coordination service that subscribes to sensor telemetry, evaluates readings against target thresholds, and sends adjustment signals to drive sensor state toward desired targets. Responds to discovery pings with service metadata.

Communication between services flows exclusively through the message bus (NATS). Neither service calls the other directly.

### Layered Composition Architecture

Both services follow the LCA pattern adapted from the [herald](https://github.com/JaimeStill/herald) project:

- **Cold start** — Configuration loading, subsystem creation
- **Hot start** — Bus connection, HTTP server listen, readiness
- **Graceful shutdown** — Reverse-order teardown with timeout

### Package Hierarchy

```
cmd/                → Entry points (package main), one per service
internal/           → Private application packages
  config/           → Shared configuration (three-phase finalize)
  sensor/           → Sensor module wiring (domain.go, routes.go, api.go)
    telemetry/      → Telemetry publisher domain (System + Handler)
    alerts/         → Alert publisher domain with NATS headers (System + Handler)
  dispatch/         → Dispatch module wiring (domain.go, routes.go, api.go)
    monitoring/     → Telemetry monitoring domain (System + Handler)
    alerting/       → Alert monitoring domain with queue groups (System + Handler)
pkg/                → Reusable library packages
  lifecycle/        → Startup/shutdown coordination
  bus/              → Message bus System (connection + subscription management)
  signal/           → Signal envelope type
  contracts/        → Shared cross-service contracts
    telemetry/      → Telemetry subject constants + Reading type
    alerts/         → Alert priority type, header keys, subject constants
  discovery/        → Discovery domain (System + Handler + ServiceInfo)
  routes/           → Route group composition
  module/           → HTTP module/router system
  middleware/       → HTTP middleware (Logger)
  handlers/         → JSON response helpers
tests/              → Black-box tests mirroring source structure
```

Dependencies flow downward: `cmd/` → `internal/` → `pkg/`. Lower-level packages define contracts; higher-level packages implement them.

General-purpose features live in `pkg/` (bus, discovery, routes). Service-specific domains live under `internal/{service}/{domain}/` as sub-packages. Each domain exposes a `System` interface with an unexported implementing struct, following herald's repository/handler pattern.

## Configuration

Layered configuration with the `SIGNAL_` env var prefix:

1. `config.json` — Base configuration
2. `config.<SIGNAL_ENV>.json` — Environment overlay (e.g., `config.docker.json`)
3. `secrets.json` — Gitignored secrets
4. `SIGNAL_*` env vars — Final overrides

Each config struct follows the three-phase finalize pattern: `loadDefaults()` → `loadEnv()` → `validate()`.

## Docker Infrastructure

NATS runs in Docker via Compose:

```bash
docker compose up -d    # Start NATS with JetStream on :4222, monitoring on :8222
docker compose down     # Stop and remove
```

## Development

```bash
# Terminal 1: sensor service
mise run sensor         # or: go run ./cmd/sensor

# Terminal 2: dispatch service
mise run dispatch       # or: go run ./cmd/dispatch

# Testing
mise run test           # go test ./tests/...
mise run vet            # go vet ./...

# Hot reload
air -c .air.sensor.toml
air -c .air.dispatch.toml
```

## NATS Concepts

### Subjects

Subjects are dot-delimited strings that form a routing hierarchy. Publishers send to a subject; subscribers match against it.

- **Literal** — `signal.discovery.ping` matches exactly one subject
- **Single-token wildcard (`*`)** — `signal.telemetry.temp.*` matches `signal.telemetry.temp.zone-a` but not `signal.telemetry.humidity.zone-a`
- **Multi-token wildcard (`>`)** — `signal.telemetry.>` matches all subjects starting with `signal.telemetry.`, at any depth

### Pub/Sub (Phase 1–2)

Fire-and-forget messaging. A publisher sends a message to a subject; every active subscriber on that subject receives a copy. No acknowledgment, no persistence. If no subscriber is listening, the message is lost.

### Request/Reply (Phase 1)

A publisher sends a request and collects responses within a timeout. NATS creates a unique inbox subject for replies. Used for discovery ping — broadcast a request, gather responses from all listening services.

### Queue Groups (Phase 3)

Queue groups turn fan-out into work distribution. Multiple subscribers join a named group; NATS delivers each message to exactly one member of the group.

- **Subscribe** (fan-out) — Every subscriber gets every message. Good for observation where each instance needs the full picture.
- **QueueSubscribe** (work distribution) — Each message goes to one group member. Good for processing where each message should be handled once. Think of it like a load balancer distributing jobs across runner nodes.

Adding members increases throughput. Removing a member redistributes its share automatically — no configuration change needed.

### Message Headers (Phase 3)

HTTP-like key-value metadata attached to NATS messages without modifying the payload. Headers travel alongside the message but are decoded separately from the body. Useful for routing metadata (priority, source, trace IDs) that subscribers can inspect without deserializing the full payload.

## NATS Subject Namespace

| Subject Pattern | Purpose |
|---|---|
| `signal.discovery.ping` | Service discovery |
| `signal.telemetry.{type}.{zone}` | Sensor readings |
| `signal.alerts.{severity}` | Priority-tagged alerts |
| `signal.control.{target}` | Adjustment commands |
| `signal.threshold.{key}` | Configuration changes |

## API Endpoints

### Sensor (`:3000`)

| Method | Endpoint | Phase | Description |
|---|---|---|---|
| `GET` | `/healthz` | 1 | Health check |
| `GET` | `/readyz` | 1 | Readiness check |
| `POST` | `/api/discovery/ping` | 1 | Broadcast discovery ping, collect service responses |
| `POST` | `/api/telemetry/start` | 2 | Start telemetry publisher |
| `POST` | `/api/telemetry/stop` | 2 | Stop telemetry publisher |
| `GET` | `/api/telemetry/status` | 2 | Publisher state (running, interval, types, zones) |
| `POST` | `/api/alerts/start` | 3 | Start alert publisher with NATS headers |
| `POST` | `/api/alerts/stop` | 3 | Stop alert publisher |
| `GET` | `/api/alerts/status` | 3 | Alert publisher state |

### Dispatch (`:3001`)

| Method | Endpoint | Phase | Description |
|---|---|---|---|
| `GET` | `/healthz` | 1 | Health check |
| `GET` | `/readyz` | 1 | Readiness check |
| `POST` | `/api/discovery/ping` | 1 | Broadcast discovery ping, collect service responses |
| `GET` | `/api/monitoring/stream` | 2 | SSE stream of telemetry signals |
| `GET` | `/api/monitoring/status` | 2 | Subscription state, message counts |
| `GET` | `/api/alerting/stream` | 3 | SSE stream of alerts (queue group distributed) |
| `GET` | `/api/alerting/status` | 3 | Alert subscription state, message counts |

## Demonstration Phases

Each phase explores a set of NATS capabilities, adding new HTTP endpoints to both services. Phases are implemented sequentially, each in its own branch and PR. See individual phase docs for implementation details.

| Phase | Focus | NATS Concepts |
|---|---|---|
| [Phase 1](phase-01.md) | Foundation + Discovery Ping | Core pub/sub, request/reply, structured payloads |
| [Phase 2](phase-02.md) | Telemetry Pub/Sub | Subject hierarchies, wildcard subscriptions |
| [Phase 3](phase-03.md) | Queue Groups + Headers | Load balancing, message metadata |
| [Phase 4](phase-04.md) | Bidirectional Control | Command/control, closed-loop coordination |
| [Phase 5](phase-05.md) | JetStream | Durable streams, consumers, replay |
| [Phase 6](phase-06.md) | Key-Value Store | Distributed state, watches, optimistic concurrency |
| [Phase 7](phase-07.md) | Object Store | Large blob storage, chunked transfer |
| [Phase 8](phase-08.md) | WebSocket Projection | Real-time signal visualization in browser |

## Prerequisites

- Go 1.26+
- Docker + Docker Compose
- [mise](https://mise.jdx.dev/) (task runner)
- [air](https://github.com/air-verse/air) (hot reload, optional)
