# Phase 6 — Key-Value Store

**Branch:** `phase-06-keyvalue`

## NATS Concepts

- **KV buckets** — Named key-value stores built on JetStream streams
- **Put/Get/Delete** — Standard CRUD operations on keys
- **Watch** — Real-time notification when keys change
- **Revision tracking** — Every update increments the key's revision number
- **Optimistic concurrency** — `Update(key, value, lastRevision)` fails if the key was modified since `lastRevision`

## Objective

Replace in-memory threshold state (Phase 4) with a NATS KV bucket. Both services can read and write thresholds. Dispatch watches for changes and adjusts control behavior in real time.

## New Endpoints

```
Both services:
  GET    /api/thresholds            → list all threshold entries
  GET    /api/thresholds/{key}      → get specific threshold value + revision
  PUT    /api/thresholds/{key}      → set threshold value (body: JSON value)
```

## KV Bucket Design

```
Bucket: thresholds
  Key pattern: {type}.{zone}  (e.g., "temp.zone-a")
  Value: JSON threshold config {"target": 72, "tolerance": 2, "unit": "°F"}
  TTL: none (persistent)
  History: 5 (keep last 5 revisions per key)
```

## Files to Create/Modify

**`pkg/bus/keyvalue.go`** (new)
- KV bucket creation/access helpers
- Watch wrapper that channels key change events

**`internal/dispatch/control.go`**
- Replace in-memory thresholds with KV bucket reads
- Watch `thresholds` bucket for changes → update control loop targets in real time
- Optimistic concurrency on threshold writes

**`internal/sensor/api.go`** and **`internal/dispatch/api.go`**
- Add threshold CRUD routes

## Verification

1. Start both services with telemetry running
2. `PUT /api/thresholds/temp.zone-a` with `{"target": 72, "tolerance": 2}` on dispatch
3. `GET /api/thresholds/temp.zone-a` on sensor → same value visible
4. Dispatch control loop adjusts sensor toward new target
5. Update threshold via sensor endpoint → dispatch watch fires, control loop adapts
6. Concurrent update test: two rapid PUTs → one succeeds, one gets revision conflict
