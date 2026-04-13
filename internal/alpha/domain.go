package alpha

import (
	"github.com/JaimeStill/signal-lab/internal/alpha/jobs"
	"github.com/JaimeStill/signal-lab/internal/alpha/monitoring"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
)

// Domain assembles all alpha domain systems.
type Domain struct {
	Discovery discovery.System
	Monitor   monitoring.System
	Jobs      jobs.System
}

// NewDomain creates the alpha domain systems.
func NewDomain(rt *Runtime) *Domain {
	return &Domain{
		Discovery: discovery.New(
			rt.Bus, rt.Info,
			rt.ResponseTimeout,
			rt.Logger,
		),
		Monitor: monitoring.New(rt.Bus, rt.Logger),
		Jobs: jobs.New(
			rt.Bus,
			rt.Info.Name,
			rt.JobInterval,
			rt.Logger,
		),
	}
}
