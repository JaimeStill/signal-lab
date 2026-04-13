// Package responder implements the beta-side command handling domain. It
// subscribes to the command subject hierarchy, dispatches per-action handlers,
// maintains a bounded in-memory ledger of handled commands, and replies via
// msg.Respond.
package responder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/commands"
)

// LedgerEntry records a single handled command and its response.
type LedgerEntry struct {
	Command  contracts.Command  `json:"command"`
	Response contracts.Response `json:"response"`
}

// System manages command handling over the bus.
type System interface {
	// Subscribe registers the command listener with the bus.
	Subscribe() error
	// Ledger returns the handled commands in order (oldest first).
	Ledger() []LedgerEntry
	// Handler creates the HTTP handler for responder endpoints.
	Handler() *Handler
}

type responder struct {
	bus       bus.System
	subject   string
	maxLedger int
	ledger    []LedgerEntry
	mu        sync.RWMutex
	logger    *slog.Logger
}

// New creates a responder system. The subject controls which NATS subject the
// responder subscribes to; in production this is contracts.SubjectWildcard, but
// tests may supply an isolated subject to avoid cross-test interference.
func New(
	b bus.System,
	subject string,
	maxLedger int,
	logger *slog.Logger,
) System {
	return &responder{
		bus:       b,
		subject:   subject,
		maxLedger: maxLedger,
		ledger:    make([]LedgerEntry, 0, maxLedger),
		logger:    logger.With("domain", "responder"),
	}
}

func (r *responder) Subscribe() error {
	return r.bus.Subscribe(r.subject, r.onCommand)
}

func (r *responder) Ledger() []LedgerEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]LedgerEntry, len(r.ledger))
	copy(result, r.ledger)
	return result
}

func (r *responder) Handler() *Handler {
	return &Handler{
		responder: r,
		logger:    r.logger,
	}
}

func (r *responder) onCommand(msg *nats.Msg) {
	sig, err := signal.Decode(msg.Data)
	if err != nil {
		r.logger.Error("failed to decode command signal", "error", err)
		return
	}

	var cmd contracts.Command
	if err := json.Unmarshal(sig.Data, &cmd); err != nil {
		r.logger.Error("failed to unmarshal command", "error", err)
		return
	}

	action := extractAction(msg.Subject)
	resp := r.dispatch(action, cmd)

	entry := LedgerEntry{
		Command:  cmd,
		Response: resp,
	}
	r.appendLedger(entry)

	replySig, err := signal.New("responder", msg.Subject, resp)
	if err != nil {
		r.logger.Error("failed to create reply signal", "error", err)
		return
	}

	replyData, err := signal.Encode(replySig)
	if err != nil {
		r.logger.Error("failed to encode reply signal", "error", err)
		return
	}

	if err := msg.Respond(replyData); err != nil {
		r.logger.Error("failed to respond", "error", err)
	}

	r.logger.Info(
		"command handled",
		"action", action,
		"command_id", cmd.ID,
		"status", resp.Status,
	)
}

func (r *responder) dispatch(action string, cmd contracts.Command) contracts.Response {
	now := time.Now().Format(time.RFC3339)

	switch contracts.Action(action) {
	case contracts.ActionPing:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "pong",
			HandledAt: now,
		}
	case contracts.ActionFlush:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "flushed",
			HandledAt: now,
		}
	case contracts.ActionRotate:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "rotated",
			HandledAt: now,
		}
	case contracts.ActionNoop:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusOK,
			Result:    "noop",
			HandledAt: now,
		}
	default:
		return contracts.Response{
			CommandID: cmd.ID,
			Status:    contracts.StatusError,
			Result:    fmt.Sprintf("unknown action: %s", action),
			HandledAt: now,
		}
	}
}

func (r *responder) appendLedger(entry LedgerEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.ledger) >= r.maxLedger {
		r.ledger = r.ledger[1:]
	}
	r.ledger = append(r.ledger, entry)
}

func extractAction(subject string) string {
	parts := strings.Split(subject, ".")
	return parts[len(parts)-1]
}
