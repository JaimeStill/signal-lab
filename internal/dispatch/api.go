package dispatch

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/discovery"
	"github.com/JaimeStill/signal-lab/pkg/module"
)

func NewModule(
	conn *nats.Conn,
	info discovery.ServiceInfo,
	timeout time.Duration,
	logger *slog.Logger,
) (*module.Module, *nats.Subscription, error) {
	disc := NewDiscovery(conn, info, timeout, logger)

	sub, err := disc.Subscribe()
	if err != nil {
		return nil, nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /discovery/ping", disc.HandlePing)

	return module.New("/api", mux), sub, nil
}
