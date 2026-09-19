package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const (
	// RequestIDHeader is the HTTP header used for tracing requests.
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey is the context key for storing the request ID.
	RequestIDKey = "request_id"
)

// maxRequestIDLen bounds client-supplied correlation IDs; anything longer is
// replaced with a generated ID.
const maxRequestIDLen = 64

// RequestID ensures every incoming request has a unique correlation ID.
// Client-supplied IDs are only honored when they are short and free of
// control/whitespace characters, so a hostile header value can never be
// reflected into response headers or log lines.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(RequestIDHeader)
		if !isValidRequestID(reqID) {
			reqID = generateID()
		}

		c.Set(RequestIDKey, reqID)
		c.Header(RequestIDHeader, reqID)
		c.Next()
	}
}

func isValidRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		b := id[i]
		// Reject control characters (incl. CR/LF) and DEL to prevent header
		// injection and log forging.
		if b < 0x20 || b == 0x7f {
			return false
		}
	}
	return true
}

// GetRequestID retrieves the request ID from Gin context, or returns empty string.
func GetRequestID(c *gin.Context) string {
	if val, exists := c.Get(RequestIDKey); exists {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func generateID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}
