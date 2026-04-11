package beta

import (
	"log/slog"

	"github.com/JaimeStill/signal-lab/internal/beta/runners"
	"github.com/JaimeStill/signal-lab/internal/beta/telemetry"
	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

// Domain assembles all beta domain systems.
type Domain struct {
	Discovery discovery.System
	Telemetry telemetry.System
	Runners   runners.System
}

// NewDomain creates the beta domain systems.
func NewDomain(
	b bus.System,
	info discovery.ServiceInfo,
	cfg *config.Config,
	logger *slog.Logger,
) *Domain {
	disc := discovery.New(
		b, info,
		cfg.Bus.ResponseTimeoutDuration(),
		logger,
	)

	telemetryConfig := &cfg.Beta.Telemetry
	tel := telemetry.New(
		b,
		info.Name,
		telemetryConfig.IntervalDuration(),
		telemetryConfig.Types,
		cfg.Beta.Zones,
		logger,
	)

	run := runners.New(
		b,
		cfg.Beta.Runners.Number(),
		contracts.SubjectWildcard,
		runners.QueueGroup,
		logger,
	)

	return &Domain{
		Discovery: disc,
		Telemetry: tel,
		Runners:   run,
	}
}
