package dispatch

import (
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/routes"
)

func registerRoutes(mux *http.ServeMux, domain *Domain) {
	dh := domain.Discovery.Handler()
	mh := domain.Monitor.Handler()

	routes.Register(mux, dh.Routes(), mh.Routes())
}
