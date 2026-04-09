package config

import (
	"fmt"
	"os"
	"strings"
)

const EnvSensorZones = "SIGNAL_SENSOR_ZONES"

type SensorConfig struct {
	ServiceConfig
	Zones     []string        `json:"zones"`
	Telemetry TelemetryConfig `json:"telemetry"`
	Alerts    AlertsConfig    `json:"alerts"`
}

// Finalize applies defaults, environment overrides, validation,
// and finalizes sub-configs.
func (c *SensorConfig) Finalize(envPrefix string) error {
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
	if err := c.Alerts.Finalize(); err != nil {
		return fmt.Errorf("alerts: %w", err)
	}

	return nil
}

// Merge overwrites non-zero fields from overlay.
func (c *SensorConfig) Merge(overlay *SensorConfig) {
	c.ServiceConfig.Merge(&overlay.ServiceConfig)

	if len(overlay.Zones) > 0 {
		c.Zones = overlay.Zones
	}

	c.Telemetry.Merge(&overlay.Telemetry)
	c.Alerts.Merge(&overlay.Alerts)
}

func (c *SensorConfig) loadDefaults() {
	if len(c.Zones) == 0 {
		c.Zones = []string{
			"server-room",
			"ops-center",
		}
	}
}

func (c *SensorConfig) loadEnv() {
	if v := os.Getenv(EnvSensorZones); v != "" {
		c.Zones = strings.Split(v, ",")
	}
}

func (c *SensorConfig) validate() error {
	if len(c.Zones) == 0 {
		return fmt.Errorf("sensor zones must not be empty")
	}
	return nil
}
