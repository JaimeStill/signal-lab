package config

import (
	"fmt"
	"os"
	"time"
)

const EnvJobsInterval = "SIGNAL_JOBS_INTERVAL"

// JobsConfig holds job publisher parameters.
type JobsConfig struct {
	Interval string `json:"interval"`
}

// IntervalDuration returns Interval as a time.Duration.
func (c *JobsConfig) IntervalDuration() time.Duration {
	d, _ := time.ParseDuration(c.Interval)
	return d
}

// Finalize applies defaults, environment overrides, and validation.
func (c *JobsConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *JobsConfig) Merge(overlay *JobsConfig) {
	if overlay.Interval != "" {
		c.Interval = overlay.Interval
	}
}

func (c *JobsConfig) loadDefaults() {
	if c.Interval == "" {
		c.Interval = "500ms"
	}
}

func (c *JobsConfig) loadEnv() {
	if v := os.Getenv(EnvJobsInterval); v != "" {
		c.Interval = v
	}
}

func (c *JobsConfig) validate() error {
	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid jobs interval: %w", err)
	}
	return nil
}
