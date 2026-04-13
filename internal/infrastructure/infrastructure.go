package infrastructure

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

// Infrastructure holds the core subsystems shared by all service modules:
// lifecycle coordination, structured logging, NATS bus, and service identity.
// It is constructed once at server startup and threaded through the wiring layer.
type Infrastructure struct {
	Lifecycle       *lifecycle.Coordinator
	Logger          *slog.Logger
	Bus             bus.System
	Info            discovery.ServiceInfo
	ShutdownTimeout time.Duration
}

// New creates an Infrastructure from the application config and a per-service
// ServiceConfig pointer. Config is consumed during construction to derive the
// bus, service identity, and shutdown timeout; it is not retained as a field.
func New(
	cfg *config.Config,
	svc *config.ServiceConfig,
	logger *slog.Logger,
) *Infrastructure {
	return &Infrastructure{
		Lifecycle:       lifecycle.New(),
		Logger:          logger,
		Bus:             bus.New(&cfg.Bus, cfg.ShutdownTimeoutDuration(), logger),
		ShutdownTimeout: cfg.ShutdownTimeoutDuration(),
		Info: discovery.ServiceInfo{
			ID:          uuid.New().String(),
			Name:        svc.Name,
			Endpoint:    fmt.Sprintf("http://%s", svc.Addr()),
			Description: svc.Description,
		},
	}
}

// Start connects the bus to NATS and registers lifecycle hooks.
// Call this after New and before building the HTTP handler.
func (infra *Infrastructure) Start() error {
	return infra.Bus.Start(infra.Lifecycle)
}
