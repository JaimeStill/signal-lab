# signal-lab

A progressive [NATS](https://nats.io) learning repository built around two Go web services — **sensor** and **dispatch** — that communicate via signal-based coordination through a shared message bus.

## Overview

signal-lab explores NATS capabilities through concrete demonstrations exposed as HTTP endpoints on real web services. Each phase introduces new NATS concepts, building from basic pub/sub to durable streams, distributed state, and real-time WebSocket projection.

**Sensor** publishes simulated environment telemetry (temperature, humidity, pressure) and responds to control adjustments. **Dispatch** monitors telemetry, evaluates thresholds, and sends adjustment signals to drive sensor state toward desired targets. Communication flows exclusively through NATS — neither service calls the other directly.

## Prerequisites

- [Go](https://go.dev/) 1.26+
- [Docker](https://www.docker.com/) + Docker Compose
- [mise](https://mise.jdx.dev/) (task runner)
- [air](https://github.com/air-verse/air) (hot reload, optional)

## Getting Started

Start the NATS infrastructure:

```bash
docker compose up -d
```

This starts NATS with JetStream on port `4222` and the monitoring dashboard on port `8222`.

Run the services in separate terminals:

```bash
# Terminal 1
mise run sensor

# Terminal 2
mise run dispatch
```

Sensor listens on `:3000`, dispatch on `:3001`.

## Demonstration Phases

Each phase explores a set of NATS capabilities, adding new HTTP endpoints to both services. Phases are implemented sequentially, each in its own branch and PR.

| Phase | Focus | NATS Concepts |
|---|---|---|
| [Phase 1](_project/phase-01.md) | Foundation + Discovery Ping | Core pub/sub, request/reply, structured payloads |
| [Phase 2](_project/phase-02.md) | Telemetry Pub/Sub | Subject hierarchies, wildcard subscriptions |
| [Phase 3](_project/phase-03.md) | Queue Groups + Headers | Load balancing, message metadata |
| [Phase 4](_project/phase-04.md) | Bidirectional Control | Command/control, closed-loop coordination |
| [Phase 5](_project/phase-05.md) | JetStream | Durable streams, consumers, replay |
| [Phase 6](_project/phase-06.md) | Key-Value Store | Distributed state, watches, optimistic concurrency |
| [Phase 7](_project/phase-07.md) | Object Store | Large blob storage, chunked transfer |
| [Phase 8](_project/phase-08.md) | WebSocket Projection | Real-time signal visualization in browser |

## Project Structure

```
cmd/           → Service entry points
  sensor/      → Environment monitoring service (:3000)
  dispatch/    → Response coordination service (:3001)
internal/      → Private application packages
  config/      → Shared configuration
  sensor/      → Sensor domain logic
  dispatch/    → Dispatch domain logic
pkg/           → Reusable library packages
  lifecycle/   → Startup/shutdown coordination
  bus/         → Message bus connection management
  signal/      → Signal envelope type
  discovery/   → Shared discovery types
  module/      → HTTP module/router system
  middleware/  → HTTP middleware (Logger)
  handlers/    → JSON response helpers
tests/         → Black-box tests mirroring source structure
_project/      → Architecture docs and phase implementation briefs
```

## API

Both services expose the following endpoints:

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Health check — always returns `{"status": "ok"}` |
| `GET` | `/readyz` | Readiness check — reports `ready` when lifecycle and bus are healthy |
| `POST` | `/api/discovery/ping` | Broadcasts a discovery ping over NATS, returns `[]ServiceInfo` from responding services |

```bash
# health check
curl localhost:3000/healthz

# readiness check
curl localhost:3000/readyz

# discover other services (requires both services running)
curl -s -X POST localhost:3000/api/discovery/ping | jq .
curl -s -X POST localhost:3001/api/discovery/ping | jq .
```

## Development

```bash
mise run sensor       # run sensor service
mise run dispatch     # run dispatch service
mise run test         # run all tests
mise run vet          # run go vet

# hot reload
air -c .air.sensor.toml
air -c .air.dispatch.toml
```

See [`_project/README.md`](_project/README.md) for detailed architecture documentation.
