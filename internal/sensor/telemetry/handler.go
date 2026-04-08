package telemetry

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

// Handler provides HTTP endpoints for telemetry.
type Handler struct {
	telemetry System
	logger    *slog.Logger
}

// Routes returns the telemetry route group.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/telemetry",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/start", Handler: h.HandleStart},
			{Method: "POST", Pattern: "/stop", Handler: h.HandleStop},
			{Method: "GET", Pattern: "/status", Handler: h.HandleStatus},
		},
	}
}

// HandleStart begins telemetry publishing.
func (h *Handler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if err := h.telemetry.Start(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}

	handlers.RespondJSON(w, http.StatusOK, h.telemetry.Status())
}

// HandleStop stops telemetry publishing.
func (h *Handler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if err := h.telemetry.Stop(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}

	handlers.RespondJSON(w, http.StatusOK, h.telemetry.Status())
}

// HandleStatus returns the current telemetry publisher state.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.telemetry.Status())
}
