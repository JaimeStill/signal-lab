# internal/config

Shared configuration for both services. Follows the three-phase finalization pattern: `loadDefaults()` → `loadEnv()` → `validate()`.

Config loading order: `config.json` (base) → `config.<SIGNAL_ENV>.json` (overlay) → `secrets.json` (gitignored) → `SIGNAL_*` env vars (overrides).

## Files (Phase 1)

- `config.go` — Root config struct, `Load()`, `Merge()`, finalization
- `nats.go` — Bus connection config (URL, reconnect settings)
- `server.go` — HTTP server config (host, port, timeouts)
