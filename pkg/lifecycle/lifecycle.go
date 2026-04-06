package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ReadinessChecker reports whether a subsystem is ready to serve traffic.
type ReadinessChecker interface {
	Ready() bool
}

// Coordniator manages startup and shutdown hooks for the application lifecycle.
type Coordinator struct {
	checkers   []ReadinessChecker
	ctx        context.Context
	cancel     context.CancelFunc
	startupWg  sync.WaitGroup
	shutdownWg sync.WaitGroup
	ready      bool
	readyMu    sync.RWMutex
}

// New creates a Coordinator with a cancellable context.
func New() *Coordinator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Coordinator{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context return the coordinator's context, cancelled on shutdown.
func (c *Coordinator) Context() context.Context {
	return c.ctx
}

// OnStartup registers a function to run concurrently during startup.
func (c *Coordinator) OnStartup(fn func()) {
	c.startupWg.Go(fn)
}

// OnShutdown registers a function to run concurrently during shutdown.
// Shutdown hooks should block on <-c.Context().Done() before executing cleanup.
func (c *Coordinator) OnShutdown(fn func()) {
	c.shutdownWg.Go(fn)
}

// RegisterChecker adds a ReadinessChecker that Ready() will consult.
func (c *Coordinator) RegisterChecker(rc ReadinessChecker) {
	c.checkers = append(c.checkers, rc)
}

// Ready returns true after all startup hooks have completed.
func (c *Coordinator) Ready() bool {
	c.readyMu.RLock()
	defer c.readyMu.RUnlock()
	if !c.ready {
		return false
	}
	for _, rc := range c.checkers {
		if !rc.Ready() {
			return false
		}
	}
	return true
}

// WaitForStartup blocks until all startup hooks have completed and sets the ready flag.
func (c *Coordinator) WaitForStartup() {
	c.startupWg.Wait()
	c.readyMu.Lock()
	c.ready = true
	c.readyMu.Unlock()
}

// Shutdown cancels the context and waits for shutdown hooks to complete
// within the given timeout.
func (c *Coordinator) Shutdown(timeout time.Duration) error {
	c.cancel()

	done := make(chan struct{})
	go func() {
		c.shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("shutdown timeout after %v", timeout)
	}
}
