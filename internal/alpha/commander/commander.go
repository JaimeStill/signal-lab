// Package commander implements the alpha-side command dispatch domain. It issues
// commands to the responder via NATS request/reply and maintains a bounded
// in-memory history of issued commands and their outcomes.
package commander

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/commands"
)

// HistoryEntry records a single issued command along with its outcome — either
// a successful response or a timeout error.
type HistoryEntry struct {
	Command  contracts.Command   `json:"command"`
	Response *contracts.Response `json:"response,omitempty"`
	Error    string              `json:"error,omitempty"`
	At       time.Time           `json:"at"`
}

// System manages command dispatch over the bus via NATS request/reply.
type System interface {
	// Issue sends a command to the responder and waits for a reply or timeout.
	Issue(action, payload string) (*contracts.Response, error)
	// History returns recent issued commands with their outcomes, newest first.
	History() []HistoryEntry
	// Handler creates the HTTP handler for commander endpoints.
	Handler() *Handler
}

type commander struct {
	bus           bus.System
	source        string
	subjectPrefix string
	timeout       time.Duration
	maxHistory    int
	history       []HistoryEntry
	mu            sync.Mutex
	logger        *slog.Logger
}

// New creates a commander system. The subjectPrefix controls the NATS subject
// namespace; in production this is contracts.SubjectPrefix, but tests may
// supply an isolated prefix to avoid cross-test interference.
func New(
	b bus.System,
	source string,
	subjectPrefix string,
	timeout time.Duration,
	maxHistory int,
	logger *slog.Logger,
) System {
	return &commander{
		bus:           b,
		source:        source,
		subjectPrefix: subjectPrefix,
		timeout:       timeout,
		maxHistory:    maxHistory,
		history:       make([]HistoryEntry, 0, maxHistory),
		logger:        logger.With("domain", "commander"),
	}
}

func (c *commander) Issue(action, payload string) (*contracts.Response, error) {
	cmd := contracts.Command{
		ID:       uuid.New().String(),
		Action:   contracts.Action(action),
		Payload:  payload,
		IssuedAt: time.Now().Format(time.RFC3339),
	}

	subject := fmt.Sprintf("%s.%s", c.subjectPrefix, action)

	sig, err := signal.New(c.source, subject, cmd)
	if err != nil {
		return nil, fmt.Errorf("create signal: %w", err)
	}

	data, err := signal.Encode(sig)
	if err != nil {
		return nil, fmt.Errorf("encode signal: %w", err)
	}

	msg, err := c.bus.Conn().Request(subject, data, c.timeout)
	if err != nil {
		entry := HistoryEntry{
			Command: cmd,
			Error:   err.Error(),
			At:      time.Now(),
		}
		c.appendHistory(entry)

		c.logger.Error(
			"command timeout",
			"action", action,
			"command_id", cmd.ID,
			"error", err,
		)
		return nil, err
	}

	replySig, err := signal.Decode(msg.Data)
	if err != nil {
		return nil, fmt.Errorf("decode reply signal: %w", err)
	}

	var resp contracts.Response
	if err := json.Unmarshal(replySig.Data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	entry := HistoryEntry{
		Command:  cmd,
		Response: &resp,
		At:       time.Now(),
	}
	c.appendHistory(entry)

	c.logger.Info(
		"command replied",
		"action", action,
		"command_id", cmd.ID,
		"status", resp.Status,
	)

	return &resp, nil
}

func (c *commander) History() []HistoryEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]HistoryEntry, len(c.history))
	copy(result, c.history)
	slices.Reverse(result)
	return result
}

func (c *commander) Handler() *Handler {
	return &Handler{
		commander: c,
		logger:    c.logger,
	}
}

func (c *commander) appendHistory(entry HistoryEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.history) >= c.maxHistory {
		c.history = c.history[1:]
	}
	c.history = append(c.history, entry)
}
