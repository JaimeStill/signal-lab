package signal

import "encoding/json"

// Encode marshals a Signal to JSON bytes.
func Encode(s Signal) ([]byte, error) {
	return json.Marshal(s)
}

// Decode unmarshals JSON bytes into a Signal.
func Decode(data []byte) (Signal, error) {
	var s Signal
	err := json.Unmarshal(data, &s)
	return s, err
}
