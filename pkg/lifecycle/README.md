# pkg/lifecycle

Service startup and shutdown coordination. Manages ordered hook execution for cold start → hot start → graceful shutdown lifecycle.

Adapted from `~/code/herald/pkg/lifecycle/`.

## Files (Phase 1)

- `lifecycle.go` — Coordinator with `OnStartup()`, `OnShutdown()`, `WaitForStartup()`, `Shutdown()`
