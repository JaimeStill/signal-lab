# Signal Lab

A progressive [NATS](https://nats.io) learning repository built around two Go web services — `alpha` and `beta` — that act as generic participants in signal-based coordination through a shared NATS bus. Each phase adds a new self-contained domain that illustrates one NATS capability.

## Table of contents

- [Overview](#overview)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Development](#development)
  - [Project Structure](#project-structure)
- [Demonstrations](#demonstrations)
  - [Phase 1 — Foundation + Discovery Ping](#phase-1--foundation--discovery-ping)
  - [Phase 2 — Telemetry Pub/Sub](#phase-2--telemetry-pubsub)
  - [Phase 3 — Queue Groups + Headers (Runner Cluster)](#phase-3--queue-groups--headers-runner-cluster)
  - [Phase 4 — Request/Reply Command Dispatch](#phase-4--requestreply-command-dispatch)

## Overview

signal-lab explores NATS capabilities through concrete demonstrations exposed as HTTP endpoints on real web services. The two runtimes — `alpha` (the dependent service) and `beta` (the functional service) — are generic participants in a multi-service communication architecture. Each phase adds a new pair of domain packages that illustrate one NATS concept in isolation, from basic pub/sub through queue-group work distribution, durable streams, distributed state, object storage, and real-time browser projection.

Phases do not depend on each other. The runtime is the shared substrate; each phase exercises it in a different way. Communication between alpha and beta flows exclusively through NATS — neither service calls the other directly.

See [`_project/README.md`](_project/README.md) for detailed architecture documentation and the full phase roadmap.

## Getting Started

### Prerequisites

- [Go](https://go.dev/) 1.26+
- [Docker](https://www.docker.com/) + Docker Compose
- [mise](https://mise.jdx.dev/) (task runner)
- [air](https://github.com/air-verse/air) (hot reload, optional)

### Development

Start the NATS infrastructure:

```bash
docker compose up -d
```

This starts NATS with JetStream on port `4222` and the monitoring dashboard on port `8222`.

Run the services in separate terminals:

```bash
# Terminal 1
mise run alpha

# Terminal 2
mise run beta
```

Alpha listens on `:3000`, beta on `:3001`.

Additional tasks:

```bash
mise run test    # run all tests (go test ./tests/...)
mise run vet     # run go vet
```

Hot reload:

```bash
air -c .air.alpha.toml
air -c .air.beta.toml
```

### Project Structure

```
cmd/                → Service entry points (package main)
  alpha/            → Dependent service (:3000)
  beta/             → Functional service (:3001)
internal/           → Private application packages
  config/           → Shared configuration with three-phase finalize
  infrastructure/   → Infrastructure struct (Lifecycle, Bus, Logger, ServiceInfo)
  alpha/            → Alpha module wiring + per-phase domain sub-packages
    monitoring/     → Telemetry subscriber (Phase 2)
    jobs/           → Job dispatcher (Phase 3)
    commander/      → Command issuer with request/reply (Phase 4)
  beta/             → Beta module wiring + per-phase domain sub-packages
    telemetry/      → Telemetry publisher (Phase 2)
    runners/        → Runner cluster with queue groups (Phase 3)
    responder/      → Command responder with per-action dispatch (Phase 4)
pkg/                → Reusable library packages
  lifecycle/        → Startup/shutdown coordination
  bus/              → Message bus System (connection + subscriptions)
  signal/           → Signal envelope type
  discovery/        → Discovery domain (System + Handler + ServiceInfo)
  routes/           → Route group composition
  module/           → HTTP module/router system
  middleware/       → HTTP middleware (Logger)
  handlers/         → JSON response helpers
  contracts/        → Shared cross-service contracts
    telemetry/      → Telemetry subjects + Reading type (Phase 2)
    jobs/           → Jobs subjects + Job type + header keys (Phase 3)
    commands/       → Command subjects + Action/Status enums + Command/Response types (Phase 4)
tests/              → Black-box tests mirroring source structure
_project/           → Architecture docs and phase implementation briefs
```

## Demonstrations

Each demonstration pairs a NATS concept with a concrete domain implementation. Every subsection below covers one phase and documents the concept being illustrated, the domain established to facilitate it, the API endpoints within that domain, and copy-paste execution instructions.

New phase subsections are added here as each phase ships.

### Phase 1 — Foundation + Discovery Ping

**NATS concept:** request/reply with inbox subjects. A requester broadcasts to a subject and collects all responses that arrive within a timeout window.

**Domain:** `pkg/discovery/` provides a shared discovery `System` used by both services. Each service publishes its `ServiceInfo` when it receives a ping and can initiate discovery as a requester via an HTTP endpoint.

**API endpoints (both services):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness check |
| `GET` | `/readyz` | Readiness check |
| `POST` | `/api/discovery/ping` | Broadcast ping on `signal.discovery.ping`, return collected service responses |

**Execution:**

```bash
# terminal 1
mise run alpha

# terminal 2
mise run beta

# discover from either side
curl -s -X POST localhost:3000/api/discovery/ping | jq
curl -s -X POST localhost:3001/api/discovery/ping | jq
```

### Phase 2 — Telemetry Pub/Sub

**NATS concept:** subject hierarchies and wildcard subscriptions. Publishers emit on `signal.telemetry.{type}.{zone}`; subscribers match with `signal.telemetry.>` to receive the whole hierarchy at any depth.

**Domain:** `internal/beta/telemetry/` publishes simulated environment readings (temperature, humidity, pressure) on a ticker across configured zones. `internal/alpha/monitoring/` subscribes to the full telemetry wildcard, tracks per-subject counts, and exposes an SSE stream of incoming signals.

**API endpoints:**

| Service | Method | Path | Description |
|---|---|---|---|
| beta | `POST` | `/api/telemetry/start` | Begin publishing simulated readings |
| beta | `POST` | `/api/telemetry/stop` | Stop publishing |
| beta | `GET` | `/api/telemetry/status` | Publisher state (running, interval, types, zones) |
| alpha | `GET` | `/api/monitoring/stream` | SSE stream of received telemetry signals |
| alpha | `GET` | `/api/monitoring/status` | Subscription state and per-subject counts |

**Execution:**

```bash
curl -s -X POST localhost:3001/api/telemetry/start | jq
curl -s -N localhost:3000/api/monitoring/stream    # Ctrl+C to stop
curl -s localhost:3000/api/monitoring/status | jq
curl -s -X POST localhost:3001/api/telemetry/stop | jq
```

### Phase 3 — Queue Groups + Headers (Runner Cluster)

**NATS concept:** queue-group subscriptions distribute each message to exactly one member of a named group — turning fan-out into work distribution. NATS headers carry routing metadata (priority, type, trace ID) alongside the payload, inspectable without decoding the body. Members can be attached and detached dynamically; NATS redistributes the remaining members' share without any publisher reconfiguration.

**Domain:** `internal/alpha/jobs/` publishes jobs on `signal.jobs.{type}` with headers for job ID, priority, type, and trace ID. `internal/beta/runners/` spawns N in-process runners (`runner-0`, `runner-1`, ...) all joined to the `beta-runners` queue group. Each runner owns its own subscription handle, counts, and lifecycle — the cluster is a thin coordinator that iterates over runners. Cluster-level operations are eager idempotent; per-runner operations are idempotent-via-error.

**API endpoints:**

| Service | Method | Path | Description |
|---|---|---|---|
| alpha | `POST` | `/api/jobs/start` | Begin publishing jobs |
| alpha | `POST` | `/api/jobs/stop` | Stop publishing |
| alpha | `GET` | `/api/jobs/status` | Publisher state (running, interval) |
| beta | `POST` | `/api/runners/subscribe` | Attach every runner in the cluster |
| beta | `POST` | `/api/runners/unsubscribe` | Drain every runner in the cluster |
| beta | `POST` | `/api/runners/{id}/subscribe` | Attach a single runner by ID |
| beta | `POST` | `/api/runners/{id}/unsubscribe` | Drain a single runner by ID |
| beta | `GET` | `/api/runners/status` | Cluster state with per-runner subscription state and counts |

**Execution:**

```bash
# Start jobs and watch initial distribution across all runners
curl -s -X POST localhost:3000/api/jobs/start | jq
sleep 3
curl -s localhost:3001/api/runners/status | jq

# Detach one runner mid-stream and observe redistribution
curl -s -X POST localhost:3001/api/runners/runner-0/unsubscribe | jq
sleep 3
curl -s localhost:3001/api/runners/status | jq    # runner-0 frozen; others pick up its share

# Reattach the runner and observe rebalancing
curl -s -X POST localhost:3001/api/runners/runner-0/subscribe | jq
sleep 3
curl -s localhost:3001/api/runners/status | jq

# Drain the entire cluster (jobs stop being processed by anyone)
curl -s -X POST localhost:3001/api/runners/unsubscribe | jq
sleep 2
curl -s localhost:3001/api/runners/status | jq    # all subscribed: false, counts frozen

# Reattach the cluster and stop publishing
curl -s -X POST localhost:3001/api/runners/subscribe | jq
curl -s -X POST localhost:3000/api/jobs/stop | jq
```

The `/api/runners/status` response shows per-runner subscription state and per-subject counts (runner count reflects `runtime.NumCPU()` by default):

```json
{
  "subscribed": true,
  "count": 4,
  "runners": {
    "runner-0": {
      "subscribed": true,
      "counts": {"signal.jobs.compute": 3, "signal.jobs.io": 2}
    },
    "runner-1": {
      "subscribed": true,
      "counts": {"signal.jobs.compute": 2, "signal.jobs.analysis": 4}
    },
    "runner-2": {
      "subscribed": true,
      "counts": {"signal.jobs.io": 3, "signal.jobs.analysis": 2}
    },
    "runner-3": {
      "subscribed": true,
      "counts": {"signal.jobs.compute": 3, "signal.jobs.io": 2}
    }
  }
}
```

### Phase 4 — Request/Reply Command Dispatch

**NATS concept:** point-to-point request/reply with reply inboxes and timeouts. A requester calls `nc.Request(subject, body, timeout)`; NATS auto-generates a unique reply inbox, publishes the request, and waits for the first response. The responder replies to the message's `Reply` field via `msg.Respond()`. Unlike Phase 1's broadcast discovery (which collects all responses within a window), Phase 4 is a one-to-one RPC-style exchange: one request, one reply.

**Domain:** `internal/alpha/commander/` issues commands on `signal.commands.{action}` and records the outcome (response or timeout) in a bounded in-memory history. `internal/beta/responder/` subscribes to `signal.commands.>`, dispatches per-action handlers (ping → "pong", flush → "flushed", rotate → "rotated", noop → "noop", unknown → error), maintains an in-memory ledger of handled commands, and replies with a result. The shared contract in `pkg/contracts/commands/` defines the action enum, status enum, and Command/Response payload types.

**API endpoints:**

| Service | Method | Path | Description |
|---|---|---|---|
| alpha | `POST` | `/api/commander/issue` | Issue a command: body `{"action":"...","payload":"..."}`, returns the reply or 504 on timeout |
| alpha | `GET` | `/api/commander/history` | Recent issued commands with their replies or error states |
| beta | `GET` | `/api/responder/ledger` | Commands executed by the responder, in order |

**Execution:**

```bash
# Issue a ping command — beta responds with "pong"
curl -s -X POST localhost:3000/api/commander/issue -d '{"action":"ping"}' | jq

# Try all known actions
curl -s -X POST localhost:3000/api/commander/issue -d '{"action":"flush"}' | jq
curl -s -X POST localhost:3000/api/commander/issue -d '{"action":"rotate"}' | jq
curl -s -X POST localhost:3000/api/commander/issue -d '{"action":"noop"}' | jq

# Issue an unknown action — responder replies with error status
curl -s -X POST localhost:3000/api/commander/issue -d '{"action":"explode"}' | jq

# Check the responder's ledger on beta
curl -s localhost:3001/api/responder/ledger | jq

# Check the commander's history on alpha (newest first)
curl -s localhost:3000/api/commander/history | jq

# Stop beta, then issue a command — demonstrates timeout (504)
# (stop beta in its terminal, then:)
curl -s -X POST localhost:3000/api/commander/issue -d '{"action":"ping"}' | jq
# Restart beta — commands resume successfully
```
