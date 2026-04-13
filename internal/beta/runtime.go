package beta

import (
	"time"

	"github.com/JaimeStill/signal-lab/internal/infrastructure"
)

// Runtime extends Infrastructure with beta-specific configuration values
// consumed by domain system constructors. Built in NewModule, passed to NewDomain.
type Runtime struct {
	*infrastructure.Infrastructure
	ResponseTimeout    time.Duration
	TelemetryInterval  time.Duration
	TelemetryTypes     []string
	Zones              []string
	RunnerCount        int
	ResponderMaxLedger int
}
