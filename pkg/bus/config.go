package bus

import (
	"log/slog"

	"github.com/nats-io/nats.go"
)

// Config provides NATS connection parameters
type Config interface {
	URL() string
	Options(*slog.Logger) []nats.Option
}
