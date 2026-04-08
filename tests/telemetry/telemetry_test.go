package telemetry_test

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/sensor/telemetry"
	"github.com/JaimeStill/signal-lab/pkg/bus"
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

func newTelemetry(b bus.System) telemetry.System {
	return telemetry.New(
		b,
		"test-sensor",
		200*time.Millisecond,
		[]string{"temp", "humidity"},
		[]string{"zone-a"},
		slog.Default(),
	)
}

func TestStartStop(t *testing.T) {
	b := startBus(t)
	tel := newTelemetry(b)

	status := tel.Status()
	if status.Running {
		t.Fatal("expected publisher to not be running initially")
	}

	if err := tel.Start(); err != nil {
		t.Fatal("start failed:", err)
	}

	status = tel.Status()
	if !status.Running {
		t.Fatal("expected publisher to be running after Start")
	}

	// Starting again should error
	if err := tel.Start(); err == nil {
		t.Fatal("expected error when starting already running publisher")
	}

	if err := tel.Stop(); err != nil {
		t.Fatal("stop failed:", err)
	}

	status = tel.Status()
	if status.Running {
		t.Fatal("expected publisher to not be running after Stop")
	}

	// Stopping again should error
	if err := tel.Stop(); err == nil {
		t.Fatal("expected error when stopping already stopped publisher")
	}
}

func TestStatus(t *testing.T) {
	b := startBus(t)
	tel := newTelemetry(b)

	status := tel.Status()
	if status.Interval != "200ms" {
		t.Fatalf("expected interval '200ms', got %q", status.Interval)
	}
	if len(status.Types) != 2 {
		t.Fatalf("expected 2 types, got %d", len(status.Types))
	}
	if len(status.Zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(status.Zones))
	}
}

func TestPublishesReadings(t *testing.T) {
	b := startBus(t)
	tel := newTelemetry(b)

	// Subscribe to all telemetry subjects
	received := make(chan *nats.Msg, 16)
	sub, err := b.Conn().Subscribe("signal.telemetry.>", func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatal("subscribe failed:", err)
	}
	defer sub.Unsubscribe()

	if err := tel.Start(); err != nil {
		t.Fatal("start failed:", err)
	}
	defer tel.Stop()

	// Wait for at least one tick (200ms interval, 2 types * 1 zone = 2 messages per tick)
	deadline := time.After(2 * time.Second)
	var messages []*nats.Msg

	for len(messages) < 2 {
		select {
		case msg := <-received:
			messages = append(messages, msg)
		case <-deadline:
			t.Fatalf("timed out, received only %d messages", len(messages))
		}
	}

	// Verify first message is a valid signal with a reading payload
	sig, err := signal.Decode(messages[0].Data)
	if err != nil {
		t.Fatal("failed to decode signal:", err)
	}

	if sig.Source != "test-sensor" {
		t.Fatalf("expected source 'test-sensor', got %q", sig.Source)
	}

	var reading telemetry.Reading
	if err := json.Unmarshal(sig.Data, &reading); err != nil {
		t.Fatal("failed to unmarshal reading:", err)
	}

	if reading.Type == "" {
		t.Fatal("expected non-empty reading type")
	}
	if reading.Zone == "" {
		t.Fatal("expected non-empty reading zone")
	}
	if reading.Unit == "" {
		t.Fatal("expected non-empty reading unit")
	}
}

func TestSubjectHierarchy(t *testing.T) {
	b := startBus(t)
	tel := newTelemetry(b)

	subjects := make(map[string]bool)
	sub, err := b.Conn().Subscribe("signal.telemetry.>", func(msg *nats.Msg) {
		subjects[msg.Subject] = true
	})
	if err != nil {
		t.Fatal("subscribe failed:", err)
	}
	defer sub.Unsubscribe()

	if err := tel.Start(); err != nil {
		t.Fatal("start failed:", err)
	}

	// Wait for a couple of ticks
	time.Sleep(500 * time.Millisecond)
	tel.Stop()

	// With types [temp, humidity] and zones [zone-a], expect these subjects:
	expected := []string{
		"signal.telemetry.temp.zone-a",
		"signal.telemetry.humidity.zone-a",
	}

	for _, exp := range expected {
		if !subjects[exp] {
			t.Errorf("expected subject %q not seen, got: %v", exp, subjects)
		}
	}
}

func TestHandler(t *testing.T) {
	b := startBus(t)
	tel := newTelemetry(b)

	handler := tel.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	routes := handler.Routes()
	if routes.Prefix != "/telemetry" {
		t.Fatalf("expected prefix '/telemetry', got %q", routes.Prefix)
	}
	if len(routes.Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes.Routes))
	}
}
