# Phase 3: Queue Groups + Headers — Implementation Guide

**Issue:** #5 | **Branch:** `phase-03-queues-headers`

## Problem Context

Phase 3 introduces queue groups and NATS headers via new alerts/alerting domains. During early implementation, three design gaps surfaced: no shared contracts for cross-service types/constants, config conflation in `ServiceConfig`, and zones duplicated across domain configs. This guide addresses convention alignment first, then completes the Phase 3 wiring.

## Already Implemented

- [x] `pkg/bus/bus.go` — `QueueSubscribe` on System interface
- [x] `internal/config/alerts.go` — `AlertsConfig` with Interval
- [x] `internal/config/server.go` — Alerts field, omitempty removed
- [x] `internal/config/config.go` — `Sensor.Alerts.Finalize()` added
- [x] `config.json` — alerts interval in sensor block
- [x] `internal/sensor/alerts/` — Alert publisher with headers (alerts.go + handler.go)
- [x] `internal/dispatch/alerting/` — Queue group subscriber with SSE (alerting.go + handler.go)

---

## Section 1: Convention Alignment

### Step 1 — Create `pkg/contracts/telemetry/telemetry.go`

New file. Extracts cross-service telemetry contract from `internal/sensor/telemetry/telemetry.go` and `internal/dispatch/monitoring/monitoring.go`.

```go
package telemetry

// SubjectPrefix is the base subject for telemetry signals.
const SubjectPrefix = "signal.telemetry"

// SubjectWildcard matches all telemetry subjects.
const SubjectWildcard = "signal.telemetry.>"

// Reading represents a single telemetry measurement.
type Reading struct {
	Type  string  `json:"type"`
	Zone  string  `json:"zone"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
```

### Step 2 — Create `pkg/contracts/alerts/alerts.go`

New file. Extracts cross-service alerts contract from `internal/sensor/alerts/alerts.go` and `internal/dispatch/alerting/alerting.go`.

```go
package alerts

// AlertPriority represents the severity level of an alert signal.
type AlertPriority string

const (
	PriorityLow      AlertPriority = "low"
	PriorityNormal   AlertPriority = "normal"
	PriorityHigh     AlertPriority = "high"
	PriorityCritical AlertPriority = "critical"
)

// SubjectPrefix is the base subject for alert signals.
const SubjectPrefix = "signal.alerts"

// SubjectWildcard matches all alert subjects.
const SubjectWildcard = "signal.alerts.>"

// NATS header keys for alert metadata.
const (
	HeaderPriority = "Signal-Priority"
	HeaderSource   = "Signal-Source"
	HeaderTraceID  = "Signal-Trace-ID"
)

// Alert represents a priority-tagged alert signal.
type Alert struct {
	Severity AlertPriority `json:"severity"`
	Message  string        `json:"message"`
	Zone     string        `json:"zone"`
}
```

### Step 3 — Update `internal/sensor/telemetry/telemetry.go`

Remove `Reading` struct (lines 18-23) and `subjectPrefix` constant (line 15). Import from contract.

**Imports** — add:
```go
contracts "github.com/JaimeStill/signal-lab/pkg/contracts/telemetry"
```

**Remove** the `subjectPrefix` constant and the `Reading` struct entirely.

**Add** a type alias to preserve the local API:
```go
type Reading = contracts.Reading
```

**Update** the `publish()` method — replace `subjectPrefix` with `contracts.SubjectPrefix`:
```go
subject := fmt.Sprintf("%s.%s.%s", contracts.SubjectPrefix, typ, zone)
```

### Step 4 — Update `internal/dispatch/monitoring/monitoring.go`

Remove `telemetrySubject` constant (line 15). Import from contract.

**Imports** — add:
```go
contracts "github.com/JaimeStill/signal-lab/pkg/contracts/telemetry"
```

**Remove** the `telemetrySubject` constant.

**Update** `Subscribe()` — replace `telemetrySubject` with `contracts.SubjectWildcard`:
```go
if err := m.bus.Subscribe(contracts.SubjectWildcard, m.onTelemetry); err != nil {
```

**Update** the log line in `Subscribe()`:
```go
m.logger.Info("subscribed to telemetry", "subject", contracts.SubjectWildcard)
```

### Step 5 — Update `internal/sensor/alerts/alerts.go`

Remove `AlertPriority` type, priority constants, `Alert` struct, `subjectPrefix` constant, and hardcoded header strings. Import from contract.

**Imports** — replace `"github.com/JaimeStill/signal-lab/pkg/signal"` block, adding:
```go
contracts "github.com/JaimeStill/signal-lab/pkg/contracts/alerts"
```

**Remove** entirely:
- `subjectPrefix` constant (line 17)
- `AlertPriority` type and constants (lines 19-27)
- `Alert` struct (lines 29-34)

**Add** type aliases to preserve the local API:
```go
type AlertPriority = contracts.AlertPriority
type Alert = contracts.Alert
```

**Add** local constants referencing the contract:
```go
const (
	PriorityLow      = contracts.PriorityLow
	PriorityNormal   = contracts.PriorityNormal
	PriorityHigh     = contracts.PriorityHigh
	PriorityCritical = contracts.PriorityCritical
)
```

**Update** `publish()` — replace `subjectPrefix` and header strings:
```go
subject := fmt.Sprintf("%s.%s", contracts.SubjectPrefix, priority)
```
```go
msg.Header.Set(contracts.HeaderPriority, string(priority))
msg.Header.Set(contracts.HeaderSource, a.source)
msg.Header.Set(contracts.HeaderTraceID, sig.ID)
```

### Step 6 — Update `internal/dispatch/alerting/alerting.go`

Remove `alertsSubject` constant and hardcoded header/priority strings. Import from contract.

**Imports** — add:
```go
contracts "github.com/JaimeStill/signal-lab/pkg/contracts/alerts"
```

**Update** constants — replace `alertsSubject`:
```go
const queueGroup = "dispatch-workers"
```

**Update** `Subscribe()` — replace `alertsSubject` with `contracts.SubjectWildcard`:
```go
if err := a.bus.QueueSubscribe(
    contracts.SubjectWildcard,
    queueGroup,
    a.onAlert,
); err != nil {
```

**Update** log line:
```go
a.logger.Info("subscribed to alerts", "subject", contracts.SubjectWildcard, "queue", queueGroup)
```

**Update** `onAlert()` — replace hardcoded header keys and priority strings:
```go
for _, key := range []string{contracts.HeaderPriority, contracts.HeaderSource, contracts.HeaderTraceID} {
```
```go
priority := sig.Metadata[contracts.HeaderPriority]
switch contracts.AlertPriority(priority) {
case contracts.PriorityCritical:
    a.logger.Error(
        "critical priority alert",
        "subject", msg.Subject,
        "trace_id", sig.Metadata[contracts.HeaderTraceID],
    )
case contracts.PriorityHigh:
    a.logger.Warn(
        "high priority alert",
        "subject", msg.Subject,
        "trace_id", sig.Metadata[contracts.HeaderTraceID],
    )
default:
    a.logger.Info(
        "alert received",
        "subject", msg.Subject,
        "priority", priority,
    )
}
```

### Step 7 — Config decomposition: `internal/config/server.go`

Strip `ServiceConfig` to shared fields only. Remove `Telemetry`, `Alerts`, and their merge calls.

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ServiceConfig holds per-service HTTP server parameters.
type ServiceConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Addr returns the host:port listen address.
func (c *ServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Finalize applies defaults, environment overrides, and validation.
// envPrefix distinguishes services (e.g. "SENSOR", "DISPATCH").
func (c *ServiceConfig) Finalize(envPrefix string) error {
	c.loadDefaults()
	c.loadEnv(envPrefix)
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *ServiceConfig) Merge(overlay *ServiceConfig) {
	if overlay.Host != "" {
		c.Host = overlay.Host
	}
	if overlay.Port != 0 {
		c.Port = overlay.Port
	}
	if overlay.Name != "" {
		c.Name = overlay.Name
	}
	if overlay.Description != "" {
		c.Description = overlay.Description
	}
}

func (c *ServiceConfig) loadDefaults() {
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
}

func (c *ServiceConfig) loadEnv(prefix string) {
	if v := os.Getenv(parseVar(prefix, "HOST")); v != "" {
		c.Host = v
	}
	if v := os.Getenv(parseVar(prefix, "PORT")); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Port = port
		}
	}
	if v := os.Getenv(parseVar(prefix, "NAME")); v != "" {
		c.Name = v
	}
	if v := os.Getenv(parseVar(prefix, "DESCRIPTION")); v != "" {
		c.Description = v
	}
}

func (c *ServiceConfig) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.Name == "" {
		return fmt.Errorf("service name is required")
	}
	return nil
}

func parseVar(prefix, variable string) string {
	variable = strings.TrimPrefix(variable, "_")
	return "SIGNAL_" + prefix + "_" + variable
}
```

### Step 8 — Create `internal/config/sensor.go`

New file. `SensorConfig` embeds `ServiceConfig` and owns Zones + domain configs.

```go
package config

import (
	"fmt"
	"os"
	"strings"
)

const EnvSensorZones = "SIGNAL_SENSOR_ZONES"

// SensorConfig holds sensor-specific configuration.
type SensorConfig struct {
	ServiceConfig
	Zones     []string        `json:"zones"`
	Telemetry TelemetryConfig `json:"telemetry"`
	Alerts    AlertsConfig    `json:"alerts"`
}

// Finalize applies defaults, environment overrides, validation,
// and finalizes sub-configs.
func (c *SensorConfig) Finalize(envPrefix string) error {
	if err := c.ServiceConfig.Finalize(envPrefix); err != nil {
		return err
	}

	c.loadDefaults()
	c.loadEnv()

	if err := c.validate(); err != nil {
		return err
	}
	if err := c.Telemetry.Finalize(); err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	if err := c.Alerts.Finalize(); err != nil {
		return fmt.Errorf("alerts: %w", err)
	}

	return nil
}

// Merge overwrites non-zero fields from overlay.
func (c *SensorConfig) Merge(overlay *SensorConfig) {
	c.ServiceConfig.Merge(&overlay.ServiceConfig)

	if len(overlay.Zones) > 0 {
		c.Zones = overlay.Zones
	}

	c.Telemetry.Merge(&overlay.Telemetry)
	c.Alerts.Merge(&overlay.Alerts)
}

func (c *SensorConfig) loadDefaults() {
	if len(c.Zones) == 0 {
		c.Zones = []string{
			"server-room",
			"ops-center",
		}
	}
}

func (c *SensorConfig) loadEnv() {
	if v := os.Getenv(EnvSensorZones); v != "" {
		c.Zones = strings.Split(v, ",")
	}
}

func (c *SensorConfig) validate() error {
	if len(c.Zones) == 0 {
		return fmt.Errorf("sensor zones must not be empty")
	}
	return nil
}
```

### Step 9 — Update `internal/config/telemetry.go`

Remove `Zones` field and all zone-related logic. Keep `Interval` and `Types` only.

```go
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	EnvTelemetryInterval = "SIGNAL_TELEMETRY_INTERVAL"
	EnvTelemetryTypes    = "SIGNAL_TELEMETRY_TYPES"
)

// TelemetryConfig holds telemetry publisher parameters.
type TelemetryConfig struct {
	Interval string   `json:"interval"`
	Types    []string `json:"types"`
}

// IntervalDuration returns Interval as a time.Duration.
func (c *TelemetryConfig) IntervalDuration() time.Duration {
	d, _ := time.ParseDuration(c.Interval)
	return d
}

// Finalize applies defaults, environment overrides, and validation.
func (c *TelemetryConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *TelemetryConfig) Merge(overlay *TelemetryConfig) {
	if overlay.Interval != "" {
		c.Interval = overlay.Interval
	}
	if len(overlay.Types) > 0 {
		c.Types = overlay.Types
	}
}

func (c *TelemetryConfig) loadDefaults() {
	if c.Interval == "" {
		c.Interval = "2s"
	}
	if len(c.Types) == 0 {
		c.Types = []string{
			"temp",
			"humidity",
			"pressure",
		}
	}
}

func (c *TelemetryConfig) loadEnv() {
	if v := os.Getenv(EnvTelemetryInterval); v != "" {
		c.Interval = v
	}
	if v := os.Getenv(EnvTelemetryTypes); v != "" {
		c.Types = strings.Split(v, ",")
	}
}

func (c *TelemetryConfig) validate() error {
	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid telemetry interval: %w", err)
	}
	if len(c.Types) == 0 {
		return fmt.Errorf("telemetry types must not be empty")
	}
	return nil
}
```

### Step 10 — Update `internal/config/config.go`

Change `Sensor` field type to `SensorConfig`. Simplify finalize chain since `SensorConfig.Finalize` handles its own sub-configs.

**Replace** the `Config` struct:
```go
type Config struct {
	Bus             BusConfig     `json:"bus"`
	Sensor          SensorConfig  `json:"sensor"`
	Dispatch        ServiceConfig `json:"dispatch"`
	ShutdownTimeout string        `json:"shutdown_timeout"`
}
```

**Replace** the `Merge` method:
```go
func (c *Config) Merge(overlay *Config) {
	if overlay.ShutdownTimeout != "" {
		c.ShutdownTimeout = overlay.ShutdownTimeout
	}
	c.Bus.Merge(&overlay.Bus)
	c.Sensor.Merge(&overlay.Sensor)
	c.Dispatch.Merge(&overlay.Dispatch)
}
```

**Replace** the `finalize` method — remove the separate `Sensor.Telemetry.Finalize()` and `Sensor.Alerts.Finalize()` calls since `SensorConfig.Finalize` handles them:
```go
func (c *Config) finalize() error {
	c.loadDefaults()
	c.loadEnv()

	if err := c.validate(); err != nil {
		return err
	}
	if err := c.Bus.Finalize(); err != nil {
		return fmt.Errorf("bus: %w", err)
	}
	if err := c.Sensor.Finalize("SENSOR"); err != nil {
		return fmt.Errorf("sensor: %w", err)
	}
	if err := c.Dispatch.Finalize("DISPATCH"); err != nil {
		return fmt.Errorf("dispatch: %w", err)
	}
	return nil
}
```

### Step 11 — Update `config.json`

Lift `zones` from `telemetry` to the `sensor` level:

```json
{
  "bus": {
    "url": "nats://localhost:4222",
    "max_reconnects": 10,
    "reconnect_wait": "2s",
    "response_timeout": "500ms"
  },
  "sensor": {
    "port": 3000,
    "name": "sensor",
    "description": "Environment monitoring service",
    "zones": ["server-room", "ops-center"],
    "telemetry": {
      "interval": "2s",
      "types": ["temp", "humidity", "pressure"]
    },
    "alerts": {
      "interval": "3s"
    }
  },
  "dispatch": {
    "port": 3001,
    "name": "dispatch",
    "description": "Notification and response coordination service"
  },
  "shutdown_timeout": "30s"
}
```

---

## Section 2: Phase 3 Completion

### Step 12 — Update `internal/sensor/domain.go`

Add `Alerts` field and construct with `cfg.Sensor.Zones`. Pass zones from `cfg.Sensor.Zones` to both telemetry and alerts.

```go
package sensor

import (
	"log/slog"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/sensor/alerts"
	"github.com/JaimeStill/signal-lab/internal/sensor/telemetry"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
)

// Domain assembles all sensor domain systems.
type Domain struct {
	Discovery discovery.System
	Telemetry telemetry.System
	Alerts    alerts.System
}

// NewDomain creates the sensor domain systems.
func NewDomain(
	b bus.System,
	info discovery.ServiceInfo,
	cfg *config.Config,
	logger *slog.Logger,
) *Domain {
	telCfg := &cfg.Sensor.Telemetry
	alertCfg := &cfg.Sensor.Alerts

	disc := discovery.New(
		b, info,
		cfg.Bus.ResponseTimeoutDuration(),
		logger,
	)

	tel := telemetry.New(
		b,
		info.Name,
		telCfg.IntervalDuration(),
		telCfg.Types,
		cfg.Sensor.Zones,
		logger,
	)

	alt := alerts.New(
		b,
		info.Name,
		alertCfg.IntervalDuration(),
		cfg.Sensor.Zones,
		logger,
	)

	return &Domain{
		Discovery: disc,
		Telemetry: tel,
		Alerts:    alt,
	}
}
```

### Step 13 — Update `internal/sensor/routes.go`

Register alerts handler routes.

```go
package sensor

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	dh := domain.Discovery.Handler()
	th := domain.Telemetry.Handler()
	ah := domain.Alerts.Handler()

	routes.Register(
		mux,
		dh.Routes(),
		th.Routes(),
		ah.Routes(),
	)
}
```

### Step 14 — Update `internal/dispatch/domain.go`

Add `Alerting` field.

```go
package dispatch

import (
	"log/slog"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/dispatch/alerting"
	"github.com/JaimeStill/signal-lab/internal/dispatch/monitoring"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
)

// Domain assembles all dispatch domain systems.
type Domain struct {
	Discovery discovery.System
	Monitor   monitoring.System
	Alerting  alerting.System
}

// NewDomain creates the dispatch domain systems.
func NewDomain(
	b bus.System,
	info discovery.ServiceInfo,
	cfg *config.Config,
	logger *slog.Logger,
) *Domain {
	return &Domain{
		Discovery: discovery.New(
			b, info,
			cfg.Bus.ResponseTimeoutDuration(),
			logger,
		),
		Monitor:  monitoring.New(b, logger),
		Alerting: alerting.New(b, logger),
	}
}
```

### Step 15 — Update `internal/dispatch/routes.go`

Register alerting handler routes.

```go
package dispatch

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	dh := domain.Discovery.Handler()
	mh := domain.Monitor.Handler()
	ah := domain.Alerting.Handler()

	routes.Register(mux, dh.Routes(), mh.Routes(), ah.Routes())
}
```

### Step 16 — Update `internal/dispatch/api.go`

Subscribe the alerting system.

```go
package dispatch

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/module"
)

// NewModule creates the dispatch API with discovery, monitoring, and alerting domains.
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

	if err := domain.Alerting.Subscribe(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	registerRoutes(mux, domain)

	return module.New("/api", mux), nil
}
```

---

## Validation Criteria

- [ ] `go vet ./...` — clean
- [ ] `go build ./cmd/sensor && go build ./cmd/dispatch` — compiles
- [ ] No hardcoded subject prefixes or header keys in `internal/` packages
- [ ] `internal/sensor/telemetry/` and `internal/dispatch/monitoring/` unchanged in behavior (Phase 2 preserved)
- [ ] `config.json` has zones at sensor level, not inside telemetry
- [ ] Dispatch config has no telemetry or alerts fields
