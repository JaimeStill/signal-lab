package jobs_test

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/alpha/jobs"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
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

func newJobs(b bus.System) jobs.System {
	return jobs.New(
		b,
		"test-alpha",
		100*time.Millisecond,
		slog.Default(),
	)
}

func TestStartStop(t *testing.T) {
	b := startBus(t)
	j := newJobs(b)

	status := j.Status()
	if status.Running {
		t.Fatal("expected publisher to not be running initially")
	}

	if err := j.Start(); err != nil {
		t.Fatal("start failed:", err)
	}

	status = j.Status()
	if !status.Running {
		t.Fatal("expected publisher to be running after Start")
	}

	if err := j.Start(); err == nil {
		t.Fatal("expected error when starting already running publisher")
	}

	if err := j.Stop(); err != nil {
		t.Fatal("stop failed:", err)
	}

	status = j.Status()
	if status.Running {
		t.Fatal("expected publisher to not be running after Stop")
	}

	if err := j.Stop(); err == nil {
		t.Fatal("expected error when stopping already stopped publisher")
	}
}

func TestStatus(t *testing.T) {
	b := startBus(t)
	j := newJobs(b)

	status := j.Status()
	if status.Interval != "100ms" {
		t.Fatalf("expected interval '100ms', got %q", status.Interval)
	}
}

// receiveFromSource pulls messages from the channel until one matches the
// given signal source. Filters out messages produced by other parallel tests
// that share the NATS server. Fails the test on timeout.
func receiveFromSource(t *testing.T, ch <-chan *nats.Msg, source string, timeout time.Duration) (*nats.Msg, signal.Signal) {
	t.Helper()

	deadline := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			sig, err := signal.Decode(msg.Data)
			if err != nil {
				continue
			}
			if sig.Source == source {
				return msg, sig
			}
		case <-deadline:
			t.Fatalf("timed out waiting for message from source %q", source)
		}
	}
}

func TestPublishesJobs(t *testing.T) {
	b := startBus(t)
	j := newJobs(b)

	received := make(chan *nats.Msg, 64)
	sub, err := b.Conn().Subscribe(contracts.SubjectWildcard, func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatal("subscribe failed:", err)
	}
	defer sub.Unsubscribe()

	if err := j.Start(); err != nil {
		t.Fatal("start failed:", err)
	}
	defer j.Stop()

	_, sig := receiveFromSource(t, received, "test-alpha", 2*time.Second)

	var job contracts.Job
	if err := json.Unmarshal(sig.Data, &job); err != nil {
		t.Fatal("failed to unmarshal job:", err)
	}

	if job.ID == "" {
		t.Fatal("expected non-empty job ID")
	}
	if job.Type == "" {
		t.Fatal("expected non-empty job type")
	}
	if job.Priority == "" {
		t.Fatal("expected non-empty job priority")
	}
	if job.Payload == "" {
		t.Fatal("expected non-empty job payload")
	}
}

func TestSubjectHierarchy(t *testing.T) {
	b := startBus(t)
	j := newJobs(b)

	subjects := make(map[string]bool)
	sub, err := b.Conn().Subscribe(contracts.SubjectWildcard, func(msg *nats.Msg) {
		// Filter out messages from other parallel test packages.
		sig, err := signal.Decode(msg.Data)
		if err != nil || sig.Source != "test-alpha" {
			return
		}
		subjects[msg.Subject] = true
	})
	if err != nil {
		t.Fatal("subscribe failed:", err)
	}
	defer sub.Unsubscribe()

	if err := j.Start(); err != nil {
		t.Fatal("start failed:", err)
	}

	// Wait long enough to see multiple ticks across different random types
	time.Sleep(2 * time.Second)
	j.Stop()

	if len(subjects) == 0 {
		t.Fatal("expected at least one subject, got none")
	}

	validPrefix := contracts.SubjectPrefix + "."
	for subject := range subjects {
		if len(subject) <= len(validPrefix) || subject[:len(validPrefix)] != validPrefix {
			t.Errorf("subject %q does not start with expected prefix %q", subject, validPrefix)
		}
	}
}

func TestHeaders(t *testing.T) {
	b := startBus(t)
	j := newJobs(b)

	received := make(chan *nats.Msg, 64)
	sub, err := b.Conn().Subscribe(contracts.SubjectWildcard, func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatal("subscribe failed:", err)
	}
	defer sub.Unsubscribe()

	if err := j.Start(); err != nil {
		t.Fatal("start failed:", err)
	}
	defer j.Stop()

	msg, _ := receiveFromSource(t, received, "test-alpha", 2*time.Second)

	jobID := msg.Header.Get(contracts.HeaderJobID)
	if jobID == "" {
		t.Fatal("expected Job-ID header")
	}

	priority := msg.Header.Get(contracts.HeaderPriority)
	if priority == "" {
		t.Fatal("expected Job-Priority header")
	}
	validPriorities := map[string]bool{
		string(contracts.PriorityLow):    true,
		string(contracts.PriorityNormal): true,
		string(contracts.PriorityHigh):   true,
	}
	if !validPriorities[priority] {
		t.Fatalf("unexpected priority %q", priority)
	}

	jobType := msg.Header.Get(contracts.HeaderType)
	if jobType == "" {
		t.Fatal("expected Job-Type header")
	}
	validTypes := map[string]bool{
		string(contracts.TypeCompute):  true,
		string(contracts.TypeIO):       true,
		string(contracts.TypeAnalysis): true,
	}
	if !validTypes[jobType] {
		t.Fatalf("unexpected job type %q", jobType)
	}

	traceID := msg.Header.Get(contracts.HeaderTraceID)
	if traceID == "" {
		t.Fatal("expected Signal-Trace-ID header")
	}
}

func TestHandler(t *testing.T) {
	b := startBus(t)
	j := newJobs(b)

	handler := j.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	routes := handler.Routes()
	if routes.Prefix != "/jobs" {
		t.Fatalf("expected prefix '/jobs', got %q", routes.Prefix)
	}
	if len(routes.Routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes.Routes))
	}
}
