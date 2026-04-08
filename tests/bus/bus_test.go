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

func tryStart(t *testing.T) bus.System {
	t.Helper()
	logger := slog.Default()
	cfg := &testConfig{url: nats.DefaultURL}

	b := bus.New(cfg, 5*time.Second, logger)
	lc := lifecycle.New()

	if err := b.Start(lc); err != nil {
		t.Skip("NATS not available, skipping:", err)
	}

	t.Cleanup(func() {
		lc.Shutdown(5 * time.Second)
	})

	return b
}

func TestNew(t *testing.T) {
	logger := slog.Default()
	cfg := &testConfig{url: nats.DefaultURL}

	b := bus.New(cfg, 5*time.Second, logger)
	if b.Conn() != nil {
		t.Fatal("expected nil connection before Start")
	}
}

func TestStartSuccess(t *testing.T) {
	b := tryStart(t)

	if b.Conn() == nil {
		t.Fatal("expected non-nil connection after Start")
	}
	if !b.Conn().IsConnected() {
		t.Fatal("expected connection to be active")
	}
}

func TestStartFailure(t *testing.T) {
	logger := slog.Default()
	cfg := &testConfig{url: "nats://invalid:9999"}

	b := bus.New(cfg, 5*time.Second, logger)
	lc := lifecycle.New()

	if err := b.Start(lc); err == nil {
		t.Fatal("expected connection error")
	}
}

func TestReadyViaLifecycle(t *testing.T) {
	logger := slog.Default()
	cfg := &testConfig{url: nats.DefaultURL}

	b := bus.New(cfg, 5*time.Second, logger)
	lc := lifecycle.New()

	if err := b.Start(lc); err != nil {
		t.Skip("NATS not available, skipping:", err)
	}
	t.Cleanup(func() { lc.Shutdown(5 * time.Second) })

	lc.WaitForStartup()

	if !lc.Ready() {
		t.Fatal("expected lifecycle Ready() to be true after bus Start")
	}
}

func TestSubscribe(t *testing.T) {
	b := tryStart(t)

	if err := b.Subscribe("test.subject", func(_ *nats.Msg) {}); err != nil {
		t.Fatal("subscribe failed:", err)
	}

	// Duplicate subscription should fail
	if err := b.Subscribe("test.subject", func(_ *nats.Msg) {}); err == nil {
		t.Fatal("expected error for duplicate subscription")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := tryStart(t)

	if err := b.Subscribe("test.subject", func(_ *nats.Msg) {}); err != nil {
		t.Fatal("subscribe failed:", err)
	}

	if err := b.Unsubscribe("test.subject"); err != nil {
		t.Fatal("unsubscribe failed:", err)
	}

	// Unsubscribing again should fail
	if err := b.Unsubscribe("test.subject"); err == nil {
		t.Fatal("expected error for unknown subject")
	}

	// Should be able to re-subscribe after unsubscribe
	if err := b.Subscribe("test.subject", func(_ *nats.Msg) {}); err != nil {
		t.Fatal("re-subscribe after unsubscribe failed:", err)
	}
}

func TestLifecycleShutdown(t *testing.T) {
	logger := slog.Default()
	cfg := &testConfig{url: nats.DefaultURL}

	b := bus.New(cfg, 5*time.Second, logger)
	lc := lifecycle.New()

	if err := b.Start(lc); err != nil {
		t.Skip("NATS not available, skipping:", err)
	}

	b.Subscribe("test.shutdown", func(_ *nats.Msg) {})
	lc.WaitForStartup()

	if err := lc.Shutdown(5 * time.Second); err != nil {
		t.Fatal("shutdown failed:", err)
	}

	if !b.Conn().IsClosed() {
		t.Fatal("expected connection to be closed after lifecycle shutdown")
	}
}

func TestSubscribeAndReceive(t *testing.T) {
	b := tryStart(t)

	received := make(chan string, 1)
	if err := b.Subscribe("test.receive", func(msg *nats.Msg) {
		received <- string(msg.Data)
	}); err != nil {
		t.Fatal("subscribe failed:", err)
	}

	if err := b.Conn().Publish("test.receive", []byte("hello")); err != nil {
		t.Fatal("publish failed:", err)
	}

	select {
	case msg := <-received:
		if msg != "hello" {
			t.Fatalf("expected 'hello', got %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}
