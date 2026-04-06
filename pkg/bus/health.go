package bus

import "github.com/nats-io/nats.go"

// Checker implements lifecycle.ReadinessChecker for a NATS connection.
type Checker struct {
	conn *nats.Conn
}

// NewChecker creates a readiness checker fo the given NATS connection.
func NewChecker(conn *nats.Conn) *Checker {
	return &Checker{conn: conn}
}

// Ready reports wehther the NATS connection is active.

func (c *Checker) Ready() bool {
	return c.conn != nil && c.conn.IsConnected()
}
