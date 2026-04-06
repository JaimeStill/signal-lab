package bus

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

// Config provides NATS connection parameters
type Config interface {
	URL() string
	Options(*slog.Logger) []nats.Option
}

// Connect establishes a NATS connection using the provided config.
func Connect(cfg Config, logger *slog.Logger) (*nats.Conn, error) {
	log := logger.With("system", "bus")
	opts := cfg.Options(log)

	conn, err := nats.Connect(cfg.URL(), opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	log.Info("connected to nats", "ur", cfg.URL())
	return conn, nil
}

// Drain gracefully drains the NATS connection within the given timeout.
func Drain(conn *nats.Conn, timeout time.Duration) error {
	closed := make(chan struct{})
	conn.SetClosedHandler(func(_ *nats.Conn) {
		close(closed)
	})

	if err := conn.Drain(); err != nil {
		return fmt.Errorf("nats drain: %w", err)
	}

	select {
	case <-closed:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("nats drain timeout after %v", timeout)
	}
}

// RegisterLifecycle wires NATS drain into the lifecycle shutdown hooks.
func RegisterLifecycle(conn *nats.Conn, lc *lifecycle.Coordinator, timeout time.Duration) {
	logger := slog.With("system", "bus")

	lc.RegisterChecker(NewChecker(conn))

	lc.OnShutdown(func() {
		<-lc.Context().Done()
		logger.Info("draining nats connection")

		if err := Drain(conn, timeout); err != nil {
			logger.Error("nats drain failed", "error", err)
		} else {
			logger.Info("nats drain complete")
		}
	})
}
