package infrastructure_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JaimeStill/signal-lab/internal/config"
	"github.com/JaimeStill/signal-lab/internal/infrastructure"
)

func TestNew(t *testing.T) {
	cfg := loadTestConfig(t)
	logger := slog.With("service", cfg.Alpha.Name)

	infra := infrastructure.New(cfg, &cfg.Alpha.ServiceConfig, logger)

	if infra.Lifecycle == nil {
		t.Fatal("expected non-nil Lifecycle")
	}
	if infra.Bus == nil {
		t.Fatal("expected non-nil Bus")
	}
	if infra.Logger == nil {
		t.Fatal("expected non-nil Logger")
	}
	if infra.ShutdownTimeout != cfg.ShutdownTimeoutDuration() {
		t.Fatalf("expected shutdown timeout %v, got %v", cfg.ShutdownTimeoutDuration(), infra.ShutdownTimeout)
	}
}

func TestServiceInfo(t *testing.T) {
	cfg := loadTestConfig(t)

	tests := []struct {
		name string
		svc  *config.ServiceConfig
	}{
		{"alpha", &cfg.Alpha.ServiceConfig},
		{"beta", &cfg.Beta.ServiceConfig},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.With("service", tc.svc.Name)
			infra := infrastructure.New(cfg, tc.svc, logger)

			if infra.Info.ID == "" {
				t.Fatal("expected non-empty ID")
			}
			if infra.Info.Name != tc.svc.Name {
				t.Fatalf("expected name %q, got %q", tc.svc.Name, infra.Info.Name)
			}

			expectedEndpoint := "http://" + tc.svc.Addr()
			if infra.Info.Endpoint != expectedEndpoint {
				t.Fatalf("expected endpoint %q, got %q", expectedEndpoint, infra.Info.Endpoint)
			}
			if infra.Info.Description != tc.svc.Description {
				t.Fatalf("expected description %q, got %q", tc.svc.Description, infra.Info.Description)
			}
		})
	}
}

func TestUniqueIDs(t *testing.T) {
	cfg := loadTestConfig(t)
	logger := slog.Default()

	infra1 := infrastructure.New(cfg, &cfg.Alpha.ServiceConfig, logger)
	infra2 := infrastructure.New(cfg, &cfg.Alpha.ServiceConfig, logger)

	if infra1.Info.ID == infra2.Info.ID {
		t.Fatal("expected unique IDs across Infrastructure instances")
	}
}

func TestStartRequiresNATS(t *testing.T) {
	cfg := loadTestConfig(t)
	cfg.Bus.Addr = "nats://localhost:19999"
	logger := slog.Default()

	infra := infrastructure.New(cfg, &cfg.Alpha.ServiceConfig, logger)

	err := infra.Start()
	if err == nil {
		t.Fatal("expected error when NATS is unreachable")
	}
	if !strings.Contains(err.Error(), "connect") {
		t.Fatalf("expected connection error, got: %v", err)
	}
}

func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()

	dir := t.TempDir()
	data := []byte(`{
		"bus": {"url": "nats://localhost:4222", "max_reconnects": 1, "reconnect_wait": "100ms", "response_timeout": "500ms"},
		"alpha": {"port": 3000, "name": "alpha", "description": "test alpha"},
		"beta": {"port": 3001, "name": "beta", "description": "test beta"},
		"shutdown_timeout": "5s"
	}`)

	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0644); err != nil {
		t.Fatal("write config:", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal("getwd:", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal("chdir:", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })

	cfg, err := config.Load()
	if err != nil {
		t.Fatal("load config:", err)
	}

	return cfg
}
