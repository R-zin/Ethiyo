package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/R-zin/Ethiyo/internal/stream"
	"github.com/gin-gonic/gin"
)

// SSE event names. "upstream_error" is used instead of "error" because
// EventSource reserves the "error" event for connection failures.
const (
	eventPosition      = "position"
	eventUpstreamError = "upstream_error"
)

// StreamOptions tunes the server-sent events endpoint.
type StreamOptions struct {
	// Heartbeat is how often a comment line is written to keep idle
	// connections (and intermediary proxies) alive.
	Heartbeat time.Duration
	// MaxLifetime bounds a single connection; EventSource reconnects
	// transparently, which rebalances clients across instances.
	MaxLifetime time.Duration
	// WriteTimeout bounds each individual write, so a stalled client cannot
	// pin a goroutine forever.
	WriteTimeout time.Duration
	// RetryMillis is the reconnection delay advertised to EventSource.
	RetryMillis int
}

func (o *StreamOptions) withDefaults() StreamOptions {
	out := *o
	if out.Heartbeat <= 0 {
		out.Heartbeat = 15 * time.Second
	}
	if out.MaxLifetime <= 0 {
		out.MaxLifetime = 30 * time.Minute
	}
	if out.WriteTimeout <= 0 {
		out.WriteTimeout = 10 * time.Second
	}
	if out.RetryMillis <= 0 {
		out.RetryMillis = 5000
	}
	return out
}

// StreamHandler serves live bus positions as server-sent events.
type StreamHandler struct {
	service chalo.Service
	hub     *stream.Hub
	opts    StreamOptions
}

// NewStreamHandler creates a StreamHandler.
func NewStreamHandler(service chalo.Service, hub *stream.Hub, opts StreamOptions) *StreamHandler {
	return &StreamHandler{service: service, hub: hub, opts: opts.withDefaults()}
}

// Stream pushes position updates for a bus until the client disconnects.
// Errors that occur before streaming starts (invalid code, unknown bus,
// capacity) are returned as regular JSON error responses.
// GET /api/v1/bus/:buscode/stream
func (h *StreamHandler) Stream(c *gin.Context) {
	validCode, err := models.ValidateBusCode(c.Param("buscode"))
	if err != nil {
		HandleError(c, err)
		return
	}

	// Reserve capacity before doing upstream work so a saturated hub fails fast.
	sub, err := h.hub.Subscribe(validCode)
	if err != nil {
		HandleError(c, err)
		return
	}
	defer sub.Close()

	ctx := c.Request.Context()
	initial, err := h.service.GetBusTracking(ctx, validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	rc := http.NewResponseController(c.Writer)
	w := c.Writer
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no") // disable nginx response buffering
	w.WriteHeader(http.StatusOK)

	write := func(payload string) bool {
		// Per-write deadline: replaces the server-wide WriteTimeout, which
		// would otherwise cut every stream off after a few seconds.
		_ = rc.SetWriteDeadline(time.Now().Add(h.opts.WriteTimeout))
		if _, err := io.WriteString(w, payload); err != nil {
			return false
		}
		return rc.Flush() == nil
	}

	if !write(fmt.Sprintf("retry: %d\n\n", h.opts.RetryMillis)) || !write(formatEvent(eventPosition, initial)) {
		return
	}

	heartbeat := time.NewTicker(h.opts.Heartbeat)
	defer heartbeat.Stop()
	lifetime := time.NewTimer(h.opts.MaxLifetime)
	defer lifetime.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lifetime.C:
			return // client reconnects automatically
		case <-heartbeat.C:
			if !write(": ping\n\n") {
				return
			}
		case ev, ok := <-sub.C:
			if !ok {
				return // hub shut down
			}
			var payload string
			if ev.Err != nil {
				_, code, message := classifyError(ev.Err)
				payload = formatEvent(eventUpstreamError, models.ErrorDetail{Code: code, Message: message})
			} else {
				payload = formatEvent(eventPosition, ev.Info)
			}
			if !write(payload) {
				return
			}
		}
	}
}

// formatEvent renders one SSE frame. JSON never contains raw newlines, so a
// single data line is always well-formed.
func formatEvent(name string, v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to encode stream event", "event", name, "error", err)
		data = []byte(`{}`)
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", name, data)
}
