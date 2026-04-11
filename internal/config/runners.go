package config

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
)

const EnvRunnersCount = "SIGNAL_RUNNERS_COUNT"

// RunnersConfig holds runner cluster sizing parameters. Count is encoded as a
// string so the literal "auto" can be distinguished from an explicit positive
// integer; "auto" resolves to runtime.NumCPU() at access time via Number().
type RunnersConfig struct {
	Count string `json:"count"`
}

// Number resolves the configured count to its numeric value, auto-sizing to
// the number of available CPU threads when "auto" is specified. validate()
// guarantees the string is parseable before this is called.
func (c *RunnersConfig) Number() int {
	if c.Count == "auto" {
		return runtime.NumCPU()
	}
	n, _ := strconv.Atoi(c.Count)
	return n
}

// Finalize applies defaults, environment overrides, and validation.
func (c *RunnersConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-empty fields from overlay.
func (c *RunnersConfig) Merge(overlay *RunnersConfig) {
	if overlay.Count != "" {
		c.Count = overlay.Count
	}
}

func (c *RunnersConfig) loadDefaults() {
	if c.Count == "" {
		c.Count = "auto"
	}
}

func (c *RunnersConfig) loadEnv() {
	if v := os.Getenv(EnvRunnersCount); v != "" {
		c.Count = v
	}
}

func (c *RunnersConfig) validate() error {
	if c.Count == "auto" {
		return nil
	}
	n, err := strconv.Atoi(c.Count)
	if err != nil {
		return fmt.Errorf("invalid runners count %q: must be \"auto\" or a positive integer", c.Count)
	}
	if n < 1 {
		return fmt.Errorf("runners count must be >= 1, got %d", n)
	}
	return nil
}
