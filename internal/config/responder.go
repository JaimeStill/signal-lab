package config

import (
	"fmt"
	"os"
	"strconv"
)

// EnvResponderMaxLedger overrides the maximum number of ledger entries retained.
const EnvResponderMaxLedger = "SIGNAL_RESPONDER_MAX_LEDGER"

// ResponderConfig holds command responder parameters for the beta service.
type ResponderConfig struct {
	MaxLedger int `json:"max_ledger"`
}

// Finalize applies defaults, environment overrides, and validation.
func (c *ResponderConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *ResponderConfig) Merge(overlay *ResponderConfig) {
	if overlay.MaxLedger != 0 {
		c.MaxLedger = overlay.MaxLedger
	}
}

func (c *ResponderConfig) loadDefaults() {
	if c.MaxLedger == 0 {
		c.MaxLedger = 64
	}
}

func (c *ResponderConfig) loadEnv() {
	if v := os.Getenv(EnvResponderMaxLedger); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxLedger = n
		}
	}
}

func (c *ResponderConfig) validate() error {
	if c.MaxLedger < 1 {
		return fmt.Errorf("responder max_ledger must be >= 1, got %d", c.MaxLedger)
	}
	return nil
}
