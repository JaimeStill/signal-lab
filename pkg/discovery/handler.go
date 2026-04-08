package discovery

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

// Handler provides HTTP endpoints for discovery.
type Handler struct {
	discovery System
	logger    *slog.Logger
}

// Routes returns the discovery route group.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/discovery",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/ping", Handler: h.HandlePing},
		},
	}
}

// HandlePing broadcasts a discovery ping and returns responding services.
func (h *Handler) HandlePing(w http.ResponseWriter, r *http.Request) {
	services, err := h.discovery.Ping()
	if err != nil {
		handlers.RespondError(w, h.logger, http.StatusInternalServerError, err)
		return
	}

	handlers.RespondJSON(w, http.StatusOK, services)
}
