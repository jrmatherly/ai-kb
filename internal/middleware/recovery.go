package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/sirupsen/logrus"
)

// Recovery is a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get request ID from context
				ctx := c.Request.Context()
				requestID, _ := c.Get("RequestID")

				// Print stacktrace
				stacktrace := debug.Stack()
				// Log error with structured logger
				logger.ErrorWithFields(ctx, fmt.Errorf("panic: %v", err), logrus.Fields{
					"request_id": requestID,
					"stacktrace": string(stacktrace),
				})

				// Recovery wraps ErrorHandler, and a panic skips ErrorHandler's post-Next code,
				// so the envelope is written here. The panic value stays in the log: it can
				// carry internal state the client must not see.
				c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(c, internalServerError()))
			}
		}()

		c.Next()
	}
}
