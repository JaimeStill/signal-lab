package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/telemetry"
)

type Reading = contracts.Reading

// Status reports the current state of the telemetry publisher.
type Status struct {
	Running  bool     `json:"running"`
	Interval string   `json:"interval"`
	Types    []string `json:"types"`
	Zones    []string `json:"zones"`
}

// System manages telemetry publishing over the bus.
type System interface {
	// Start begins publishing simulated readings at the configured interval.
	Start() error
	// Stop stops the publisher.
	Stop() error
	// Status returns the current publisher state.
	Status() Status
	// Handler creates the HTTP handler for telemetry endpoints.
	Handler() *Handler
}

type telemetry struct {
	bus      bus.System
	source   string
	interval time.Duration
	types    []string
	zones    []string
	running  bool
	cancel   context.CancelFunc
	mu       sync.Mutex
	logger   *slog.Logger
}

// New creates a telemetry system.
func New(
	b bus.System,
	source string,
	interval time.Duration,
	types []string,
	zones []string,
	logger *slog.Logger,
) System {
	return &telemetry{
		bus:      b,
		source:   source,
		interval: interval,
		types:    types,
		zones:    zones,
		logger:   logger.With("domain", "telemetry"),
	}
}

// Start begins publishing simulated readings at the configured interval.
func (t *telemetry) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return fmt.Errorf("publisher already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.running = true

	go t.publish(ctx)

	t.logger.Info("publisher started", "interval", t.interval)
	return nil
}

// Stop stops the publisher.
func (t *telemetry) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return fmt.Errorf("publisher not running")
	}

	t.cancel()
	t.running = false

	t.logger.Info("publisher stopped")
	return nil
}

// Status returns the current publisher state.
func (t *telemetry) Status() Status {
	t.mu.Lock()
	defer t.mu.Unlock()

	return Status{
		Running:  t.running,
		Interval: t.interval.String(),
		Types:    t.types,
		Zones:    t.zones,
	}
}

// Handler creates the HTTP handler for telemetry endpoints.
func (t *telemetry) Handler() *Handler {
	return &Handler{
		telemetry: t,
		logger:    t.logger,
	}
}

func (t *telemetry) publish(ctx context.Context) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, typ := range t.types {
				for _, zone := range t.zones {
					reading := Reading{
						Type:  typ,
						Zone:  zone,
						Value: generateValue(typ),
						Unit:  unitFor(typ),
					}

					subject := fmt.Sprintf("%s.%s.%s", contracts.SubjectPrefix, typ, zone)

					sig, err := signal.New(t.source, subject, reading)
					if err != nil {
						t.logger.Error("failed to create signal", "error", err)
						continue
					}

					data, err := signal.Encode(sig)
					if err != nil {
						t.logger.Error("failed to encode signal", "error", err)
						continue
					}

					if err := t.bus.Conn().Publish(subject, data); err != nil {
						t.logger.Error(
							"failed to publish reading",
							"subject", subject,
							"error", err,
						)
					}
				}
			}
		}
	}
}

func generateValue(typ string) float64 {
	switch typ {
	case "temp":
		return 18.0 + rand.Float64()*12.0 // 18-30°C
	case "humidity":
		return 30.0 + rand.Float64()*50.0 // 30-80%
	case "pressure":
		return 980.0 + rand.Float64()*40.0 // 980-1020 hPa
	default:
		return rand.Float64() * 100.0
	}
}

func unitFor(typ string) string {
	switch typ {
	case "temp":
		return "°C"
	case "humidity":
		return "%"
	case "pressure":
		return "hPa"
	default:
		return ""
	}
}
