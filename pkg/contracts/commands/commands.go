// Package commands defines the cross-service contract for the request/reply
// command dispatch demonstration: subject constants, action and status enums,
// and the Command/Response payload types shared by the commander and responder.
package commands

import "fmt"

// SubjectPrefix is the base subject for command signals.
const SubjectPrefix = "signal.commands"

// SubjectWildcard matches all command subjects under SubjectPrefix.
const SubjectWildcard = SubjectPrefix + ".>"

// Action identifies the command being dispatched. The action token is appended
// to SubjectPrefix to form the full publish subject.
type Action string

const (
	ActionPing   Action = "ping"
	ActionFlush  Action = "flush"
	ActionRotate Action = "rotate"
	ActionNoop   Action = "noop"
)

// Subject returns the full NATS subject for the given action.
func Subject(action Action) string {
	return fmt.Sprintf("%s.%s", SubjectPrefix, action)
}

// Status indicates whether the responder handled a command successfully.
type Status string

const (
	StatusOK    Status = "ok"
	StatusError Status = "error"
)

// Command is the request payload published on signal.commands.{action}
// and consumed by the responder.
type Command struct {
	ID       string `json:"id"`
	Action   Action `json:"action"`
	Payload  string `json:"payload"`
	IssuedAt string `json:"issued_at"`
}

// Response is the reply payload returned by the responder after handling
// a command.
type Response struct {
	CommandID string `json:"command_id"`
	Status    Status `json:"status"`
	Result    string `json:"result"`
	HandledAt string `json:"handled_at"`
}
