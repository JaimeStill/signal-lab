# Phase 7 — Shared Artifact Store

**Branch:** `phase-07-objstore`

## NATS Concepts

- **Object store buckets** — Named object stores built on JetStream. Unlike the KV bucket (values ≤ 1 MiB by default), object stores are designed for arbitrarily large binary blobs.
- **Chunked transfer** — Objects larger than the chunk size are automatically split into chunks on Put and reassembled on Get. The client doesn't see chunks — it just sees an `io.Reader`/`io.Writer` interface.
- **Object metadata** — Each object carries a name, description, size, modified timestamp, and content digest. Metadata can be queried without downloading the body.
- **List / Get / Put / Delete** — Standard object operations. Lists are ordered by name.

## Objective

Alpha uploads artifacts (arbitrary binary blobs with names) to a shared object store bucket. Beta downloads, lists, and inspects them. Both services see the same bucket — alpha's upload is immediately visible to beta's vault without any explicit notification or message. The demonstration exercises large-blob transfer through NATS with chunked payloads that would never fit in a single message envelope.

## Domain

**Shared contract (`pkg/contracts/artifacts/`):**
- `BucketName = "artifacts"`
- `ObjectInfo{Name, Description, Size, Modified, Digest}` for metadata responses
- No subject constants — object store operations aren't pub/sub; they're request/reply RPCs handled by JetStream internally

**Alpha `uploader` domain (`internal/alpha/uploader/`):**
- `System` interface with `Store(name, description string, body io.Reader) (ObjectInfo, error)`, `List() ([]ObjectInfo, error)`, `Handler() *Handler`
- `Store` uses `objStore.Put(meta, body)` so the body reader is streamed in chunks
- Creates the `artifacts` bucket at startup if it doesn't exist
- `List` is included on alpha so the uploader can see what it has uploaded — even though beta owns the primary read surface

**Beta `vault` domain (`internal/beta/vault/`):**
- `System` interface with `List() ([]ObjectInfo, error)`, `Fetch(name string) (io.ReadCloser, ObjectInfo, error)`, `Info(name string) (ObjectInfo, error)`, `Delete(name string) error`, `Handler() *Handler`
- `Fetch` uses `objStore.Get(name)` which returns an `ObjectResult` wrapping the chunked data as a standard reader — the handler streams the bytes to the HTTP response without buffering the whole object in memory
- `Info` uses `objStore.GetInfo(name)` to query metadata without touching the body

## Object Store Configuration

```
Bucket: artifacts
  Storage:  File
  MaxBytes: 100 MiB        # total bucket size cap
  TTL:      0 (persistent)
```

## Configuration

**`ArtifactsConfig`** (shared — added to both `AlphaConfig` and `BetaConfig`):
- `Bucket string` — bucket name, default `"artifacts"`
- `MaxBytes int64` — bucket size cap
- Env vars `SIGNAL_ARTIFACTS_BUCKET`, `SIGNAL_ARTIFACTS_MAX_BYTES`

## New Endpoints

```
Alpha:
  POST   /api/uploader/store/{name}    → upload a binary body as object {name}
                                         headers: Content-Description (optional), Content-Type
  GET    /api/uploader/list             → objects this uploader has stored

Beta:
  GET    /api/vault                     → list every object in the bucket
  GET    /api/vault/{name}              → download an object by name (streams chunks)
  GET    /api/vault/{name}/info         → metadata only: size, digest, modified
  DELETE /api/vault/{name}              → remove an object
```

## Verification

1. Start both services. `GET /api/vault` on beta returns an empty list.
2. Upload a small text file via alpha:
   ```
   curl -X POST --data-binary @notes.txt \
        -H "Content-Description: release notes draft" \
        http://localhost:3000/api/uploader/store/notes.txt
   ```
   Response includes `{name, size, digest, modified}`.
3. `GET /api/vault` on beta → returns the object in the list. `GET /api/vault/notes.txt` → downloads the file contents. `GET /api/vault/notes.txt/info` → returns metadata.
4. Upload a larger binary (e.g., a 20 MiB file). Confirm the upload succeeds (chunked transfer handles it) and beta can download it byte-for-byte identically. Compute a hash on both sides — they match.
5. `DELETE /api/vault/notes.txt` on beta → object is removed. Subsequent `GET /api/vault/notes.txt` returns 404.
6. Stop beta mid-download of a large object → the HTTP request aborts. The object remains in the bucket; restart beta, re-download → works.

## Phase Independence

The `artifacts` bucket is owned entirely by this phase. No content or metadata from other phases is stored here. No cross-phase references to the bucket. The object store pattern is demonstrated through the single narrative of "alpha uploads, beta reads" without coupling to any other domain.
