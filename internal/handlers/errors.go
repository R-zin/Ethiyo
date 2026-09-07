package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/R-zin/Ethiyo/internal/auth"
	"github.com/R-zin/Ethiyo/internal/browser"
	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
)

// HandleError maps service and domain errors to structured HTTP responses.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, models.ErrBusCodeRequired) || errors.Is(err, models.ErrInvalidBusCodeFormat):
		c.JSON(http.StatusBadRequest, models.NewErrorResponse("INVALID_BUS_CODE", err.Error()))

	case errors.Is(err, auth.ErrInvalidState) || errors.Is(err, auth.ErrExpiredState):
		c.JSON(http.StatusBadRequest, models.NewErrorResponse("INVALID_OAUTH_STATE", err.Error()))

	case errors.Is(err, auth.ErrOAuthNotConfigured):
		c.JSON(http.StatusServiceUnavailable, models.NewErrorResponse("OAUTH_NOT_CONFIGURED", err.Error()))

	case errors.Is(err, chalo.ErrBusNotFound):
		c.JSON(http.StatusNotFound, models.NewErrorResponse("BUS_NOT_FOUND", "Bus route not found on Chalo"))

	case errors.Is(err, chalo.ErrInvalidRouteURL):
		c.JSON(http.StatusBadRequest, models.NewErrorResponse("INVALID_ROUTE_URL", err.Error()))

	case errors.Is(err, chalo.ErrChaloTimeout) || errors.Is(err, context.DeadlineExceeded):
		c.JSON(http.StatusGatewayTimeout, models.NewErrorResponse("CHALO_TIMEOUT", "Timed out communicating with Chalo"))

	case errors.Is(err, chalo.ErrDiscoveryFailed) || errors.Is(err, chalo.ErrUpstreamService):
		c.JSON(http.StatusBadGateway, models.NewErrorResponse("UPSTREAM_CHALO_ERROR", "Upstream Chalo service failure"))

	case errors.Is(err, browser.ErrConcurrencyLimitReached):
		c.JSON(http.StatusServiceUnavailable, models.NewErrorResponse("BROWSER_BUSY", "Browser capacity is currently saturated; please retry shortly"))

	case errors.Is(err, context.Canceled):
		c.JSON(http.StatusBadRequest, models.NewErrorResponse("REQUEST_CANCELED", "Client request was canceled"))

	default:
		c.JSON(http.StatusInternalServerError, models.NewErrorResponse("INTERNAL_SERVER_ERROR", "An unexpected error occurred"))
	}
}
