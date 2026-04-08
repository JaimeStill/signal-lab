package bus

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

// System manages the NATS connection, subscriptions, and lifecycle coordination
type System interface {
	// Conn returns the underlying NATS connection for publishing and ephemeral operations.
	Conn() *nats.Conn
	// Subscribe creates a NATS subscription and tracks it by subject.
	Subscribe(subject string, handler nats.MsgHandler) error
	// Unsubscribe drains and removes a tracked subscription by subject.
	Unsubscribe(subject string) error
	// Start connects to NATS and registers lifecycle hooks.
	Start(lc *lifecycle.Coordinator) error
}

type bus struct {
	cfg     Config
	conn    *nats.Conn
	subs    map[string]*nats.Subscription
	mu      sync.Mutex
	logger  *slog.Logger
	timeout time.Duration
}

// New creates a bus system. No connection is established until Start is called.
func New(cfg Config, timeout time.Duration, logger *slog.Logger) System {
	return &bus{
		cfg:     cfg,
		subs:    make(map[string]*nats.Subscription),
		logger:  logger.With("system", "bus"),
		timeout: timeout,
	}
}

// Conn returns the underlying NATS connection.
func (b *bus) Conn() *nats.Conn {
	return b.conn
}

// Subscribe creates a NATS subscription and tracks it by subject.
func (b *bus) Subscribe(subject string, handler nats.MsgHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.subs[subject]; exists {
		return fmt.Errorf("subscription already exists for subject: %s", subject)
	}

	sub, err := b.conn.Subscribe(subject, handler)
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", subject, err)
	}

	b.subs[subject] = sub
	b.logger.Info("subscribed", "subject", subject)
	return nil
}

// Unsubscribe drains and removes a tracked subscription by subject.
func (b *bus) Unsubscribe(subject string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub, exists := b.subs[subject]
	if !exists {
		return fmt.Errorf("no subscription for subject: %s", subject)
	}

	if err := sub.Drain(); err != nil {
		return fmt.Errorf("drain subscription %s: %w", subject, err)
	}

	delete(b.subs, subject)
	b.logger.Info("unsubscribed", "subject", subject)
	return nil
}

// Start connects to NATS and registers lifecycle hooks.
func (b *bus) Start(lc *lifecycle.Coordinator) error {
	opts := b.cfg.Options(b.logger)

	conn, err := nats.Connect(b.cfg.URL(), opts...)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}

	b.conn = conn
	b.logger.Info("connected to nats", "url", b.cfg.URL())

	lc.RegisterChecker(b)

	lc.OnShutdown(func() {
		<-lc.Context().Done()
		b.logger.Info("draining bus")

		if err := b.drain(); err != nil {
			b.logger.Error("bus drain failed", "error", err)
		} else {
			b.logger.Info("bus drain complete")
		}
	})

	return nil
}

// Ready reports whether the NATS connection is active.
func (b *bus) Ready() bool {
	return b.conn != nil && b.conn.IsConnected()
}

func (b *bus) drain() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var errs []error

	for subject, sub := range b.subs {
		if err := sub.Drain(); err != nil {
			errs = append(errs, fmt.Errorf("drain subscription %s: %w", subject, err))
		}
	}

	closed := make(chan struct{})
	b.conn.SetClosedHandler(func(_ *nats.Conn) {
		close(closed)
	})

	if err := b.conn.Drain(); err != nil {
		errs = append(errs, fmt.Errorf("nats drain: %w", err))
		return errors.Join(errs...)
	}

	select {
	case <-closed:
	case <-time.After(b.timeout):
		errs = append(errs, fmt.Errorf("nats drain timeout after %v", b.timeout))
	}

	return errors.Join(errs...)
}
