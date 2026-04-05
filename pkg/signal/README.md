# pkg/signal

Core signal envelope type — the universal message structure carried over the bus between services.

## Files (Phase 1)

- `signal.go` — `Signal` struct (ID, Source, Subject, Timestamp, Data, Metadata), constructors
- `encoding.go` — JSON encode/decode helpers
