package alerts

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

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/alerts"
)

type AlertPriority = contracts.AlertPriority
type Alert = contracts.Alert

const (
	PriorityLow      = contracts.PriorityLow
	PriorityNormal   = contracts.PriorityNormal
	PriorityHigh     = contracts.PriorityHigh
	PriorityCritical = contracts.PriorityCritical
)

// Status reports the current state of the alert publisher.
type Status struct {
	Running  bool     `json:"running"`
	Interval string   `json:"interval"`
	Zones    []string `json:"zones"`
}

// System manages alert publishing over the bus.
type System interface {
	// Start begins publishing simulated alerts at the configured interval.
	Start() error
	// Stop stops the publisher.
	Stop() error
	// Status returns the current publisher state.
	Status() Status
	// Handler creates the HTTP handler for alert endpoints.
	Handler() *Handler
}

type alerts struct {
	bus      bus.System
	source   string
	interval time.Duration
	zones    []string
	running  bool
	cancel   context.CancelFunc
	mu       sync.Mutex
	logger   *slog.Logger
}

// New creates an alerts system
func New(
	b bus.System,
	source string,
	interval time.Duration,
	zones []string,
	logger *slog.Logger,
) System {
	return &alerts{
		bus:      b,
		source:   source,
		interval: interval,
		zones:    zones,
		logger:   logger.With("domain", "alerts"),
	}
}

// Start begins publishing simulated alerts at the configured interval.
func (a *alerts) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("publisher already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.running = true

	go a.publish(ctx)

	a.logger.Info("publisher started", "interval", a.interval)
	return nil
}

// Stop stops the publisher.
func (a *alerts) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return fmt.Errorf("publisher not running")
	}

	a.cancel()
	a.running = false

	a.logger.Info("publisher stopped")
	return nil
}

// Status returns the current publisher state.
func (a *alerts) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()

	return Status{
		Running:  a.running,
		Interval: a.interval.String(),
		Zones:    a.zones,
	}
}

// Handler creates the HTTP handler for alert endpoints.
func (a *alerts) Handler() *Handler {
	return &Handler{
		alerts: a,
		logger: a.logger,
	}
}

func (a *alerts) publish(ctx context.Context) {
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, zone := range a.zones {
				priority := randomPriority()

				alert := Alert{
					Severity: priority,
					Message:  alertMessage(priority, zone),
					Zone:     zone,
				}

				subject := fmt.Sprintf("%s.%s", contracts.SubjectPrefix, priority)

				sig, err := signal.New(a.source, subject, alert)
				if err != nil {
					a.logger.Error("failed to create signal", "error", err)
					continue
				}

				data, err := signal.Encode(sig)
				if err != nil {
					a.logger.Error("failed to encode signal", "error", err)
					continue
				}

				msg := &nats.Msg{
					Subject: subject,
					Data:    data,
					Header:  nats.Header{},
				}
				msg.Header.Set(contracts.HeaderPriority, string(priority))
				msg.Header.Set(contracts.HeaderSource, a.source)
				msg.Header.Set(contracts.HeaderTraceID, sig.ID)

				if err := a.bus.Conn().PublishMsg(msg); err != nil {
					a.logger.Error(
						"failed to publish alert",
						"subject", subject,
						"error", err,
					)
				}
			}
		}
	}
}

// randomPriority returns a weighted random priority.
// Distribution: ~70% normal, ~`5% low, ~`0% high, ~5% critical.
func randomPriority() AlertPriority {
	n := rand.IntN(100)
	switch {
	case n < 5:
		return PriorityCritical
	case n < 15:
		return PriorityHigh
	case n < 30:
		return PriorityLow
	default:
		return PriorityNormal
	}
}

func alertMessage(priority AlertPriority, zone string) string {
	switch priority {
	case PriorityCritical:
		return fmt.Sprintf("cirtical condition detected in %s", zone)
	case PriorityHigh:
		return fmt.Sprintf("elevated readings in %s", zone)
	case PriorityLow:
		return fmt.Sprintf("minor fluctuation in %s", zone)
	default:
		return fmt.Sprintf("routing check in %s", zone)
	}
}
