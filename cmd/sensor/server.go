package main

import (
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

// Server composes all sensor subsystems.
type Server struct {
	cfg    *config.Config
	lc     *lifecycle.Coordinator
	conn   *nats.Conn
	http   *httpServer
	sub    *nats.Subscription
	info   discovery.ServiceInfo
	logger *slog.Logger
}

// NewServer creates the sensor server.
func NewServer(cfg *config.Config) *Server {
	logger := slog.With("service", cfg.Sensor.Name)

	return &Server{
		cfg:    cfg,
		lc:     lifecycle.New(),
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
	conn, err := bus.Connect(&s.cfg.Bus, s.logger)
	if err != nil {
		return err
	}
	s.conn = conn
	bus.RegisterLifecycle(conn, s.lc, s.cfg.ShutdownTimeoutDuration())

	handler, sub, err := buildHandler(
		s.lc, s.conn, s.info,
		s.cfg.Bus.ResponseTimeoutDuration(),
		s.logger,
	)
	if err != nil {
		return err
	}
	s.sub = sub

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
