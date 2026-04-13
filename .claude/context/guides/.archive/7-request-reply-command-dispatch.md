# 7 - Request/Reply Command Dispatch

## Problem Context

Phase 4 demonstrates NATS request/reply as a point-to-point RPC mechanism. Alpha issues commands to beta via `nc.Request()` and waits for acknowledgments. Beta subscribes to a command subject hierarchy, dispatches per-action handlers, maintains a ledger, and replies. This is distinct from Phase 1's broadcast discovery — Phase 4 is one request, one reply, with timeout semantics.

## Architecture Approach

Follows the established domain patterns exactly. Two new domain packages (commander on alpha, responder on beta) plus a shared contract. Commander uses `bus.Conn().Request()` for RPC; responder uses `msg.Respond()` (same as discovery's `onPing`). No new infrastructure packages needed.

## Implementation

### Step 1: Shared Contract — `pkg/contracts/commands/commands.go`

New file:

```go
package commands

import "fmt"

const SubjectPrefix = "signal.commands"

const SubjectWildcard = SubjectPrefix + ".>"

type Action string

const (
	ActionPing   Action = "ping"
	ActionFlush  Action = "flush"
	ActionRotate Action = "rotate"
	ActionNoop   Action = "noop"
)

func Subject(action Action) string {
	return fmt.Sprintf("%s.%s", SubjectPrefix, action)
}

type Status string

const (
	StatusOK    Status = "ok"
	StatusError Status = "error"
)

type Command struct {
	ID       string `json:"id"`
	Action   Action `json:"action"`
	Payload  string `json:"payload,omitempty"`
	IssuedAt string `json:"issued_at"`
}

type Response struct {
	CommandID string `json:"command_id"`
	Status    Status `json:"status"`
	Result    string `json:"result"`
	HandledAt string `json:"handled_at"`
}
```

### Step 2a: Commander Config — `internal/config/commander.go`

New file:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	EnvCommanderTimeout    = "SIGNAL_COMMANDER_TIMEOUT"
	EnvCommanderMaxHistory = "SIGNAL_COMMANDER_MAX_HISTORY"
)

type CommanderConfig struct {
	Timeout    string `json:"timeout"`
	MaxHistory int    `json:"max_history"`
}

func (c *CommanderConfig) TimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(c.Timeout)
	return d
}

func (c *CommanderConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

func (c *CommanderConfig) Merge(overlay *CommanderConfig) {
	if overlay.Timeout != "" {
		c.Timeout = overlay.Timeout
	}
	if overlay.MaxHistory != 0 {
		c.MaxHistory = overlay.MaxHistory
	}
}

func (c *CommanderConfig) loadDefaults() {
	if c.Timeout == "" {
		c.Timeout = "2s"
	}
	if c.MaxHistory == 0 {
		c.MaxHistory = 64
	}
}

func (c *CommanderConfig) loadEnv() {
	if v := os.Getenv(EnvCommanderTimeout); v != "" {
		c.Timeout = v
	}
	if v := os.Getenv(EnvCommanderMaxHistory); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxHistory = n
		}
	}
}

func (c *CommanderConfig) validate() error {
	if _, err := time.ParseDuration(c.Timeout); err != nil {
		return fmt.Errorf("invalid commander timeout: %w", err)
	}
	if c.MaxHistory < 1 {
		return fmt.Errorf("commander max_history must be >= 1, got %d", c.MaxHistory)
	}
	return nil
}
```

### Step 2b: Responder Config — `internal/config/responder.go`

New file:

```go
package config

import (
	"fmt"
	"os"
	"strconv"
)

const EnvResponderMaxLedger = "SIGNAL_RESPONDER_MAX_LEDGER"

type ResponderConfig struct {
	MaxLedger int `json:"max_ledger"`
}

func (c *ResponderConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

func (c *ResponderConfig) Merge(overlay *ResponderConfig) {
	if overlay.MaxLedger != 0 {
		c.MaxLedger = overlay.MaxLedger
	}
}

func (c *ResponderConfig) loadDefaults() {
	if c.MaxLedger == 0 {
		c.MaxLedger = 64
	}
}

func (c *ResponderConfig) loadEnv() {
	if v := os.Getenv(EnvResponderMaxLedger); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxLedger = n
		}
	}
}

func (c *ResponderConfig) validate() error {
	if c.MaxLedger < 1 {
		return fmt.Errorf("responder max_ledger must be >= 1, got %d", c.MaxLedger)
	}
	return nil
}
```

### Step 3a: Wire CommanderConfig into AlphaConfig — `internal/config/alpha.go`

Add the Commander field and wire it into Finalize/Merge:

```go
type AlphaConfig struct {
	ServiceConfig
	Jobs      JobsConfig      `json:"jobs"`
	Commander CommanderConfig `json:"commander"`
}
```

In `Finalize`, after the `Jobs` finalization:

```go
if err := c.Commander.Finalize(); err != nil {
	return fmt.Errorf("commander: %w", err)
}
```

In `Merge`, after `c.Jobs.Merge`:

```go
c.Commander.Merge(&overlay.Commander)
```

### Step 3b: Wire ResponderConfig into BetaConfig — `internal/config/beta.go`

Add the Responder field:

```go
type BetaConfig struct {
	ServiceConfig
	Zones     []string        `json:"zones"`
	Telemetry TelemetryConfig `json:"telemetry"`
	Runners   RunnersConfig   `json:"runners"`
	Responder ResponderConfig `json:"responder"`
}
```

In `Finalize`, after the `Runners` finalization:

```go
if err := c.Responder.Finalize(); err != nil {
	return fmt.Errorf("responder: %w", err)
}
```

In `Merge`, after `c.Runners.Merge`:

```go
c.Responder.Merge(&overlay.Responder)
```

### Step 4: Alpha Commander Domain — `internal/alpha/commander/commander.go`

New file:

```go
package commander

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/commands"
)

type HistoryEntry struct {
	Command  contracts.Command   `json:"command"`
	Response *contracts.Response `json:"response,omitempty"`
	Error    string              `json:"error,omitempty"`
	At       time.Time           `json:"at"`
}

type System interface {
	Issue(action, payload string) (*contracts.Response, error)
	History() []HistoryEntry
	Handler() *Handler
}

type commander struct {
	bus        bus.System
	source     string
	timeout    time.Duration
	maxHistory int
	history    []HistoryEntry
	mu         sync.Mutex
	logger     *slog.Logger
}

func New(
	b bus.System,
	source string,
	timeout time.Duration,
	maxHistory int,
	logger *slog.Logger,
) System {
	return &commander{
		bus:        b,
		source:     source,
		timeout:    timeout,
		maxHistory: maxHistory,
		history:    make([]HistoryEntry, 0, maxHistory),
		logger:     logger.With("domain", "commander"),
	}
}

func (c *commander) Issue(action, payload string) (*contracts.Response, error) {
	cmd := contracts.Command{
		ID:       uuid.New().String(),
		Action:   contracts.Action(action),
		Payload:  payload,
		IssuedAt: time.Now().Format(time.RFC3339),
	}

	subject := contracts.Subject(contracts.Action(action))

	sig, err := signal.New(c.source, subject, cmd)
	if err != nil {
		return nil, fmt.Errorf("create signal: %w", err)
	}

	data, err := signal.Encode(sig)
	if err != nil {
		return nil, fmt.Errorf("encode signal: %w", err)
	}

	msg, err := c.bus.Conn().Request(subject, data, c.timeout)
	if err != nil {
		entry := HistoryEntry{
			Command: cmd,
			Error:   err.Error(),
			At:      time.Now(),
		}
		c.appendHistory(entry)

		c.logger.Error("command timeout",
			"action", action,
			"command_id", cmd.ID,
			"error", err,
		)
		return nil, err
	}

	replySig, err := signal.Decode(msg.Data)
	if err != nil {
		return nil, fmt.Errorf("decode reply signal: %w", err)
	}

	var resp contracts.Response
	if err := json.Unmarshal(replySig.Data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	entry := HistoryEntry{
		Command:  cmd,
		Response: &resp,
		At:       time.Now(),
	}
	c.appendHistory(entry)

	c.logger.Info("command replied",
		"action", action,
		"command_id", cmd.ID,
		"status", resp.Status,
	)

	return &resp, nil
}

func (c *commander) History() []HistoryEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]HistoryEntry, len(c.history))
	copy(result, c.history)
	slices.Reverse(result)
	return result
}

func (c *commander) Handler() *Handler {
	return &Handler{
		commander: c,
		logger:    c.logger,
	}
}

func (c *commander) appendHistory(entry HistoryEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.history) >= c.maxHistory {
		c.history = c.history[1:]
	}
	c.history = append(c.history, entry)
}
```

### Step 5: Alpha Commander Handler — `internal/alpha/commander/handler.go`

New file:

```go
package commander

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

type IssueRequest struct {
	Action  string `json:"action"`
	Payload string `json:"payload,omitempty"`
}

type Handler struct {
	commander System
	logger    *slog.Logger
}

func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/commander",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/issue", Handler: h.HandleIssue},
			{Method: "GET", Pattern: "/history", Handler: h.HandleHistory},
		},
	}
}

func (h *Handler) HandleIssue(w http.ResponseWriter, r *http.Request) {
	var req IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handlers.RespondError(w, h.logger, http.StatusBadRequest, err)
		return
	}

	resp, err := h.commander.Issue(req.Action, req.Payload)
	if err != nil {
		if errors.Is(err, nats.ErrTimeout) {
			handlers.RespondError(w, h.logger, http.StatusGatewayTimeout, err)
			return
		}
		handlers.RespondError(w, h.logger, http.StatusInternalServerError, err)
		return
	}

	handlers.RespondJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.commander.History())
}
```

### Step 6: Beta Responder Domain — `internal/beta/responder/responder.go`

New file:

```go
package responder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/commands"
)

type LedgerEntry struct {
	Command  contracts.Command  `json:"command"`
	Response contracts.Response `json:"response"`
}

type System interface {
	Subscribe() error
	Ledger() []LedgerEntry
	Handler() *Handler
}

type responder struct {
	bus       bus.System
	maxLedger int
	ledger    []LedgerEntry
	mu        sync.RWMutex
	logger    *slog.Logger
}

func New(b bus.System, maxLedger int, logger *slog.Logger) System {
	return &responder{
		bus:       b,
		maxLedger: maxLedger,
		ledger:    make([]LedgerEntry, 0, maxLedger),
		logger:    logger.With("domain", "responder"),
	}
}

func (r *responder) Subscribe() error {
	return r.bus.Subscribe(contracts.SubjectWildcard, r.onCommand)
}

func (r *responder) Ledger() []LedgerEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]LedgerEntry, len(r.ledger))
	copy(result, r.ledger)
	return result
}

func (r *responder) Handler() *Handler {
	return &Handler{
		responder: r,
		logger:    r.logger,
	}
}

func (r *responder) onCommand(msg *nats.Msg) {
	sig, err := signal.Decode(msg.Data)
	if err != nil {
		r.logger.Error("failed to decode command signal", "error", err)
		return
	}

	var cmd contracts.Command
	if err := json.Unmarshal(sig.Data, &cmd); err != nil {
		r.logger.Error("failed to unmarshal command", "error", err)
		return
	}

	action := extractAction(msg.Subject)
	resp := r.dispatch(action, cmd)

	entry := LedgerEntry{
		Command:  cmd,
		Response: resp,
	}
	r.appendLedger(entry)

	replySig, err := signal.New("responder", msg.Subject, resp)
	if err != nil {
		r.logger.Error("failed to create reply signal", "error", err)
		return
	}

	replyData, err := signal.Encode(replySig)
	if err != nil {
		r.logger.Error("failed to encode reply signal", "error", err)
		return
	}

	if err := msg.Respond(replyData); err != nil {
		r.logger.Error("failed to respond", "error", err)
	}

	r.logger.Info("command handled",
		"action", action,
		"command_id", cmd.ID,
		"status", resp.Status,
	)
}

func (r *responder) dispatch(action string, cmd contracts.Command) contracts.Response {
	now := time.Now().Format(time.RFC3339)

	switch contracts.Action(action) {
	case contracts.ActionPing:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "pong",
			HandledAt: now,
		}
	case contracts.ActionFlush:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "flushed",
			HandledAt: now,
		}
	case contracts.ActionRotate:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "rotated",
			HandledAt: now,
		}
	case contracts.ActionNoop:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "noop",
			HandledAt: now,
		}
	default:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusError,
			Result:    fmt.Sprintf("unknown action: %s", action),
			HandledAt: now,
		}
	}
}

func (r *responder) appendLedger(entry LedgerEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.ledger) >= r.maxLedger {
		r.ledger = r.ledger[1:]
	}
	r.ledger = append(r.ledger, entry)
}

func extractAction(subject string) string {
	parts := strings.Split(subject, ".")
	return parts[len(parts)-1]
}
```

### Step 7: Beta Responder Handler — `internal/beta/responder/handler.go`

New file:

```go
package responder

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

type Handler struct {
	responder System
	logger    *slog.Logger
}

func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/responder",
		Routes: []routes.Route{
			{Method: "GET", Pattern: "/ledger", Handler: h.HandleLedger},
		},
	}
}

func (h *Handler) HandleLedger(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.responder.Ledger())
}
```

### Step 8: Wire Alpha

**`internal/alpha/runtime.go`** — add `CommandTimeout`:

```go
type Runtime struct {
	*infrastructure.Infrastructure
	ResponseTimeout   time.Duration
	JobInterval       time.Duration
	CommandTimeout    time.Duration
	CommandMaxHistory int
}
```

**`internal/alpha/domain.go`** — add Commander to Domain:

```go
import (
	"github.com/JaimeStill/signal-lab/internal/alpha/commander"
	"github.com/JaimeStill/signal-lab/internal/alpha/jobs"
	"github.com/JaimeStill/signal-lab/internal/alpha/monitoring"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
)

type Domain struct {
	Discovery discovery.System
	Monitor   monitoring.System
	Jobs      jobs.System
	Commander commander.System
}

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
		Commander: commander.New(
			rt.Bus,
			rt.Info.Name,
			rt.CommandTimeout,
			rt.CommandMaxHistory,
			rt.Logger,
		),
	}
}
```

**`internal/alpha/routes.go`** — register commander routes:

```go
func registerRoutes(mux *http.ServeMux, domain *Domain) {
	discoveryHandler := domain.Discovery.Handler()
	monitorHandler := domain.Monitor.Handler()
	jobsHandler := domain.Jobs.Handler()
	commanderHandler := domain.Commander.Handler()

	routes.Register(
		mux,
		discoveryHandler.Routes(),
		monitorHandler.Routes(),
		jobsHandler.Routes(),
		commanderHandler.Routes(),
	)
}
```

**`internal/alpha/api.go`** — add CommandTimeout to Runtime:

```go
rt := &Runtime{
	Infrastructure:    infra,
	ResponseTimeout:   cfg.Bus.ResponseTimeoutDuration(),
	JobInterval:       cfg.Alpha.Jobs.IntervalDuration(),
	CommandTimeout:    cfg.Alpha.Commander.TimeoutDuration(),
	CommandMaxHistory: cfg.Alpha.Commander.MaxHistory,
}
```

No `Subscribe()` call for commander — it's a requester, not a listener.

### Step 9: Wire Beta

**`internal/beta/runtime.go`** — add `ResponderMaxLedger`:

```go
type Runtime struct {
	*infrastructure.Infrastructure
	ResponseTimeout    time.Duration
	TelemetryInterval  time.Duration
	TelemetryTypes     []string
	Zones              []string
	RunnerCount        int
	ResponderMaxLedger int
}
```

**`internal/beta/domain.go`** — add Responder to Domain:

```go
import (
	"github.com/JaimeStill/signal-lab/internal/beta/responder"
	"github.com/JaimeStill/signal-lab/internal/beta/runners"
	"github.com/JaimeStill/signal-lab/internal/beta/telemetry"
	"github.com/JaimeStill/signal-lab/pkg/discovery"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

type Domain struct {
	Discovery discovery.System
	Telemetry telemetry.System
	Runners   runners.System
	Responder responder.System
}

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

	resp := responder.New(rt.Bus, rt.ResponderMaxLedger, rt.Logger)

	return &Domain{
		Discovery: disc,
		Telemetry: tel,
		Runners:   run,
		Responder: resp,
	}
}
```

**`internal/beta/routes.go`** — register responder routes:

```go
func registerRoutes(mux *http.ServeMux, domain *Domain) {
	discoveryHandler := domain.Discovery.Handler()
	telemetryHandler := domain.Telemetry.Handler()
	runnersHandler := domain.Runners.Handler()
	responderHandler := domain.Responder.Handler()

	routes.Register(
		mux,
		discoveryHandler.Routes(),
		telemetryHandler.Routes(),
		runnersHandler.Routes(),
		responderHandler.Routes(),
	)
}
```

**`internal/beta/api.go`** — add `ResponderMaxLedger` to Runtime construction and subscribe responder at startup:

In the Runtime literal, add:

```go
ResponderMaxLedger: cfg.Beta.Responder.MaxLedger,
```

After the existing `domain.Runners.Subscribe()` call, add:

```go
if err := domain.Responder.Subscribe(); err != nil {
	return nil, err
}
```

## Validation Criteria

- [ ] `mise run vet` clean
- [ ] `curl -X POST localhost:3000/api/commander/issue -d '{"action":"ping"}'` returns `{"command_id":"...","status":"ok","result":"pong","handled_at":"..."}`
- [ ] `curl localhost:3001/api/responder/ledger` shows the ping command entry
- [ ] `curl localhost:3000/api/commander/history` shows the issued command with its reply
- [ ] `curl -X POST localhost:3000/api/commander/issue -d '{"action":"explode"}'` returns a response with `"status":"error"` and `"result":"unknown action: explode"`
- [ ] Stop beta, issue a command from alpha → HTTP 504 timeout
- [ ] Restart beta, issue command → successful round-trip resumes
- [ ] Phase 1–3 endpoints unaffected
