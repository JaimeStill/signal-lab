# Phase 9 — NATS-Native Chat Room

**Branch:** `phase-09-nats-native`

## NATS Concepts

- **NATS WebSocket gateway** — NATS server exposes a built-in WebSocket listener (separate from its TCP port). Browser clients can connect directly without any intermediate bridge.
- **`nats.ws` JavaScript client** — First-class NATS client library for the browser. Provides the full NATS API (publish, subscribe, request/reply, JetStream) over the WebSocket gateway with the same semantics as the Go client.
- **Browser as a peer** — Once the browser is a NATS client, it talks to other NATS clients on equal terms. Go services, CLI tools, and browsers share one subject namespace and one messaging primitive.
- **JetStream for history replay** — When a browser reconnects, it consumes from a JetStream stream to catch up on messages it missed while disconnected.

## Objective

Demonstrate that NATS can replace the entire dual-transport stack of Phase 8 (Go WebSocket hub + REST endpoints). In Phase 9, the browser connects directly to NATS via the server's WebSocket gateway — no Go-managed hub, no custom protocol, no REST translation layer. Mutable commands use NATS request/reply, event streaming uses pub/sub, history uses JetStream. The Go server's role shrinks to: serve the embedded SPA, optionally participate as a NATS client for domain-specific behaviors. That's it.

The chat room is the concrete demonstration vehicle. Multiple browser tabs connect, join a room, exchange messages, see each other's presence, and replay history when they reconnect. All communication flows through NATS subjects — the Go code never handles a WebSocket message.

**Phase 9 validates whether a single messaging primitive can fully replace the two-transport approach while preserving the same functional surface established in Phase 8.**

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│ Browser (tab 1) │     │ Browser (tab 2) │
│   nats.ws       │     │   nats.ws       │
└────────┬────────┘     └────────┬────────┘
         │                       │
         │  WebSocket to NATS    │
         │  (ws://localhost:8080)│
         ▼                       ▼
┌──────────────────────────────────────────┐
│             NATS Server                  │
│   - TCP gateway   (nats://  :4222)       │
│   - WS gateway    (ws://    :8080)       │
│   - JetStream enabled                    │
└──────────────┬───────────────────────────┘
               │
    ┌──────────┴──────────┐
    │                     │
    ▼                     ▼
┌─────────┐         ┌──────────┐
│  alpha  │         │   beta   │
│ (Go)    │         │  (Go)    │
│ - SPA   │         │ - historian
│ - greeter         │         │
└─────────┘         └──────────┘
```

Every participant — browser tabs, alpha, beta — is a NATS client. The NATS server is the only piece in the middle.

## Domain

**No shared Go contract package needed for protocol** — the contract is the subject namespace plus the JSON shapes defined in the SPA source. Since the browser and server are peers on the bus, there's no Go↔browser translation layer. (A thin `pkg/contracts/chat/` for the message schema is still useful for the Go-side participants.)

**Shared contract (`pkg/contracts/chat/`):**
- `SubjectRoom(room) = "signal.chat.rooms." + room`
- Subject patterns:
  - `signal.chat.rooms.{room}.messages` — broadcast chat messages (pub/sub)
  - `signal.chat.rooms.{room}.presence` — join/leave events (pub/sub)
  - `signal.chat.rooms.{room}.commands.join` — room entry (request/reply)
  - `signal.chat.rooms.{room}.commands.list` — current roster (request/reply)
- `Message{ID, Room, From, Body, Timestamp}` schema
- `PresenceEvent{Room, User, Action, Timestamp}` where `Action` is `join` or `leave`

**JetStream stream:**
```
Stream: CHAT_HISTORY
  Subjects: signal.chat.rooms.>
  Storage:  File
  Retention: Limits
  MaxAge:   24h
```

When a browser reconnects, it consumes from an ephemeral consumer on `CHAT_HISTORY` starting from its last seen sequence.

**Alpha `greeter` domain (optional, `internal/alpha/greeter/`):**
- A Go-side NATS client that subscribes to `signal.chat.rooms.>.presence` and publishes a welcome message to the room's `.messages` subject when someone joins
- Pure NATS client — no HTTP, no WebSocket code
- `System` interface with `Subscribe() error`, `Handler() *Handler` (only for `/status`)

**Beta `historian` domain (optional, `internal/beta/historian/`):**
- Runs a durable JetStream consumer that ensures `CHAT_HISTORY` stream is provisioned and enforces retention policies
- Optionally exposes a minimal `/api/historian/status` for operational visibility (stream size, retention state)

**Web client source (`app/chat/`):**
- SPA built with `nats.ws`, a small UI framework, and a bundler
- Connects to `ws://localhost:8080` at startup
- Uses request/reply for join: publishes on `commands.join` and waits for a roster response
- Subscribes to `messages` and `presence` for real-time updates
- On reconnect, consumes from the JetStream `CHAT_HISTORY` stream to replay missed messages
- Embedded into alpha's Go binary via `//go:embed app/chat/dist/*`

## Go Services' Responsibilities

**Minimized by design:**

| Responsibility | Phase 8 | Phase 9 |
|---|---|---|
| Serve embedded SPA | Yes | Yes |
| WebSocket upgrade endpoint | `/ws` | None — NATS owns it |
| WebSocket protocol handling | Full custom protocol | None |
| Client connection lifecycle | Full management | None |
| REST endpoints for chat operations | Yes | None — commands are NATS request/reply |
| Message fan-out to clients | Hub-managed | NATS-native |
| History replay | Hub buffer | JetStream stream |
| Domain logic as NATS client | Optional | Optional (greeter, historian) |

In Phase 9, the Go code that used to implement the hub and protocol is replaced by configuration (enabling the NATS WebSocket gateway) and a small amount of SPA-side code using `nats.ws`.

## Infrastructure Changes

**`docker-compose.yml`** (or NATS server config):
- Enable the WebSocket gateway on port `8080`
- Enable JetStream (already on)

Example NATS config snippet:
```
websocket {
  port: 8080
  no_tls: true
  compression: true
}

jetstream {
  store_dir: "/data/jetstream"
}
```

## New Endpoints (HTTP — minimal)

```
Alpha:
  GET  /                      → embedded chat SPA
  GET  /dist/*                → SPA static assets
  GET  /api/greeter/status    → greeter subscription state (if greeter domain exists)

Beta:
  GET  /api/historian/status  → historian/stream state (if historian domain exists)
```

Everything else is NATS subjects.

## Dependencies

- **Go side:** no new dependencies — everything uses the existing NATS Go client
- **SPA side:** `nats.ws` (the JavaScript NATS client)
- **Infra:** NATS server with WebSocket gateway enabled (a config change, no new service)

## Verification

1. Bring up the stack: `docker compose up -d` (NATS with WebSocket gateway), `mise run alpha`, `mise run beta`.
2. Open two browser tabs at `http://localhost:3000/`. Each SPA connects to `ws://localhost:8080` — check browser devtools to confirm the WebSocket is to NATS, not alpha.
3. In tab 1, join room "lobby" via the UI. Presence event fires; tab 2 (if also in lobby) sees tab 1's arrival. The alpha greeter publishes "welcome to lobby!" as a chat message.
4. Send a message from tab 1 → tab 2 receives it instantly via the shared NATS subscription.
5. From a terminal: `nats pub signal.chat.rooms.lobby.messages '{"from":"cli","body":"hi"}'`. Both tabs receive it — the CLI is just another NATS client.
6. Close tab 1. Post more messages from tab 2 and the CLI. Reopen tab 1 and rejoin → it consumes from the `CHAT_HISTORY` JetStream stream to replay what it missed.
7. `grep -rn "websocket" internal/ pkg/ cmd/` in the repo → returns zero hits. The chat domain's "bridge" is NATS configuration + SPA code, not Go code.

## Phase Independence

Phase 9's domain (chat room, greeter, historian, `CHAT_HISTORY` stream) is self-contained. It does not read or write any other phase's subjects, buckets, or streams. Phase 8's inspector and Phase 9's chat room can coexist in the same runtime — they demonstrate two different approaches to "NATS in the browser" and provide a direct before/after comparison of the two transport models.

## Retrospective Note

Phase 9 is the capstone for signal-lab's NATS exploration. It's where the repository's thesis — "NATS can be the single orchestration primitive for event-driven systems" — is directly tested. A successful Phase 9 means: a real-time, multi-client, persistent chat room running on top of NATS alone, with zero lines of WebSocket handling code in the Go services. If Phase 9 works as designed, it validates that the dual-transport tradition (REST for commands, WebSocket for streams) can be replaced by a single well-chosen messaging substrate.
