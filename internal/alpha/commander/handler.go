package commander

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/nats-io/nats.go"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

// IssueRequest is the JSON body for the POST /issue endpoint.
type IssueRequest struct {
	Action  string `json:"action"`
	Payload string `json:"payload,omitempty"`
}

// Handler provides HTTP endpoints for command dispatch.
type Handler struct {
	commander System
	logger    *slog.Logger
}

// Routes returns the commander route group.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/commander",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/issue", Handler: h.HandleIssue},
			{Method: "GET", Pattern: "/history", Handler: h.HandleHistory},
		},
	}
}

// HandleIssue dispatches a command and returns the response or a 504 on timeout.
func (h *Handler) HandleIssue(w http.ResponseWriter, r *http.Request) {
	var req IssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		handlers.RespondError(w, h.logger, http.StatusBadRequest, err)
		return
	}

	resp, err := h.commander.Issue(req.Action, req.Payload)
	if err != nil {
		if errors.Is(err, nats.ErrTimeout) {
			handlers.RespondError(w, h.logger, http.StatusGatewayTimeout, err)
			return
		}
		handlers.RespondError(w, h.logger, http.StatusInternalServerError, err)
		return
	}

	handlers.RespondJSON(w, http.StatusOK, resp)
}

// HandleHistory returns the recent command history.
func (h *Handler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.commander.History())
}
