package alerts_test

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/sensor/alerts"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/alerts"
	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
	"github.com/JaimeStill/signal-lab/pkg/signal"
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

func newAlerts(b bus.System) alerts.System {
	return alerts.New(
		b,
		"test-sensor",
		200*time.Millisecond,
		[]string{"zone-a"},
		slog.Default(),
	)
}

func TestStartStop(t *testing.T) {
	b := startBus(t)
	alt := newAlerts(b)

	status := alt.Status()
	if status.Running {
		t.Fatal("expected publisher to not be running initially")
	}

	if err := alt.Start(); err != nil {
		t.Fatal("start failed:", err)
	}

	status = alt.Status()
	if !status.Running {
		t.Fatal("expected publisher to be running after Start")
	}

	if err := alt.Start(); err == nil {
		t.Fatal("expected error when starting already running publisher")
	}

	if err := alt.Stop(); err != nil {
		t.Fatal("stop failed:", err)
	}

	status = alt.Status()
	if status.Running {
		t.Fatal("expected publisher to not be running after Stop")
	}

	if err := alt.Stop(); err == nil {
		t.Fatal("expected error when stopping already stopped publisher")
	}
}

func TestStatus(t *testing.T) {
	b := startBus(t)
	alt := newAlerts(b)

	status := alt.Status()
	if status.Interval != "200ms" {
		t.Fatalf("expected interval '200ms', got %q", status.Interval)
	}
	if len(status.Zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(status.Zones))
	}
}

func TestPublishesAlerts(t *testing.T) {
	b := startBus(t)
	alt := newAlerts(b)

	received := make(chan *nats.Msg, 16)
	sub, err := b.Conn().Subscribe(contracts.SubjectWildcard, func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatal("subscribe failed:", err)
	}
	defer sub.Unsubscribe()

	if err := alt.Start(); err != nil {
		t.Fatal("start failed:", err)
	}
	defer alt.Stop()

	deadline := time.After(2 * time.Second)
	var msg *nats.Msg

	select {
	case msg = <-received:
	case <-deadline:
		t.Fatal("timed out waiting for alert message")
	}

	// Verify signal envelope
	sig, err := signal.Decode(msg.Data)
	if err != nil {
		t.Fatal("failed to decode signal:", err)
	}

	if sig.Source != "test-sensor" {
		t.Fatalf("expected source 'test-sensor', got %q", sig.Source)
	}

	// Verify alert payload
	var alert contracts.Alert
	if err := json.Unmarshal(sig.Data, &alert); err != nil {
		t.Fatal("failed to unmarshal alert:", err)
	}

	if alert.Zone != "zone-a" {
		t.Fatalf("expected zone 'zone-a', got %q", alert.Zone)
	}
	if alert.Severity == "" {
		t.Fatal("expected non-empty severity")
	}
	if alert.Message == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestHeaders(t *testing.T) {
	b := startBus(t)
	alt := newAlerts(b)

	received := make(chan *nats.Msg, 16)
	sub, err := b.Conn().Subscribe(contracts.SubjectWildcard, func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatal("subscribe failed:", err)
	}
	defer sub.Unsubscribe()

	if err := alt.Start(); err != nil {
		t.Fatal("start failed:", err)
	}
	defer alt.Stop()

	deadline := time.After(2 * time.Second)
	var msg *nats.Msg

	select {
	case msg = <-received:
	case <-deadline:
		t.Fatal("timed out waiting for alert message")
	}

	// Verify NATS headers
	priority := msg.Header.Get(contracts.HeaderPriority)
	if priority == "" {
		t.Fatal("expected Signal-Priority header")
	}

	validPriorities := map[string]bool{
		string(contracts.PriorityLow):      true,
		string(contracts.PriorityNormal):    true,
		string(contracts.PriorityHigh):      true,
		string(contracts.PriorityCritical):  true,
	}
	if !validPriorities[priority] {
		t.Fatalf("unexpected priority %q", priority)
	}

	source := msg.Header.Get(contracts.HeaderSource)
	if source != "test-sensor" {
		t.Fatalf("expected Signal-Source 'test-sensor', got %q", source)
	}

	traceID := msg.Header.Get(contracts.HeaderTraceID)
	if traceID == "" {
		t.Fatal("expected Signal-Trace-ID header")
	}
}

func TestHandler(t *testing.T) {
	b := startBus(t)
	alt := newAlerts(b)

	handler := alt.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	routes := handler.Routes()
	if routes.Prefix != "/alerts" {
		t.Fatalf("expected prefix '/alerts', got %q", routes.Prefix)
	}
	if len(routes.Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes.Routes))
	}
}
