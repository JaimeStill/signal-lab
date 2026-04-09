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

func TestQueueSubscribe(t *testing.T) {
	b := tryStart(t)

	received := make(chan string, 1)
	if err := b.QueueSubscribe("test.queue", "workers", func(msg *nats.Msg) {
		received <- string(msg.Data)
	}); err != nil {
		t.Fatal("queue subscribe failed:", err)
	}

	if err := b.Conn().Publish("test.queue", []byte("hello")); err != nil {
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

func TestQueueSubscribeDuplicate(t *testing.T) {
	b := tryStart(t)

	if err := b.QueueSubscribe("test.queue.dup", "workers", func(_ *nats.Msg) {}); err != nil {
		t.Fatal("queue subscribe failed:", err)
	}

	if err := b.QueueSubscribe("test.queue.dup", "workers", func(_ *nats.Msg) {}); err == nil {
		t.Fatal("expected error for duplicate queue subscription")
	}
}

func TestQueueSubscribeDistribution(t *testing.T) {
	b1 := tryStart(t)
	b2 := tryStart(t)

	const total = 100
	count1 := make(chan struct{}, total)
	count2 := make(chan struct{}, total)

	if err := b1.QueueSubscribe("test.queue.dist", "workers", func(_ *nats.Msg) {
		count1 <- struct{}{}
	}); err != nil {
		t.Fatal("queue subscribe b1 failed:", err)
	}

	if err := b2.QueueSubscribe("test.queue.dist", "workers", func(_ *nats.Msg) {
		count2 <- struct{}{}
	}); err != nil {
		t.Fatal("queue subscribe b2 failed:", err)
	}

	// Allow subscriptions to propagate
	b1.Conn().Flush()
	b2.Conn().Flush()

	for range total {
		if err := b1.Conn().Publish("test.queue.dist", []byte("msg")); err != nil {
			t.Fatal("publish failed:", err)
		}
	}
	b1.Conn().Flush()

	// Wait for all messages to arrive
	time.Sleep(500 * time.Millisecond)

	got1 := len(count1)
	got2 := len(count2)

	if got1+got2 != total {
		t.Fatalf("expected %d total messages, got %d + %d = %d", total, got1, got2, got1+got2)
	}

	// Each subscriber should get at least 20% (generous bound for 100 messages)
	if got1 < 20 || got2 < 20 {
		t.Fatalf("uneven distribution: b1=%d, b2=%d (expected each >= 20)", got1, got2)
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
