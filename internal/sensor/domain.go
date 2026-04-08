package sensor

import (
	"log/slog"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/sensor/telemetry"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
)

// Domain assembles all sensor domain systems.
type Domain struct {
	Discovery discovery.System
	Telemetry telemetry.System
}

// NewDomain creates the sensor domain systems.
func NewDomain(
	b bus.System,
	info discovery.ServiceInfo,
	cfg *config.Config,
	logger *slog.Logger,
) *Domain {
	telCfg := &cfg.Sensor.Telemetry

	disc := discovery.New(
		b, info,
		cfg.Bus.ResponseTimeoutDuration(),
		logger,
	)

	tel := telemetry.New(
		b,
		info.Name,
		telCfg.IntervalDuration(),
		telCfg.Types,
		telCfg.Zones,
		logger,
	)

	return &Domain{
		Discovery: disc,
		Telemetry: tel,
	}
}
