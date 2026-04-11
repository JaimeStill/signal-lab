# Phase 5 — JetStream Event Log

**Branch:** `phase-05-jetstream`

## NATS Concepts

- **JetStream streams** — Durable message storage layered on core NATS. A stream captures messages from one or more subjects and persists them (file or memory) with configurable retention (limits, interest, work queue, byte caps).
- **Pull consumers** — The consumer explicitly fetches messages in batches, rather than being pushed to. Preferred for controlled throughput and horizontal scaling.
- **Explicit acknowledgment** — The consumer must `ack` each message; unacked messages are redelivered after `AckWait` elapses or when `MaxDeliver` is reached. Survivable by design.
- **Durable consumers** — Named consumers persist on the server. Clients can disconnect, restart, and resume from their last acked sequence without losing position.
- **Replay** — Ephemeral consumers can start from any point (`DeliverByStartSequence`, `DeliverByStartTime`) to re-process history.

## Objective

Alpha writes events to a JetStream-backed append-only journal; beta consumes them through a durable pull consumer with explicit ack. The demonstration exercises durability and replay end-to-end: beta can be paused (simulating a crash), events keep accumulating in the stream, and when beta resumes it picks up from the last acked sequence without gaps or duplicates. A replay endpoint creates an ephemeral consumer that re-reads the stream from an arbitrary sequence.

## Domain

**Shared contract (`pkg/contracts/journal/`):**
- `SubjectPrefix = "signal.journal"` and `SubjectWildcard = "signal.journal.>"`
- `Event{ID, Kind, Payload, Timestamp}` payload type
- `Kind` enum: `info`, `warn`, `action`, `audit` (representative categories)
- Subject format: `signal.journal.{kind}` so consumers could filter by kind if desired

**Alpha `journal` domain (`internal/alpha/journal/`):**
- `System` interface with `Append(kind, payload) (*AppendResult, error)`, `Info() StreamInfo`, `Handler() *Handler`
- `Append` publishes via `js.PublishAsync` (or synchronous `js.Publish` for simplicity); returns the assigned sequence number and stream name
- `Info` wraps `js.StreamInfo` to expose message count, byte size, first/last sequence
- Creates (or updates) the `JOURNAL` stream on `Subscribe`/startup if it doesn't exist

**Beta `archivist` domain (`internal/beta/archivist/`):**
- `System` interface with `Start() error`, `Pause() error`, `Resume() error`, `Replay(fromSeq uint64) error`, `Status() ArchivistStatus`, `Handler() *Handler`
- `Start` attaches a durable pull consumer named `beta-archivist` to the `JOURNAL` stream, begins a pull loop goroutine that fetches batches and acks after processing
- `Pause` stops the pull loop (drains current batch, then idles) — simulates crash
- `Resume` restarts the loop from the consumer's last acked position
- `Replay` creates an ephemeral consumer at the given sequence and processes up to a bounded count, then cleans up
- `Status` reports `{running, lastAckedSeq, pending, redeliveries, replaying}`

## Stream Configuration

```
Stream: JOURNAL
  Subjects: signal.journal.>
  Storage:  File
  Retention: Limits
  MaxMsgs:   10000
  MaxAge:    24h

Consumer: beta-archivist
  Durable:    yes
  AckPolicy:  Explicit
  AckWait:    30s
  MaxDeliver: 3
  Filter:     signal.journal.>
```

## Configuration

**`JournalConfig`** (alpha sub-config):
- `Stream string` — stream name, default `"JOURNAL"`
- `MaxMessages int` and `MaxAge string` for retention tuning
- Env vars `SIGNAL_JOURNAL_STREAM`, `SIGNAL_JOURNAL_MAX_MESSAGES`, `SIGNAL_JOURNAL_MAX_AGE`

**`ArchivistConfig`** (beta sub-config):
- `Durable string` — consumer name, default `"beta-archivist"`
- `BatchSize int` — pull batch size
- `AckWait string` — ack timeout
- Env vars `SIGNAL_ARCHIVIST_DURABLE`, `SIGNAL_ARCHIVIST_BATCH`, `SIGNAL_ARCHIVIST_ACKWAIT`

## New Endpoints

```
Alpha:
  POST /api/journal/append           → append an event: body {kind, payload}, returns assigned sequence
  GET  /api/journal/info             → stream info: messages, bytes, first/last sequence

Beta:
  GET  /api/archivist/status         → consumer state: running, last acked seq, pending, redeliveries
  POST /api/archivist/pause          → stop pulling (simulates crash)
  POST /api/archivist/resume         → restart pulling from last acked seq
  POST /api/archivist/replay         → body {from_seq}, creates ephemeral consumer to re-read history
```

## Verification

1. Start both services with docker's NATS instance (JetStream enabled).
2. `POST /api/journal/append` on alpha a handful of times with different kinds. `GET /api/archivist/status` on beta should show the consumer processing them (lastAckedSeq advancing).
3. `POST /api/archivist/pause` on beta. Append more events from alpha. `GET /api/journal/info` shows the stream growing; `GET /api/archivist/status` shows the consumer idle and `pending` increasing.
4. `POST /api/archivist/resume` — beta picks up exactly where it left off, drains the pending backlog, and catches up to the current sequence. No duplicates, no gaps.
5. Shut down beta entirely (kill the process). Append more events from alpha. Restart beta → its durable consumer state is preserved server-side, and it resumes from the last acked sequence.
6. `POST /api/archivist/replay` with `{"from_seq": 1}` on beta. Beta spins up an ephemeral consumer that re-reads the entire stream from the beginning and bumps a replay-specific counter in the status response.
7. Intentional NAK test: configure `ArchivistConfig` to NAK a specific kind, append that kind → observe redelivery up to `MaxDeliver`, after which the message is treated as dead-lettered.

## Phase Independence

The journal/archivist domains are self-contained. The event shape is defined in this phase's own contracts package. No dependency on telemetry, jobs, commands, or any other phase's subject namespace. The `JOURNAL` stream exists only if this phase's domains are running.
