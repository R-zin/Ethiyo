package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/R-zin/Ethiyo/internal/models"
	"github.com/gin-gonic/gin"
)

// Recovery returns a Gin middleware that recovers from panics and returns a structured 500 JSON.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				reqID := GetRequestID(c)
				stack := string(debug.Stack())

				slog.Error("panic recovered in handler",
					"request_id", reqID,
					"error", fmt.Sprintf("%v", r),
					"stack", stack,
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, models.NewErrorResponse(
					"INTERNAL_SERVER_ERROR",
					"An unexpected internal error occurred",
				))
			}
		}()
		c.Next()
	}
}
