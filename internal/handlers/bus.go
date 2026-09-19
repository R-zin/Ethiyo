package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
)

// BusHandler provides HTTP endpoints for bus route tracking and discovery.
type BusHandler struct {
	service chalo.Service
}

// NewBusHandler creates a BusHandler.
func NewBusHandler(service chalo.Service) *BusHandler {
	return &BusHandler{
		service: service,
	}
}

// --- V1 Modern Endpoints ---

// TrackBus returns the live tracking details for a bus code.
// GET /api/v1/bus/:buscode/track
func (h *BusHandler) TrackBus(c *gin.Context) {
	busCode := c.Param("buscode")
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	info, err := h.service.GetBusTracking(c.Request.Context(), validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.NewSuccessResponse(info))
}

// RouteDetails returns both the live tracking and scheduler route details for a bus code.
// GET /api/v1/bus/:buscode/route
func (h *BusHandler) RouteDetails(c *gin.Context) {
	busCode := c.Param("buscode")
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	info, err := h.service.GetBusRoute(c.Request.Context(), validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.NewSuccessResponse(info))
}

// GetURL returns only the tracking URL for a bus code.
// GET /api/v1/bus/:buscode/url
func (h *BusHandler) GetURL(c *gin.Context) {
	busCode := c.Param("buscode")
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	trackURL, err := h.service.GetTrackingURL(c.Request.Context(), validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, models.NewSuccessResponse(gin.H{
		"bus_code":     validCode,
		"tracking_url": trackURL,
	}))
}

// --- Legacy Backward-Compatible Endpoints ---

// LegacyTrackRoute handles the deprecated /trackroute endpoint.
// GET /trackroute?buscode=...
func (h *BusHandler) LegacyTrackRoute(c *gin.Context) {
	busCode := c.DefaultQuery("buscode", "")
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	_, err = h.service.FetchPublicRoute(c.Request.Context(), validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// LegacyTrackXHR handles the deprecated /trackxhr endpoint.
// GET /trackxhr?buscode=...
func (h *BusHandler) LegacyTrackXHR(c *gin.Context) {
	busCode := c.DefaultQuery("buscode", "")
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	info, err := h.service.GetBusRoute(c.Request.Context(), validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"route":   info.TrackingURL,
		"cookie":  info.RouteURL, // Preserved for legacy contract compatibility
		"error":   nil,
	})
}

// LegacyTestRoute handles the deprecated /testroute endpoint.
// GET /testroute?buscode=...
func (h *BusHandler) LegacyTestRoute(c *gin.Context) {
	busCode := c.Query("buscode")
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	info, err := h.service.GetBusRoute(c.Request.Context(), validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	data, err := h.service.FetchRouteDetails(c.Request.Context(), info.RouteURL)
	if err != nil {
		HandleError(c, err)
		return
	}

	// If valid JSON, return as JSON object; otherwise return as raw string
	var parsed any
	if err := json.Unmarshal(data, &parsed); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
			"data":    parsed,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"raw":     string(data),
	})
}

// LegacyGetBusURL handles the deprecated /get_bus_url endpoint.
// GET /get_bus_url?buscode=...
func (h *BusHandler) LegacyGetBusURL(c *gin.Context) {
	busCode := c.Query("buscode")
	validCode, err := models.ValidateBusCode(busCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	trackURL, err := h.service.GetTrackingURL(c.Request.Context(), validCode)
	if err != nil {
		HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": trackURL,
	})
}
