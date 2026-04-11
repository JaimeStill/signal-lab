package jobs

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

// Handler provides HTTP endpoints for the jobs publisher.
type Handler struct {
	jobs   System
	logger *slog.Logger
}

// Routes returns the jobs route group.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/jobs",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/start", Handler: h.HandleStart},
			{Method: "POST", Pattern: "/stop", Handler: h.HandleStop},
			{Method: "GET", Pattern: "/status", Handler: h.HandleStatus},
		},
	}
}

// HandleStart begins job publishing.
func (h *Handler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if err := h.jobs.Start(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.jobs.Status())
}

// HandleStop stops job publishing.
func (h *Handler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if err := h.jobs.Stop(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.jobs.Status())
}

// HandleStatus returns the current jobs publisher state.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.jobs.Status())
}
