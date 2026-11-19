package handlers

import (
	"net/http"

	"github.com/R-zin/Ethiyo/internal/breaker"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	browserPath string
	upstream    func() breaker.Status
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(browserPath string) *HealthHandler {
	return &HealthHandler{
		browserPath: browserPath,
	}
}

// WithUpstream adds the live-tracking upstream's circuit status to health
// responses.
func (h *HealthHandler) WithUpstream(status func() breaker.Status) *HealthHandler {
	h.upstream = status
	return h
}

// Check responds with server status. It always answers 200 while the process
// can serve requests: Chalo being down degrades responses (last known
// positions) but must not make load balancers restart a healthy instance.
func (h *HealthHandler) Check(c *gin.Context) {
	body := gin.H{
		"message": "ok",
		"status":  "healthy",
		"browser": h.browserPath != "",
	}
	if h.upstream != nil {
		st := h.upstream()
		body["upstream"] = st
		body["degraded"] = st.State != breaker.Closed
	}
	c.JSON(http.StatusOK, body)
}
