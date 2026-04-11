package config

import (
	"fmt"
	"os"
	"strings"
)

const EnvBetaZones = "SIGNAL_BETA_ZONES"

// BetaConfig is the beta service configuration. It embeds ServiceConfig for
// shared web service fields and adds the beta-specific Zones list,
// TelemetryConfig, and RunnersConfig sub-configs.
type BetaConfig struct {
	ServiceConfig
	Zones     []string        `json:"zones"`
	Telemetry TelemetryConfig `json:"telemetry"`
	Runners   RunnersConfig   `json:"runners"`
}

// Finalize applies defaults, environment overrides, validation,
// and finalizes sub-configs.
func (c *BetaConfig) Finalize(envPrefix string) error {
	if err := c.ServiceConfig.Finalize(envPrefix); err != nil {
		return err
	}

	c.loadDefaults()
	c.loadEnv()

	if err := c.validate(); err != nil {
		return err
	}
	if err := c.Telemetry.Finalize(); err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	if err := c.Runners.Finalize(); err != nil {
		return fmt.Errorf("runners: %w", err)
	}

	return nil
}

// Merge overwrites non-zero fields from overlay.
func (c *BetaConfig) Merge(overlay *BetaConfig) {
	c.ServiceConfig.Merge(&overlay.ServiceConfig)

	if len(overlay.Zones) > 0 {
		c.Zones = overlay.Zones
	}

	c.Telemetry.Merge(&overlay.Telemetry)
	c.Runners.Merge(&overlay.Runners)
}

func (c *BetaConfig) loadDefaults() {
	if len(c.Zones) == 0 {
		c.Zones = []string{
			"server-room",
			"ops-center",
		}
	}
}

func (c *BetaConfig) loadEnv() {
	if v := os.Getenv(EnvBetaZones); v != "" {
		c.Zones = strings.Split(v, ",")
	}
}

func (c *BetaConfig) validate() error {
	if len(c.Zones) == 0 {
		return fmt.Errorf("beta zones must not be empty")
	}
	return nil
}
