package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger returns a Gin middleware that records structured logs using slog.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		reqID := GetRequestID(c)

		// Redact sensitive query parameters from log if present
		safePath := path
		if rawQuery != "" && (path == "/auth/google/callback" || path == "/api/v1/auth/google/callback") {
			safePath = path + "?[REDACTED]"
		} else if rawQuery != "" {
			safePath = path + "?" + rawQuery
		}

		attrs := []any{
			"request_id", reqID,
			"method", method,
			"path", safePath,
			"status", status,
			"latency_ms", float64(latency.Microseconds()) / 1000.0,
			"client_ip", clientIP,
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		if status >= 500 {
			slog.Error("http request error", attrs...)
		} else if status >= 400 {
			slog.Warn("http request warning", attrs...)
		} else {
			slog.Info("http request", attrs...)
		}
	}
}

// ContextHandler wraps an slog.Handler to inject request_id from context.Context into log records.
type ContextHandler struct {
	slog.Handler
}

// NewContextHandler wraps the provided slog.Handler to automatically attach request_id from context.
func NewContextHandler(h slog.Handler) *ContextHandler {
	return &ContextHandler{Handler: h}
}

// Handle injects the request_id attribute if found in ctx and not already present on the record.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
			hasReqID := false
			r.Attrs(func(a slog.Attr) bool {
				if a.Key == RequestIDKey {
					hasReqID = true
					return false
				}
				return true
			})
			if !hasReqID {
				r.AddAttrs(slog.String(RequestIDKey, reqID))
			}
		}
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new ContextHandler wrapping the handler with attributes.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new ContextHandler wrapping the handler with a group.
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{Handler: h.Handler.WithGroup(name)}
}
