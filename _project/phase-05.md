# Phase 5 — JetStream Streams & Consumers

**Branch:** `phase-05-jetstream`

## NATS Concepts

- **JetStream streams** — Durable message storage with configurable retention (limits, interest, work queue)
- **Pull consumers** — Client explicitly fetches messages in batches (preferred over push for scaling)
- **Explicit acknowledgment** — Consumer must ack each message; unacked messages are redelivered
- **Replay** — Consumers can start from any point in the stream's history
- **Consumer durability** — Named consumers survive client disconnection and service restarts

## Objective

Create a `TELEMETRY` JetStream stream capturing all telemetry signals. Dispatch uses a durable pull consumer with explicit ack for reliable processing. Add replay and history query endpoints.

## New Endpoints

```
Dispatch:
  GET  /api/monitoring/history     → query stream for historical readings (with filters)
  POST /api/monitoring/replay      → replay signals from a point in time
```

## Stream Configuration

```
Stream: TELEMETRY
  Subjects: signal.telemetry.>
  Storage: File
  Retention: Limits (max age, max messages)
  Replicas: 1

Consumer: dispatch-telemetry
  Durable: yes
  Ack Policy: Explicit
  Max Deliver: 3
  Ack Wait: 30s
  Filter: signal.telemetry.>
```

## Files to Create/Modify

**`pkg/bus/jetstream.go`** (new)
- Stream creation/management helpers
- Consumer creation helpers
- Pull consumer fetch wrapper

**`internal/dispatch/monitoring.go`**
- Switch from core NATS subscription to JetStream pull consumer
- Explicit ack after processing each message
- History query: fetch messages from stream by time range or sequence
- Replay: create ephemeral consumer starting from a specific time/sequence

**`internal/config/nats.go`**
- Add JetStream stream configuration (retention, limits)

## Verification

1. Start both services, publish telemetry for 30 seconds
2. Stop and restart dispatch → consumer resumes from last acked message (no duplicates, no gaps)
3. `GET /api/monitoring/history?since=5m` → returns historical readings
4. `POST /api/monitoring/replay` with timestamp → replays from that point
5. Intentionally NAK a message → observe redelivery
