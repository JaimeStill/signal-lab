package beta

import (
	"github.com/JaimeStill/signal-lab/internal/beta/responder"
	"github.com/JaimeStill/signal-lab/internal/beta/runners"
	"github.com/JaimeStill/signal-lab/internal/beta/telemetry"
	"github.com/JaimeStill/signal-lab/pkg/discovery"

	cmdcontracts "github.com/JaimeStill/signal-lab/pkg/contracts/commands"
	jobcontracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

// Domain assembles all beta domain systems.
type Domain struct {
	Discovery discovery.System
	Telemetry telemetry.System
	Runners   runners.System
	Responder responder.System
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
		jobcontracts.SubjectWildcard,
		runners.QueueGroup,
		rt.Logger,
	)

	resp := responder.New(
		rt.Bus,
		cmdcontracts.SubjectWildcard,
		rt.ResponderMaxLedger,
		rt.Logger,
	)

	return &Domain{
		Discovery: disc,
		Telemetry: tel,
		Runners:   run,
		Responder: resp,
	}
}
