package beta

import (
	"github.com/JaimeStill/signal-lab/internal/beta/runners"
	"github.com/JaimeStill/signal-lab/internal/beta/telemetry"
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
func NewDomain(rt *Runtime) *Domain {
	disc := discovery.New(
		rt.Bus, rt.Info,
		rt.ResponseTimeout,
		rt.Logger,
	)

	tel := telemetry.New(
		rt.Bus,
		rt.Info.Name,
		rt.TelemetryInterval,
		rt.TelemetryTypes,
		rt.Zones,
		rt.Logger,
	)

	run := runners.New(
		rt.Bus,
		rt.RunnerCount,
		contracts.SubjectWildcard,
		runners.QueueGroup,
		rt.Logger,
	)

	return &Domain{
		Discovery: disc,
		Telemetry: tel,
		Runners:   run,
	}
}
