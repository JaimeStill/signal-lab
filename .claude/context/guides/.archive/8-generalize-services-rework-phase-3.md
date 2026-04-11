# Issue #8 — Generalize services + rework Phase 3 as runner cluster

## Problem Context

Two prerequisite problems block Phase 4:

1. **Sensor/dispatch are fixed domain identities.** Every phase has to accommodate an environment-monitoring metaphor. Phase 4's closed-loop story collides with the phase-preservation rule because the services were given semantic identities each new phase fights against.
2. **Phase 3's alerts/alerting pair doesn't demonstrate queue groups.** One publisher + one queue-subscriber with a queue group of one is indistinguishable from a plain subscribe. The load-balancing story is invisible.

This session fixes both together by renaming the services to generic participants (`alpha`, `beta`) and reworking Phase 3 into a runner-cluster demo where alpha publishes jobs and beta hosts N in-process runners competing on the same queue group.

## Architecture Approach

- **`alpha`** = dependent service (port 3001). Hosts job dispatcher (Phase 3) and telemetry monitor (Phase 2).
- **`beta`** = functional service (port 3000). Hosts telemetry publisher (Phase 2) and runner cluster (Phase 3).
- Phase 3 subject: `signal.jobs.{type}`. Jobs carry NATS headers (`Job-ID`, `Job-Priority`, `Job-Type`, `Signal-Trace-ID`).
- Runners bypass `bus.QueueSubscribe` (which tracks subscriptions by subject in a map, preventing multiple subscribers on the same subject in one process). Instead they call `bus.Conn().QueueSubscribe(...)` directly N times — NATS distributes messages across the N subscribers in the `beta-runners` queue group. The overall NATS connection drain handles shutdown of these subscriptions.
- Phase 1 (discovery) and Phase 2 (telemetry/monitoring) are preserved with only rename-level changes.

---

## Implementation

### Step 1 — Rename sensor → beta, dispatch → alpha

Mechanical rename across the tree. Do this FIRST so later steps work on the new layout.

**Directory / file moves:**

```bash
git mv cmd/sensor cmd/beta
git mv cmd/dispatch cmd/alpha
git mv internal/sensor/telemetry internal/beta/telemetry
git mv internal/sensor/api.go internal/beta/api.go
git mv internal/sensor/domain.go internal/beta/domain.go
git mv internal/sensor/routes.go internal/beta/routes.go
git mv internal/dispatch/monitoring internal/alpha/monitoring
git mv internal/dispatch/api.go internal/alpha/api.go
git mv internal/dispatch/domain.go internal/alpha/domain.go
git mv internal/dispatch/routes.go internal/alpha/routes.go
git mv internal/config/sensor.go internal/config/beta.go
git mv .air.sensor.toml .air.beta.toml
git mv .air.dispatch.toml .air.alpha.toml
# Remove now-empty parents
rmdir internal/sensor internal/dispatch
```

**Package declaration updates:**

- `internal/beta/api.go`, `internal/beta/domain.go`, `internal/beta/routes.go`: change `package sensor` → `package beta`
- `internal/alpha/api.go`, `internal/alpha/domain.go`, `internal/alpha/routes.go`: change `package dispatch` → `package alpha`
- `internal/beta/telemetry/*.go`: keep `package telemetry` (unchanged)
- `internal/alpha/monitoring/*.go`: keep `package monitoring` (unchanged)

**Import path updates** across the entire repo:

- `github.com/JaimeStill/signal-lab/internal/sensor` → `github.com/JaimeStill/signal-lab/internal/beta`
- `github.com/JaimeStill/signal-lab/internal/sensor/telemetry` → `github.com/JaimeStill/signal-lab/internal/beta/telemetry`
- `github.com/JaimeStill/signal-lab/internal/dispatch` → `github.com/JaimeStill/signal-lab/internal/alpha`
- `github.com/JaimeStill/signal-lab/internal/dispatch/monitoring` → `github.com/JaimeStill/signal-lab/internal/alpha/monitoring`

Use `gopls rename` or a repo-wide find-replace. Touches `cmd/beta/*`, `cmd/alpha/*`, `internal/beta/*`, `internal/alpha/*`, `tests/*`.

**Type rename in `internal/config/beta.go`:**

- Type name: `SensorConfig` → `BetaConfig`
- All method receivers: `func (c *SensorConfig)` → `func (c *BetaConfig)`
- Method parameter types: `Merge(overlay *SensorConfig)` → `Merge(overlay *BetaConfig)`
- Env var constant: `EnvSensorZones = "SIGNAL_SENSOR_ZONES"` → `EnvBetaZones = "SIGNAL_BETA_ZONES"`
- Error message in validate: `"sensor zones must not be empty"` → `"beta zones must not be empty"`

**Field and env prefix updates in `internal/config/config.go`:**

Change the struct fields:
```go
// before
Sensor          SensorConfig  `json:"sensor"`
Dispatch        ServiceConfig `json:"dispatch"`

// after
Beta  BetaConfig  `json:"beta"`
Alpha AlphaConfig `json:"alpha"`
```

Update `Merge`:
```go
// before
c.Sensor.Merge(&overlay.Sensor)
c.Dispatch.Merge(&overlay.Dispatch)

// after
c.Beta.Merge(&overlay.Beta)
c.Alpha.Merge(&overlay.Alpha)
```

Update `finalize`:
```go
// before
if err := c.Sensor.Finalize("SENSOR"); err != nil {
    return fmt.Errorf("sensor: %w", err)
}
if err := c.Dispatch.Finalize("DISPATCH"); err != nil {
    return fmt.Errorf("dispatch: %w", err)
}

// after
if err := c.Beta.Finalize("BETA"); err != nil {
    return fmt.Errorf("beta: %w", err)
}
if err := c.Alpha.Finalize("ALPHA"); err != nil {
    return fmt.Errorf("alpha: %w", err)
}
```

**Update call sites** throughout the codebase that reference `cfg.Sensor.*` and `cfg.Dispatch.*`:

- `cmd/beta/server.go`: `cfg.Sensor.Name` → `cfg.Beta.Name`, `cfg.Sensor.Addr()` → `cfg.Beta.Addr()`, `cfg.Sensor.Description` → `cfg.Beta.Description`
- `cmd/alpha/server.go`: same pattern with `cfg.Alpha.*`
- `internal/beta/domain.go`: `cfg.Sensor.Telemetry` → `cfg.Beta.Telemetry`, `cfg.Sensor.Zones` → `cfg.Beta.Zones`, etc.
- `internal/alpha/domain.go`: `cfg.Dispatch.*` → `cfg.Alpha.*`

**Update log/shutdown messages**:

- `cmd/beta/main.go`: `log.Println("sensor stopped gracefully")` → `log.Println("beta stopped gracefully")`
- `cmd/alpha/main.go`: same with "alpha"

**`.air.beta.toml`:** update all sensor references:

```toml
root = "."
tmp_dir = "tmp/beta"

[build]
cmd = "go build -o ./tmp/beta/server ./cmd/beta"
bin = "./tmp/beta/server"
include_ext = ["go", "json"]
exclude_dir = ["tmp", "tests", "cmd/alpha", "internal/alpha"]
```

**`.air.alpha.toml`:**

```toml
root = "."
tmp_dir = "tmp/alpha"

[build]
cmd = "go build -o ./tmp/alpha/server ./cmd/alpha"
bin = "./tmp/alpha/server"
include_ext = ["go", "json"]
exclude_dir = ["tmp", "tests", "cmd/beta", "internal/beta"]
```

**`.mise.toml`:**

```toml
[tasks.beta]
description = "Run beta service"
run = "go run ./cmd/beta"

[tasks.alpha]
description = "Run alpha service"
run = "go run ./cmd/alpha"

[tasks.test]
description = "Run all tests"
run = "go test ./tests/..."

[tasks.vet]
description = "Run go vet"
run = "go vet ./..."
```

**`.gitignore`** (if it has `tmp/sensor` or `tmp/dispatch` entries): update to `tmp/beta`, `tmp/alpha`.

**Final grep check** after all moves are done:

```bash
grep -rn "sensor\|dispatch" --include="*.go" --include="*.json" --include="*.toml" --include="*.md" .
```

Remaining hits should only be in phase docs (`_project/phase-0X.md`) which get handled in Step 11, or inside doc string content that's explicitly about the service rename. Nothing in code should still say sensor/dispatch.

---

### Step 2 — Delete Phase 3 alerts/alerting code

```bash
rm -rf internal/beta/alerts          # old sensor/alerts after rename
rm -rf internal/alpha/alerting        # old dispatch/alerting after rename
rm -rf pkg/contracts/alerts
rm -f internal/config/alerts.go
rm -rf tests/alerts
```

These are fully replaced by the runner-cluster implementation in later steps.

---

### Step 3 — Refactor the config package

Create three new config files and update `beta.go`.

**`internal/config/jobs.go`** (new, complete):

```go
package config

import (
	"fmt"
	"os"
	"time"
)

const EnvJobsInterval = "SIGNAL_JOBS_INTERVAL"

type JobsConfig struct {
	Interval string `json:"interval"`
}

func (c *JobsConfig) IntervalDuration() time.Duration {
	d, _ := time.ParseDuration(c.Interval)
	return d
}

func (c *JobsConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

func (c *JobsConfig) Merge(overlay *JobsConfig) {
	if overlay.Interval != "" {
		c.Interval = overlay.Interval
	}
}

func (c *JobsConfig) loadDefaults() {
	if c.Interval == "" {
		c.Interval = "500ms"
	}
}

func (c *JobsConfig) loadEnv() {
	if v := os.Getenv(EnvJobsInterval); v != "" {
		c.Interval = v
	}
}

func (c *JobsConfig) validate() error {
	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid jobs interval: %w", err)
	}
	return nil
}
```

**`internal/config/runners.go`** (new, complete):

Convention: `Count` is stored as a string — either the literal `"auto"` (which resolves to `runtime.NumCPU()`) or a string-encoded positive integer (`"4"`, `"16"`). This mirrors the existing `TelemetryConfig.Interval` pattern (string-encoded duration parsed through an accessor) and keeps the project's `Merge` convention intact: "non-empty overrides," with `"auto"` treated as an explicit value that can be used in overlays to downgrade a pinned count back to auto-sizing.

```go
package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
)

const EnvRunnersCount = "SIGNAL_RUNNERS_COUNT"

type RunnersConfig struct {
	Count string `json:"count"` // "auto" or a positive integer
}

// Number resolves the configured count to its numeric value, auto-sizing to
// the number of available CPU threads when "auto" is specified. validate()
// guarantees the string is parseable before this is called.
func (c *RunnersConfig) Number() int {
	if c.Count == "auto" {
		return runtime.NumCPU()
	}
	n, _ := strconv.Atoi(c.Count)
	return n
}

func (c *RunnersConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

func (c *RunnersConfig) Merge(overlay *RunnersConfig) {
	if overlay.Count != "" {
		c.Count = overlay.Count
	}
}

func (c *RunnersConfig) loadDefaults() {
	if c.Count == "" {
		c.Count = "auto"
	}
}

func (c *RunnersConfig) loadEnv() {
	if v := os.Getenv(EnvRunnersCount); v != "" {
		c.Count = v
	}
}

func (c *RunnersConfig) validate() error {
	if c.Count == "auto" {
		return nil
	}
	n, err := strconv.Atoi(c.Count)
	if err != nil {
		return fmt.Errorf("invalid runners count %q: must be \"auto\" or a positive integer", c.Count)
	}
	if n < 1 {
		return fmt.Errorf("runners count must be >= 1, got %d", n)
	}
	return nil
}
```

**`internal/config/alpha.go`** (new, complete):

```go
package config

import "fmt"

type AlphaConfig struct {
	ServiceConfig
	Jobs JobsConfig `json:"jobs"`
}

func (c *AlphaConfig) Finalize(envPrefix string) error {
	if err := c.ServiceConfig.Finalize(envPrefix); err != nil {
		return err
	}
	if err := c.Jobs.Finalize(); err != nil {
		return fmt.Errorf("jobs: %w", err)
	}
	return nil
}

func (c *AlphaConfig) Merge(overlay *AlphaConfig) {
	c.ServiceConfig.Merge(&overlay.ServiceConfig)
	c.Jobs.Merge(&overlay.Jobs)
}
```

**`internal/config/beta.go`** (incremental changes after the rename):

- Remove the `Alerts AlertsConfig` field and its finalize/merge handling
- Add the `Runners RunnersConfig` field and its finalize/merge handling

The updated struct and methods:

```go
// field list within BetaConfig
type BetaConfig struct {
	ServiceConfig
	Zones     []string        `json:"zones"`
	Telemetry TelemetryConfig `json:"telemetry"`
	Runners   RunnersConfig   `json:"runners"`
}

// inside BetaConfig.Finalize, replace the alerts block:
if err := c.Runners.Finalize(); err != nil {
	return fmt.Errorf("runners: %w", err)
}

// inside BetaConfig.Merge, replace the alerts merge line:
c.Runners.Merge(&overlay.Runners)
```

Keep everything else in `beta.go` (Zones default, validate, loadDefaults, loadEnv) as-is — the zones default of `["server-room", "ops-center"]` is still used by telemetry.

**`internal/config/config.go`** (promote the Step 1 placeholder to the real type):

Step 1 left `Config.Alpha` as a `ServiceConfig` so the codebase would compile between Step 1 and Step 3. Now that `AlphaConfig` exists, promote the field:

```go
// before (the Step 1 placeholder)
Alpha           ServiceConfig `json:"alpha"`

// after (Step 3 promotion)
Alpha           AlphaConfig   `json:"alpha"`
```

The `c.Alpha.Merge(&overlay.Alpha)` and `c.Alpha.Finalize("ALPHA")` calls don't change — both `ServiceConfig` and `AlphaConfig` expose those methods with matching signatures, so nothing else in `config.go` needs to move.

**Without this edit, Step 7's wiring will fail to compile** when `internal/alpha/domain.go` tries to read `cfg.Alpha.Jobs.IntervalDuration()` — the Jobs field doesn't exist on `ServiceConfig`.

**`config.json`** (update the runtime config to match the new schema):

The current `config.json` carries an obsolete `beta.alerts` section (the field was removed from `BetaConfig`) and is missing `beta.runners` and `alpha.jobs` sections. JSON unmarshal silently ignores the stale `alerts` key and `loadDefaults` will fill in the missing sub-configs at runtime, but the file is out of sync with the code. Replace it with:

```json
{
  "bus": {
    "url": "nats://localhost:4222",
    "max_reconnects": 10,
    "reconnect_wait": "2s",
    "response_timeout": "500ms"
  },
  "alpha": {
    "port": 3000,
    "name": "alpha",
    "description": "Dependent service — consumes telemetry, dispatches jobs, and orchestrates workloads",
    "jobs": {
      "interval": "500ms"
    }
  },
  "beta": {
    "port": 3001,
    "name": "beta",
    "description": "Functional service — provides telemetry, runner clusters, and other capabilities",
    "zones": ["server-room", "ops-center"],
    "telemetry": {
      "interval": "2s",
      "types": ["temp", "humidity", "pressure"]
    },
    "runners": {
      "count": "auto"
    }
  },
  "shutdown_timeout": "30s"
}
```

`config.docker.json` needs no changes — it only overrides the NATS URL.

---

### Step 4 — Create `pkg/contracts/jobs/`

**`pkg/contracts/jobs/jobs.go`** (new, complete):

```go
package jobs

const SubjectPrefix = "signal.jobs"
const SubjectWildcard = SubjectPrefix + ".>"

type JobType string

const (
	TypeCompute  JobType = "compute"
	TypeIO       JobType = "io"
	TypeAnalysis JobType = "analysis"
)

type JobPriority string

const (
	PriorityLow    JobPriority = "low"
	PriorityNormal JobPriority = "normal"
	PriorityHigh   JobPriority = "high"
)

// NATS header keys for job metadata.
const (
	HeaderJobID    = "Job-ID"
	HeaderPriority = "Job-Priority"
	HeaderType     = "Job-Type"
	HeaderTraceID  = "Signal-Trace-ID"
)

type Job struct {
	ID       string      `json:"id"`
	Type     JobType     `json:"type"`
	Priority JobPriority `json:"priority"`
	Payload  string      `json:"payload"`
}
```

---

### Step 5 — Create `internal/alpha/jobs/` (job dispatcher)

Modeled on the old `sensor/alerts` domain — ticker-driven publisher with NATS headers.

**`internal/alpha/jobs/jobs.go`** (new, complete):

```go
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

type Status struct {
	Running  bool   `json:"running"`
	Interval string `json:"interval"`
}

type System interface {
	Start() error
	Stop() error
	Status() Status
	Handler() *Handler
}

type jobs struct {
	bus      bus.System
	source   string
	interval time.Duration
	running  bool
	cancel   context.CancelFunc
	mu       sync.Mutex
	logger   *slog.Logger
}

func New(
	b bus.System,
	source string,
	interval time.Duration,
	logger *slog.Logger,
) System {
	return &jobs{
		bus:      b,
		source:   source,
		interval: interval,
		logger:   logger.With("domain", "jobs"),
	}
}

func (j *jobs) Start() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.running {
		return fmt.Errorf("publisher already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	j.cancel = cancel
	j.running = true

	go j.publish(ctx)

	j.logger.Info("publisher started", "interval", j.interval)
	return nil
}

func (j *jobs) Stop() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if !j.running {
		return fmt.Errorf("publisher not running")
	}

	j.cancel()
	j.running = false

	j.logger.Info("publisher stopped")
	return nil
}

func (j *jobs) Status() Status {
	j.mu.Lock()
	defer j.mu.Unlock()

	return Status{
		Running:  j.running,
		Interval: j.interval.String(),
	}
}

func (j *jobs) Handler() *Handler {
	return &Handler{
		jobs:   j,
		logger: j.logger,
	}
}

func (j *jobs) publish(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobType := randomType()
			priority := randomPriority()

			job := contracts.Job{
				ID:       fmt.Sprintf("job-%d", time.Now().UnixNano()),
				Type:     jobType,
				Priority: priority,
				Payload:  payloadFor(jobType),
			}

			subject := fmt.Sprintf("%s.%s", contracts.SubjectPrefix, jobType)

			sig, err := signal.New(j.source, subject, job)
			if err != nil {
				j.logger.Error("failed to create signal", "error", err)
				continue
			}

			data, err := signal.Encode(sig)
			if err != nil {
				j.logger.Error("failed to encode signal", "error", err)
				continue
			}

			msg := &nats.Msg{
				Subject: subject,
				Data:    data,
				Header:  nats.Header{},
			}
			msg.Header.Set(contracts.HeaderJobID, job.ID)
			msg.Header.Set(contracts.HeaderPriority, string(priority))
			msg.Header.Set(contracts.HeaderType, string(jobType))
			msg.Header.Set(contracts.HeaderTraceID, sig.ID)

			if err := j.bus.Conn().PublishMsg(msg); err != nil {
				j.logger.Error(
					"failed to publish job",
					"subject", subject,
					"error", err,
				)
			}
		}
	}
}

// randomType returns a uniformly distributed job type.
func randomType() contracts.JobType {
	types := []contracts.JobType{
		contracts.TypeCompute,
		contracts.TypeIO,
		contracts.TypeAnalysis,
	}
	return types[rand.IntN(len(types))]
}

// randomPriority returns a weighted random priority (~60% normal, ~25% low, ~15% high).
func randomPriority() contracts.JobPriority {
	n := rand.IntN(100)
	switch {
	case n < 15:
		return contracts.PriorityHigh
	case n < 40:
		return contracts.PriorityLow
	default:
		return contracts.PriorityNormal
	}
}

func payloadFor(t contracts.JobType) string {
	switch t {
	case contracts.TypeCompute:
		return "compute batch work"
	case contracts.TypeIO:
		return "read/write operation"
	case contracts.TypeAnalysis:
		return "analysis task"
	default:
		return "generic workload"
	}
}
```

**`internal/alpha/jobs/handler.go`** (new, complete):

```go
package jobs

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

type Handler struct {
	jobs   System
	logger *slog.Logger
}

func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/jobs",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/start", Handler: h.HandleStart},
			{Method: "POST", Pattern: "/stop", Handler: h.HandleStop},
			{Method: "GET", Pattern: "/status", Handler: h.HandleStatus},
		},
	}
}

func (h *Handler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if err := h.jobs.Start(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.jobs.Status())
}

func (h *Handler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if err := h.jobs.Stop(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.jobs.Status())
}

func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.jobs.Status())
}
```

---

### Step 6 — Create `internal/beta/runners/` (Runner primitive + cluster manager)

This domain has a plural concept (runners), so the design is bottom-up: define the `Runner` primitive first as a self-contained type with its own state, lifecycle methods, and concurrency boundary. The cluster System becomes a thin manager that iterates over runners and coordinates collection-level invariants.

**Important implementation notes:**

1. `bus.QueueSubscribe` tracks subscriptions by subject in an internal map (`pkg/bus/bus.go:76-79`), which rejects multiple subscribers on the same subject within one process. Runners bypass that helper and call `nc.QueueSubscribe(...)` directly on the raw connection.
2. Each `Runner` owns its own `*nats.Subscription` handle, `counts` map, and mutex. The cluster does not reach into those fields — it calls the runner's exported methods.
3. `Subscribe()` is idempotent-via-error on both layers: a second call returns `"already subscribed"` instead of silently doubling capacity. The cluster implements all-or-none rollback: if any runner fails to attach, already-attached runners are unsubscribed before returning.
4. Shutdown is still safe via the bus's `nc.Drain()` if `Unsubscribe()` is never called explicitly — the explicit lifecycle methods are additive, not a replacement.

**`internal/beta/runners/runner.go`** (new, complete — the primitive):

```go
package runners

import (
	"fmt"
	"log/slog"
	"maps"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

// RunnerStatus is a point-in-time snapshot of a single runner.
type RunnerStatus struct {
	Subscribed bool             `json:"subscribed"`
	Counts     map[string]int64 `json:"counts"`
}

// Runner is a single queue-group subscriber with its own identity, subscription
// handle, and per-subject message counters. Runners are the unit of work
// distribution within a cluster — each runner is one member of a NATS queue
// group, and NATS delivers any given job to exactly one runner.
type Runner struct {
	ID string

	counts map[string]int64
	sub    *nats.Subscription
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewRunner creates a runner with an ID derived from its position in the
// cluster. It does not attach to NATS; call Subscribe to join a queue group
// on a live connection.
func NewRunner(index int, logger *slog.Logger) *Runner {
	id := fmt.Sprintf("runner-%d", index)
	return &Runner{
		ID:     id,
		counts: make(map[string]int64),
		logger: logger.With("runner_id", id),
	}
}

// Subscribe attaches the runner to the given subject and queue group on the
// provided NATS connection. Idempotent-via-error: returns an error if the
// runner is already subscribed.
func (r *Runner) Subscribe(nc *nats.Conn, subject, queue string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sub != nil {
		return fmt.Errorf("runner %s already subscribed", r.ID)
	}

	sub, err := nc.QueueSubscribe(subject, queue, r.handle)
	if err != nil {
		return fmt.Errorf("queue subscribe: %w", err)
	}

	r.sub = sub
	return nil
}

// Unsubscribe drains the runner's subscription, letting any in-flight messages
// complete processing before detaching. Returns an error if not subscribed.
func (r *Runner) Unsubscribe() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sub == nil {
		return fmt.Errorf("runner %s not subscribed", r.ID)
	}

	if err := r.sub.Drain(); err != nil {
		return fmt.Errorf("drain: %w", err)
	}

	r.sub = nil
	return nil
}

// Subscribed reports whether the runner is currently attached to a NATS
// subject. Cheap state check that does not allocate.
func (r *Runner) Subscribed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sub != nil
}

// Status returns a point-in-time snapshot of this runner's subscription state
// and per-subject message counts. The returned counts map is a copy and is
// safe to read without locking.
func (r *Runner) Status() RunnerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[string]int64, len(r.counts))
	maps.Copy(counts, r.counts)

	return RunnerStatus{
		Subscribed: r.sub != nil,
		Counts:     counts,
	}
}

// handle is the runner's NATS message handler. Decodes the signal envelope,
// extracts header metadata for priority-based logging, and increments the
// per-subject counter.
func (r *Runner) handle(msg *nats.Msg) {
	sig, err := signal.Decode(msg.Data)
	if err != nil {
		r.logger.Error("failed to decode job", "error", err)
		return
	}

	priority := msg.Header.Get(contracts.HeaderPriority)
	jobID := msg.Header.Get(contracts.HeaderJobID)
	jobType := msg.Header.Get(contracts.HeaderType)
	traceID := msg.Header.Get(contracts.HeaderTraceID)

	switch contracts.JobPriority(priority) {
	case contracts.PriorityHigh:
		r.logger.Warn(
			"high priority job",
			"job_id", jobID,
			"type", jobType,
			"trace_id", traceID,
		)
	default:
		r.logger.Info(
			"job handled",
			"job_id", jobID,
			"priority", priority,
			"type", jobType,
		)
	}

	r.mu.Lock()
	r.counts[msg.Subject]++
	r.mu.Unlock()

	_ = sig // envelope decoded for validation; body unused here
}
```

**`internal/beta/runners/runners.go`** (new, complete — the cluster manager):

```go
package runners

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/JaimeStill/signal-lab/pkg/bus"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

const queueGroup = "beta-runners"

// Status reports the cluster state and a snapshot of every runner. The cluster
// Subscribed flag is true only when every runner is currently attached.
type Status struct {
	Subscribed bool                    `json:"subscribed"`
	Count      int                     `json:"count"`
	Runners    map[string]RunnerStatus `json:"runners"`
}

type System interface {
	Subscribe() error
	Unsubscribe() error
	SubscribeRunner(id string) error
	UnsubscribeRunner(id string) error
	Status() Status
	Handler() *Handler
}

type cluster struct {
	bus     bus.System
	runners map[string]*Runner
	logger  *slog.Logger
}

func New(b bus.System, count int, logger *slog.Logger) System {
	clusterLogger := logger.With("domain", "runners")
	runners := make(map[string]*Runner, count)

	for i := range count {
		r := NewRunner(i, clusterLogger)
		runners[r.ID] = r
	}

	return &cluster{
		bus:     b,
		runners: runners,
		logger:  clusterLogger,
	}
}

// Subscribe attaches every runner in the cluster that is not currently
// subscribed. Per-runner failures are joined and returned. Idempotent: runners
// already attached are skipped, so calling on a fully-subscribed cluster is a
// no-op that returns nil.
func (c *cluster) Subscribe() error {
	var errs []error
	for _, r := range c.runners {
		if r.Subscribed() {
			continue
		}
		if err := c.SubscribeRunner(r.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Unsubscribe drains every runner in the cluster that is currently subscribed.
// Per-runner failures are joined and returned. Idempotent: runners already
// detached are skipped, so calling on a fully-unsubscribed cluster is a no-op
// that returns nil.
func (c *cluster) Unsubscribe() error {
	var errs []error
	for _, r := range c.runners {
		if !r.Subscribed() {
			continue
		}
		if err := c.UnsubscribeRunner(r.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SubscribeRunner attaches a single runner identified by ID. Idempotent-via-
// error: returns an error if the runner is already subscribed.
func (c *cluster) SubscribeRunner(id string) error {
	r, ok := c.runners[id]
	if !ok {
		return fmt.Errorf("runner %s not found", id)
	}
	return r.Subscribe(c.bus.Conn(), contracts.SubjectWildcard, queueGroup)
}

// UnsubscribeRunner drains a single runner identified by ID. Idempotent-via-
// error: returns an error if the runner is not currently subscribed.
func (c *cluster) UnsubscribeRunner(id string) error {
	r, ok := c.runners[id]
	if !ok {
		return fmt.Errorf("runner %s not found", id)
	}
	return r.Unsubscribe()
}

// Status returns a snapshot of the cluster and every runner's status. The
// cluster Subscribed flag reflects whether every runner is currently attached.
func (c *cluster) Status() Status {
	snapshot := make(map[string]RunnerStatus, len(c.runners))
	allSubscribed := true
	for id, r := range c.runners {
		rs := r.Status()
		snapshot[id] = rs
		if !rs.Subscribed {
			allSubscribed = false
		}
	}

	return Status{
		Subscribed: allSubscribed,
		Count:      len(c.runners),
		Runners:    snapshot,
	}
}

func (c *cluster) Handler() *Handler {
	return &Handler{
		cluster: c,
		logger:  c.logger,
	}
}
```

**`internal/beta/runners/handler.go`** (new, complete):

```go
package runners

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

type Handler struct {
	cluster System
	logger  *slog.Logger
}

func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/runners",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/subscribe", Handler: h.HandleSubscribe},
			{Method: "POST", Pattern: "/unsubscribe", Handler: h.HandleUnsubscribe},
			{Method: "POST", Pattern: "/{id}/subscribe", Handler: h.HandleRunnerSubscribe},
			{Method: "POST", Pattern: "/{id}/unsubscribe", Handler: h.HandleRunnerUnsubscribe},
			{Method: "GET", Pattern: "/status", Handler: h.HandleStatus},
		},
	}
}

func (h *Handler) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := h.cluster.Subscribe(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

func (h *Handler) HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if err := h.cluster.Unsubscribe(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

func (h *Handler) HandleRunnerSubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cluster.SubscribeRunner(id); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

func (h *Handler) HandleRunnerUnsubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cluster.UnsubscribeRunner(id); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}
```

---

### Step 7 — Wire the new domains into service modules

**`internal/beta/domain.go`** (replace the Alerts wiring with Runners):

Change the imports:
```go
// remove
"github.com/JaimeStill/signal-lab/internal/beta/alerts"

// add
"github.com/JaimeStill/signal-lab/internal/beta/runners"
```

Update the Domain struct:
```go
type Domain struct {
	Discovery discovery.System
	Telemetry telemetry.System
	Runners   runners.System
}
```

Replace the alerts construction in `NewDomain`:
```go
// remove
alertConfig := &cfg.Sensor.Alerts
...
alt := alerts.New(
    b,
    info.Name,
    alertConfig.IntervalDuration(),
    cfg.Sensor.Zones,
    logger,
)

// add (anywhere inside NewDomain after the discovery/telemetry blocks)
runnerCluster := runners.New(
	b,
	cfg.Beta.Runners.Number(),
	logger,
)
```

Update the return:
```go
return &Domain{
	Discovery: disc,
	Telemetry: tel,
	Runners:   runnerCluster,
}
```

All `cfg.Sensor.*` references should already have become `cfg.Beta.*` as part of Step 1.

**`internal/beta/routes.go`:**

```go
package beta

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	discoveryHandler := domain.Discovery.Handler()
	telemetryHandler := domain.Telemetry.Handler()
	runnersHandler := domain.Runners.Handler()

	routes.Register(
		mux,
		discoveryHandler.Routes(),
		telemetryHandler.Routes(),
		runnersHandler.Routes(),
	)
}
```

**`internal/beta/api.go`:** add the runner subscription call alongside discovery:

```go
// inside NewModule, after domain.Discovery.Subscribe():
if err := domain.Runners.Subscribe(); err != nil {
	return nil, err
}
```

**`internal/alpha/domain.go`** (replace Alert with Jobs):

Change the imports:
```go
// remove
"github.com/JaimeStill/signal-lab/internal/alpha/alerting"

// add
"github.com/JaimeStill/signal-lab/internal/alpha/jobs"
```

Update the Domain struct:
```go
type Domain struct {
	Discovery discovery.System
	Monitor   monitoring.System
	Jobs      jobs.System
}
```

Replace the alerting construction in `NewDomain`:
```go
return &Domain{
	Discovery: discovery.New(
		b, info,
		cfg.Bus.ResponseTimeoutDuration(),
		logger,
	),
	Monitor: monitoring.New(b, logger),
	Jobs: jobs.New(
		b,
		info.Name,
		cfg.Alpha.Jobs.IntervalDuration(),
		logger,
	),
}
```

**`internal/alpha/routes.go`:**

```go
package alpha

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	discoveryHandler := domain.Discovery.Handler()
	monitorHandler := domain.Monitor.Handler()
	jobsHandler := domain.Jobs.Handler()

	routes.Register(
		mux,
		discoveryHandler.Routes(),
		monitorHandler.Routes(),
		jobsHandler.Routes(),
	)
}
```

**`internal/alpha/api.go`:** remove the `domain.Alert.Subscribe()` call (jobs does not need a startup subscription — it publishes via HTTP-triggered Start). Keep `domain.Discovery.Subscribe()` and `domain.Monitor.Subscribe()`.

Final shape:
```go
func NewModule(
	b bus.System,
	info discovery.ServiceInfo,
	cfg *config.Config,
	logger *slog.Logger,
) (*module.Module, error) {
	domain := NewDomain(b, info, cfg, logger)

	if err := domain.Discovery.Subscribe(); err != nil {
		return nil, err
	}

	if err := domain.Monitor.Subscribe(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	registerRoutes(mux, domain)

	return module.New("/api", mux), nil
}
```

---

### Step 8 — Verify `config.json`

`config.json` was updated as part of Step 3 (alongside the config package refactor), since the schema is owned by the same concern. This step is a verification checkpoint:

- Confirm `config.json` reflects the schema shown in Step 3 — `alpha.jobs.interval`, `beta.runners.count`, no `beta.alerts` section, ports swapped (alpha=3000, beta=3001).
- Run `go vet ./...` — should be clean.
- Run `mise run beta` and `mise run alpha` in two terminals — both should bind to their configured ports without config errors.

If any of these fail, return to Step 3's `config.json` block and reconcile.

---

### Step 9 — Restructure the repo root `README.md`

Full replacement:

````markdown
# Signal Lab

A progressive [NATS](https://nats.io) learning repository built around two Go web services — `alpha` and `beta` — that act as generic participants in signal-based coordination through a shared NATS bus. Each phase adds a new domain to illustrate a specific NATS capability.

## Table of contents

- [Overview](#overview)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Development](#development)
  - [Project Structure](#project-structure)
- [Demonstrations](#demonstrations)
  - [Phase 1 — Foundation + Discovery Ping](#phase-1--foundation--discovery-ping)
  - [Phase 2 — Telemetry Pub/Sub](#phase-2--telemetry-pubsub)
  - [Phase 3 — Queue Groups + Headers (Runner Cluster)](#phase-3--queue-groups--headers-runner-cluster)

## Overview

signal-lab explores NATS capabilities through concrete demonstrations exposed as HTTP endpoints on real web services. The two runtimes — `alpha` (the dependent service) and `beta` (the functional service) — are generic participants in a multi-service communication architecture. Each phase adds a new pair of domain packages that illustrate one NATS concept in isolation, from basic pub/sub through durable streams, distributed state, and real-time WebSocket projection.

Communication between alpha and beta flows exclusively through NATS — neither service calls the other directly.

## Getting Started

### Prerequisites

- [Go](https://go.dev/) 1.26+
- [Docker](https://www.docker.com/) + Docker Compose
- [mise](https://mise.jdx.dev/) (task runner)
- [air](https://github.com/air-verse/air) (hot reload, optional)

### Development

Start the NATS infrastructure:

```bash
docker compose up -d
```

This starts NATS with JetStream on port `4222` and the monitoring dashboard on port `8222`.

Run the services in separate terminals:

```bash
# Terminal 1
mise run beta

# Terminal 2
mise run alpha
```

Alpha listens on `:3000`, beta on `:3001`.

Additional tasks:

```bash
mise run test    # run all tests
mise run vet     # run go vet
```

Hot reload:

```bash
air -c .air.beta.toml
air -c .air.alpha.toml
```

### Project Structure

```
cmd/                → Service entry points
  alpha/            → Dependent service (:3000)
  beta/             → Functional service (:3001)
internal/           → Private application packages
  config/           → Shared configuration with three-phase finalize
  alpha/            → Alpha module wiring + domain sub-packages
    monitoring/     → Telemetry subscriber (Phase 2)
    jobs/           → Job dispatcher (Phase 3)
  beta/             → Beta module wiring + domain sub-packages
    telemetry/      → Telemetry publisher (Phase 2)
    runners/        → Runner cluster with queue groups (Phase 3)
pkg/                → Reusable library packages
  lifecycle/        → Startup/shutdown coordination
  bus/              → Message bus System (connection + subscriptions)
  signal/           → Signal envelope type
  discovery/        → Discovery domain (System + Handler)
  routes/           → Route group composition
  module/           → HTTP module/router system
  middleware/       → HTTP middleware (Logger)
  handlers/         → JSON response helpers
  contracts/        → Shared cross-service contracts
    telemetry/      → Telemetry subjects + Reading type (Phase 2)
    jobs/           → Jobs subjects + Job type + header keys (Phase 3)
tests/              → Black-box tests mirroring source structure
_project/           → Architecture docs and phase implementation briefs
```

## Demonstrations

Each demonstration pairs a NATS concept with a concrete domain implementation. Every subsection below covers one phase and documents the concept being illustrated, the domain established to facilitate it, the API endpoints within that domain, and copy-paste execution instructions.

### Phase 1 — Foundation + Discovery Ping

**NATS concept:** request/reply with inbox subjects. A requester broadcasts to a subject and collects all responses that arrive within a timeout window.

**Domain:** `pkg/discovery/` provides a shared discovery `System` used by both services. Each service publishes its `ServiceInfo` when it receives a ping and can initiate discovery as a requester via an HTTP endpoint.

**API endpoints (both services):**

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness check |
| `GET` | `/readyz` | Readiness check |
| `POST` | `/api/discovery/ping` | Broadcast ping on `signal.discovery.ping`, return collected service responses |

**Execution:**

```bash
# terminal 1
mise run beta

# terminal 2
mise run alpha

# discover from either side
curl -s -X POST localhost:3000/api/discovery/ping | jq
curl -s -X POST localhost:3001/api/discovery/ping | jq
```

### Phase 2 — Telemetry Pub/Sub

**NATS concept:** subject hierarchies and wildcard subscriptions. Publishers emit on `signal.telemetry.{type}.{zone}`; subscribers match with `signal.telemetry.>` to receive the whole hierarchy.

**Domain:** `internal/beta/telemetry/` publishes simulated environment readings (temperature, humidity, pressure) on a ticker. `internal/alpha/monitoring/` subscribes to the full telemetry wildcard, tracks per-subject counts, and exposes an SSE stream of incoming signals.

**API endpoints:**

| Service | Method | Path | Description |
|---|---|---|---|
| beta | `POST` | `/api/telemetry/start` | Begin publishing simulated readings |
| beta | `POST` | `/api/telemetry/stop` | Stop publishing |
| beta | `GET` | `/api/telemetry/status` | Publisher state (running, interval, types, zones) |
| alpha | `GET` | `/api/monitoring/stream` | SSE stream of received telemetry signals |
| alpha | `GET` | `/api/monitoring/status` | Subscription state and per-subject counts |

**Execution:**

```bash
curl -s -X POST localhost:3001/api/telemetry/start | jq
curl -s -N localhost:3000/api/monitoring/stream    # Ctrl+C to stop
curl -s localhost:3000/api/monitoring/status | jq
curl -s -X POST localhost:3001/api/telemetry/stop | jq
```

### Phase 3 — Queue Groups + Headers (Runner Cluster)

**NATS concept:** queue-group subscriptions distribute each message to exactly one member of a named group — turning fan-out into work distribution. NATS headers carry routing metadata (priority, type, trace ID) alongside the payload, inspectable without decoding the body.

**Domain:** `internal/alpha/jobs/` publishes jobs on `signal.jobs.{type}` with headers for job ID, priority, type, and trace ID. `internal/beta/runners/` spawns N in-process runners (distinct runner IDs: `runner-0`, `runner-1`, ...) all joined to the `beta-runners` queue group. The cluster size defaults to `runtime.NumCPU()` when `runners.count` is 0 in config, or can be set explicitly. Each job is delivered to exactly one runner; per-runner counts expose the distribution.

**API endpoints:**

| Service | Method | Path | Description |
|---|---|---|---|
| alpha | `POST` | `/api/jobs/start` | Begin publishing jobs |
| alpha | `POST` | `/api/jobs/stop` | Stop publishing |
| alpha | `GET` | `/api/jobs/status` | Publisher state (running, interval) |
| beta | `POST` | `/api/runners/subscribe` | Attach the entire cluster to the queue group |
| beta | `POST` | `/api/runners/unsubscribe` | Drain the entire cluster |
| beta | `POST` | `/api/runners/{id}/subscribe` | Attach a single runner by ID |
| beta | `POST` | `/api/runners/{id}/unsubscribe` | Drain a single runner by ID |
| beta | `GET` | `/api/runners/status` | Cluster state with per-runner status snapshots |

**Execution:**

```bash
# Start jobs and watch initial distribution across all runners
curl -s -X POST localhost:3000/api/jobs/start | jq
sleep 3
curl -s localhost:3001/api/runners/status | jq

# Detach one runner mid-stream and observe redistribution
curl -s -X POST localhost:3001/api/runners/runner-0/unsubscribe | jq
sleep 3
curl -s localhost:3001/api/runners/status | jq    # runner-0 subscribed: false, others picking up its share

# Reattach the runner and observe rebalancing
curl -s -X POST localhost:3001/api/runners/runner-0/subscribe | jq
sleep 3
curl -s localhost:3001/api/runners/status | jq

# Drain the entire cluster (jobs stop being processed by anyone)
curl -s -X POST localhost:3001/api/runners/unsubscribe | jq
sleep 2
curl -s localhost:3001/api/runners/status | jq    # all subscribed: false, counts frozen

# Reattach the cluster and stop publishing
curl -s -X POST localhost:3001/api/runners/subscribe | jq
curl -s -X POST localhost:3000/api/jobs/stop | jq
```

The `/api/runners/status` response shows per-runner subscription state and per-subject counts:

```json
{
  "subscribed": true,
  "count": 4,
  "runners": {
    "runner-0": {
      "subscribed": true,
      "counts": {"signal.jobs.compute": 3, "signal.jobs.io": 2}
    },
    "runner-1": {
      "subscribed": true,
      "counts": {"signal.jobs.compute": 2, "signal.jobs.analysis": 4}
    },
    "runner-2": {
      "subscribed": true,
      "counts": {"signal.jobs.io": 3, "signal.jobs.analysis": 2}
    },
    "runner-3": {
      "subscribed": true,
      "counts": {"signal.jobs.compute": 3, "signal.jobs.io": 2}
    }
  }
}
```
````

---

### Step 10 — Add `## Documentation Conventions` section to `.claude/CLAUDE.md`

Append this section near the end of `.claude/CLAUDE.md` (before or after the Go Conventions section, whichever reads better):

````markdown
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
````

Also update the existing sections in `.claude/CLAUDE.md` for the rename:

- Opening paragraph: describe alpha and beta as generic participants rather than sensor/dispatch
- **Package Structure** section: replace the `internal/sensor/` and `internal/dispatch/` subtrees with `internal/alpha/` (monitoring, jobs) and `internal/beta/` (telemetry, runners)
- **Config Decomposition** paragraph: describe `AlphaConfig` and `BetaConfig` both embedding `ServiceConfig`
- **Systems Pattern** and **Domain Separation** bullet examples: replace `pkg/discovery/` / `internal/{service}/{domain}/` references as needed (sensor → beta, dispatch → alpha)
- **NATS Subject Namespace** list: replace `signal.alerts.{severity}` with `signal.jobs.{type}`
- **Development** section: `mise run sensor` → `mise run beta`, `mise run dispatch` → `mise run alpha`, `.air.sensor.toml` → `.air.beta.toml`, `.air.dispatch.toml` → `.air.alpha.toml`

---

### Step 11 — Update `_project/` docs

**`_project/README.md`:** global rename plus Phase 3 description updates.

- Service descriptions at the top: rewrite to describe alpha as "the dependent service — consumes telemetry and dispatches jobs" and beta as "the functional service — publishes telemetry and hosts runner clusters." Both are generic participants hosting per-phase domain packages.
- Package Hierarchy tree: replace `sensor/`, `dispatch/`, `alerts/`, `alerting/`, `contracts/alerts/` with `beta/`, `alpha/`, `jobs/`, `runners/`, `contracts/jobs/`.
- NATS Subject Namespace table: replace `signal.alerts.{severity}` row with `signal.jobs.{type}`.
- Phase 3 row in the phases table: description becomes "Queue Groups + Headers (Runner Cluster)".
- Rewrite the "NATS Concepts > Queue Groups" paragraph to frame it as a runner cluster dividing work.
- Rewrite the API Endpoint tables:
  - Remove Phase 3 alerts rows from sensor side
  - Remove Phase 3 alerting rows from dispatch side
  - Beta table: add `GET /api/runners/status` (Phase 3)
  - Alpha table: add `POST /api/jobs/start`, `POST /api/jobs/stop`, `GET /api/jobs/status` (Phase 3)
  - Rename all "Sensor" / "Dispatch" headers to "Beta" / "Alpha"
- Development section: rename commands.

**`_project/phase-03.md`:** full rewrite. Replace existing content with:

```markdown
# Phase 3 — Queue Groups + Headers (Runner Cluster)

**Branch:** `<issue-number>-generalize-services-rework-phase-3`

## NATS Concepts

- **Queue groups** — Multiple subscribers join a named group; NATS delivers each message to exactly one member. Turns fan-out pub/sub into distributed work processing.
- **NATS headers** — HTTP-like key/value metadata attached to messages. Routing information that subscribers inspect without deserializing the payload.

## Objective

Alpha publishes jobs via a ticker on `signal.jobs.{type}` with NATS headers. Beta hosts a cluster of N in-process runners, each a queue subscriber on `signal.jobs.>` joined to the `beta-runners` queue group. NATS distributes each job to exactly one runner; per-runner counts expose the distribution.

## Domains

- `pkg/contracts/jobs/` — Job subject constants, `JobType` and `JobPriority` enums, NATS header keys, `Job` payload type.
- `internal/alpha/jobs/` — Ticker-driven publisher. `System` interface with `Start/Stop/Status/Handler`. Publishes a random `Job` each tick with headers (`Job-ID`, `Job-Priority`, `Job-Type`, `Signal-Trace-ID`).
- `internal/beta/runners/` — Runner cluster. `System` interface with `Subscribe/Status/Handler`. Spawns N direct-connection queue subscribers so multiple runners can share a subject in one process.

## New Endpoints

```
Alpha:
  POST /api/jobs/start       → begin publishing jobs
  POST /api/jobs/stop        → stop publishing
  GET  /api/jobs/status      → publisher state

Beta:
  GET  /api/runners/status   → cluster state with per-runner, per-subject counts
```

## Verification

1. Start both services.
2. `POST /api/jobs/start` on alpha.
3. Wait a few seconds.
4. `GET /api/runners/status` on beta: every runner should have non-zero counts across one or more subjects. Counts should roughly balance over time.
5. `POST /api/jobs/stop` on alpha.
6. Phase 1 (discovery) and Phase 2 (telemetry/monitoring) still work unchanged.

## Notes on implementation

- Runners call `bus.Conn().QueueSubscribe(...)` directly because `bus.QueueSubscribe` tracks subscriptions by subject and rejects duplicates within a process. The overall NATS drain handles shutdown.
- Runner IDs (`runner-0`, `runner-1`, ...) are assigned at construction time. The handler closure binds each runner ID so counts can be attributed per-runner without runtime lookups.
- Jobs carry priority in a NATS header. Runners branch their log level on header value without decoding the Job body — demonstrating the header-based routing pattern.
```

**`_project/phase-01.md`, `phase-02.md`:** global rename sensor→beta, dispatch→alpha. No conceptual changes.

**`_project/phase-04.md` through `_project/phase-08.md`:** global rename sensor→beta, dispatch→alpha. Do NOT update conceptual content — Phase 4's content will be rewritten when Issue #7 is updated during closeout.

---

## Validation Criteria

- [ ] Every `sensor` and `dispatch` reference in code/config/toml/json is now `beta` or `alpha`. Remaining hits only in `_project/phase-0X.md` where appropriate after the rename pass
- [ ] `go vet ./...` clean
- [ ] `go run ./cmd/alpha` starts, binds `:3000`, `GET /healthz` returns 200
- [ ] `go run ./cmd/beta` starts, binds `:3001`, `GET /healthz` returns 200
- [ ] `POST /api/discovery/ping` on both services returns the other service in the response list
- [ ] `POST /api/telemetry/start` on beta → telemetry flows → `GET /api/monitoring/status` on alpha shows non-zero per-subject counts
- [ ] `POST /api/jobs/start` on alpha → after a few seconds, `GET /api/runners/status` on beta shows every runner with non-zero counts, spread across `signal.jobs.compute`, `signal.jobs.io`, `signal.jobs.analysis`
- [ ] `POST /api/runners/runner-0/unsubscribe` → status shows `runner-0.subscribed: false`, remaining runners' counts continue to grow, runner-0 counts frozen
- [ ] `POST /api/runners/runner-0/subscribe` → status shows `runner-0.subscribed: true`, runner-0 counts resume growing
- [ ] `POST /api/runners/unsubscribe` → cluster `subscribed: false`, every runner `subscribed: false`, counts stop incrementing across the board
- [ ] `POST /api/runners/subscribe` → cluster `subscribed: true`, every runner `subscribed: true`, counts resume
- [ ] `POST /api/runners/subscribe` is eager-idempotent: calling twice on a fully-attached cluster returns 200 with no joined errors
- [ ] `POST /api/runners/unsubscribe` is eager-idempotent: calling twice on a fully-detached cluster returns 200 with no joined errors
- [ ] `POST /api/runners/{id}/subscribe` on an already-attached runner returns 409 Conflict with "runner already subscribed" (per-runner ops are idempotent-via-error)
- [ ] `POST /api/runners/{id}/unsubscribe` on a detached runner returns 409 Conflict with "runner not subscribed"
- [ ] `POST /api/runners/runner-99/subscribe` (nonexistent ID) returns a 409 Conflict with "runner not found" message
- [ ] After `POST /api/runners/runner-0/unsubscribe`, calling `POST /api/runners/subscribe` reattaches only runner-0 (the others are skipped because they're already attached) and returns 200
- [ ] NATS monitoring UI (`:8222`) shows queue group `beta-runners` with N subscribers on `signal.jobs.>` when fully attached
- [ ] `internal/beta/alerts/`, `internal/alpha/alerting/`, `pkg/contracts/alerts/`, `internal/config/alerts.go`, `tests/alerts/` are deleted
- [ ] `config.json` has new `beta` and `alpha` sections with `runners` and `jobs` sub-configs
- [ ] `README.md` follows the new Overview / Getting Started / Demonstrations structure
- [ ] `.claude/CLAUDE.md` has the new `## Documentation Conventions` section
- [ ] `_project/phase-03.md` fully rewritten for the runner cluster
