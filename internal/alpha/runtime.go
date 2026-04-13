package alpha

import (
	"time"

	"github.com/JaimeStill/signal-lab/internal/infrastructure"
)

// Runtime extends Infrastructure with alpha-specific configuration values
// consumed by domain system constructors. Built in NewModule, passed to NewDomain.
type Runtime struct {
	*infrastructure.Infrastructure
	ResponseTimeout time.Duration
	JobInterval     time.Duration
}
