package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	BaseConfigFile       = "config.json"
	OverlayConfigPattern = "config.%s.json"
	SecretsConfigFile    = "secrets.json"

	EnvSignalEnv       = "SIGNAL_ENV"
	EnvShutdownTimeout = "SIGNAL_SHUTDOWN_TIMEOUT"
)

// Config is the root configuration for signal-lab services.
type Config struct {
	Bus             BusConfig     `json:"bus"`
	Sensor          ServiceConfig `json:"sensor"`
	Dispatch        ServiceConfig `json:"dispatch"`
	ShutdownTimeout string        `json:"shutdown_timeout"`
}

// Env returns the SIGNAL_ENV value, defaulting to "local".
func (c *Config) Env() string {
	if env := os.Getenv(EnvSignalEnv); env != "" {
		return env
	}
	return "local"
}

// ShutdownTimeoutDuration return ShutdownTimeout as a time.Duration.
func (c *Config) ShutdownTimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(c.ShutdownTimeout)
	return d
}

// Load reads the base config, applies any environment overlay and secrets,
// then finalizes all values.
func Load() (*Config, error) {
	cfg := &Config{}

	if _, err := os.Stat(BaseConfigFile); err == nil {
		loaded, err := load(BaseConfigFile)
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}

	if path := overlayPath(); path != "" {
		overlay, err := load(path)
		if err != nil {
			return nil, fmt.Errorf("load overlay %s: %w", path, err)
		}
		cfg.Merge(overlay)
	}

	if _, err := os.Stat(SecretsConfigFile); err == nil {
		secrets, err := load(SecretsConfigFile)
		if err != nil {
			return nil, err
		}
		cfg.Merge(secrets)
	}

	if err := cfg.finalize(); err != nil {
		return nil, fmt.Errorf("finalize config: %w", err)
	}

	return cfg, nil
}

// Merge overwrites non-zero fields from overlay across all sub-configs.
func (c *Config) Merge(overlay *Config) {
	if overlay.ShutdownTimeout != "" {
		c.ShutdownTimeout = overlay.ShutdownTimeout
	}
	c.Bus.Merge(&overlay.Bus)
	c.Sensor.Merge(&overlay.Sensor)
	c.Dispatch.Merge(&overlay.Dispatch)
}

func (c *Config) finalize() error {
	c.loadDefaults()
	c.loadEnv()

	if err := c.validate(); err != nil {
		return err
	}
	if err := c.Bus.Finalize(); err != nil {
		return fmt.Errorf("bus: %w", err)
	}
	if err := c.Sensor.Finalize("SENSOR"); err != nil {
		return fmt.Errorf("sensor: %w", err)
	}
	if err := c.Sensor.Telemetry.Finalize(); err != nil {
		return fmt.Errorf("sensor telemetry: %w", err)
	}
	if err := c.Dispatch.Finalize("DISPATCH"); err != nil {
		return fmt.Errorf("dispatch: %w", err)
	}
	return nil
}

func (c *Config) loadDefaults() {
	if c.ShutdownTimeout == "" {
		c.ShutdownTimeout = "30s"
	}
}

func (c *Config) loadEnv() {
	if v := os.Getenv(EnvShutdownTimeout); v != "" {
		c.ShutdownTimeout = v
	}
}

func (c *Config) validate() error {
	if _, err := time.ParseDuration(c.ShutdownTimeout); err != nil {
		return fmt.Errorf("invalid shutdown_timeout: %w", err)
	}
	return nil
}

func load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

func overlayPath() string {
	if env := os.Getenv(EnvSignalEnv); env != "" {
		path := fmt.Sprintf(OverlayConfigPattern, env)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
