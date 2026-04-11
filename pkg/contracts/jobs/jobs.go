// Package jobs defines the cross-service contract for the runner cluster
// demonstration: subject constants, payload type, NATS header keys, and the
// JobType / JobPriority enums shared by publishers and subscribers.
package jobs

// SubjectPrefix is the base subject for job signals.
const SubjectPrefix = "signal.jobs"

// SubjectWildcard matches all job subjects under SubjectPrefix.
const SubjectWildcard = SubjectPrefix + ".>"

// JobType partitions jobs by handling category. The type is appended to
// SubjectPrefix to form the full publish subject.
type JobType string

const (
	TypeCompute  JobType = "compute"
	TypeIO       JobType = "io"
	TypeAnalysis JobType = "analysis"
)

// JobPriority is carried in the Job-Priority NATS header so subscribers can
// branch on priority without decoding the payload.
type JobPriority string

const (
	PriorityLow    JobPriority = "low"
	PriorityNormal JobPriority = "normal"
	PriorityHigh   JobPriority = "high"
)

// NATS header keys for job metadata. Headers travel alongside the message
// envelope and let subscribers inspect routing information without
// deserializing the payload.
const (
	HeaderJobID    = "Job-ID"
	HeaderPriority = "Job-Priority"
	HeaderType     = "Job-Type"
	HeaderTraceID  = "Signal-Trace-ID"
)

// Job is the payload published on signal.jobs.{type} and consumed by runners.
type Job struct {
	ID       string      `json:"id"`
	Type     JobType     `json:"type"`
	Priority JobPriority `json:"priority"`
	Payload  string      `json:"payload"`
}
