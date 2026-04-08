package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	EnvTelemetryInterval = "SIGNAL_TELEMETRY_INTERVAL"
	EnvTelemetryTypes    = "SIGNAL_TELEMETRY_TYPES"
	EnvTelemetryZones    = "SIGNAL_TELEMETRY_ZONES"
)

// TelemetryConfig holds telemetry publisher parameters.
type TelemetryConfig struct {
	Interval string   `json:"interval"`
	Types    []string `json:"types"`
	Zones    []string `json:"zones"`
}

// IntervalDuration returns Interval as a time.Duration.
func (c *TelemetryConfig) IntervalDuration() time.Duration {
	d, _ := time.ParseDuration(c.Interval)
	return d
}

// Finalize applies defaults, environment overrides, and validation.
func (c *TelemetryConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *TelemetryConfig) Merge(overlay *TelemetryConfig) {
	if overlay.Interval != "" {
		c.Interval = overlay.Interval
	}
	if len(overlay.Types) > 0 {
		c.Types = overlay.Types
	}
	if len(overlay.Zones) > 0 {
		c.Zones = overlay.Zones
	}
}

func (c *TelemetryConfig) loadDefaults() {
	if c.Interval == "" {
		c.Interval = "2s"
	}
	if len(c.Types) == 0 {
		c.Types = []string{
			"temp",
			"humidity",
			"pressure",
		}
	}
	if len(c.Zones) == 0 {
		c.Zones = []string{
			"zone-a",
			"zone-b",
		}
	}
}

func (c *TelemetryConfig) loadEnv() {
	if v := os.Getenv(EnvTelemetryInterval); v != "" {
		c.Interval = v
	}
	if v := os.Getenv(EnvTelemetryTypes); v != "" {
		c.Types = strings.Split(v, ",")
	}
	if v := os.Getenv(EnvTelemetryZones); v != "" {
		c.Zones = strings.Split(v, ",")
	}
}

func (c *TelemetryConfig) validate() error {
	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid telemetry interval: %w", err)
	}
	if len(c.Types) == 0 {
		return fmt.Errorf("telemetry types must not be empty")
	}
	if len(c.Zones) == 0 {
		return fmt.Errorf("telemetry zones must not be empty")
	}
	return nil
}
