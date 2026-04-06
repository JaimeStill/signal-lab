package bus_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

type testConfig struct {
	url string
}

func (c *testConfig) URL() string { return c.url }
func (c *testConfig) Options(logger *slog.Logger) []nats.Option {
	return []nats.Option{
		nats.MaxReconnects(1),
		nats.ReconnectWait(100 * time.Millisecond),
	}
}

func tryConnect(t *testing.T) *nats.Conn {
	t.Helper()
	logger := slog.Default()
	cfg := &testConfig{url: nats.DefaultURL}

	conn, err := bus.Connect(cfg, logger)
	if err != nil {
		t.Skip("NATS not available, skipping:", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestConnectSuccess(t *testing.T) {
	conn := tryConnect(t)
	if !conn.IsConnected() {
		t.Fatal("expected connection to be active")
	}
}

func TestConnectFailure(t *testing.T) {
	logger := slog.Default()
	cfg := &testConfig{url: "nats://invalid:9999"}

	_, err := bus.Connect(cfg, logger)
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestChecker(t *testing.T) {
	conn := tryConnect(t)

	checker := bus.NewChecker(conn)
	if !checker.Ready() {
		t.Fatal("expected Ready() to be true when connected")
	}

	conn.Close()
	if checker.Ready() {
		t.Fatal("expected Ready() to be false after close")
	}
}

func TestDrain(t *testing.T) {
	logger := slog.Default()
	cfg := &testConfig{url: nats.DefaultURL}

	conn, err := bus.Connect(cfg, logger)
	if err != nil {
		t.Skip("NATS not available, skipping:", err)
	}

	if err := bus.Drain(conn, 5*time.Second); err != nil {
		t.Fatal("drain failed:", err)
	}

	if !conn.IsClosed() {
		t.Fatal("expected connection to be closed after drain")
	}
}

func TestRegisterLifecycle(t *testing.T) {
	logger := slog.Default()
	cfg := &testConfig{url: nats.DefaultURL}

	conn, err := bus.Connect(cfg, logger)
	if err != nil {
		t.Skip("NATS not available, skipping:", err)
	}

	lc := lifecycle.New()
	bus.RegisterLifecycle(conn, lc, 5*time.Second)
	lc.WaitForStartup()

	if err := lc.Shutdown(5 * time.Second); err != nil {
		t.Fatal("shutdown failed:", err)
	}

	if !conn.IsClosed() {
		t.Fatal("expected connection to be closed after lifecycle shutdown")
	}
}
