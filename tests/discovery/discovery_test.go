package discovery_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

type testConfig struct{}

func (c *testConfig) URL() string { return nats.DefaultURL }
func (c *testConfig) Options(logger *slog.Logger) []nats.Option {
	return []nats.Option{
		nats.MaxReconnects(1),
		nats.ReconnectWait(100 * time.Millisecond),
	}
}

func startBus(t *testing.T) bus.System {
	t.Helper()
	logger := slog.Default()

	b := bus.New(&testConfig{}, 5*time.Second, logger)
	lc := lifecycle.New()

	if err := b.Start(lc); err != nil {
		t.Skip("NATS not available, skipping:", err)
	}

	t.Cleanup(func() { lc.Shutdown(5 * time.Second) })
	return b
}

func newInfo(name string) discovery.ServiceInfo {
	return discovery.ServiceInfo{
		ID:          "test-id-" + name,
		Name:        name,
		Endpoint:    "http://localhost:0",
		Health:      "ok",
		Description: "test " + name,
	}
}

func TestSubscribe(t *testing.T) {
	b := startBus(t)
	logger := slog.Default()
	info := newInfo("test-service")

	disc := discovery.New(b, info, 500*time.Millisecond, logger)

	if err := disc.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
}

func TestPingSelf(t *testing.T) {
	b := startBus(t)
	logger := slog.Default()
	info := newInfo("lonely-service")

	disc := discovery.New(b, info, 500*time.Millisecond, logger)

	if err := disc.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}

	// Ping with only self listening — should get no responses
	// (discovery ignores pings from self)
	services, err := disc.Ping()
	if err != nil {
		t.Fatal("ping failed:", err)
	}

	if len(services) != 0 {
		t.Fatalf("expected 0 responses (self-ping ignored), got %d", len(services))
	}
}

func TestPingDiscovery(t *testing.T) {
	// Use two separate bus instances to simulate two services
	b1 := startBus(t)
	b2 := startSecondBus(t)
	logger := slog.Default()

	infoA := newInfo("service-a")
	infoB := newInfo("service-b")

	discA := discovery.New(b1, infoA, 500*time.Millisecond, logger)
	discB := discovery.New(b2, infoB, 500*time.Millisecond, logger)

	// Subscribe B to respond to pings
	if err := discB.Subscribe(); err != nil {
		t.Fatal("subscribe B failed:", err)
	}

	// Give NATS a moment to propagate the subscription
	time.Sleep(50 * time.Millisecond)

	// A pings — should discover B
	services, err := discA.Ping()
	if err != nil {
		t.Fatal("ping failed:", err)
	}

	if len(services) != 1 {
		t.Fatalf("expected 1 response, got %d", len(services))
	}

	if services[0].Name != "service-b" {
		t.Fatalf("expected service-b, got %q", services[0].Name)
	}
}

func TestHandler(t *testing.T) {
	b := startBus(t)
	logger := slog.Default()
	info := newInfo("test-service")

	disc := discovery.New(b, info, 500*time.Millisecond, logger)
	handler := disc.Handler()

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	routes := handler.Routes()
	if routes.Prefix != "/discovery" {
		t.Fatalf("expected prefix '/discovery', got %q", routes.Prefix)
	}
	if len(routes.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes.Routes))
	}
	if routes.Routes[0].Method != "POST" {
		t.Fatalf("expected POST method, got %q", routes.Routes[0].Method)
	}
}

func startSecondBus(t *testing.T) bus.System {
	t.Helper()
	logger := slog.Default()

	b := bus.New(&testConfig{}, 5*time.Second, logger)
	lc := lifecycle.New()

	if err := b.Start(lc); err != nil {
		t.Skip("NATS not available, skipping:", err)
	}

	t.Cleanup(func() { lc.Shutdown(5 * time.Second) })
	return b
}
