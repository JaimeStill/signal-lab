package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"

	contracts "github.com/JaimeStill/signal-lab/pkg/contracts/jobs"
)

// Status reports the current state of the jobs publisher.
type Status struct {
	Running  bool   `json:"running"`
	Interval string `json:"interval"`
}

// System manages job publishing over the bus.
type System interface {
	// Start begins publishing simulated jobs at the configured interval.
	Start() error
	// Stop stops the publisher.
	Stop() error
	// Status returns the current publisher state.
	Status() Status
	// Handler creates the HTTP handler for jobs endpoints.
	Handler() *Handler
}

type jobs struct {
	bus      bus.System
	source   string
	interval time.Duration
	running  bool
	cancel   context.CancelFunc
	mu       sync.Mutex
	logger   *slog.Logger
}

// New creates a jobs system.
func New(
	b bus.System,
	source string,
	interval time.Duration,
	logger *slog.Logger,
) System {
	return &jobs{
		bus:      b,
		source:   source,
		interval: interval,
		logger:   logger.With("domain", "jobs"),
	}
}

// Start begins publishing simulated jobs at the configured interval.
func (j *jobs) Start() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.running {
		return fmt.Errorf("publisher already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	j.cancel = cancel
	j.running = true

	go j.publish(ctx)

	j.logger.Info("publisher started", "interval", j.interval)
	return nil
}

// Stop stops the publisher.
func (j *jobs) Stop() error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if !j.running {
		return fmt.Errorf("publisher not running")
	}

	j.cancel()
	j.running = false

	j.logger.Info("publisher stopped")
	return nil
}

// Status returns the current publisher state.
func (j *jobs) Status() Status {
	j.mu.Lock()
	defer j.mu.Unlock()

	return Status{
		Running:  j.running,
		Interval: j.interval.String(),
	}
}

// Handler creates the HTTP handler for jobs endpoints.
func (j *jobs) Handler() *Handler {
	return &Handler{
		jobs:   j,
		logger: j.logger,
	}
}

func (j *jobs) publish(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobType := randomType()
			priority := randomPriority()

			job := contracts.Job{
				ID:       fmt.Sprintf("job-%d", time.Now().UnixNano()),
				Type:     jobType,
				Priority: priority,
				Payload:  payloadFor(jobType),
			}

			subject := fmt.Sprintf("%s.%s", contracts.SubjectPrefix, jobType)

			sig, err := signal.New(j.source, subject, job)
			if err != nil {
				j.logger.Error("failed to create signal", "error", err)
				continue
			}

			data, err := signal.Encode(sig)
			if err != nil {
				j.logger.Error("failed to encode signal", "error", err)
				continue
			}

			msg := &nats.Msg{
				Subject: subject,
				Data:    data,
				Header:  nats.Header{},
			}
			msg.Header.Set(contracts.HeaderJobID, job.ID)
			msg.Header.Set(contracts.HeaderPriority, string(priority))
			msg.Header.Set(contracts.HeaderType, string(jobType))
			msg.Header.Set(contracts.HeaderTraceID, sig.ID)

			if err := j.bus.Conn().PublishMsg(msg); err != nil {
				j.logger.Error(
					"failed to publish job",
					"subject", subject,
					"error", err,
				)
			}
		}
	}
}

// randomType returns a uniformly distributed job type.
func randomType() contracts.JobType {
	types := []contracts.JobType{
		contracts.TypeCompute,
		contracts.TypeIO,
		contracts.TypeAnalysis,
	}
	return types[rand.IntN(len(types))]
}

// randomPriority returns a weighted random priority (~60% normal, ~25% low, ~15% high).
func randomPriority() contracts.JobPriority {
	n := rand.IntN(100)
	switch {
	case n < 15:
		return contracts.PriorityHigh
	case n < 40:
		return contracts.PriorityLow
	default:
		return contracts.PriorityNormal
	}
}

func payloadFor(t contracts.JobType) string {
	switch t {
	case contracts.TypeCompute:
		return "compute batch work"
	case contracts.TypeIO:
		return "read/write operation"
	case contracts.TypeAnalysis:
		return "analysis task"
	default:
		return "generic workload"
	}
}
