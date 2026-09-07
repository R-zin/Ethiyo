package middleware

import (
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
