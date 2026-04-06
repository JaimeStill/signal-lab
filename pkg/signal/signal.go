package signal

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Signal is the envelope type for all messages on the bus.
type Signal struct {
	ID        string            `json:"id"`
	Source    string            `json:"source"`
	Subject   string            `json:"subject"`
	Timestamp time.Time         `json:"timestamp"`
	Data      json.RawMessage   `json:"data"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// New creates a Signal with a generated UUID and current timestamp.
func New(source, subject string, data any) (Signal, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Signal{}, fmt.Errorf("marshal signal data: %w", err)
	}

	return Signal{
		ID:        uuid.New().String(),
		Source:    source,
		Subject:   subject,
		Timestamp: time.Now(),
		Data:      raw,
	}, nil
}
