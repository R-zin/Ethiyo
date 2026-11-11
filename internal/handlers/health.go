package handlers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check and readiness requests.
type HealthHandler struct {
	mu          sync.RWMutex
	browserPath string
	isRunning   func() bool
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(browserPath string, isRunning ...func() bool) *HealthHandler {
	h := &HealthHandler{
		browserPath: browserPath,
	}
	if len(isRunning) > 0 && isRunning[0] != nil {
		h.isRunning = isRunning[0]
	} else {
		h.isRunning = func() bool { return true }
	}
	return h
}

// Check responds with basic server liveness status.
func (h *HealthHandler) Check(c *gin.Context) {
	h.mu.RLock()
	bp := h.browserPath
	h.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"status":  "healthy",
		"browser": bp != "",
	})
}

// Ready responds with server readiness based on browser path and server running state.
func (h *HealthHandler) Ready(c *gin.Context) {
	h.mu.RLock()
	bp := h.browserPath
	running := h.isRunning == nil || h.isRunning()
	h.mu.RUnlock()

	ready := bp != "" && running
	if !ready {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"message": "unavailable",
			"status":  "not ready",
			"ready":   false,
			"browser": bp != "",
			"running": running,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"status":  "ready",
		"ready":   true,
		"browser": true,
		"running": true,
	})
}

// SetBrowserPath dynamically updates the configured browser path.
func (h *HealthHandler) SetBrowserPath(path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.browserPath = path
}
