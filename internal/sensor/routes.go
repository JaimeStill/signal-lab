package sensor

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	dh := domain.Discovery.Handler()
	th := domain.Telemetry.Handler()

	routes.Register(
		mux,
		dh.Routes(),
		th.Routes(),
	)
}
