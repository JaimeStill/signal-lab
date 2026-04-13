package responder

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

// Handler provides HTTP endpoints for the command responder.
type Handler struct {
	responder System
	logger    *slog.Logger
}

// Routes returns the responder route group.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/responder",
		Routes: []routes.Route{
			{Method: "GET", Pattern: "/ledger", Handler: h.HandleLedger},
		},
	}
}

// HandleLedger returns the command execution ledger.
func (h *Handler) HandleLedger(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.responder.Ledger())
}
