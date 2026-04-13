# signal-lab

A progressive NATS learning repository built around two Go web services — **alpha** and **beta** — that act as generic participants in signal-based coordination through a shared message bus.

## Vision

This repository builds practical familiarity with [NATS](https://nats.io) as the single orchestration primitive for event-driven service communication. Each phase is a self-contained demonstration of one NATS capability, implemented as a new pair of domain packages layered into a common two-service runtime. Phases do not depend on each other and do not extend a single overarching narrative — the runtime is the shared substrate, and each phase exercises it in a different way.

## Architecture

### Two Services, One Bus

**alpha** — The dependent service. Acts as a consumer, requester, or orchestrator in each phase's demonstration. Listens on `:3000`.

**beta** — The functional service. Acts as a provider, responder, or executor in each phase's demonstration. Listens on `:3001`.

Both services are generic participants. The role each plays (publisher, subscriber, requester, responder, job source, runner cluster host, etc.) is defined by the phase's domain packages, not by the service itself. Every phase adds a new pair of domain packages — typically one under each of `internal/alpha/` and `internal/beta/` — that exercise a specific NATS pattern.

Communication between services flows exclusively through NATS. Neither service calls the other directly.

### Layered Composition Architecture

Both services follow the LCA pattern adapted from the [herald](https://github.com/JaimeStill/herald) project:

- **Cold start** — Configuration loading, subsystem creation
- **Hot start** — Bus connection, HTTP server listen, readiness
- **Graceful shutdown** — Reverse-order teardown with timeout

### Package Hierarchy

```
cmd/                → Entry points (package main), one per service
  alpha/            → Dependent service (:3000)
  beta/             → Functional service (:3001)
internal/           → Private application packages
  config/           → Shared configuration (three-phase finalize)
  infrastructure/   → Infrastructure struct (Lifecycle, Bus, Logger, ServiceInfo)
  alpha/            → Alpha module wiring + per-phase domain sub-packages
    monitoring/     → Phase 2 telemetry subscriber
    jobs/           → Phase 3 job dispatcher
  beta/             → Beta module wiring + per-phase domain sub-packages
    telemetry/      → Phase 2 telemetry publisher
    runners/        → Phase 3 runner cluster (Runner primitive + cluster System)
pkg/                → Reusable library packages
  lifecycle/        → Startup/shutdown coordination
  bus/              → Message bus System (connection + subscription management)
  signal/           → Signal envelope type
  contracts/        → Shared cross-service contracts
    telemetry/      → Telemetry subject constants + Reading type
    jobs/           → Jobs subject constants + Job type + header keys
  discovery/        → Discovery domain (System + Handler + ServiceInfo)
  routes/           → Route group composition
  module/           → HTTP module/router system
  middleware/       → HTTP middleware (Logger)
  handlers/         → JSON response helpers
tests/              → Black-box tests mirroring source structure
```

Dependencies flow downward: `cmd/` → `internal/` → `pkg/`. Lower-level packages define contracts (interfaces); higher-level packages implement them.

General-purpose features live in `pkg/` (bus, discovery, routes, lifecycle). Cross-service contracts live under `pkg/contracts/{domain}/` — these define the data and protocol layer between services (payload types, subject constants, header keys). Service-specific domains live under `internal/{service}/{domain}/` as sub-packages. Each domain exposes a `System` interface with an unexported implementing struct, following herald's repository/handler pattern.

### Phase Preservation

Each phase's domains and endpoints remain intact once shipped. New phases add new domain packages; they do not modify or depend on the domain packages of prior phases. Only `pkg/` infrastructure and the service wiring layers (`domain.go`, `routes.go`, `api.go`) are modified to integrate new domains.

The one exception: a phase may be refactored to more faithfully demonstrate its own NATS concept (as Phase 3 was reworked from alerts/alerting into the runner cluster). Such refactors are distinct from cross-phase mutations driven by a later phase's requirements — the latter are not allowed.

## Configuration

Layered configuration with the `SIGNAL_` env var prefix:

1. `config.json` — Base configuration
2. `config.<SIGNAL_ENV>.json` — Environment overlay (e.g., `config.docker.json`)
3. `secrets.json` — Gitignored secrets
4. `SIGNAL_*` env vars — Final overrides

Each config struct follows the three-phase finalize pattern: `loadDefaults()` → `loadEnv()` → `validate()`.

`ServiceConfig` holds only shared web service fields (Host, Port, Name, Description). Service-specific configs embed `ServiceConfig` and add domain sub-configs. `AlphaConfig` embeds `ServiceConfig` and adds `JobsConfig`. `BetaConfig` embeds `ServiceConfig` and adds `Zones`, `TelemetryConfig`, and `RunnersConfig`.

## Docker Infrastructure

NATS runs in Docker via Compose:

```bash
docker compose up -d    # Start NATS with JetStream on :4222, monitoring on :8222
docker compose down     # Stop and remove
```

## Development

```bash
# Terminal 1: alpha service
mise run alpha          # or: go run ./cmd/alpha

# Terminal 2: beta service
mise run beta           # or: go run ./cmd/beta

# Testing
mise run test           # go test ./tests/...
mise run vet            # go vet ./...

# Hot reload
air -c .air.alpha.toml
air -c .air.beta.toml
```

## NATS Concepts

### Subjects

Subjects are dot-delimited strings that form a routing hierarchy. Publishers send to a subject; subscribers match against it.

- **Literal** — `signal.discovery.ping` matches exactly one subject
- **Single-token wildcard (`*`)** — `signal.telemetry.temp.*` matches `signal.telemetry.temp.zone-a` but not `signal.telemetry.humidity.zone-a`
- **Multi-token wildcard (`>`)** — `signal.telemetry.>` matches all subjects starting with `signal.telemetry.`, at any depth

### Pub/Sub (Phase 2)

Fire-and-forget messaging. A publisher sends a message to a subject; every active subscriber on that subject receives a copy. No acknowledgment, no persistence. If no subscriber is listening, the message is lost.

### Request/Reply (Phase 1, Phase 4)

A publisher sends a request and collects responses within a timeout. NATS creates a unique inbox subject for replies. Phase 1 uses request/reply for broadcast discovery (gather responses from all listeners); Phase 4 uses it for point-to-point command dispatch with acknowledgment.

### Queue Groups (Phase 3)

Queue groups turn fan-out into work distribution. Multiple subscribers join a named group; NATS delivers each message to exactly one member of the group.

- **Subscribe** (fan-out) — Every subscriber gets every message. Good for observation where each instance needs the full picture.
- **QueueSubscribe** (work distribution) — Each message goes to one group member. Good for processing where each message should be handled once. Think of it like a load balancer distributing jobs across runner nodes.

Adding members increases throughput. Removing a member redistributes its share automatically — no configuration change needed.

### Message Headers (Phase 3)

HTTP-like key-value metadata attached to NATS messages without modifying the payload. Headers travel alongside the message but are decoded separately from the body. Useful for routing metadata (priority, source, trace IDs) that subscribers can inspect without deserializing the full payload.

### JetStream (Phase 5, Phase 9)

Durable message storage layered on top of NATS core. Streams capture messages from one or more subjects; consumers read from streams with explicit acknowledgment, replay from arbitrary positions, and redelivery on unack. Survives service restarts and disconnects.

### Key-Value Store (Phase 6)

Named key-value buckets backed by JetStream. Supports Put/Get/Delete, real-time watches on key changes, and optimistic concurrency via revision numbers for conflict-safe updates.

### Object Store (Phase 7)

Large-blob storage backed by JetStream with automatic chunking. Objects carry metadata (size, checksum, modified time) and are accessed by name. Suitable for artifacts that don't fit in a message payload.

### WebSocket Bridging (Phase 8)

Application-level WebSocket hub managed by the Go server. Browser clients connect to the Go service, which subscribes to NATS on their behalf and fans messages out over the WebSocket connection. Classic "backend-for-frontend" pattern.

### NATS-Native Browser Clients (Phase 9)

NATS's built-in WebSocket gateway allows browser clients (via the `nats.ws` JavaScript library) to connect directly to NATS as first-class peers. Eliminates the Go WebSocket hub entirely — the browser, alpha, and beta all speak NATS natively on the same subject namespace.

## NATS Subject Namespace

| Subject Pattern | Purpose | Phase |
|---|---|---|
| `signal.discovery.ping` | Service discovery | 1 |
| `signal.telemetry.{type}.{zone}` | Telemetry readings | 2 |
| `signal.jobs.{type}` | Job distribution with headers | 3 |
| `signal.commands.{action}` | Command dispatch via request/reply | 4 |
| `signal.journal.{kind}` | Append-only event log (JetStream) | 5 |
| `signal.settings.{key}` | Settings KV watches | 6 |
| `signal.artifacts.{name}` | Object store notifications | 7 |
| (inspector subscribes to any subject) | WebSocket-bridged inspector | 8 |
| `signal.chat.rooms.{room}.*` | NATS-native chat room | 9 |

## API Endpoints

### Alpha (`:3000`)

| Method | Endpoint | Phase | Description |
|---|---|---|---|
| `GET` | `/healthz` | 1 | Liveness check |
| `GET` | `/readyz` | 1 | Readiness check |
| `POST` | `/api/discovery/ping` | 1 | Broadcast discovery ping, collect service responses |
| `GET` | `/api/monitoring/stream` | 2 | SSE stream of telemetry signals |
| `GET` | `/api/monitoring/status` | 2 | Monitoring subscription state and per-subject message counts |
| `POST` | `/api/jobs/start` | 3 | Start the jobs publisher |
| `POST` | `/api/jobs/stop` | 3 | Stop the jobs publisher |
| `GET` | `/api/jobs/status` | 3 | Publisher state (running, interval) |

### Beta (`:3001`)

| Method | Endpoint | Phase | Description |
|---|---|---|---|
| `GET` | `/healthz` | 1 | Liveness check |
| `GET` | `/readyz` | 1 | Readiness check |
| `POST` | `/api/discovery/ping` | 1 | Broadcast discovery ping, collect service responses |
| `POST` | `/api/telemetry/start` | 2 | Start the telemetry publisher |
| `POST` | `/api/telemetry/stop` | 2 | Stop the telemetry publisher |
| `GET` | `/api/telemetry/status` | 2 | Publisher state (running, interval, types, zones) |
| `POST` | `/api/runners/subscribe` | 3 | Attach every runner in the cluster |
| `POST` | `/api/runners/unsubscribe` | 3 | Drain every runner in the cluster |
| `POST` | `/api/runners/{id}/subscribe` | 3 | Attach a single runner by ID |
| `POST` | `/api/runners/{id}/unsubscribe` | 3 | Drain a single runner by ID |
| `GET` | `/api/runners/status` | 3 | Cluster state with per-runner subscription state and counts |

## Demonstration Phases

Each phase explores a set of NATS capabilities, adding new HTTP endpoints (or WebSocket/NATS-native clients) to the services. Phases are implemented sequentially, each in its own branch and PR, but their domains are self-contained and do not depend on prior phases.

| Phase | Focus | NATS Concepts |
|---|---|---|
| [Phase 1](phase-01.md) | Foundation + Discovery Ping | Core pub/sub, request/reply, structured payloads |
| [Phase 2](phase-02.md) | Telemetry Pub/Sub | Subject hierarchies, wildcard subscriptions |
| [Phase 3](phase-03.md) | Queue Groups + Headers (Runner Cluster) | Queue-group work distribution, NATS headers, dynamic membership |
| [Phase 4](phase-04.md) | Request/Reply Command Dispatch | Request/reply, reply inboxes, timeouts, correlation |
| [Phase 5](phase-05.md) | JetStream Event Log | Durable streams, pull consumers, explicit ack, replay |
| [Phase 6](phase-06.md) | Key-Value Distributed Settings | KV buckets, watches, optimistic concurrency (CAS) |
| [Phase 7](phase-07.md) | Shared Artifact Store | Object store, chunked transfer, cross-service blob sharing |
| [Phase 8](phase-08.md) | Subject Inspector (WebSocket Bridge) | Custom Go WebSocket hub, protocol design, fan-out |
| [Phase 9](phase-09.md) | NATS-Native Chat Room | NATS WebSocket gateway, `nats.ws` browser client, JetStream history |

## Prerequisites

- Go 1.26+
- Docker + Docker Compose
- [mise](https://mise.jdx.dev/) (task runner)
- [air](https://github.com/air-verse/air) (hot reload, optional)
