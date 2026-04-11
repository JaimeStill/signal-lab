# Phase 1 — Foundation + Discovery Ping

**Branch:** `phase-01-foundation`

## NATS Concepts

- **Core pub/sub** — Publishing messages to subjects, subscribing to receive them
- **Request/reply** — Sending a request and collecting responses with a timeout
- **Structured payloads** — JSON-encoded message bodies with typed envelopes

## Objective

Stand up both services with full LCA lifecycle (config, bus connection, HTTP serving, graceful shutdown) and implement a discovery ping endpoint that demonstrates service-to-service communication over NATS.

Each service exposes `POST /api/discovery/ping` which broadcasts a ping over NATS and collects structured `ServiceInfo` responses from all listening services.

## Endpoint Surface

```
Both services:
  GET  /healthz                → {"status": "ok"}
  GET  /readyz                 → {"status": "ready"|"not ready"}
  POST /api/discovery/ping     → returns []ServiceInfo from responding services
```

## Files to Create

### pkg/ — Reusable library packages

**`pkg/lifecycle/lifecycle.go`**
Adapt from `~/code/herald/pkg/lifecycle/lifecycle.go`. Coordinator with:
- `OnStartup(hook)` — register startup hooks (run concurrently)
- `OnShutdown(hook)` — register shutdown hooks (block on context done)
- `WaitForStartup()` — block until all startup hooks complete
- `Shutdown(timeout)` — cancel context, wait for hooks with timeout
- `Ready() bool` — true after startup completes
- `Context() context.Context` — lifecycle-scoped context

**`pkg/bus/conn.go`**
NATS connection management:
- `Connect(cfg BusConfig, logger *slog.Logger) (*nats.Conn, error)` — connect with reconnect/disconnect handlers
- `Drain(conn *nats.Conn, timeout time.Duration) error` — graceful drain for shutdown
- `RegisterLifecycle(conn *nats.Conn, lc *lifecycle.Coordinator)` — wire drain into shutdown hooks
- Reconnect and disconnect handlers log via slog

**`pkg/bus/health.go`**
- `Ready(conn *nats.Conn) bool` — checks `conn.IsConnected()`

**`pkg/signal/signal.go`**
Signal envelope:
```go
type Signal struct {
    ID        string            `json:"id"`
    Source    string            `json:"source"`
    Subject   string            `json:"subject"`
    Timestamp time.Time         `json:"timestamp"`
    Data      json.RawMessage   `json:"data"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}
```
- `New(source, subject string, data any) (Signal, error)` — constructor with UUID and timestamp

**`pkg/signal/encoding.go`**
- `Encode(s Signal) ([]byte, error)` — JSON marshal
- `Decode(data []byte) (Signal, error)` — JSON unmarshal

**`pkg/discovery/discovery.go`**
Shared discovery types:
```go
type ServiceInfo struct {
    Name        string `json:"name"`
    ID          string `json:"id"`
    Endpoint    string `json:"endpoint"`
    Health      string `json:"health"`
    Description string `json:"description"`
}
```

**`pkg/module/module.go`** and **`pkg/module/router.go`**
Adapt from `~/code/herald/pkg/module/`. Module wraps prefix + handler + middleware. Router dispatches to modules by prefix.

**`pkg/middleware/middleware.go`**
Adapt from `~/code/herald/pkg/middleware/middleware.go`. Include:
- `System` interface — ordered middleware stack
- `CORS` — configurable origins, methods, credentials
- `Logger` — structured request logging with slog

**`pkg/handlers/handlers.go`**
Adapt from `~/code/herald/pkg/handlers/handlers.go`:
- `RespondJSON(w, status, data)` — marshal and write JSON response
- `RespondError(w, logger, status, err)` — log and write error JSON

### internal/ — Application packages

**`internal/config/config.go`**
Root config with three-phase finalize:
```go
type Config struct {
    Bus             BusConfig   `json:"bus"`
    Alpha           AlphaConfig `json:"alpha"`
    Beta            BetaConfig  `json:"beta"`
    ShutdownTimeout string      `json:"shutdown_timeout"`
}
```
- `Load() (*Config, error)` — load from config files + env vars
- `Merge(overlay *Config)` — merge overlay values

**`internal/config/server.go`**
```go
type ServiceConfig struct {
    Host        string `json:"host"`
    Port        int    `json:"port"`
    Name        string `json:"name"`
    Description string `json:"description"`
}
```

**`internal/config/nats.go`**
```go
type BusConfig struct {
    URL           string `json:"url"`
    MaxReconnects int    `json:"max_reconnects"`
    ReconnectWait string `json:"reconnect_wait"`
}
```

**`internal/alpha/api.go`**
Module assembly for alpha service. Creates mux, registers discovery routes, applies middleware.

**`internal/alpha/discovery.go`**
- NATS subscription handler: listens on `signal.discovery.ping`, replies with alpha's `ServiceInfo`
- HTTP handler: `POST /api/discovery/ping` — publishes ping, collects replies with timeout, returns `[]ServiceInfo`

**`internal/beta/api.go`**
Module assembly for beta service (mirrors alpha pattern).

**`internal/beta/discovery.go`**
Same discovery pattern as alpha, responding with beta's `ServiceInfo`.

### cmd/ — Entry points

**`cmd/alpha/main.go`**
Process entry: signal handling (SIGINT/SIGTERM), config load, server init, `server.Start()`, wait for signal, `server.Shutdown()`.

**`cmd/alpha/server.go`**
Server struct: holds config, lifecycle coordinator, NATS connection, HTTP server. `Start()` connects to NATS, starts HTTP, waits for startup. `Shutdown()` drains NATS, stops HTTP.

**`cmd/alpha/http.go`**
HTTP server lifecycle: `ListenAndServe()`, graceful shutdown with timeout.

**`cmd/alpha/modules.go`**
Assembles alpha API module, health/readiness endpoints, middleware stack (CORS + Logger).

**`cmd/beta/`** — mirrors `cmd/alpha/` structure, loads beta config section.

## Herald Files to Reference

| Herald File | Adapt To |
|---|---|
| `pkg/lifecycle/lifecycle.go` | `pkg/lifecycle/lifecycle.go` |
| `pkg/module/module.go` + `router.go` | `pkg/module/` |
| `pkg/middleware/middleware.go` | `pkg/middleware/middleware.go` |
| `pkg/handlers/handlers.go` | `pkg/handlers/handlers.go` |
| `cmd/server/main.go` | `cmd/alpha/main.go`, `cmd/beta/main.go` |
| `cmd/server/server.go` | `cmd/alpha/server.go`, `cmd/beta/server.go` |
| `cmd/server/http.go` | `cmd/alpha/http.go`, `cmd/beta/http.go` |
| `cmd/server/modules.go` | `cmd/alpha/modules.go`, `cmd/beta/modules.go` |
| `internal/config/config.go` | `internal/config/config.go` |
| `internal/config/server.go` | `internal/config/server.go` |

## Verification

1. `docker compose up -d` — NATS healthy
2. `mise run alpha` (terminal 1) — boots on :3000, connects to NATS
3. `mise run beta` (terminal 2) — boots on :3001, connects to NATS
4. `curl http://localhost:3000/healthz` → `{"status": "ok"}`
5. `curl http://localhost:3001/healthz` → `{"status": "ok"}`
6. `curl -X POST http://localhost:3000/api/discovery/ping` → JSON array containing beta's ServiceInfo
7. `curl -X POST http://localhost:3001/api/discovery/ping` → JSON array containing alpha's ServiceInfo
8. `mise run test` — all tests pass
