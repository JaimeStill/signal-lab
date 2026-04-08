package monitoring

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/JaimeStill/signal-lab/pkg/handlers"
	"github.com/JaimeStill/signal-lab/pkg/routes"
	"github.com/JaimeStill/signal-lab/pkg/signal"
)

// Handler provides HTTP endpoints for monitoring.
type Handler struct {
	monitor System
	signals <-chan signal.Signal
	logger  *slog.Logger
}

// Routes returns the monitoring route group.
func (h *Handler) Routes() routes.Group {
	return routes.Group{
		Prefix: "/monitoring",
		Routes: []routes.Route{
			{Method: "GET", Pattern: "/stream", Handler: h.HandleStream},
			{Method: "GET", Pattern: "/status", Handler: h.HandleStatus},
		},
	}
}

// HandleStream sends telemetry signals as Server-Sent Events.
func (h *Handler) HandleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		handlers.RespondError(
			w,
			h.logger,
			http.StatusInternalServerError,
			fmt.Errorf("streaming not supported"),
		)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	h.logger.Info("SSE client connected")
	defer h.logger.Info("SSE client disconnected")

	for {
		select {
		case <-r.Context().Done():
			return
		case sig := <-h.signals:
			data, err := json.Marshal(sig)
			if err != nil {
				h.logger.Error("failed to marshal signal", "error", err)
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

// HandleStatus returns the current monitoring state.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	handlers.RespondJSON(w, http.StatusOK, h.monitor.Status())
}
