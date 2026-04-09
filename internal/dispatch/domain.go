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
	Alert     alerting.System
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
		Monitor: monitoring.New(b, logger),
		Alert:   alerting.New(b, logger),
	}
}
