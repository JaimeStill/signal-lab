# 7 - Request/Reply Command Dispatch

## Summary

Phase 4 implements NATS request/reply as point-to-point RPC. Alpha's new `commander` domain issues commands via `nc.Request(subject, body, timeout)`; beta's new `responder` domain subscribes to `signal.commands.>`, dispatches per-action handlers, and replies via `msg.Respond()`. The shared `pkg/contracts/commands/` contract defines subject constants, action enum (`ping`, `flush`, `rotate`, `noop`), status enum (`ok`, `error`), and Command/Response payload types. The commander records issued commands and their outcomes (response or timeout) in a bounded history ring; the responder maintains a bounded ledger of handled commands. Both bounds are config-driven.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| RPC primitive | `nc.Request()` directly (not `PublishRequest` + `ChanSubscribe`) | Phase 4 is one-to-one, so NATS's built-in inbox/correlation is sufficient. The multi-reply pattern is only needed for broadcast discovery (Phase 1). |
| History/ledger bounds | Config-driven (`CommanderConfig.MaxHistory`, `ResponderConfig.MaxLedger`), not hardcoded constants | Allows runtime tuning without code changes. Defaults to 64; override via `SIGNAL_COMMANDER_MAX_HISTORY` / `SIGNAL_RESPONDER_MAX_LEDGER`. |
| Subject injection | Commander and responder accept subject/prefix as constructor parameters | Required for test isolation — identical to the runners pattern. Production wiring passes `contracts.SubjectPrefix` and `contracts.SubjectWildcard`; tests pass per-test namespaced subjects. |
| History ordering | Newest-first in `History()` via `slices.Reverse()` | Natural UI/debugging order. Internal storage is append-only oldest-first; reversal happens on read. |
| Unknown action handling | Responder returns `Response{Status: "error", Result: "unknown action: {action}"}` instead of dropping or not responding | Preserves the request/reply contract — every request gets a reply. The commander can record the error without a timeout. |
| JSON tag style | `snake_case` (`issued_at`, `command_id`, `handled_at`, `max_history`, `max_ledger`) | Matches convention across the rest of the codebase. |

## Files Modified

**Created:**
- `pkg/contracts/commands/commands.go` — shared contract
- `internal/config/commander.go` — `CommanderConfig` with timeout + max_history
- `internal/config/responder.go` — `ResponderConfig` with max_ledger
- `internal/alpha/commander/commander.go` — System + unexported struct
- `internal/alpha/commander/handler.go` — HTTP handler (issue, history)
- `internal/beta/responder/responder.go` — System + unexported struct
- `internal/beta/responder/handler.go` — HTTP handler (ledger)
- `tests/commander/commander_test.go` — 9 tests
- `tests/responder/responder_test.go` — 8 tests

**Modified (wiring and docs):**
- `internal/config/alpha.go` — embed `CommanderConfig`
- `internal/config/beta.go` — embed `ResponderConfig`
- `internal/alpha/runtime.go` — add `CommandTimeout`, `CommandMaxHistory`
- `internal/alpha/domain.go` — construct commander with `contracts.SubjectPrefix`
- `internal/alpha/routes.go` — register commander routes
- `internal/alpha/api.go` — thread commander config into Runtime
- `internal/beta/runtime.go` — add `ResponderMaxLedger`
- `internal/beta/domain.go` — construct responder with `contracts.SubjectWildcard`
- `internal/beta/routes.go` — register responder routes
- `internal/beta/api.go` — thread responder config into Runtime, subscribe at startup
- `README.md` — Phase 4 demonstration section, updated project structure
- `_project/README.md` — Phase 4 endpoints, updated package tree
- `.claude/CLAUDE.md` — domain list, config decomposition, subject namespace

## Patterns Established

- **Config-driven collection bounds.** Domains that maintain bounded in-memory collections (history rings, ledgers) accept the bound from config rather than hardcoding a constant. The default lives in `loadDefaults()`; env var override lives in `loadEnv()`; lower bound validated in `validate()`.
- **Subject parameter injection.** Every domain that subscribes to a NATS subject or publishes to a subject hierarchy accepts the subject/prefix as a constructor parameter. Production wiring injects the `contracts` constant; tests inject per-test namespaced subjects via `testSubjectPrefix(t)` / `testSubjectWildcard(t)` helpers. This is now consistent across runners, commander, and responder.
- **Test isolation via `t.Name()`-derived namespaces.** Tests that use a shared NATS server derive subject namespaces from `t.Name()` to avoid cross-test interference. Subtests (`t.Run(name, ...)`) must capture the parent test's prefix before entering the subtest closure, because `t.Name()` inside the subtest includes the subtest suffix.

## Validation Results

- `go vet ./...` — clean
- `go test ./tests/...` — all 11 test packages pass (9 commander tests, 8 responder tests, all pre-existing tests green)
- Manual end-to-end verified:
  - `POST /api/commander/issue {"action":"ping"}` → 200 with `{"status":"ok","result":"pong"}`
  - All four known actions (ping, flush, rotate, noop) handled correctly
  - Unknown action `explode` returns 200 with error status
  - Stopping beta causes issued commands to 504-timeout; commander history records the timeout
  - Restarting beta resumes successful round-trips
  - Phases 1–3 endpoints unaffected
