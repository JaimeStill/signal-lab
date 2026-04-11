package runners

import (
	"fmt"
	"log/slog"
	"maps"
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

// RunnerStatus is a point-in-time snapshot of a single runner.
type RunnerStatus struct {
	Subscribed bool             `json:"subscribed"`
	Counts     map[string]int64 `json:"counts"`
}

// Runner is a single queue-group subscriber with its own identity, subscription
// handle, and per-subject message counters. Runners are the unit of work
// distribution within a cluster — each runner is one member of a NATS queue
// group, and NATS delivers any given job to exactly one runner.
type Runner struct {
	ID     string
	counts map[string]int64
	sub    *nats.Subscription
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewRunner creates a runner with an ID derived from its position in the
// cluster. It does not attach to NATS; call Subscribe to join a queue group
// on a live connection.
func NewRunner(index int, logger *slog.Logger) *Runner {
	id := fmt.Sprintf("runner-%d", index)
	return &Runner{
		ID:     id,
		counts: make(map[string]int64),
		logger: logger.With("runner_id", id),
	}
}

// Subscribe attaches the runner to the given subject and queue group on the
// provided NATS connection. Idempotent-via-error: returns an error if the
// runner is already subscribed.
func (r *Runner) Subscribe(nc *nats.Conn, subject, queue string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sub != nil {
		return fmt.Errorf("runner %s already subscribed", r.ID)
	}

	sub, err := nc.QueueSubscribe(subject, queue, r.handle)
	if err != nil {
		return fmt.Errorf("queue subscribe: %w", err)
	}

	r.sub = sub
	return nil
}

// Unsubscribe drains the runner's subscription, letting any in-flight messages
// complete processing before detaching. Returns an error if not subscribed.
func (r *Runner) Unsubscribe() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sub == nil {
		return fmt.Errorf("runner %s not subscribed", r.ID)
	}

	if err := r.sub.Drain(); err != nil {
		return fmt.Errorf("drain: %w", err)
	}

	r.sub = nil
	return nil
}

// Subscribed reports whether the runner is currently attached to a NATS
// subject. Cheap state check that does not allocate.
func (r *Runner) Subscribed() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sub != nil
}

// Status returns a point-in-time snapshot of this runner's subscription state
// and per-subject message counts. The returned counts map is a copy and is
// safe to read without locking.
func (r *Runner) Status() RunnerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[string]int64, len(r.counts))
	maps.Copy(counts, r.counts)

	return RunnerStatus{
		Subscribed: r.sub != nil,
		Counts:     counts,
	}
}

// handle is the runner's NATS message handler. Decodes the signal envelope,
// extracts header metadata for priority-based logging, and increments the
// per-subject counter.
func (r *Runner) handle(msg *nats.Msg) {
	sig, err := signal.Decode(msg.Data)
	if err != nil {
		r.logger.Error("failed to decode job", "error", err)
		return
	}

	priority := msg.Header.Get(contracts.HeaderPriority)
	jobID := msg.Header.Get(contracts.HeaderJobID)
	jobType := msg.Header.Get(contracts.HeaderType)
	traceID := msg.Header.Get(contracts.HeaderTraceID)

	switch contracts.JobPriority(priority) {
	case contracts.PriorityHigh:
		r.logger.Warn(
			"high priority job",
			"job_id", jobID,
			"type", jobType,
			"trace_id", traceID,
		)
	default:
		r.logger.Info(
			"job handled",
			"job_id", jobID,
			"priority", priority,
			"type", jobType,
		)
	}

	r.mu.Lock()
	r.counts[msg.Subject]++
	r.mu.Unlock()

	_ = sig // envelope decoded for validation; body unused here
}
