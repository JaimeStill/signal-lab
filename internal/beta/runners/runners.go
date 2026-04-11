package runners

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/JaimeStill/signal-lab/pkg/bus"
)

// QueueGroup is the standard queue group identifier for the beta runner
// cluster. Tests may construct clusters with a different queue group to
// achieve isolation from concurrent test packages sharing the same NATS
// server.
const QueueGroup = "beta-runners"

// Status reports the cluster state and a snapshot of every runner. The cluster
// Subscribed flag is true only when every runner is currently attached.
type Status struct {
	Subscribed bool                    `json:"subscribed"`
	Count      int                     `json:"count"`
	Runners    map[string]RunnerStatus `json:"runners"`
}

// System manages the runner cluster: cluster-level lifecycle, per-runner
// lifecycle, and snapshot composition.
type System interface {
	// Subscribe attaches every runner that is not already subscribed.
	Subscribe() error
	// Unsubscribe drains every runner that is currently subscribed.
	Unsubscribe() error
	// SubscribeRunner attaches a single runner identified by ID.
	SubscribeRunner(id string) error
	// UnsubscribeRunner drains a single runner identified by ID.
	UnsubscribeRunner(id string) error
	// Status returns a snapshot of the cluster and every runner.
	Status() Status
	// Handler creates the HTTP handler for runner endpoints.
	Handler() *Handler
}

type cluster struct {
	bus        bus.System
	subject    string
	queue      string
	runners    map[string]*Runner
	mu         sync.RWMutex
	subscribed bool
	logger     *slog.Logger
}

// New creates a runner cluster with the given number of in-process runners,
// each identified by its position. The subject and queue parameters control
// what every runner subscribes to and which queue group it joins. Runners are
// constructed but not subscribed; call Subscribe to attach them to the bus.
func New(
	b bus.System,
	count int,
	subject, queue string,
	logger *slog.Logger,
) System {
	clusterLogger := logger.With("domain", "runners")
	runners := make(map[string]*Runner, count)

	for i := range count {
		r := NewRunner(i, clusterLogger)
		runners[r.ID] = r
	}

	return &cluster{
		bus:     b,
		subject: subject,
		queue:   queue,
		runners: runners,
		logger:  clusterLogger,
	}
}

// Subscribe attaches every runner in the cluster that is not currently
// subscribed. Per-runner failures are joined and returned. Idempotent: runners
// already attached are skipped, so calling on a fully-subscribed cluster is a
// no-op that returns nil.
func (c *cluster) Subscribe() error {
	var errs []error
	for _, r := range c.runners {
		if r.Subscribed() {
			continue
		}
		if err := c.SubscribeRunner(r.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Unsubscribe drains every runner in the cluster that is currently subscribed.
// Per-runner failures are joined and returned. Idempotent: runners already
// detached are skipped, so calling on a fully-unsubscribed cluster is a no-op
// that returns nil.
func (c *cluster) Unsubscribe() error {
	var errs []error
	for _, r := range c.runners {
		if !r.Subscribed() {
			continue
		}
		if err := c.UnsubscribeRunner(r.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// SubscribeRunner attaches a single runner identified by ID. Idempotent-via-
// error: returns an error if the runner is already subscribed or not found.
func (c *cluster) SubscribeRunner(id string) error {
	r, ok := c.runners[id]
	if !ok {
		return fmt.Errorf("runner %s not found", id)
	}
	return r.Subscribe(c.bus.Conn(), c.subject, c.queue)
}

// UnsubscribeRunner drains a single runner identified by ID. Idempotent-via-
// error: returns an error if the runner is not currently subscribed or not
// found.
func (c *cluster) UnsubscribeRunner(id string) error {
	r, ok := c.runners[id]
	if !ok {
		return fmt.Errorf("runner %s not found", id)
	}
	return r.Unsubscribe()
}

// Status returns a snapshot of the cluster and every runner's status. The
// cluster Subscribed flag reflects whether every runner is currently attached.
func (c *cluster) Status() Status {
	snapshot := make(map[string]RunnerStatus, len(c.runners))
	allSubscribed := true
	for id, r := range c.runners {
		rs := r.Status()
		snapshot[id] = rs
		if !rs.Subscribed {
			allSubscribed = false
		}
	}

	return Status{
		Subscribed: allSubscribed,
		Count:      len(c.runners),
		Runners:    snapshot,
	}
}

// Handler creates the HTTP handler for runner endpoints.
func (c *cluster) Handler() *Handler {
	return &Handler{
		cluster: c,
		logger:  c.logger,
	}
}
