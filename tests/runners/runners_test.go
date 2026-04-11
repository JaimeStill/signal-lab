package runners_test

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/beta/runners"
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

// testNamespace returns a sanitized identifier derived from the test name,
// safe to use inside NATS subjects and queue group names. Per-test namespaces
// give every test its own subject space and queue group, eliminating cross-
// test pollution when tests run in parallel against a shared NATS server.
func testNamespace(t *testing.T) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return strings.ToLower(name)
}

func testSubjectPrefix(t *testing.T) string {
	return fmt.Sprintf("test.runners.%s", testNamespace(t))
}

func testSubjectWildcard(t *testing.T) string {
	return testSubjectPrefix(t) + ".>"
}

func testQueueGroup(t *testing.T) string {
	return fmt.Sprintf("test-runners-%s", testNamespace(t))
}

func newCluster(t *testing.T, b bus.System, count int) runners.System {
	t.Helper()
	c := runners.New(b, count, testSubjectWildcard(t), testQueueGroup(t), slog.Default())
	t.Cleanup(func() { _ = c.Unsubscribe() })
	return c
}

// publishJob sends a single job message via the raw bus connection on the
// test's isolated subject namespace so the runners receive it without
// interference from other parallel tests.
func publishJob(t *testing.T, b bus.System, id string) {
	t.Helper()

	job := contracts.Job{
		ID:       id,
		Type:     contracts.TypeCompute,
		Priority: contracts.PriorityNormal,
		Payload:  "test",
	}
	subject := fmt.Sprintf("%s.%s", testSubjectPrefix(t), contracts.TypeCompute)

	sig, err := signal.New("test-publisher", subject, job)
	if err != nil {
		t.Fatal("signal create failed:", err)
	}
	data, err := signal.Encode(sig)
	if err != nil {
		t.Fatal("signal encode failed:", err)
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	msg.Header.Set(contracts.HeaderJobID, job.ID)
	msg.Header.Set(contracts.HeaderPriority, string(job.Priority))
	msg.Header.Set(contracts.HeaderType, string(job.Type))
	msg.Header.Set(contracts.HeaderTraceID, sig.ID)

	if err := b.Conn().PublishMsg(msg); err != nil {
		t.Fatal("publish failed:", err)
	}
}

// waitForTotal polls Status() until the total per-subject count across all
// runners reaches expected, or the deadline elapses.
func waitForTotal(t *testing.T, c runners.System, expected int64, timeout time.Duration) runners.Status {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		status := c.Status()
		var total int64
		for _, rs := range status.Runners {
			for _, count := range rs.Counts {
				total += count
			}
		}
		if total >= expected {
			return status
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d messages, total was %d", expected, total)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestConstruction(t *testing.T) {
	b := startBus(t)
	c := newCluster(t, b, 3)

	status := c.Status()
	if status.Count != 3 {
		t.Fatalf("expected count 3, got %d", status.Count)
	}
	if status.Subscribed {
		t.Fatal("expected cluster to be unsubscribed before Subscribe()")
	}
	if len(status.Runners) != 3 {
		t.Fatalf("expected 3 runners in snapshot, got %d", len(status.Runners))
	}
	for id, rs := range status.Runners {
		if rs.Subscribed {
			t.Errorf("expected runner %s to be unsubscribed initially", id)
		}
		if len(rs.Counts) != 0 {
			t.Errorf("expected runner %s to have empty counts initially", id)
		}
	}
}

func TestClusterSubscribeUnsubscribe(t *testing.T) {
	b := startBus(t)
	c := newCluster(t, b, 3)

	if err := c.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}

	status := c.Status()
	if !status.Subscribed {
		t.Fatal("expected cluster Subscribed=true after Subscribe()")
	}
	for id, rs := range status.Runners {
		if !rs.Subscribed {
			t.Errorf("expected runner %s to be subscribed", id)
		}
	}

	// Eager idempotent: second Subscribe is a no-op
	if err := c.Subscribe(); err != nil {
		t.Fatalf("second Subscribe should be a no-op, got error: %v", err)
	}

	if err := c.Unsubscribe(); err != nil {
		t.Fatal("unsubscribe failed:", err)
	}

	status = c.Status()
	if status.Subscribed {
		t.Fatal("expected cluster Subscribed=false after Unsubscribe()")
	}
	for id, rs := range status.Runners {
		if rs.Subscribed {
			t.Errorf("expected runner %s to be unsubscribed", id)
		}
	}

	// Eager idempotent: second Unsubscribe is a no-op
	if err := c.Unsubscribe(); err != nil {
		t.Fatalf("second Unsubscribe should be a no-op, got error: %v", err)
	}
}

func TestPerRunnerSubscribe(t *testing.T) {
	b := startBus(t)
	c := newCluster(t, b, 3)

	if err := c.SubscribeRunner("runner-0"); err != nil {
		t.Fatal("SubscribeRunner failed:", err)
	}

	status := c.Status()
	if status.Subscribed {
		t.Fatal("expected cluster Subscribed=false (only one runner attached)")
	}
	if !status.Runners["runner-0"].Subscribed {
		t.Fatal("expected runner-0 to be subscribed")
	}
	if status.Runners["runner-1"].Subscribed {
		t.Fatal("expected runner-1 to be unsubscribed")
	}

	// Idempotent-via-error: second SubscribeRunner errors
	if err := c.SubscribeRunner("runner-0"); err == nil {
		t.Fatal("expected error subscribing already-attached runner")
	}

	// Nonexistent runner
	if err := c.SubscribeRunner("runner-99"); err == nil {
		t.Fatal("expected error for nonexistent runner")
	}

	// Cluster Subscribe() should attach only the runners that aren't already
	// subscribed (runner-1 and runner-2)
	if err := c.Subscribe(); err != nil {
		t.Fatalf("cluster Subscribe should succeed and skip runner-0, got: %v", err)
	}

	status = c.Status()
	if !status.Subscribed {
		t.Fatal("expected cluster Subscribed=true after eager Subscribe()")
	}
}

func TestPerRunnerUnsubscribe(t *testing.T) {
	b := startBus(t)
	c := newCluster(t, b, 3)

	if err := c.Subscribe(); err != nil {
		t.Fatal("cluster Subscribe failed:", err)
	}

	if err := c.UnsubscribeRunner("runner-1"); err != nil {
		t.Fatal("UnsubscribeRunner failed:", err)
	}

	status := c.Status()
	if status.Subscribed {
		t.Fatal("expected cluster Subscribed=false after detaching one runner")
	}
	if status.Runners["runner-1"].Subscribed {
		t.Fatal("expected runner-1 to be unsubscribed")
	}
	if !status.Runners["runner-0"].Subscribed {
		t.Fatal("expected runner-0 to remain subscribed")
	}

	// Idempotent-via-error: second UnsubscribeRunner errors
	if err := c.UnsubscribeRunner("runner-1"); err == nil {
		t.Fatal("expected error unsubscribing already-detached runner")
	}

	// Nonexistent runner
	if err := c.UnsubscribeRunner("runner-99"); err == nil {
		t.Fatal("expected error for nonexistent runner")
	}
}

func TestWorkDistribution(t *testing.T) {
	b := startBus(t)
	c := newCluster(t, b, 3)

	if err := c.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}

	const total = 30
	for i := range total {
		publishJob(t, b, fmt.Sprintf("dist-%d", i))
	}

	status := waitForTotal(t, c, total, 3*time.Second)

	// Every runner should have received at least one message — proves the
	// queue group is distributing work across the cluster.
	for id, rs := range status.Runners {
		var runnerTotal int64
		for _, count := range rs.Counts {
			runnerTotal += count
		}
		if runnerTotal == 0 {
			t.Errorf("runner %s received no messages, distribution failed", id)
		}
	}

	// Total across all runners should equal what was published (no drops)
	var grandTotal int64
	for _, rs := range status.Runners {
		for _, count := range rs.Counts {
			grandTotal += count
		}
	}
	if grandTotal != total {
		t.Fatalf("expected total %d across all runners, got %d", total, grandTotal)
	}
}

func TestSingleRunnerReceivesAll(t *testing.T) {
	b := startBus(t)
	c := newCluster(t, b, 3)

	// Only attach runner-0; the other two stay detached
	if err := c.SubscribeRunner("runner-0"); err != nil {
		t.Fatal("SubscribeRunner failed:", err)
	}

	const total = 10
	for i := range total {
		publishJob(t, b, fmt.Sprintf("solo-%d", i))
	}

	status := waitForTotal(t, c, total, 3*time.Second)

	// runner-0 must have received everything
	var runner0Total int64
	for _, count := range status.Runners["runner-0"].Counts {
		runner0Total += count
	}
	if runner0Total != total {
		t.Fatalf("expected runner-0 to receive %d messages, got %d", total, runner0Total)
	}

	// Other runners must have received nothing
	for _, id := range []string{"runner-1", "runner-2"} {
		var otherTotal int64
		for _, count := range status.Runners[id].Counts {
			otherTotal += count
		}
		if otherTotal != 0 {
			t.Errorf("expected runner %s to receive 0 messages, got %d", id, otherTotal)
		}
	}
}

func TestHandler(t *testing.T) {
	b := startBus(t)
	c := newCluster(t, b, 3)

	handler := c.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	routes := handler.Routes()
	if routes.Prefix != "/runners" {
		t.Fatalf("expected prefix '/runners', got %q", routes.Prefix)
	}
	if len(routes.Routes) != 5 {
		t.Fatalf("expected 5 routes (subscribe, unsubscribe, runner sub/unsub, status), got %d", len(routes.Routes))
	}
}
