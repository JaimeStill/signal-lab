# cmd/sensor

Entry point for the sensor service. Handles signal management, configuration loading, server initialization, and graceful shutdown.

## Files (Phase 1)

- `main.go` — Process entry, signal handling, config load, startup/shutdown
- `server.go` — Server struct composing infrastructure, modules, and lifecycle
- `http.go` — HTTP server lifecycle management
- `modules.go` — Module assembly and middleware registration
