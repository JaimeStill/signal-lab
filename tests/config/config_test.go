package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JaimeStill/signal-lab/internal/config"
)

func TestLoadDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "config.json", `{
		"bus": {"url": "nats://localhost:4222", "max_reconnects": 5, "reconnect_wait": "1s", "response_timeout": "500ms"},
		"alpha": {"port": 3000, "name": "alpha", "description": "test alpha"},
		"beta": {"port": 3001, "name": "beta", "description": "test beta"},
		"shutdown_timeout": "10s"
	}`)

	chdir(t, dir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal("load failed:", err)
	}

	if cfg.Bus.URL() != "nats://localhost:4222" {
		t.Fatalf("expected bus URL 'nats://localhost:4222', got %q", cfg.Bus.URL())
	}
	if cfg.Alpha.Port != 3000 {
		t.Fatalf("expected alpha port 3000, got %d", cfg.Alpha.Port)
	}
	if cfg.Beta.Port != 3001 {
		t.Fatalf("expected beta port 3001, got %d", cfg.Beta.Port)
	}
}

func TestEnvVarOverrides(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "config.json", `{
		"bus": {"url": "nats://localhost:4222", "max_reconnects": 5, "reconnect_wait": "1s", "response_timeout": "500ms"},
		"alpha": {"port": 3000, "name": "alpha", "description": "test"},
		"beta": {"port": 3001, "name": "beta", "description": "test"},
		"shutdown_timeout": "10s"
	}`)

	chdir(t, dir)
	t.Setenv("SIGNAL_BUS_URL", "nats://override:4222")
	t.Setenv("SIGNAL_SHUTDOWN_TIMEOUT", "60s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal("load failed:", err)
	}

	if cfg.Bus.URL() != "nats://override:4222" {
		t.Fatalf("expected bus URL 'nats://override:4222', got %q", cfg.Bus.URL())
	}
	if cfg.ShutdownTimeoutDuration() != 60*time.Second {
		t.Fatalf("expected shutdown timeout 60s, got %v", cfg.ShutdownTimeoutDuration())
	}
}

func TestMerge(t *testing.T) {
	base := config.Config{
		ShutdownTimeout: "10s",
	}
	overlay := config.Config{
		ShutdownTimeout: "30s",
	}

	base.Merge(&overlay)

	if base.ShutdownTimeout != "30s" {
		t.Fatalf("expected '30s', got %q", base.ShutdownTimeout)
	}
}

func TestShutdownTimeoutDuration(t *testing.T) {
	cfg := config.Config{ShutdownTimeout: "15s"}
	if cfg.ShutdownTimeoutDuration() != 15*time.Second {
		t.Fatalf("expected 15s, got %v", cfg.ShutdownTimeoutDuration())
	}
}

func TestOverlayLoading(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "config.json", `{
		"bus": {"url": "nats://localhost:4222", "max_reconnects": 5, "reconnect_wait": "1s", "response_timeout": "500ms"},
		"alpha": {"port": 3000, "name": "alpha", "description": "base"},
		"beta": {"port": 3001, "name": "beta", "description": "base"},
		"shutdown_timeout": "10s"
	}`)
	writeConfig(t, dir, "config.docker.json", `{
		"bus": {"url": "nats://signal-nats:4222"}
	}`)

	chdir(t, dir)
	t.Setenv("SIGNAL_ENV", "docker")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal("load failed:", err)
	}

	if cfg.Bus.URL() != "nats://signal-nats:4222" {
		t.Fatalf("expected overlay URL 'nats://signal-nats:4222', got %q", cfg.Bus.URL())
	}
}

func writeConfig(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal("write config:", err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal("getwd:", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal("chdir:", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}
