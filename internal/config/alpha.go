package config

import "fmt"

// AlphaConfig is the alpha service configuration. It embeds ServiceConfig for
// shared web service fields and adds the alpha-specific JobsConfig and
// CommanderConfig sub-configs.
type AlphaConfig struct {
	ServiceConfig
	Jobs      JobsConfig      `json:"jobs"`
	Commander CommanderConfig `json:"commander"`
}

// Finalize applies defaults, environment overrides, validation, and finalizes
// sub-configs.
func (c *AlphaConfig) Finalize(envPrefix string) error {
	if err := c.ServiceConfig.Finalize(envPrefix); err != nil {
		return err
	}
	if err := c.Jobs.Finalize(); err != nil {
		return fmt.Errorf("jobs: %w", err)
	}
	if err := c.Commander.Finalize(); err != nil {
		return fmt.Errorf("commander: %w", err)
	}
	return nil
}

// Merge overwrites non-zero fields from overlay across the embedded
// ServiceConfig and the JobsConfig sub-config.
func (c *AlphaConfig) Merge(overlay *AlphaConfig) {
	c.ServiceConfig.Merge(&overlay.ServiceConfig)
	c.Jobs.Merge(&overlay.Jobs)
	c.Commander.Merge(&overlay.Commander)
}
