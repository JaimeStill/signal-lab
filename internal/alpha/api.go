package alpha

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/infrastructure"
	"github.com/JaimeStill/signal-lab/pkg/module"
)

// NewModule creates the alpha API module with discovery, monitoring, jobs, and
// commander domains and registers their lifecycle subscriptions on the bus.
func NewModule(
	infra *infrastructure.Infrastructure,
	cfg *config.Config,
) (*module.Module, error) {
	rt := &Runtime{
		Infrastructure:    infra,
		ResponseTimeout:   cfg.Bus.ResponseTimeoutDuration(),
		JobInterval:       cfg.Alpha.Jobs.IntervalDuration(),
		CommandTimeout:    cfg.Alpha.Commander.TimeoutDuration(),
		CommandMaxHistory: cfg.Alpha.Commander.MaxHistory,
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
