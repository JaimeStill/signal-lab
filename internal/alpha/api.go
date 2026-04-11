package alpha

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/module"
)

// NewModule creates the alpha API module with discovery, monitoring, and jobs
// domains and registers their lifecycle subscriptions on the bus.
func NewModule(
	b bus.System,
	info discovery.ServiceInfo,
	cfg *config.Config,
	logger *slog.Logger,
) (*module.Module, error) {
	domain := NewDomain(b, info, cfg, logger)

	if err := domain.Discovery.Subscribe(); err != nil {
		return nil, err
	}

	if err := domain.Monitor.Subscribe(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	registerRoutes(mux, domain)

	return module.New("/api", mux), nil
}
