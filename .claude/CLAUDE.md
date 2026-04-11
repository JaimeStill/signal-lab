# signal-lab

Progressive NATS learning repository exploring signal-based coordination between two generic Go web services — `alpha` (dependent) and `beta` (functional). See `_project/README.md` for full architecture and roadmap.

## Reference Project

The `~/code/herald` project is available for reference. signal-lab's architecture, package structure, and service patterns are adapted from herald's Layered Composition Architecture. Consult it for proven patterns when implementing new packages or infrastructure.

## Architecture

signal-lab follows the Layered Composition Architecture (LCA) from herald: cold start (config load, subsystem creation) → hot start (connections, HTTP listen) → graceful shutdown (reverse-order teardown).

### Package Structure

- `cmd/` — Entry points (`package main`), one per service
- `internal/` — Private application packages (config, service-specific domains)
- `pkg/` — Reusable library packages (lifecycle, bus, signal, discovery, routes, module, middleware, handlers)
- `pkg/contracts/` — Shared cross-service contracts (types, constants, subject prefixes, header keys)

#### `pkg/` vs `internal/` Boundary

Features in `pkg/` are general-purpose infrastructure usable by any service (bus, discovery, routes, lifecycle). Shared cross-service contracts live under `pkg/contracts/{domain}/` — these define the data and protocol layer between services (payload types, subject constants, header keys). Service-specific domains live under `internal/{service}/{domain}/` as sub-packages (e.g., `internal/beta/telemetry/`, `internal/alpha/monitoring/`, `internal/alpha/jobs/`, `internal/beta/runners/`).

#### Phase Preservation

Each phase's domains and endpoints remain intact once shipped. New phases add new domain packages; they do not modify or depend on the domain packages of prior phases. Only `pkg/` infrastructure and the service wiring layers (`domain.go`, `routes.go`, `api.go`) are modified to integrate new domains.

**Refactor-for-illustration exception.** A phase may be refactored in a later session to more faithfully demonstrate its own NATS concept — as Phase 3 was reworked from `alerts`/`alerting` into the runner cluster, because the original pair didn't actually show queue-group distribution. This kind of refactor improves the phase's pedagogical value and is allowed. It is distinct from cross-phase mutation driven by a later phase's requirements (e.g., "modify Phase 2's telemetry so Phase 4's control loop can work"), which is not allowed.

Litmus test: "we're redesigning Phase X because the current version doesn't clearly show concept X" → refactor, allowed; "we need to modify Phase X to make Phase Y work" → cross-phase mutation, not allowed.

### Configuration Pattern

Every config struct follows the three-phase finalize pattern:
1. `loadDefaults()` — hardcoded fallbacks
2. `loadEnv(env)` — environment variable overrides
3. `validate()` — validate final values

Public API: `Finalize(env)` and `Merge(overlay)`. All env vars use the `SIGNAL_` prefix.

Config loading: `config.json` (base) → `config.<SIGNAL_ENV>.json` (overlay) → `secrets.json` (gitignored) → `SIGNAL_*` env vars (overrides).

#### Config Decomposition

`ServiceConfig` holds only shared web service fields (Host, Port, Name, Description). Service-specific configs embed `ServiceConfig` and add domain sub-configs:

- `AlphaConfig` embeds `ServiceConfig` and adds `JobsConfig` (Phase 3 publisher parameters).
- `BetaConfig` embeds `ServiceConfig` and adds `Zones`, `TelemetryConfig`, and `RunnersConfig`.

Shared fields (like `Zones`) lift to the service-specific config level rather than individual domain configs, so multiple domains within the same service can read them consistently.

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

### Domain Primitives (Bottom-Up Design)

When a domain manages a collection of instances — runners in a cluster, workers in a pool, connections in a bank, sessions in a registry — always define the primitive first as its own type with its own state, methods, and concurrency boundary **before** designing the System that manages the collection. The management System should be a thin coordinator over well-defined units, not a fat struct that directly manipulates the primitive's internals.

**Why:** Starting with the management layer leads to encoding the primitive's state as sprawling map-of-maps keyed by string IDs, with a single cluster-wide mutex protecting everything. Invariants that belong to a single instance (its subscription lifecycle, its counters, its handler) end up enforced at the wrong layer. Defining the primitive first gives each instance its own mutex, its own lifecycle methods, and its own encapsulated state. The System becomes a small iterator that composes snapshots and coordinates collection-level invariants (all-or-none, ordering, aggregate state).

**How to apply:**

1. When designing any domain with a plural concept, ask "what is *one* of these?" first. Build the primitive type with its own exported lifecycle methods (`Subscribe`, `Unsubscribe`, `Counts`, etc.) and its own mutex.
2. The management System holds `map[string]*Primitive` (keyed by ID) and iterates, calling the primitive's exported methods — never reaching into its internal state.
3. Concurrency splits cleanly: the primitive protects its own state; the System protects collection-level invariants (`subscribed` flag, iteration order, aggregate status).
4. If the primitive is non-trivial, give it its own file (`runner.go` next to `runners.go`) so the layering is visible from the file tree.
5. Partial-failure rollback in the System uses the primitive's own methods — e.g., if the fifth runner fails to subscribe, the System calls `.Unsubscribe()` on the already-attached runners rather than reaching into a parallel slice.

**Reference implementation:** `internal/beta/runners/` splits the `Runner` primitive (runner.go) from the `cluster` System (runners.go). The Runner owns its `*nats.Subscription`, `counts` map, and mutex; the cluster iterates over runners and coordinates all-or-none subscription semantics.

### Route Groups

Handlers encapsulate their own route definitions via `Routes() routes.Group` (from `pkg/routes/`). Route groups compose with prefix flattening — no central endpoint registry. New domains add routes by returning a group from their handler.

### Dependency Hierarchy

Lower-level packages (`pkg/`) define contracts (interfaces). Higher-level packages (`internal/`) implement them. Dependencies flow downward only.

### NATS Subject Namespace

**Implemented:**
- `signal.discovery.ping` — service discovery (Phase 1)
- `signal.telemetry.{type}.{zone}` — environment readings (Phase 2)
- `signal.jobs.{type}` — job distribution with headers (Phase 3)

**Planned (not yet implemented):**
- `signal.commands.{action}` — command dispatch via request/reply (Phase 4)
- `signal.journal.{kind}` — append-only event log backed by JetStream (Phase 5)
- `signal.settings.{key}` — KV-bucket watches (Phase 6)
- `signal.artifacts.{name}` — object-store notifications (Phase 7)
- (Phase 8 inspector subscribes to any subject on demand; no dedicated namespace)
- `signal.chat.rooms.{room}.*` — NATS-native chat room (Phase 9)

## Session Workflow

Development follows the `/iterative-dev` workflow with GitHub issues and implementation guides.

### Role Separation

**Developer implements:** All source code, configuration files, infrastructure definitions.

**AI owns:** Tests (`tests/`), documentation (CLAUDE.md, README, `_project/`), godoc and source comments, implementation guides, contextual artifacts (commit messages, PRs, issues), and memory updates.

### Implementation Guide Content Boundary

Implementation guides are reference documents the developer follows to write source code. They must contain only what falls within developer responsibility. Specifically, implementation guides MUST NOT include:

- **Godoc comments or source-code comments.** These are added by the AI during closeout. Code blocks in the guide should show the structural skeleton without doc comments cluttering them.
- **Test code.** Tests are an AI responsibility during closeout.
- **Documentation updates.** Changes to CLAUDE.md, README, `_project/`, memory files, or any other AI-owned doc surface are out of scope for the guide.
- **Project-management artifacts.** Commit messages, PR bodies, issue rewrites, label changes — all AI work.

Comments that DO appear inside guide code blocks must be **developer-facing integration commentary** — guidance for the developer reading the guide about how a piece fits into the larger picture (e.g., "this Subscribe is the cluster-level entrypoint, see Step 7 for wiring"). Such commentary is part of the *guide's* prose layer; it must NOT be transcribed into the source files. The developer reads it once for context, then writes the source code without it.

When in doubt: if the line would still belong in the file after a `git blame` six months later, it goes in the source. If it's only useful while the developer is following the guide, it stays in the guide.

### Workflow

1. Phase documents in `_project/phase-XX.md` define scope
2. AI creates a GitHub issue for the phase
3. AI generates an implementation guide at `.claude/context/guides/` as the step-by-step reference
4. Developer executes the guide
5. AI validates, writes tests, updates documentation
6. AI commits, pushes, creates PR, and creates the next phase's issue

### Git Workflow

Each phase is executed in a separate Claude Code session:
1. Create branch from `main`: `[issue-number]-[slug]` (kebab-case, derived from the GitHub issue)
2. Work through the phase using the implementation guide
3. AI commits, pushes, and creates a PR when implementation is verified
4. Merge PR to `main` before starting the next phase

After any planning phase, capture new conventions established during planning in `.claude/CLAUDE.md`.

## Documentation Conventions

### Root `README.md` structure

The repo root `README.md` follows a fixed structure that all sessions maintain:

- **Title + intro paragraph** summarizing signal-lab
- **Table of contents** linking to sections below
- **Overview** — project vision, the alpha/beta generic-participant model, NATS as the coordination medium
- **Getting Started**
  - **Prerequisites** — required tools
  - **Development** — docker compose, mise tasks, hot reload, tests, vet
  - **Project Structure** — annotated package tree; kept in sync with the actual repo layout
- **Demonstrations** — intro paragraph explains the section combines NATS-concept explanations with API documentation and execution instructions. Contains one subsection per *implemented* phase.

Each Demonstrations subsection follows this template:

```
### Phase N — <concept title>

**NATS concept:** <1–3 sentences explaining the NATS capability being demonstrated>

**Domain:** <which domain packages were created; what they do at a high level>

**API endpoints:** <table of method/path/description, per service if cross-service>

**Execution:** <copy-paste bash block that walks through the demonstration end-to-end>
```

Phase subsections are added only when their phase is implemented. Unimplemented phases stay tracked in `_project/phase-0X.md` briefs but do not appear in the root README.

### Closeout checklist additions

Every session closeout must update the root `README.md`:

- **Project Structure** — reflect any additions, moves, or deletions in the annotated package tree
- **Demonstrations** — if the session implemented or reworked a phase, add or update that phase's subsection (concept, domain, endpoints, execution) and update the table-of-contents entry

This sits alongside the existing closeout steps: reconcile `_project/` docs, update `.claude/CLAUDE.md`, run tests, update memory, open PR.

## Development

### Build and Run

```bash
mise run alpha      # go run ./cmd/alpha (terminal 1, port 3000)
mise run beta       # go run ./cmd/beta  (terminal 2, port 3001)
mise run test       # go test ./tests/...
mise run vet        # go vet ./...
```

### Hot Reload

```bash
air -c .air.alpha.toml    # hot reload alpha
air -c .air.beta.toml     # hot reload beta
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

The skill should grow from concrete, tested code rather than speculative patterns. Phases 1–3 are implemented and provide enough validated material (core pub/sub, request/reply, subject hierarchies, wildcard subscriptions, queue groups, NATS headers) to begin scaffolding the skill when a dedicated session is opened for it.

## Go Conventions

- **Naming**: Short, singular, lowercase package names. No type stuttering.
- **Errors**: Lowercase, no punctuation, wrapped with context (`fmt.Errorf("operation failed: %w", err)`). Package-level errors in `errors.go` with `Err` prefix.
- **Modern idioms**: `sync.WaitGroup.Go()`, `for range n`, `min()`/`max()`, `errors.Join`.
- **Parameters**: More than two → use a struct.
- **Interfaces**: Define where consumed, not where implemented. Keep minimal.
- **Bottom-up primitives**: For any domain with a plural concept, define the primitive type with its own state and methods before the System that manages the collection. See **Domain Primitives (Bottom-Up Design)** above.
- **Testing**: Black-box only (`package <name>_test`). Table-driven for parameterized cases. Tests in `tests/` mirroring source structure.
