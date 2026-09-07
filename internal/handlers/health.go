package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	browserPath string
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(browserPath string) *HealthHandler {
	return &HealthHandler{
		browserPath: browserPath,
	}
}

// Check responds with server status.
func (h *HealthHandler) Check(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"status":  "healthy",
		"browser": h.browserPath != "",
	})
}
