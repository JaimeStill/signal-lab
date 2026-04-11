# Phase 4 — Request/Reply Command Dispatch

**Branch:** `phase-04-request-reply`

## NATS Concepts

- **Request/reply with reply inboxes** — A requester calls `nc.Request(subject, body, timeout)`; NATS auto-generates a unique reply inbox, publishes the request, and waits for the first response. The responder replies to the message's `Reply` field without knowing the inbox ahead of time.
- **Timeouts** — Requests specify a timeout; if no response arrives, the call returns an error. The requester decides what timeout budget is reasonable for each operation.
- **Point-to-point correlation** — Unlike Phase 1's broadcast discovery (which collects all responses within a window), Phase 4's request/reply pattern is a one-to-one RPC-style exchange: one request, one reply.

## Objective

Alpha issues commands to beta via NATS request/reply and waits for acknowledgments. Beta hosts a responder that subscribes to the command subject hierarchy, executes a small simulated action per command, maintains a ledger of executed commands, and replies with a result. Alpha keeps a history of issued commands and their replies (or timeout errors). The demonstration shows synchronous-over-async command dispatch fully isolated from every other phase's domain.

## Domain

**Shared contract (`pkg/contracts/commands/`):**
- `SubjectPrefix = "signal.commands"` and `SubjectWildcard = "signal.commands.>"`
- `Action` enum: `ping`, `flush`, `rotate`, `noop` (representative commands with no real side effects)
- `Command{ID, Action, Payload, IssuedAt}` request type
- `Response{CommandID, Status, Result, HandledAt}` reply type, where `Status` is `ok` or `error`
- Subject format: `signal.commands.{action}` — responder subscribes to the wildcard and dispatches internally by action

**Alpha `commander` domain (`internal/alpha/commander/`):**
- `System` interface with `Issue(action, payload) (Response, error)`, `History() []HistoryEntry`, `Handler() *Handler`
- `Issue` wraps `bus.Conn().Request(subject, body, timeout)` and records the outcome (response or timeout error) in an in-memory history ring
- `timeout` comes from config (e.g., `AlphaConfig.Commander.Timeout`); default `"2s"`
- History ring bounded (e.g., last 64 entries) so it doesn't grow unbounded

**Beta `responder` domain (`internal/beta/responder/`):**
- `System` interface with `Subscribe() error`, `Ledger() []contracts.Command`, `Handler() *Handler`
- `Subscribe` uses `bus.Subscribe(contracts.SubjectWildcard, onCommand)` where `onCommand` decodes the request, dispatches to an action handler, appends to the ledger, and calls `msg.Respond(reply)` to send the response
- Per-action handlers are simple: `ping` returns "pong", `flush`/`rotate` append a ledger entry and return "ok", unknown actions return an error response
- Ledger is an append-only in-memory slice with the last N entries exposed

## Configuration

**`CommanderConfig`** (alpha sub-config):
- `Timeout string` — request timeout duration (e.g., `"2s"`)
- Env var `SIGNAL_COMMANDER_TIMEOUT`

Beta's responder is subscribe-at-startup — no dedicated config needed beyond what already exists.

## New Endpoints

```
Alpha:
  POST /api/commander/issue          → issue a command: body {action, payload}, returns the reply or 504 on timeout
  GET  /api/commander/history        → most recent issued commands with their replies or errors

Beta:
  GET  /api/responder/ledger         → commands executed by the responder, in order
```

## Subject Namespace

```
signal.commands.ping
signal.commands.flush
signal.commands.rotate
signal.commands.noop
```

Responder subscribes to `signal.commands.>`.

## Verification

1. Start both services.
2. `POST /api/commander/issue` on alpha with `{"action": "ping"}` → returns a 200 response containing beta's reply (`{"status": "ok", "result": "pong"}`).
3. `GET /api/responder/ledger` on beta → shows the ping command.
4. `GET /api/commander/history` on alpha → shows the issued command and its reply.
5. Issue a command with an unknown action like `{"action": "explode"}` → responder replies with an error status, alpha records it in history, HTTP response reflects the error.
6. Stop beta (shut down cleanly or kill it), then issue another command from alpha → the request times out, alpha's history records the timeout, HTTP returns 504 Gateway Timeout.
7. Restart beta, issue again → successful round-trip resumes.

## Phase Independence

This phase does not depend on Phase 2's telemetry, Phase 3's jobs/runners, or any other phase's domain. The command/control loop exists entirely within its own subject namespace and its own pair of domain packages, and the responder's "simulated state" (ledger) is self-contained with no cross-phase references.
