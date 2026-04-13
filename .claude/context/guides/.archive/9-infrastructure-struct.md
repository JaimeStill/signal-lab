# 9 - Encapsulate Core Subsystems into an Infrastructure Struct

## Problem Context

Both alpha and beta servers thread `lc`, `bus`, `info`, `cfg`, and `logger` as five individual parameters through the entire wiring chain: `Server → buildHandler → NewModule → NewDomain`. Each new domain perpetuates this signature bloat. Herald's `internal/infrastructure/` pattern solves this with a single struct constructed once at startup. This refactor ports that pattern to signal-lab.

## Architecture Approach

**Infrastructure struct** — holds Lifecycle, Logger, Bus, ServiceInfo, and ShutdownTimeout. Constructed via `New(cfg, svc, logger)` where `cfg` is the full `*config.Config` and `svc` is a `*config.ServiceConfig` pointer identifying which service to build for. Config is consumed during construction (to derive Bus, ServiceInfo, shutdown timeout) but not retained as a field.

**Runtime wrappers** — each service defines a `Runtime` struct in its module package (`internal/alpha/`, `internal/beta/`) that embeds `*Infrastructure` and adds service-specific config values. Runtime is built in `NewModule` and passed to `NewDomain`, which unpacks it into the primitives each domain constructor expects.

**Domain constructors unchanged** — `telemetry.New`, `monitoring.New`, `jobs.New`, `runners.New`, and `discovery.New` keep their current signatures. Infrastructure is strictly a wiring-layer concern.

## Implementation

### Step 1: Create `internal/infrastructure/infrastructure.go`

This is the core new package. The struct owns the four cross-cutting subsystems plus shutdown timeout. `New` consumes config to build everything; `Start` connects the bus.

```go
package infrastructure

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

type Infrastructure struct {
	Lifecycle       *lifecycle.Coordinator
	Logger          *slog.Logger
	Bus             bus.System
	Info            discovery.ServiceInfo
	ShutdownTimeout time.Duration
}

func New(
	cfg *config.Config,
	svc *config.ServiceConfig,
	logger *slog.Logger,
) *Infrastructure {
	return &Infrastructure{
		Lifecycle:       lifecycle.New(),
		Logger:          logger,
		Bus:             bus.New(&cfg.Bus, cfg.ShutdownTimeoutDuration(), logger),
		ShutdownTimeout: cfg.ShutdownTimeoutDuration(),
		Info: discovery.ServiceInfo{
			ID:          uuid.New().String(),
			Name:        svc.Name,
			Endpoint:    fmt.Sprintf("http://%s", svc.Addr()),
			Description: svc.Description,
		},
	}
}

func (infra *Infrastructure) Start() error {
	return infra.Bus.Start(infra.Lifecycle)
}
```

### Step 2: Create `internal/alpha/runtime.go`

Plain data struct with no constructor. Embeds `*Infrastructure` and adds the two config values alpha's domain constructors need.

```go
package alpha

import (
	"time"

	"github.com/JaimeStill/signal-lab/internal/infrastructure"
)

type Runtime struct {
	*infrastructure.Infrastructure
	ResponseTimeout time.Duration
	JobInterval     time.Duration
}
```

### Step 3: Create `internal/beta/runtime.go`

Same pattern with beta's config surface.

```go
package beta

import (
	"time"

	"github.com/JaimeStill/signal-lab/internal/infrastructure"
)

type Runtime struct {
	*infrastructure.Infrastructure
	ResponseTimeout   time.Duration
	TelemetryInterval time.Duration
	TelemetryTypes    []string
	Zones             []string
	RunnerCount       int
}
```

### Step 4: Update `internal/alpha/domain.go`

Change the `NewDomain` signature to accept `*Runtime`. The domain constructors themselves are unchanged — the body just unpacks from Runtime instead of from individual params.

Replace the current function with:

```go
func NewDomain(rt *Runtime) *Domain {
	return &Domain{
		Discovery: discovery.New(
			rt.Bus, rt.Info,
			rt.ResponseTimeout,
			rt.Logger,
		),
		Monitor: monitoring.New(rt.Bus, rt.Logger),
		Jobs: jobs.New(
			rt.Bus,
			rt.Info.Name,
			rt.JobInterval,
			rt.Logger,
		),
	}
}
```

The `Domain` struct is unchanged. Imports drop `config`, `bus`, and `discovery` (all accessed through Runtime). The `slog` import also drops.

### Step 5: Update `internal/beta/domain.go`

Same pattern. Replace `NewDomain` with:

```go
func NewDomain(rt *Runtime) *Domain {
	disc := discovery.New(
		rt.Bus, rt.Info,
		rt.ResponseTimeout,
		rt.Logger,
	)

	tel := telemetry.New(
		rt.Bus,
		rt.Info.Name,
		rt.TelemetryInterval,
		rt.TelemetryTypes,
		rt.Zones,
		rt.Logger,
	)

	run := runners.New(
		rt.Bus,
		rt.RunnerCount,
		contracts.SubjectWildcard,
		runners.QueueGroup,
		rt.Logger,
	)

	return &Domain{
		Discovery: disc,
		Telemetry: tel,
		Runners:   run,
	}
}
```

Imports drop `config`, `bus`, and `slog`. The `discovery` import also drops — `discovery.New` is still called, but the `discovery` package import stays because `Domain` has `discovery.System` as a field type. Actually, keep `discovery` for the `discovery.System` type in the `Domain` struct and the `discovery.New` call. Drop `config`, `bus`, and `slog`.

### Step 6: Update `internal/alpha/api.go`

Change `NewModule` to accept `*infrastructure.Infrastructure` and `*config.Config`. Build the Runtime inline, then pass it to `NewDomain`.

Replace the current function with:

```go
func NewModule(
	infra *infrastructure.Infrastructure,
	cfg *config.Config,
) (*module.Module, error) {
	rt := &Runtime{
		Infrastructure:  infra,
		ResponseTimeout: cfg.Bus.ResponseTimeoutDuration(),
		JobInterval:     cfg.Alpha.Jobs.IntervalDuration(),
	}

	domain := NewDomain(rt)

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

Imports drop `bus`, `discovery`, `slog`. Add `infrastructure`.

### Step 7: Update `internal/beta/api.go`

Same pattern. Replace `NewModule` with:

```go
func NewModule(
	infra *infrastructure.Infrastructure,
	cfg *config.Config,
) (*module.Module, error) {
	rt := &Runtime{
		Infrastructure:    infra,
		ResponseTimeout:   cfg.Bus.ResponseTimeoutDuration(),
		TelemetryInterval: cfg.Beta.Telemetry.IntervalDuration(),
		TelemetryTypes:    cfg.Beta.Telemetry.Types,
		Zones:             cfg.Beta.Zones,
		RunnerCount:       cfg.Beta.Runners.Number(),
	}

	domain := NewDomain(rt)

	if err := domain.Discovery.Subscribe(); err != nil {
		return nil, err
	}

	if err := domain.Runners.Subscribe(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	registerRoutes(mux, domain)

	return module.New("/api", mux), nil
}
```

Imports drop `bus`, `discovery`, `slog`. Add `infrastructure`.

### Step 8: Update `cmd/alpha/modules.go`

Change `buildHandler` to accept `*infrastructure.Infrastructure` and `*config.Config`. The `buildRouter` function stays unchanged — it still takes `*lifecycle.Coordinator`.

Replace the `buildHandler` function with:

```go
func buildHandler(
	infra *infrastructure.Infrastructure,
	cfg *config.Config,
) (http.Handler, error) {
	router := buildRouter(infra.Lifecycle)

	mod, err := alpha.NewModule(infra, cfg)
	if err != nil {
		return nil, err
	}
	router.Mount(mod)

	mw := middleware.New()
	mw.Use(middleware.Logger(infra.Logger))

	return mw.Apply(router), nil
}
```

Imports drop `bus`, `discovery`, `lifecycle`, `slog`. Add `infrastructure`.

### Step 9: Update `cmd/beta/modules.go`

Mirror Step 8. Replace `buildHandler` with the same shape, calling `beta.NewModule(infra, cfg)` instead.

Imports drop `bus`, `discovery`, `lifecycle`, `slog`. Add `infrastructure`.

### Step 10: Update `cmd/alpha/server.go`

Replace the four individual fields (`lc`, `bus`, `info`, `logger`) with a single `infra` field.

New Server struct:

```go
type Server struct {
	cfg   *config.Config
	infra *infrastructure.Infrastructure
	http  *httpServer
}
```

Replace `NewServer`:

```go
func NewServer(cfg *config.Config) *Server {
	logger := slog.With("service", cfg.Alpha.Name)

	return &Server{
		cfg:   cfg,
		infra: infrastructure.New(cfg, &cfg.Alpha.ServiceConfig, logger),
	}
}
```

Replace `Start`:

```go
func (s *Server) Start() error {
	if err := s.infra.Start(); err != nil {
		return err
	}

	handler, err := buildHandler(s.infra, s.cfg)
	if err != nil {
		return err
	}

	s.http = newHTTPServer(
		s.cfg.Alpha.Addr(),
		handler,
		s.infra.ShutdownTimeout,
		s.infra.Logger,
	)
	s.http.Start(s.infra.Lifecycle)

	go func() {
		s.infra.Lifecycle.WaitForStartup()
		s.infra.Logger.Info("all subsystems ready")
	}()

	return nil
}
```

Replace `Shutdown`:

```go
func (s *Server) Shutdown() error {
	s.infra.Logger.Info("initiating shutdown")
	return s.infra.Lifecycle.Shutdown(s.infra.ShutdownTimeout)
}
```

Imports drop `fmt`, `uuid`, `bus`, `discovery`, `lifecycle`. Add `infrastructure`. Keep `slog` for the logger creation in `NewServer`.

### Step 11: Update `cmd/beta/server.go`

Mirror Step 10. The only differences are `cfg.Beta.Name`, `&cfg.Beta.ServiceConfig`, and `s.cfg.Beta.Addr()`.

## Validation Criteria

- [ ] `go vet ./...` — clean
- [ ] `mise run test` — all existing tests pass
- [ ] Phase 1 discovery ping works end-to-end
- [ ] Phase 2 telemetry/monitoring works end-to-end
- [ ] Phase 3 jobs/runners works end-to-end
- [ ] Domain system constructors (`telemetry.New`, `monitoring.New`, `jobs.New`, `runners.New`, `discovery.New`) have unchanged signatures
