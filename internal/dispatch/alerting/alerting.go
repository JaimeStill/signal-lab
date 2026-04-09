package alerting

import (
	"fmt"
	"log/slog"
	"maps"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/alerts"
)

const queueGroup = "dispatch-workers"

// Status reports the current state of alert monitoring.
type Status struct {
	Subscribed bool             `json:"subscribed"`
	Counts     map[string]int64 `json:"counts"`
}

// System manages alert monitoring over the bus via queue group subscription.
type System interface {
	// Subscribe registers the alerts queue group subscription with the bus.
	Subscribe() error
	// Status returns subscription state and message counts.
	Status() Status
	// Handler creates the HTTP handler for alerting endpoints.
	Handler() *Handler
}

type alert struct {
	bus        bus.System
	counts     map[string]int64
	mu         sync.RWMutex
	subscribed bool
	signals    chan signal.Signal
	logger     *slog.Logger
}

// New creates an alert system.
func New(b bus.System, logger *slog.Logger) System {
	return &alert{
		bus:     b,
		counts:  make(map[string]int64),
		signals: make(chan signal.Signal, 64),
		logger:  logger.With("domain", "alerting"),
	}
}

// Subscribe registers the alerts queue group subscription with the bus.
func (a *alert) Subscribe() error {
	if err := a.bus.QueueSubscribe(
		contracts.SubjectWildcard,
		queueGroup,
		a.onAlert,
	); err != nil {
		return fmt.Errorf("queue subscribe alerts: %w", err)
	}

	a.subscribed = true

	a.logger.Info(
		"subscribed to alerts",
		"subject",
		contracts.SubjectWildcard,
		"queue",
		queueGroup,
	)

	return nil
}

// Status returns subscription state and message counts.
func (a *alert) Status() Status {
	a.mu.RLock()
	defer a.mu.RUnlock()

	counts := make(map[string]int64, len(a.counts))
	maps.Copy(counts, a.counts)

	return Status{
		Subscribed: a.subscribed,
		Counts:     counts,
	}
}

// Handler creates the HTTP handler for alerting endpoints.
func (a *alert) Handler() *Handler {
	return &Handler{
		alert:   a,
		signals: a.signals,
		logger:  a.logger,
	}
}

func (a *alert) onAlert(msg *nats.Msg) {
	sig, err := signal.Decode(msg.Data)
	if err != nil {
		a.logger.Error("failed to decode alert", "error", err)
		return
	}

	// Populate signal metadata from NATS headers
	if len(msg.Header) > 0 {
		sig.Metadata = make(map[string]string)
		for _, key := range []string{
			contracts.HeaderPriority,
			contracts.HeaderSource,
			contracts.HeaderTraceID,
		} {
			if v := msg.Header.Get(key); v != "" {
				sig.Metadata[key] = v
			}
		}
	}

	// Priority-based logging
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

	a.mu.Lock()
	a.counts[msg.Subject]++
	a.mu.Unlock()

	select {
	case a.signals <- sig:
	default:
		a.logger.Warn("signal channel full, dropping alert", "subject", msg.Subject)
	}
}
