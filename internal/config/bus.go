package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	EnvBusURL             = "SIGNAL_BUS_URL"
	EnvBusMaxReconnects   = "SIGNAL_BUS_MAX_RECONNECTS"
	EnvBusReconnectWait   = "SIGNAL_BUS_RECONNECT_WAIT"
	EnvBusResponseTimeout = "SIGNAL_BUS_RESPONSE_TIMEOUT"
)

// BusConfig holds NATS connectin parameters.
type BusConfig struct {
	Addr            string `json:"url"`
	MaxReconnects   int    `json:"max_reconnects"`
	ReconnectWait   string `json:"reconnect_wait"`
	ResponseTimeout string `json:"response_timeout"`
}

// URL returns the NATS server URL. Satisfies bus.Config.
func (c *BusConfig) URL() string {
	return c.Addr
}

// ResponseTimeoutDuration return TimeoutDuration as a time.Duration.
func (c *BusConfig) ResponseTimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(c.ResponseTimeout)
	return d
}

// Options return NATS connection options. Satisfies bus.Config.
func (c *BusConfig) Options(logger *slog.Logger) []nats.Option {
	wait, _ := time.ParseDuration(c.ReconnectWait)

	return []nats.Option{
		nats.MaxReconnects(c.MaxReconnects),
		nats.ReconnectWait(wait),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Warn("nats disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			logger.Info("nats reconnected")
		}),
	}
}

// Finalize applies defaults, environment overrides, and validation.
func (c *BusConfig) Finalize() error {
	c.loadDefaults()
	c.loadEnv()
	return c.validate()
}

// Merge overwrites non-zero fields from overlay.
func (c *BusConfig) Merge(overlay *BusConfig) {
	if overlay.Addr != "" {
		c.Addr = overlay.Addr
	}
	if overlay.MaxReconnects != 0 {
		c.MaxReconnects = overlay.MaxReconnects
	}
	if overlay.ReconnectWait != "" {
		c.ReconnectWait = overlay.ReconnectWait
	}
	if overlay.ResponseTimeout != "" {
		c.ResponseTimeout = overlay.ResponseTimeout
	}
}

func (c *BusConfig) loadDefaults() {
	if c.Addr == "" {
		c.Addr = "nats://localhost:4222"
	}
	if c.MaxReconnects == 0 {
		c.MaxReconnects = 10
	}
	if c.ReconnectWait == "" {
		c.ReconnectWait = "2s"
	}
	if c.ResponseTimeout == "" {
		c.ResponseTimeout = "500ms"
	}
}

func (c *BusConfig) loadEnv() {
	if v := os.Getenv(EnvBusURL); v != "" {
		c.Addr = v
	}
	if v := os.Getenv(EnvBusMaxReconnects); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxReconnects = n
		}
	}
	if v := os.Getenv(EnvBusReconnectWait); v != "" {
		c.ReconnectWait = v
	}
	if v := os.Getenv(EnvBusResponseTimeout); v != "" {
		c.ResponseTimeout = v
	}
}

func (c *BusConfig) validate() error {
	if c.Addr == "" {
		return fmt.Errorf("bus url is required")
	}
	if _, err := time.ParseDuration(c.ReconnectWait); err != nil {
		return fmt.Errorf("invalid reconnect_wait: %w", err)
	}
	if _, err := time.ParseDuration(c.ResponseTimeout); err != nil {
		return fmt.Errorf("invalid response_timeout: %w", err)
	}
	return nil
}
