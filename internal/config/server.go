package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ServiceConfig holds per-service HTTP server parameterss
type ServiceConfig struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Addr returns the host:port listen address.
func (c *ServiceConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Finalize applies defaults, environment overrides, and validation.
// envPrefix distinguishes services (e.g. "SENSOR", "DISPATCH")
func (c *ServiceConfig) Finalize(envPrefix string) error {
	c.loadDefaults()
	c.loadEnv(envPrefix)
	return c.validate()
}

func (c *ServiceConfig) Merge(overlay *ServiceConfig) {
	if overlay.Host != "" {
		c.Host = overlay.Host
	}
	if overlay.Port != 0 {
		c.Port = overlay.Port
	}
	if overlay.Name != "" {
		c.Name = overlay.Name
	}
	if overlay.Description != "" {
		c.Description = overlay.Description
	}
}

func (c *ServiceConfig) loadDefaults() {
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
}

func (c *ServiceConfig) loadEnv(prefix string) {
	if v := os.Getenv(parseVar(prefix, "HOST")); v != "" {
		c.Host = v
	}
	if v := os.Getenv(parseVar(prefix, "PORT")); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Port = port
		}
	}
	if v := os.Getenv(parseVar(prefix, "NAME")); v != "" {
		c.Name = v
	}
	if v := os.Getenv(parseVar(prefix, "DESCRIPTION")); v != "" {
		c.Description = v
	}
}

func (c *ServiceConfig) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.Name == "" {
		return fmt.Errorf("service name is required")
	}
	return nil
}

func parseVar(prefix, variable string) string {
	variable = strings.TrimPrefix(variable, "_")
	return "SIGNAL_" + prefix + "_" + variable
}
