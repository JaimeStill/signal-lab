package alerts

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

// Handler provides HTTP endpoints for alert publishing.
type Handler struct {
	alerts System
	logger *slog.Logger
}

// Routes returns the alerts route group.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/alerts",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/start", Handler: h.HandleStart},
			{Method: "POST", Pattern: "/stop", Handler: h.HandleStop},
			{Method: "GET", Pattern: "/status", Handler: h.HandleStatus},
		},
	}
}

// HandleStart starts the alert publisher.
func (h *Handler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if err := h.alerts.Start(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}

	handlers.RespondJSON(w, http.StatusOK, h.alerts.Status())
}

// HandleStop stops the alert publisher.
func (h *Handler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if err := h.alerts.Stop(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}

	handlers.RespondJSON(w, http.StatusOK, h.alerts.Status())
}

// HandleStatus returns the current alert publisher state.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.alerts.Status())
}
