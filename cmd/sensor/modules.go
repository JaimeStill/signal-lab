package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/sensor"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
	"github.com/JaimeStill/signal-lab/pkg/middleware"
	"github.com/JaimeStill/signal-lab/pkg/module"
)

func buildHandler(
	lc *lifecycle.Coordinator,
	conn *nats.Conn,
	info discovery.ServiceInfo,
	timeout time.Duration,
	logger *slog.Logger,
) (http.Handler, *nats.Subscription, error) {
	router := buildRouter(lc)

	mod, sub, err := sensor.NewModule(conn, info, timeout, logger)
	if err != nil {
		return nil, nil, err
	}
	router.Mount(mod)

	mw := middleware.New()
	mw.Use(middleware.Logger(logger))

	return mw.Apply(router), sub, nil
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
