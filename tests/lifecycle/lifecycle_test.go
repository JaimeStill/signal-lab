package lifecycle_test

import (
	"testing"
	"time"

	"github.com/JaimeStill/signal-lab/pkg/lifecycle"
)

func TestReadyFalseBeforeStartup(t *testing.T) {
	lc := lifecycle.New()
	if lc.Ready() {
		t.Fatal("expected Ready() to be false before WaitForStartup")
	}
}

func TestStartupHooksRun(t *testing.T) {
	lc := lifecycle.New()
	ran := false

	lc.OnStartup(func() {
		ran = true
	})
	lc.WaitForStartup()

	if !ran {
		t.Fatal("startup hook did not run")
	}
	if !lc.Ready() {
		t.Fatal("expected Ready() to be true after WaitForStartup")
	}
}

func TestShutdownCancelsContext(t *testing.T) {
	lc := lifecycle.New()
	lc.WaitForStartup()

	if err := lc.Shutdown(5 * time.Second); err != nil {
		t.Fatal("shutdown failed:", err)
	}

	select {
	case <-lc.Context().Done():
	default:
		t.Fatal("expected context to be cancelled after shutdown")
	}
}

func TestShutdownRunsHooks(t *testing.T) {
	lc := lifecycle.New()
	ran := false

	lc.OnShutdown(func() {
		<-lc.Context().Done()
		ran = true
	})
	lc.WaitForStartup()

	if err := lc.Shutdown(5 * time.Second); err != nil {
		t.Fatal("shutdown failed:", err)
	}

	if !ran {
		t.Fatal("shutdown hook did not run")
	}
}

func TestShutdownTimeout(t *testing.T) {
	lc := lifecycle.New()

	lc.OnShutdown(func() {
		<-lc.Context().Done()
		time.Sleep(1 * time.Second)
	})
	lc.WaitForStartup()

	err := lc.Shutdown(10 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestReadinessChecker(t *testing.T) {
	lc := lifecycle.New()

	checker := &mockChecker{ready: true}
	lc.RegisterChecker(checker)
	lc.WaitForStartup()

	if !lc.Ready() {
		t.Fatal("expected Ready() to be true when checker reports ready")
	}

	checker.ready = false
	if lc.Ready() {
		t.Fatal("expected Ready() to be false when checker reports not ready")
	}
}

type mockChecker struct {
	ready bool
}

func (m *mockChecker) Ready() bool {
	return m.ready
}
