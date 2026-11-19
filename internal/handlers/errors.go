package handlers

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/R-zin/Ethiyo/internal/auth"
	"github.com/R-zin/Ethiyo/internal/breaker"
	"github.com/R-zin/Ethiyo/internal/browser"
	"github.com/R-zin/Ethiyo/internal/chalo"
	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/R-zin/Ethiyo/internal/stream"
	"github.com/gin-gonic/gin"
)

// classifyError maps service and domain errors to an HTTP status and a
// stable, client-safe error code and message.
func classifyError(err error) (status int, code, message string) {
	switch {
	case errors.Is(err, models.ErrBusCodeRequired) || errors.Is(err, models.ErrInvalidBusCodeFormat):
		return http.StatusBadRequest, "INVALID_BUS_CODE", err.Error()

	case errors.Is(err, models.ErrTooManyBusCodes):
		return http.StatusBadRequest, "TOO_MANY_BUS_CODES", err.Error()

	case errors.Is(err, errInvalidFormat):
		return http.StatusBadRequest, "INVALID_FORMAT", err.Error()

	case errors.Is(err, auth.ErrInvalidState) || errors.Is(err, auth.ErrExpiredState):
		return http.StatusBadRequest, "INVALID_OAUTH_STATE", err.Error()

	case errors.Is(err, auth.ErrOAuthNotConfigured):
		return http.StatusServiceUnavailable, "OAUTH_NOT_CONFIGURED", err.Error()

	case errors.Is(err, breaker.ErrOpen):
		return http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE", "Chalo is not responding; live tracking is paused briefly to let it recover"

	case errors.Is(err, chalo.ErrBusNotFound):
		return http.StatusNotFound, "BUS_NOT_FOUND", "Bus route not found on Chalo"

	case errors.Is(err, chalo.ErrInvalidRouteURL):
		return http.StatusBadRequest, "INVALID_ROUTE_URL", err.Error()

	case errors.Is(err, chalo.ErrChaloTimeout) || errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout, "CHALO_TIMEOUT", "Timed out communicating with Chalo"

	case errors.Is(err, chalo.ErrDiscoveryFailed) || errors.Is(err, chalo.ErrUpstreamService):
		return http.StatusBadGateway, "UPSTREAM_CHALO_ERROR", "Upstream Chalo service failure"

	case errors.Is(err, browser.ErrConcurrencyLimitReached):
		return http.StatusServiceUnavailable, "BROWSER_BUSY", "Browser capacity is currently saturated; please retry shortly"

	case errors.Is(err, stream.ErrCapacity):
		return http.StatusServiceUnavailable, "STREAM_CAPACITY", "Live stream capacity reached; please retry shortly or poll /track"

	case errors.Is(err, stream.ErrClosed):
		return http.StatusServiceUnavailable, "SHUTTING_DOWN", "Server is shutting down; please reconnect"

	case errors.Is(err, context.Canceled):
		return http.StatusBadRequest, "REQUEST_CANCELED", "Client request was canceled"

	default:
		return http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "An unexpected error occurred"
	}
}

// HandleError maps service and domain errors to structured HTTP responses.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	status, code, message := classifyError(err)
	switch code {
	case "BROWSER_BUSY", "STREAM_CAPACITY", "SHUTTING_DOWN":
		// Transient capacity errors: tell well-behaved clients when to retry.
		c.Header("Retry-After", "5")
	case "UPSTREAM_UNAVAILABLE":
		c.Header("Retry-After", strconv.Itoa(openRetryAfterSeconds(err)))
	}
	c.JSON(status, models.NewErrorResponse(code, message))
}

// openRetryAfterSeconds is the time until the circuit breaker allows a trial
// call, rounded up to whole seconds (minimum 1).
func openRetryAfterSeconds(err error) int {
	var oe *breaker.OpenError
	if errors.As(err, &oe) {
		if secs := int(math.Ceil(oe.RetryAfter.Seconds())); secs > 1 {
			return secs
		}
	}
	return 1
}
