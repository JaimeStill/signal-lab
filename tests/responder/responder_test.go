package responder_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/internal/beta/responder"
	"github.com/JaimeStill/signal-lab/pkg/bus"
	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/commands"
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

func testNamespace(t *testing.T) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return strings.ToLower(name)
}

func testSubjectPrefix(t *testing.T) string {
	return fmt.Sprintf("test.responder.%s", testNamespace(t))
}

func testSubjectWildcard(t *testing.T) string {
	return testSubjectPrefix(t) + ".>"
}

func newResponder(t *testing.T, b bus.System) responder.System {
	t.Helper()
	return responder.New(b, testSubjectWildcard(t), 64, slog.Default())
}

// issueCommand sends a command via Request on the given subject prefix
// and returns the decoded response.
func issueCommand(t *testing.T, b bus.System, prefix string, action contracts.Action, payload string) contracts.Response {
	t.Helper()

	cmd := contracts.Command{
		ID:       uuid.New().String(),
		Action:   action,
		Payload:  payload,
		IssuedAt: time.Now().Format(time.RFC3339),
	}

	subject := fmt.Sprintf("%s.%s", prefix, action)

	sig, err := signal.New("test-requester", subject, cmd)
	if err != nil {
		t.Fatal("signal create failed:", err)
	}

	data, err := signal.Encode(sig)
	if err != nil {
		t.Fatal("signal encode failed:", err)
	}

	msg, err := b.Conn().Request(subject, data, 2*time.Second)
	if err != nil {
		t.Fatal("request failed:", err)
	}

	replySig, err := signal.Decode(msg.Data)
	if err != nil {
		t.Fatal("decode reply signal failed:", err)
	}

	var resp contracts.Response
	if err := json.Unmarshal(replySig.Data, &resp); err != nil {
		t.Fatal("unmarshal response failed:", err)
	}

	return resp
}

func TestSubscribe(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	if err := resp.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
}

func TestRespondPing(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	if err := resp.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
	time.Sleep(50 * time.Millisecond)

	prefix := testSubjectPrefix(t)
	reply := issueCommand(t, b, prefix, contracts.ActionPing, "")

	if reply.Status != contracts.StatusOK {
		t.Fatalf("expected status %q, got %q", contracts.StatusOK, reply.Status)
	}
	if reply.Result != "pong" {
		t.Fatalf("expected result \"pong\", got %q", reply.Result)
	}

	ledger := resp.Ledger()
	if len(ledger) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledger))
	}
}

func TestRespondAllActions(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	if err := resp.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Capture the parent test's prefix so subtests use the same namespace
	// as the responder's subscription.
	prefix := testSubjectPrefix(t)

	cases := []struct {
		action contracts.Action
		result string
	}{
		{contracts.ActionPing, "pong"},
		{contracts.ActionFlush, "flushed"},
		{contracts.ActionRotate, "rotated"},
		{contracts.ActionNoop, "noop"},
	}

	for _, tc := range cases {
		t.Run(string(tc.action), func(t *testing.T) {
			reply := issueCommand(t, b, prefix, tc.action, "")
			if reply.Status != contracts.StatusOK {
				t.Fatalf("expected status %q, got %q", contracts.StatusOK, reply.Status)
			}
			if reply.Result != tc.result {
				t.Fatalf("expected result %q, got %q", tc.result, reply.Result)
			}
		})
	}

	ledger := resp.Ledger()
	if len(ledger) != 4 {
		t.Fatalf("expected 4 ledger entries, got %d", len(ledger))
	}
}

func TestRespondUnknownAction(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	if err := resp.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
	time.Sleep(50 * time.Millisecond)

	prefix := testSubjectPrefix(t)
	reply := issueCommand(t, b, prefix, "explode", "")

	if reply.Status != contracts.StatusError {
		t.Fatalf("expected status %q, got %q", contracts.StatusError, reply.Status)
	}
	if reply.Result != "unknown action: explode" {
		t.Fatalf("expected error result, got %q", reply.Result)
	}

	ledger := resp.Ledger()
	if len(ledger) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledger))
	}
	if ledger[0].Response.Status != contracts.StatusError {
		t.Fatal("expected error status in ledger entry")
	}
}

func TestLedgerRecordsPayload(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	if err := resp.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
	time.Sleep(50 * time.Millisecond)

	prefix := testSubjectPrefix(t)
	issueCommand(t, b, prefix, contracts.ActionFlush, "critical-data")

	ledger := resp.Ledger()
	if len(ledger) != 1 {
		t.Fatalf("expected 1 ledger entry, got %d", len(ledger))
	}
	if ledger[0].Command.Payload != "critical-data" {
		t.Fatalf("expected payload \"critical-data\", got %q", ledger[0].Command.Payload)
	}
}

func TestLedgerBounded(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	if err := resp.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
	time.Sleep(50 * time.Millisecond)

	prefix := testSubjectPrefix(t)
	const maxLedger = 64
	for range maxLedger + 10 {
		issueCommand(t, b, prefix, contracts.ActionNoop, "")
	}

	ledger := resp.Ledger()
	if len(ledger) != maxLedger {
		t.Fatalf("expected ledger bounded to %d, got %d", maxLedger, len(ledger))
	}
}

func TestLedgerOrder(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	if err := resp.Subscribe(); err != nil {
		t.Fatal("subscribe failed:", err)
	}
	time.Sleep(50 * time.Millisecond)

	prefix := testSubjectPrefix(t)
	issueCommand(t, b, prefix, contracts.ActionPing, "")
	issueCommand(t, b, prefix, contracts.ActionFlush, "")
	issueCommand(t, b, prefix, contracts.ActionRotate, "")

	ledger := resp.Ledger()
	if len(ledger) != 3 {
		t.Fatalf("expected 3 ledger entries, got %d", len(ledger))
	}

	// Ledger is in order (oldest first)
	expected := []contracts.Action{
		contracts.ActionPing,
		contracts.ActionFlush,
		contracts.ActionRotate,
	}
	for i, action := range expected {
		if ledger[i].Command.Action != action {
			t.Fatalf("ledger[%d]: expected action %q, got %q", i, action, ledger[i].Command.Action)
		}
	}
}

func TestHandler(t *testing.T) {
	b := startBus(t)
	resp := newResponder(t, b)

	handler := resp.Handler()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	routes := handler.Routes()
	if routes.Prefix != "/responder" {
		t.Fatalf("expected prefix '/responder', got %q", routes.Prefix)
	}
	if len(routes.Routes) != 1 {
		t.Fatalf("expected 1 route (ledger), got %d", len(routes.Routes))
	}
}
