package beta

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	discoveryHandler := domain.Discovery.Handler()
	telemetryHandler := domain.Telemetry.Handler()
	runnersHandler := domain.Runners.Handler()

	routes.Register(
		mux,
		discoveryHandler.Routes(),
		telemetryHandler.Routes(),
		runnersHandler.Routes(),
	)
}
