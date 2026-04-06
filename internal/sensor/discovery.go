package sensor

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/signal"
)

// Discovery handles discovery ping over NATS and HTTP
type Discovery struct {
	conn    *nats.Conn
	info    discovery.ServiceInfo
	timeout time.Duration
	logger  *slog.Logger
}

// NewDiscovery creates a Discovery handler.
func NewDiscovery(
	conn *nats.Conn,
	info discovery.ServiceInfo,
	timeout time.Duration,
	logger *slog.Logger,
) *Discovery {
	return &Discovery{
		conn:    conn,
		info:    info,
		timeout: timeout,
		logger:  logger.With("sensor", "discovery"),
	}
}

// Subscribe listens on the discovery subject and replies with the service's info.
// Pings from this service are silently ignored.
func (d *Discovery) Subscribe() (*nats.Subscription, error) {
	return d.conn.Subscribe(discovery.Subject, func(msg *nats.Msg) {
		incoming, err := signal.Decode(msg.Data)
		if err != nil {
			d.logger.Error("failed to decode ping", "error", err)
			return
		}

		if incoming.Source == d.info.Name {
			return
		}

		sig, err := signal.New(d.info.Name, discovery.Subject, d.info)
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
	})
}

// HandlePing is the HTTP handler for POST /discovery/ping.
// It broadcasts a ping and collects responses from all services.
func (d *Discovery) HandlePing(w http.ResponseWriter, r *http.Request) {
	inbox := nats.NewInbox()
	responses := make(chan *nats.Msg, 16)

	sub, err := d.conn.ChanSubscribe(inbox, responses)
	if err != nil {
		handlers.RespondError(w, d.logger, http.StatusInternalServerError, err)
		return
	}
	defer sub.Unsubscribe()

	sig, err := signal.New(d.info.Name, discovery.Subject, d.info)
	if err != nil {
		handlers.RespondError(w, d.logger, http.StatusInternalServerError, err)
		return
	}

	data, err := signal.Encode(sig)
	if err != nil {
		handlers.RespondError(w, d.logger, http.StatusInternalServerError, err)
		return
	}

	if err := d.conn.PublishRequest(discovery.Subject, inbox, data); err != nil {
		handlers.RespondError(w, d.logger, http.StatusInternalServerError, err)
		return
	}

	var services []discovery.ServiceInfo
	deadline := time.After(d.timeout)

	for {
		select {
		case msg := <-responses:
			sig, err := signal.Decode(msg.Data)
			if err != nil {
				d.logger.Error("failed to decode response", "error", err)
				continue
			}

			var info discovery.ServiceInfo
			if err := json.Unmarshal(sig.Data, &info); err != nil {
				d.logger.Error("failed to unmarshal service info", "error", err)
				continue
			}

			services = append(services, info)
		case <-deadline:
			handlers.RespondJSON(w, http.StatusOK, services)
			return
		}
	}
}
