package monitoring

import (
	"fmt"
	"log/slog"
	"maps"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"
)

const telemetrySubject = "signal.telemetry.>"

// Status reports the current state of telemetry monitoring.
type Status struct {
	Subscribed bool             `json:"subscribed"`
	Counts     map[string]int64 `json:"counts"`
}

// System manages telemetry monitoring over the bus.
type System interface {
	// Subscribe registers the telemetry wildcard subscription with the bus.
	Subscribe() error
	// Status returns subscription state and message counts.
	Status() Status
	// Handler creates the HTTP handler for monitoring endpoints.
	Handler() *Handler
}

type monitor struct {
	bus        bus.System
	counts     map[string]int64
	mu         sync.RWMutex
	subscribed bool
	signals    chan signal.Signal
	logger     *slog.Logger
}

// New creates a monitoring system.
func New(b bus.System, logger *slog.Logger) System {
	return &monitor{
		bus:     b,
		counts:  make(map[string]int64),
		signals: make(chan signal.Signal, 64),
		logger:  logger.With("domain", "monitoring"),
	}
}

// Subscribe registers the telemetry wildcard subscription with the bus.
func (m *monitor) Subscribe() error {
	if err := m.bus.Subscribe(telemetrySubject, m.onTelemetry); err != nil {
		return fmt.Errorf("subscribe telemetry: %w", err)
	}

	m.subscribed = true
	m.logger.Info("subscribed to telemetry", "subject", telemetrySubject)
	return nil
}

// Status return subscription state and message counts.
func (m *monitor) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[string]int64, len(m.counts))
	maps.Copy(counts, m.counts)

	return Status{
		Subscribed: m.subscribed,
		Counts:     counts,
	}
}

// Handler creates the HTTP handler for monitoring endpoints.
func (m *monitor) Handler() *Handler {
	return &Handler{
		monitor: m,
		signals: m.signals,
		logger:  m.logger,
	}
}

func (m *monitor) onTelemetry(msg *nats.Msg) {
	sig, err := signal.Decode(msg.Data)
	if err != nil {
		m.logger.Error("failed to decode telemetry", "error", err)
		return
	}

	m.mu.Lock()
	m.counts[msg.Subject]++
	m.mu.Unlock()

	select {
	case m.signals <- sig:
	default:
		m.logger.Warn("signal channel full, dropping message", "subject", msg.Subject)
	}
}
