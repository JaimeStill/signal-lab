package main

import (
	"encoding/json"
	"net/http"

	"github.com/JaimeStill/signal-lab/internal/beta"
	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/infrastructure"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
	"github.com/JaimeStill/signal-lab/pkg/middleware"
	"github.com/JaimeStill/signal-lab/pkg/module"
)

func buildHandler(
	infra *infrastructure.Infrastructure,
	cfg *config.Config,
) (http.Handler, error) {
	router := buildRouter(infra.Lifecycle)

	mod, err := beta.NewModule(infra, cfg)
	if err != nil {
		return nil, err
	}
	router.Mount(mod)

	mw := middleware.New()
	mw.Use(middleware.Logger(infra.Logger))

	return mw.Apply(router), nil
}

func buildRouter(lc *lifecycle.Coordinator) *module.Router {
	router := module.NewRouter()

	router.HandleNative("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	router.HandleNative("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !lc.Ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "not ready"})
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	})

	return router
}
