package main

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

// Server composes all sensor subsystems.
type Server struct {
	cfg    *config.Config
	lc     *lifecycle.Coordinator
	bus    bus.System
	http   *httpServer
	info   discovery.ServiceInfo
	logger *slog.Logger
}

// NewServer creates the sensor server.
func NewServer(cfg *config.Config) *Server {
	logger := slog.With("service", cfg.Sensor.Name)

	return &Server{
		cfg:    cfg,
		lc:     lifecycle.New(),
		bus:    bus.New(&cfg.Bus, cfg.ShutdownTimeoutDuration(), logger),
		logger: logger,
		info: discovery.ServiceInfo{
			ID:          uuid.New().String(),
			Name:        cfg.Sensor.Name,
			Endpoint:    fmt.Sprintf("http://%s", cfg.Sensor.Addr()),
			Health:      "ok",
			Description: cfg.Sensor.Description,
		},
	}
}

// Start connects to NATS, builds the HTTP handler, and starts serving.
func (s *Server) Start() error {
	if err := s.bus.Start(s.lc); err != nil {
		return err
	}

	handler, err := buildHandler(s.lc, s.bus, s.info, s.cfg, s.logger)
	if err != nil {
		return err
	}

	s.http = newHTTPServer(
		s.cfg.Sensor.Addr(),
		handler,
		s.cfg.ShutdownTimeoutDuration(),
		s.logger,
	)
	s.http.Start(s.lc)

	go func() {
		s.lc.WaitForStartup()
		s.logger.Info("all subsystems ready")
	}()

	return nil
}

// Shutdown initiataes graceful shutdown.
func (s *Server) Shutdown() error {
	s.logger.Info("initiating shutdown")
	return s.lc.Shutdown(s.cfg.ShutdownTimeoutDuration())
}
