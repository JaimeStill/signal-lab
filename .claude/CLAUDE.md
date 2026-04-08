# signal-lab

Progressive NATS learning repository exploring signal-based coordination between two Go web services. See `_project/README.md` for full architecture and roadmap.

## Reference Project

The `~/code/herald` project is available for reference. signal-lab's architecture, package structure, and service patterns are adapted from herald's Layered Composition Architecture. Consult it for proven patterns when implementing new packages or infrastructure.

## Architecture

signal-lab follows the Layered Composition Architecture (LCA) from herald: cold start (config load, subsystem creation) → hot start (connections, HTTP listen) → graceful shutdown (reverse-order teardown).

### Package Structure

- `cmd/` — Entry points (`package main`), one per service
- `internal/` — Private application packages (config, service-specific domains)
- `pkg/` — Reusable library packages (lifecycle, bus, signal, discovery, routes, module, middleware, handlers)

#### `pkg/` vs `internal/` Boundary

Features in `pkg/` are general-purpose infrastructure usable by any service (bus, discovery, routes, lifecycle). Service-specific domains live under `internal/{service}/{domain}/` as sub-packages (e.g., `internal/sensor/telemetry/`, `internal/dispatch/monitoring/`).

### Configuration Pattern

Every config struct follows the three-phase finalize pattern:
1. `loadDefaults()` — hardcoded fallbacks
2. `loadEnv(env)` — environment variable overrides
3. `validate()` — validate final values

Public API: `Finalize(env)` and `Merge(overlay)`. All env vars use the `SIGNAL_` prefix.

Config loading: `config.json` (base) → `config.<SIGNAL_ENV>.json` (overlay) → `secrets.json` (gitignored) → `SIGNAL_*` env vars (overrides).

### Systems Pattern

Infrastructure and domain packages expose a `System` interface with an unexported implementing struct, mirroring herald's `database.System` pattern:

- **Infrastructure systems** (`pkg/bus/`): `New()` (cold start, pure init) → `Start(lc)` (hot start, connections + lifecycle hooks)
- **Domain systems** (`pkg/discovery/`, `internal/{service}/{domain}/`): `New()` creates the system, `Subscribe()` initializes bus subscriptions, `Handler()` returns the HTTP handler

Systems are the primary unit of composition. Each system owns its own bus interactions and produces its own handler via factory method.

### Domain Separation

Follows herald's repository/handler pattern. Each domain has three concerns:

1. **System** (domain logic) — `System` interface + unexported struct. Owns bus interactions, subscription callbacks, and domain operations. Depends on `bus.System`.
2. **Handler** (HTTP) — Struct depending on its domain `System` interface. Pure HTTP: parses requests, calls domain methods, formats responses. Never touches bus directly. Exposes `Routes() routes.Group` to encapsulate its own route definitions.
3. **Module wiring** (`api.go`) — Creates domain systems, initializes subscriptions (decoupled from handlers), collects route groups, registers with mux.

Each service's wiring layer follows herald's API pattern:
- `domain.go` — assembles all domain systems for the service
- `routes.go` — collects route groups from domain handlers, calls `routes.Register(mux, ...)`
- `api.go` — orchestrates domain creation, subscription init, route registration, returns `*module.Module`

### Route Groups

Handlers encapsulate their own route definitions via `Routes() routes.Group` (from `pkg/routes/`). Route groups compose with prefix flattening — no central endpoint registry. New domains add routes by returning a group from their handler.

### Dependency Hierarchy

Lower-level packages (`pkg/`) define contracts (interfaces). Higher-level packages (`internal/`) implement them. Dependencies flow downward only.

### NATS Subject Namespace

- `signal.discovery.ping` — service discovery
- `signal.telemetry.{type}.{zone}` — sensor readings
- `signal.control.{target}` — adjustment commands
- `signal.threshold.{key}` — configuration changes

## Session Workflow

Development is **interactive and guided** — AI describes what to do, the developer implements, AI validates.

### AI Responsibilities

- Tests (`tests/`) — all test authorship
- Documentation (`_project/`, CLAUDE.md, directory READMEs) — all doc maintenance
- Contextual artifacts — phase docs, commit messages, PRs

### Developer Implements

- All source code, configuration files, infrastructure definitions
- AI guides step by step, one logical unit at a time

### Git Workflow

Each phase is executed in a separate Claude Code session:
1. Create branch from `main`: `phase-XX-description`
2. Work interactively through the phase
3. AI commits, pushes, and creates a PR when implementation is verified
4. Merge PR to `main` before starting the next phase

Phase documents in `_project/phase-XX.md` initialize each session's scope.

After any planning phase, capture new conventions established during planning in `.claude/CLAUDE.md`. This builds the foundation for decomposing conventions into skills once development phases are complete.

## Development

### Build and Run

```bash
mise run sensor     # go run ./cmd/sensor (terminal 1)
mise run dispatch   # go run ./cmd/dispatch (terminal 2)
mise run test       # go test ./tests/...
mise run vet        # go vet ./...
```

### Hot Reload

```bash
air -c .air.sensor.toml    # hot reload sensor
air -c .air.dispatch.toml  # hot reload dispatch
```

### Local Infrastructure

```bash
docker compose up -d    # NATS (4222) + monitoring (8222)
docker compose down     # Stop and remove containers
```

## Skills

### NATS Skill (Planned — `.claude/skills/nats/`)

A NATS skill will be developed incrementally as phases are completed, capturing validated patterns discovered through implementation. Use the `skill-creator` skill when creating or updating it.

The skill should follow context decomposition — not everything belongs in `SKILL.md`. Information should be layered topically and by purpose within sub-directories, with `SKILL.md` serving as the index. See `~/code/revolutions/.claude/skills/lifesim/` for a reference example of this structure. Potential decomposition:

```
.claude/skills/nats/
  SKILL.md                    # Index — triggers, sub-commands, reference table
  patterns/                   # Validated Go client patterns
    pubsub.md                 # Core pub/sub idioms
    reqreply.md               # Request/reply patterns
    jetstream.md              # Stream/consumer configuration recipes
    keyvalue.md               # KV bucket patterns
  reference/                  # Design conventions
    subjects.md               # Subject namespace design rules
    headers.md                # Header conventions
    connection.md             # Connection lifecycle, reconnect, drain
```

Do not create the skill until Phase 1 or 2 is complete — it needs concrete, tested code to draw from, not speculative patterns.

## Go Conventions

- **Naming**: Short, singular, lowercase package names. No type stuttering.
- **Errors**: Lowercase, no punctuation, wrapped with context (`fmt.Errorf("operation failed: %w", err)`). Package-level errors in `errors.go` with `Err` prefix.
- **Modern idioms**: `sync.WaitGroup.Go()`, `for range n`, `min()`/`max()`, `errors.Join`.
- **Parameters**: More than two → use a struct.
- **Interfaces**: Define where consumed, not where implemented. Keep minimal.
- **Testing**: Black-box only (`package <name>_test`). Table-driven for parameterized cases. Tests in `tests/` mirroring source structure.
