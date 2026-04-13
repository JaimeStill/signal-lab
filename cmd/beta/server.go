package main

import (
	"log/slog"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/infrastructure"
)

// Server composes all beta subsystems.
type Server struct {
	cfg   *config.Config
	infra *infrastructure.Infrastructure
	http  *httpServer
}

// NewServer creates the beta server.
func NewServer(cfg *config.Config) *Server {
	logger := slog.With("service", cfg.Beta.Name)

	return &Server{
		cfg:   cfg,
		infra: infrastructure.New(cfg, &cfg.Beta.ServiceConfig, logger),
	}
}

// Start connects to NATS, builds the HTTP handler, and starts serving.
func (s *Server) Start() error {
	if err := s.infra.Start(); err != nil {
		return err
	}

	handler, err := buildHandler(s.infra, s.cfg)
	if err != nil {
		return err
	}

	s.http = newHTTPServer(
		s.cfg.Beta.Addr(),
		handler,
		s.infra.ShutdownTimeout,
		s.infra.Logger,
	)
	s.http.Start(s.infra.Lifecycle)

	go func() {
		s.infra.Lifecycle.WaitForStartup()
		s.infra.Logger.Info("all subsystems ready")
	}()

	return nil
}

// Shutdown initiates graceful shutdown.
func (s *Server) Shutdown() error {
	s.infra.Logger.Info("initiating shutdown")
	return s.infra.Lifecycle.Shutdown(s.infra.ShutdownTimeout)
}
