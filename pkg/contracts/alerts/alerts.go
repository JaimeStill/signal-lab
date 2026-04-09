package alerts

// SubjectPrefix is the base subject for alert signals.
const SubjectPrefix = "signal.alerts"

// SubjectWildcard matches all alert subjects.
const SubjectWildcard = "signal.alerts.>"

// AlertPriority represents the severity level of an alert signal.
type AlertPriority string

const (
	PriorityLow      AlertPriority = "low"
	PriorityNormal   AlertPriority = "normal"
	PriorityHigh     AlertPriority = "high"
	PriorityCritical AlertPriority = "critical"
)

// NATS header keys for alert metadata.
const (
	HeaderPriority = "Signal-Priority"
	HeaderSource   = "Signal-Source"
	HeaderTraceID  = "Signal-Trace-ID"
)

// Alert represents a priority-tagged alert signal.
type Alert struct {
	Severity AlertPriority `json:"severity"`
	Message  string        `json:"message"`
	Zone     string        `json:"zone"`
}
