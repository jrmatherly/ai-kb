package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// ErrorHandler is middleware that handles application errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process the request
		c.Next()

		// Check whether there are errors
		if len(c.Errors) > 0 {
			// Get the last error
			err := c.Errors.Last().Err

			appErr, ok := errors.IsAppError(err)
			if !ok || appErr.HTTPCode == http.StatusInternalServerError {
				// A 500's message and details are wrapped internal errors (SQL, storage
				// endpoints, upstream bodies). The Logger middleware records them with
				// the request ID the client receives.
				appErr = internalServerError()
			}
			c.JSON(appErr.HTTPCode, errorResponse(c, appErr))
		}
	}
}

func internalServerError() *errors.AppError {
	return errors.NewInternalServerError("Internal server error")
}

func errorResponse(c *gin.Context, appErr *errors.AppError) errors.ErrorResponse {
	return errors.ErrorResponse{Error: *appErr, RequestID: c.GetString(types.RequestIDContextKey.String())}
}
