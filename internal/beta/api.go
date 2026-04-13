package beta

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/infrastructure"
	"github.com/JaimeStill/signal-lab/pkg/module"
)

// NewModule creates the beta API module with discovery, telemetry, runners, and
// responder domains and registers their lifecycle subscriptions on the bus.
func NewModule(
	infra *infrastructure.Infrastructure,
	cfg *config.Config,
) (*module.Module, error) {
	rt := &Runtime{
		Infrastructure:     infra,
		ResponseTimeout:    cfg.Bus.ResponseTimeoutDuration(),
		TelemetryInterval:  cfg.Beta.Telemetry.IntervalDuration(),
		TelemetryTypes:     cfg.Beta.Telemetry.Types,
		Zones:              cfg.Beta.Zones,
		RunnerCount:        cfg.Beta.Runners.Number(),
		ResponderMaxLedger: cfg.Beta.Responder.MaxLedger,
	}

	domain := NewDomain(rt)

	if err := domain.Discovery.Subscribe(); err != nil {
		return nil, err
	}

	if err := domain.Runners.Subscribe(); err != nil {
		return nil, err
	}

	if err := domain.Responder.Subscribe(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	registerRoutes(mux, domain)

	return module.New("/api", mux), nil
}
