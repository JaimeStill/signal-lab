package alpha

import (
	"log/slog"

	"github.com/JaimeStill/signal-lab/internal/alpha/jobs"
	"github.com/JaimeStill/signal-lab/internal/alpha/monitoring"
	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
)

// Domain assembles all alpha domain systems.
type Domain struct {
	Discovery discovery.System
	Monitor   monitoring.System
	Jobs      jobs.System
}

// NewDomain creates the alpha domain systems.
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
		Jobs: jobs.New(
			b,
			info.Name,
			cfg.Alpha.Jobs.IntervalDuration(),
			logger,
		),
	}
}
