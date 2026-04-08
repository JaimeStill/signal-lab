package discovery

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/bus"
	"github.com/JaimeStill/signal-lab/pkg/signal"
)

// Subject is the NATS subject for discovery pings.
const Subject = "signal.discovery.ping"

// ServiceInfo describes a running service instance.
type ServiceInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"`
	Health      string `json:"health"`
	Description string `json:"description"`
}

// System manages discovery ping interactions over the bus.
type System interface {
	// Subscribe registers the discovery ping listener with the bus.
	Subscribe() error
	// Ping broadcasts a discovery request and collects responses.
	Ping() ([]ServiceInfo, error)
	// Handler creates the HTTP handler for discovery endpoints.
	Handler() *Handler
}

type discovery struct {
	bus     bus.System
	info    ServiceInfo
	timeout time.Duration
	logger  *slog.Logger
}

// New creates a discovery system.
func New(
	b bus.System,
	info ServiceInfo,
	timeout time.Duration,
	logger *slog.Logger,
) System {
	return &discovery{
		bus:     b,
		info:    info,
		timeout: timeout,
		logger:  logger.With("domain", "discovery"),
	}
}

// Subscribe registers the discovery ping listener with the bus.
func (d *discovery) Subscribe() error {
	return d.bus.Subscribe(Subject, d.onPing)
}

// Ping broadcasts a discovery request and collects responses.
func (d *discovery) Ping() ([]ServiceInfo, error) {
	inbox := nats.NewInbox()
	responses := make(chan *nats.Msg, 16)

	sub, err := d.bus.Conn().ChanSubscribe(inbox, responses)
	if err != nil {
		return nil, fmt.Errorf("subscribe inbox: %w", err)
	}
	defer close(responses)
	defer sub.Unsubscribe()

	sig, err := signal.New(d.info.Name, Subject, d.info)
	if err != nil {
		return nil, fmt.Errorf("create signal: %w", err)
	}

	data, err := signal.Encode(sig)
	if err != nil {
		return nil, fmt.Errorf("encode signal: %w", err)
	}

	if err := d.bus.Conn().PublishRequest(Subject, inbox, data); err != nil {
		return nil, fmt.Errorf("publish request: %w", err)
	}

	var services []ServiceInfo
	deadline := time.After(d.timeout)

	for {
		select {
		case msg := <-responses:
			sig, err := signal.Decode(msg.Data)
			if err != nil {
				d.logger.Error("failed to decode response", "error", err)
				continue
			}

			var info ServiceInfo
			if err := json.Unmarshal(sig.Data, &info); err != nil {
				d.logger.Error("failed to unmarshal service info", "error", err)
				continue
			}

			services = append(services, info)
		case <-deadline:
			return services, nil
		}
	}
}

// Handler creates the HTTP handler for discovery endpoints.
func (d *discovery) Handler() *Handler {
	return &Handler{
		discovery: d,
		logger:    d.logger,
	}
}

func (d *discovery) onPing(msg *nats.Msg) {
	incoming, err := signal.Decode(msg.Data)
	if err != nil {
		d.logger.Error("failed to decode ping", "error", err)
		return
	}

	if incoming.Source == d.info.Name {
		return
	}

	sig, err := signal.New(d.info.Name, Subject, d.info)
	if err != nil {
		d.logger.Error("failed to create signal", "error", err)
		return
	}

	data, err := signal.Encode(sig)
	if err != nil {
		d.logger.Error("failed to encode signal", "error", err)
		return
	}

	if err := msg.Respond(data); err != nil {
		d.logger.Error("failed to respond to ping", "error", err)
	}
}
