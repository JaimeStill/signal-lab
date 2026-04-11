# Phase 6 — Key-Value Distributed Settings

**Branch:** `phase-06-keyvalue`

## NATS Concepts

- **KV buckets** — Named key-value stores built on JetStream. `Put`, `Get`, `Delete`, and `Keys` work like a familiar KV API. Values are opaque byte slices (JSON, binary, whatever).
- **Watches** — A KV bucket can be watched for real-time change notifications. Every `Put`/`Delete` fires an event with the key, new value, and revision number.
- **Revision tracking** — Every write increments the key's revision. Reads return the current revision along with the value.
- **Optimistic concurrency** — `Update(key, value, revision)` succeeds only if the server-side revision matches `revision`. Two concurrent writers using the same expected revision → one wins, the other gets a revision mismatch and retries with the fresh value.
- **History retention** — The bucket can keep N historical revisions per key, enabling audit trails.

## Objective

Both services read and write a shared `settings` KV bucket and watch it for changes. Updates from alpha appear on beta in real time (and vice versa). Concurrent writes to the same key resolve via optimistic concurrency — one succeeds, the other receives a precondition-failed response and can retry with the current revision. The demonstration shows distributed state synchronization with conflict-safe updates, entirely inside the KV bucket — no custom coordination protocol needed.

## Domain

**Shared contract (`pkg/contracts/settings/`):**
- `BucketName = "settings"`
- `Setting{Key, Value, Revision, Updated}` response type
- `ChangeEvent{Key, Value, Revision, Operation}` for the watch stream (where `Operation` is `put` or `delete`)

**Both services host a `settings` domain (`internal/alpha/settings/` and `internal/beta/settings/`):**
- Symmetric surface — both services expose the same endpoints and read/write the same bucket. The only difference is the service name carried in log context.
- `System` interface with `Get(key) (Setting, error)`, `List() ([]Setting, error)`, `Put(key, value, expectedRevision) (Setting, error)`, `Delete(key, expectedRevision) error`, `Watch(ctx) <-chan ChangeEvent`, `Handler() *Handler`
- `Put` uses `kv.Update(key, value, expectedRevision)` when `expectedRevision > 0` for CAS; falls back to `kv.Put(key, value)` when `expectedRevision == 0` (initial create or unconditional replace)
- `Watch` wraps `kv.WatchAll` and translates NATS KV watcher events to domain `ChangeEvent`s; exposed via an SSE handler so clients can see changes in real time
- Creates the `settings` bucket at startup if it doesn't exist

## KV Bucket Configuration

```
Bucket:  settings
  Storage:     File
  History:     5              # keep last 5 revisions per key
  TTL:         0 (persistent)
  Replicas:    1
```

## New Endpoints (same surface on alpha and beta)

```
GET    /api/settings                  → list all keys with current values and revisions
GET    /api/settings/{key}            → get a single key's value + revision
PUT    /api/settings/{key}            → set value; If-Match header carries expected revision for CAS
DELETE /api/settings/{key}            → delete key; If-Match header required for CAS
GET    /api/settings/watch            → SSE stream of change events
```

**CAS semantics:**
- `PUT` with no `If-Match` header → unconditional put (creates or replaces)
- `PUT` with `If-Match: <revision>` → updates only if the current revision matches
  - Returns `412 Precondition Failed` on mismatch with the current revision in the response body
- Successful writes return the new `Setting` including the new revision

## Configuration

**`SettingsConfig`** (shared between alpha and beta, added to both `AlphaConfig` and `BetaConfig`):
- `Bucket string` — bucket name, default `"settings"`
- `History int` — how many historical revisions to retain per key
- Env vars `SIGNAL_SETTINGS_BUCKET`, `SIGNAL_SETTINGS_HISTORY`

## Verification

1. Start both services. `GET /api/settings` on either returns an empty list.
2. `PUT /api/settings/theme` on alpha with `{"value": "dark"}` → returns `{key: "theme", value: "dark", revision: 1}`. `GET /api/settings/theme` on beta returns the same value — KV propagation works.
3. Open `GET /api/settings/watch` on beta (SSE). In a separate terminal, `PUT /api/settings/color` on alpha → beta's SSE stream emits a `put` event immediately.
4. `PUT /api/settings/theme` on alpha with `If-Match: 1` and a new value → succeeds, revision advances to 2. Try another `PUT` with `If-Match: 1` → returns 412 with the current revision (2) in the error body.
5. Restart beta → its settings domain reconnects to the bucket and its watch resumes without missing events (KV watch replays current state on reconnect).
6. `DELETE /api/settings/color` with `If-Match` header → key is removed. `GET /api/settings/color` returns 404. The watch stream emits a `delete` event.

## Phase Independence

Nothing in this phase references Phase 4 thresholds, Phase 2 telemetry, Phase 3 jobs, or anything else. The `settings` bucket is created by this phase's domains and used only by them. The KV pattern is illustrated in isolation: put, get, watch, CAS. Any "setting" keys are arbitrary — the bucket is a demonstration surface, not a shared application config store for other phases.
