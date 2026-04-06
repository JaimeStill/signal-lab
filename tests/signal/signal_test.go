package signal_test

import (
	"encoding/json"
	"testing"

	"github.com/JaimeStill/signal-lab/pkg/signal"
)

func TestNewSignal(t *testing.T) {
	sig, err := signal.New("test-source", "test.subject", map[string]string{"key": "value"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if sig.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if sig.Source != "test-source" {
		t.Fatalf("expected source 'test-source', got %q", sig.Source)
	}
	if sig.Subject != "test.subject" {
		t.Fatalf("expected subject 'test.subject', got %q", sig.Subject)
	}
	if sig.Timestamp.IsZero() {
		t.Fatal("expected non-zero timestamp")
	}
	if sig.Data == nil {
		t.Fatal("expected non-nil data")
	}
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	original, err := signal.New("source", "subject", map[string]string{"hello": "world"})
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	encoded, err := signal.Encode(original)
	if err != nil {
		t.Fatal("encode error:", err)
	}

	decoded, err := signal.Decode(encoded)
	if err != nil {
		t.Fatal("decode error:", err)
	}

	if decoded.ID != original.ID {
		t.Fatalf("ID mismatch: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Source != original.Source {
		t.Fatalf("Source mismatch: got %q, want %q", decoded.Source, original.Source)
	}
	if decoded.Subject != original.Subject {
		t.Fatalf("Subject mismatch: got %q, want %q", decoded.Subject, original.Subject)
	}
	if string(decoded.Data) != string(original.Data) {
		t.Fatalf("Data mismatch: got %s, want %s", decoded.Data, original.Data)
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	_, err := signal.Decode([]byte("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMetadataOmitEmpty(t *testing.T) {
	sig, err := signal.New("source", "subject", "data")
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	encoded, err := signal.Encode(sig)
	if err != nil {
		t.Fatal("encode error:", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal("unmarshal error:", err)
	}

	if _, exists := raw["metadata"]; exists {
		t.Fatal("expected metadata key to be omitted when nil")
	}
}
