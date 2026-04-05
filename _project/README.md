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
cmd/           → Entry points (package main), one per service
internal/      → Private application packages
  config/      → Shared configuration (three-phase finalize)
  sensor/      → Sensor domain logic
  dispatch/    → Dispatch domain logic
pkg/           → Reusable library packages
  lifecycle/   → Startup/shutdown coordination
  bus/         → Message bus connection management
  signal/      → Signal envelope type
  discovery/   → Shared discovery types
  module/      → HTTP module/router system
  middleware/  → HTTP middleware (CORS, Logger)
  handlers/    → JSON response helpers
tests/         → Black-box tests mirroring source structure
```

Dependencies flow downward: `cmd/` → `internal/` → `pkg/`. Lower-level packages define contracts; higher-level packages implement them.

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

## NATS Subject Namespace

| Subject Pattern | Purpose |
|---|---|
| `signal.discovery.ping` | Service discovery |
| `signal.telemetry.{type}.{zone}` | Sensor readings |
| `signal.control.{target}` | Adjustment commands |
| `signal.threshold.{key}` | Configuration changes |

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
