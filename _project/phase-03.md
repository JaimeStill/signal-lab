# Phase 3 — Queue Groups + Headers (Runner Cluster)

**Branch:** `8-generalize-services-rework-phase-3`

## NATS Concepts

- **Queue groups** — Multiple subscribers joined to a named group; NATS delivers each message to exactly one member. Turns fan-out pub/sub into distributed work processing.
- **Dynamic membership** — Adding or removing members rebalances the work share automatically, without reconfiguring publishers.
- **Message headers** — HTTP-like key-value metadata attached alongside the payload. Subscribers inspect headers for routing information (priority, type, trace ID) without decoding the body.

## Objective

Alpha publishes jobs on a ticker; beta hosts a cluster of N in-process runners all joined to the same queue group. NATS distributes each job to exactly one runner, and per-runner counters expose the distribution. Runners can be attached and detached individually at runtime — the cluster responds with proportional rebalancing that NATS handles for free.

## Domain

**Shared contract (`pkg/contracts/jobs/`):**
- `SubjectPrefix = "signal.jobs"` and `SubjectWildcard = "signal.jobs.>"`
- `Job{ID, Type, Priority, Payload}` payload type
- `JobType` enum: `compute`, `io`, `analysis`
- `JobPriority` enum: `low`, `normal`, `high`
- Header keys: `Job-ID`, `Job-Priority`, `Job-Type`, `Signal-Trace-ID`

**Alpha `jobs` domain (`internal/alpha/jobs/`):**
- `System` interface with `Start / Stop / Status / Handler`
- Ticker-driven publisher producing one job per tick with random type and weighted-random priority
- Each job is packaged in a `signal.Signal` envelope and published via `bus.Conn().PublishMsg` so the NATS headers travel alongside the body
- Status reports `{running, interval}`

**Beta `runners` domain (`internal/beta/runners/`):**
- `Runner` primitive (`runner.go`) — one queue-group subscriber with its own `*nats.Subscription`, per-subject counter map, mutex, and scoped logger. Owns its own lifecycle: `Subscribe / Unsubscribe / Subscribed / Status`. The runner owns its identity format (`runner-N`) derived from its position.
- Cluster `System` (`runners.go`) — thin coordinator over `map[string]*Runner`. Iterates runners for cluster-level eager operations. Bypasses `bus.QueueSubscribe` (which tracks subscriptions by subject) and uses `bus.Conn().QueueSubscribe` directly so N runners can share the same subject in one process.
- Cluster operations are eager idempotent: `Subscribe()` attaches every runner not already subscribed; `Unsubscribe()` drains every runner currently subscribed. Per-runner operations (`SubscribeRunner(id)`, `UnsubscribeRunner(id)`) are idempotent-via-error.
- `Status()` derives the cluster-wide `Subscribed` flag from "all runners currently attached."

## Configuration

**`RunnersConfig`** (beta sub-config):
- `Count string` — `"auto"` resolves to `runtime.NumCPU()` via `Number() int`, otherwise a positive integer
- Env var `SIGNAL_RUNNERS_COUNT`

**`JobsConfig`** (alpha sub-config):
- `Interval string` — duration like `"500ms"`
- Env var `SIGNAL_JOBS_INTERVAL`

## New Endpoints

```
Alpha:
  POST /api/jobs/start              → begin publishing jobs
  POST /api/jobs/stop               → stop publishing
  GET  /api/jobs/status             → publisher state

Beta:
  POST /api/runners/subscribe        → attach every runner in the cluster
  POST /api/runners/unsubscribe      → drain every runner in the cluster
  POST /api/runners/{id}/subscribe   → attach a single runner by ID
  POST /api/runners/{id}/unsubscribe → drain a single runner by ID
  GET  /api/runners/status           → cluster state with per-runner snapshots
```

## Subject Namespace

```
signal.jobs.compute
signal.jobs.io
signal.jobs.analysis
```

All three match the cluster's subscription on `signal.jobs.>`.

## Verification

1. Start both services and the NATS container. Runner count defaults to `runtime.NumCPU()` via the `"auto"` config value.
2. `POST /api/jobs/start` on alpha — publisher begins emitting jobs at the configured interval.
3. Wait a few seconds, then `GET /api/runners/status` on beta. Every runner should have non-zero counts spread across the three subjects, demonstrating queue-group distribution.
4. `POST /api/runners/runner-0/unsubscribe` on beta. The returned snapshot shows `runner-0.subscribed: false`. Wait a few more seconds — runner-0's counts are frozen, while the other runners' counts continue to grow (NATS redistributes the detached runner's share).
5. `POST /api/runners/runner-0/subscribe` to reattach. runner-0's counts resume growing.
6. `POST /api/runners/unsubscribe` — cluster fully drains. All runners show `subscribed: false`; counts freeze across the board.
7. `POST /api/runners/subscribe` to restore the full cluster, then `POST /api/jobs/stop` to end the demonstration.
8. Every HTTP write operation responds with the full cluster snapshot so the caller can confirm the new state in one round trip.

## Notes on the Runner Primitive

The runner cluster is the reference implementation of the **bottom-up domain primitives** convention: the `Runner` type is defined first in its own file with its own state, methods, and concurrency boundary; the cluster `System` is a thin iterator that composes per-runner snapshots and coordinates collection-level invariants. The cluster does not reach into a runner's internal state — it calls the runner's exported methods. See the convention in `.claude/CLAUDE.md § Domain Primitives (Bottom-Up Design)`.
