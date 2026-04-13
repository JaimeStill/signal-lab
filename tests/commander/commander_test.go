package commander_test

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/alpha/commander"
	"github.com/JaimeStill/signal-lab/internal/beta/responder"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/commands"
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

func testNamespace(t *testing.T) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return strings.ToLower(name)
}

func testSubjectPrefix(t *testing.T) string {
	return fmt.Sprintf("test.commands.%s", testNamespace(t))
}

func testSubjectWildcard(t *testing.T) string {
	return testSubjectPrefix(t) + ".>"
}

// newPair creates a commander on one bus and a responder on another,
// subscribes the responder, and returns both. Using separate bus instances
// mirrors the two-service architecture. Per-test subject namespacing
// eliminates cross-test pollution.
func newPair(t *testing.T) (commander.System, responder.System) {
	t.Helper()

	alphaBus := startBus(t)
	betaBus := startSecondBus(t)

	cmd := commander.New(alphaBus, "test-alpha", testSubjectPrefix(t), 2*time.Second, 64, slog.Default())
	resp := responder.New(betaBus, testSubjectWildcard(t), 64, slog.Default())

	if err := resp.Subscribe(); err != nil {
		t.Fatal("responder subscribe failed:", err)
	}

	// Give NATS a moment to propagate the subscription
	time.Sleep(50 * time.Millisecond)

	return cmd, resp
}

func TestIssuePing(t *testing.T) {
	cmd, _ := newPair(t)

	resp, err := cmd.Issue("ping", "")
	if err != nil {
		t.Fatal("issue failed:", err)
	}

	if resp.Status != contracts.StatusOK {
		t.Fatalf("expected status %q, got %q", contracts.StatusOK, resp.Status)
	}
	if resp.Result != "pong" {
		t.Fatalf("expected result \"pong\", got %q", resp.Result)
	}
	if resp.CommandID == "" {
		t.Fatal("expected non-empty command_id in response")
	}
	if resp.HandledAt == "" {
		t.Fatal("expected non-empty handled_at in response")
	}
}

func TestIssueAllActions(t *testing.T) {
	cmd, _ := newPair(t)

	cases := []struct {
		action string
		result string
	}{
		{"ping", "pong"},
		{"flush", "flushed"},
		{"rotate", "rotated"},
		{"noop", "noop"},
	}

	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			resp, err := cmd.Issue(tc.action, "")
			if err != nil {
				t.Fatalf("issue %q failed: %v", tc.action, err)
			}
			if resp.Status != contracts.StatusOK {
				t.Fatalf("expected status %q, got %q", contracts.StatusOK, resp.Status)
			}
			if resp.Result != tc.result {
				t.Fatalf("expected result %q, got %q", tc.result, resp.Result)
			}
		})
	}
}

func TestIssueUnknownAction(t *testing.T) {
	cmd, _ := newPair(t)

	resp, err := cmd.Issue("explode", "")
	if err != nil {
		t.Fatal("issue failed:", err)
	}

	if resp.Status != contracts.StatusError {
		t.Fatalf("expected status %q, got %q", contracts.StatusError, resp.Status)
	}
	if resp.Result != "unknown action: explode" {
		t.Fatalf("expected error result, got %q", resp.Result)
	}
}

func TestIssueWithPayload(t *testing.T) {
	cmd, resp := newPair(t)

	_, err := cmd.Issue("flush", "important-data")
	if err != nil {
		t.Fatal("issue failed:", err)
	}

	ledger := resp.Ledger()
	if len(ledger) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledger))
	}
	if ledger[0].Command.Payload != "important-data" {
		t.Fatalf("expected payload \"important-data\", got %q", ledger[0].Command.Payload)
	}
}

func TestIssueTimeout(t *testing.T) {
	// Commander with no responder — request should time out
	b := startBus(t)
	cmd := commander.New(b, "test-alpha", testSubjectPrefix(t), 200*time.Millisecond, 64, slog.Default())

	_, err := cmd.Issue("ping", "")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHistory(t *testing.T) {
	cmd, _ := newPair(t)

	// Issue two commands
	_, err := cmd.Issue("ping", "")
	if err != nil {
		t.Fatal("first issue failed:", err)
	}
	_, err = cmd.Issue("flush", "")
	if err != nil {
		t.Fatal("second issue failed:", err)
	}

	history := cmd.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	// History is newest first
	if history[0].Command.Action != contracts.ActionFlush {
		t.Fatalf("expected newest entry to be flush, got %q", history[0].Command.Action)
	}
	if history[1].Command.Action != contracts.ActionPing {
		t.Fatalf("expected oldest entry to be ping, got %q", history[1].Command.Action)
	}

	// Successful entries should have a response and no error
	for _, entry := range history {
		if entry.Response == nil {
			t.Fatal("expected non-nil response in history entry")
		}
		if entry.Error != "" {
			t.Fatalf("expected empty error, got %q", entry.Error)
		}
	}
}

func TestHistoryRecordsTimeout(t *testing.T) {
	b := startBus(t)
	cmd := commander.New(b, "test-alpha", testSubjectPrefix(t), 200*time.Millisecond, 64, slog.Default())

	_, _ = cmd.Issue("ping", "")

	history := cmd.History()
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Response != nil {
		t.Fatal("expected nil response for timed-out command")
	}
	if history[0].Error == "" {
		t.Fatal("expected non-empty error for timed-out command")
	}
}

func TestHistoryBounded(t *testing.T) {
	cmd, _ := newPair(t)

	const maxHistory = 64
	for range maxHistory + 10 {
		_, err := cmd.Issue("noop", "")
		if err != nil {
			t.Fatal("issue failed:", err)
		}
	}

	history := cmd.History()
	if len(history) != maxHistory {
		t.Fatalf("expected history bounded to %d, got %d", maxHistory, len(history))
	}
}

func TestHandler(t *testing.T) {
	b := startBus(t)
	cmd := commander.New(b, "test-alpha", testSubjectPrefix(t), 2*time.Second, 64, slog.Default())

	handler := cmd.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	routes := handler.Routes()
	if routes.Prefix != "/commander" {
		t.Fatalf("expected prefix '/commander', got %q", routes.Prefix)
	}
	if len(routes.Routes) != 2 {
		t.Fatalf("expected 2 routes (issue, history), got %d", len(routes.Routes))
	}
}
