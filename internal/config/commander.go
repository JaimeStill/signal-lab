package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	// EnvCommanderTimeout overrides the request timeout for command dispatch.
	EnvCommanderTimeout = "SIGNAL_COMMANDER_TIMEOUT"
	// EnvCommanderMaxHistory overrides the maximum number of history entries retained.
	EnvCommanderMaxHistory = "SIGNAL_COMMANDER_MAX_HISTORY"
)

// CommanderConfig holds command dispatch parameters for the alpha service.
type CommanderConfig struct {
	Timeout    string `json:"timeout"`
	MaxHistory int    `json:"max_history"`
}

// TimeoutDuration returns Timeout as a time.Duration.
func (c *CommanderConfig) TimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(c.Timeout)
	return d
}

// Finalize applies defaults, environment overrides, and validation.
func (c *CommanderConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *CommanderConfig) Merge(overlay *CommanderConfig) {
	if overlay.Timeout != "" {
		c.Timeout = overlay.Timeout
	}
	if overlay.MaxHistory != 0 {
		c.MaxHistory = overlay.MaxHistory
	}
}

func (c *CommanderConfig) loadDefaults() {
	if c.Timeout == "" {
		c.Timeout = "2s"
	}
	if c.MaxHistory == 0 {
		c.MaxHistory = 64
	}
}

func (c *CommanderConfig) loadEnv() {
	if v := os.Getenv(EnvCommanderTimeout); v != "" {
		c.Timeout = v
	}
	if v := os.Getenv(EnvCommanderMaxHistory); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxHistory = n
		}
	}
}

func (c *CommanderConfig) validate() error {
	if _, err := time.ParseDuration(c.Timeout); err != nil {
		return fmt.Errorf("invalid commander timeout: %w", err)
	}
	if c.MaxHistory < 1 {
		return fmt.Errorf("commander max_history mst be >= 1, got %d", c.MaxHistory)
	}
	return nil
}
