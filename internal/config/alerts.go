package config

import (
	"fmt"
	"os"
	"time"
)

const (
	EnvAlertsInterval = "SIGNAL_ALERTS_INTERVAL"
)

// AlertsConfig holds alert publisher parameters.
type AlertsConfig struct {
	Interval string `json:"interval"`
}

// IntervalDuration returns Interval as a time.Duration.
func (c *AlertsConfig) IntervalDuration() time.Duration {
	d, _ := time.ParseDuration(c.Interval)
	return d
}

// Finalize applies defaults, environment overrides, and validation.
func (c *AlertsConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *AlertsConfig) Merge(overlay *AlertsConfig) {
	if overlay.Interval != "" {
		c.Interval = overlay.Interval
	}
}

func (c *AlertsConfig) loadDefaults() {
	if c.Interval == "" {
		c.Interval = "3s"
	}
}

func (c *AlertsConfig) loadEnv() {
	if v := os.Getenv(EnvAlertsInterval); v != "" {
		c.Interval = v
	}
}

func (c *AlertsConfig) validate() error {
	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid alerts interval: %w", err)
	}
	return nil
}
