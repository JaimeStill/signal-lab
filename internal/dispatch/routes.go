package dispatch

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	discoveryHandler := domain.Discovery.Handler()
	monitorHandler := domain.Monitor.Handler()
	alertHandler := domain.Alert.Handler()

	routes.Register(
		mux,
		discoveryHandler.Routes(),
		monitorHandler.Routes(),
		alertHandler.Routes(),
	)
}
