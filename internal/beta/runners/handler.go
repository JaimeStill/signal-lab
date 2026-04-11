package runners

import (
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
)

// Handler provides HTTP endpoints for the runner cluster.
type Handler struct {
	cluster System
	logger  *slog.Logger
}

// Routes returns the runners route group with cluster-level and per-runner
// lifecycle endpoints plus the status snapshot.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/runners",
		Routes: []routes.Route{
			{Method: "POST", Pattern: "/subscribe", Handler: h.HandleSubscribe},
			{Method: "POST", Pattern: "/unsubscribe", Handler: h.HandleUnsubscribe},
			{Method: "POST", Pattern: "/{id}/subscribe", Handler: h.HandleRunnerSubscribe},
			{Method: "POST", Pattern: "/{id}/unsubscribe", Handler: h.HandleRunnerUnsubscribe},
			{Method: "GET", Pattern: "/status", Handler: h.HandleStatus},
		},
	}
}

// HandleSubscribe attaches every runner not already subscribed.
func (h *Handler) HandleSubscribe(w http.ResponseWriter, r *http.Request) {
	if err := h.cluster.Subscribe(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

// HandleUnsubscribe drains every runner currently subscribed.
func (h *Handler) HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if err := h.cluster.Unsubscribe(); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

// HandleRunnerSubscribe attaches a single runner identified by the {id} path
// parameter.
func (h *Handler) HandleRunnerSubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cluster.SubscribeRunner(id); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

// HandleRunnerUnsubscribe drains a single runner identified by the {id} path
// parameter.
func (h *Handler) HandleRunnerUnsubscribe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.cluster.UnsubscribeRunner(id); err != nil {
		handlers.RespondError(w, h.logger, http.StatusConflict, err)
		return
	}
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}

// HandleStatus returns the current cluster snapshot.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.cluster.Status())
}
