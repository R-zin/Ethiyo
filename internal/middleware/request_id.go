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

// RequestID ensures every incoming request has a unique correlation ID.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader(RequestIDHeader)
		if reqID == "" {
			reqID = generateID()
		}

		c.Set(RequestIDKey, reqID)
		c.Header(RequestIDHeader, reqID)
		c.Next()
	}
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
