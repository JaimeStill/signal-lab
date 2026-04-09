package telemetry

// SubjectPrefix is the base subject for telemetry signals.
const SubjectPrefix = "signal.telemetry"

// SubjectWildcard matches all telemtry subjects.
const SubjectWildcard = SubjectPrefix + ".>"

// Reading represents a single telemetry measurement.
type Reading struct {
	Type  string  `json:"type"`
	Zone  string  `json:"zone"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
