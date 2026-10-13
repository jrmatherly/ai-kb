package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/errors"
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
			if !ok {
				appErr = &errors.AppError{
					Code:     errors.ErrInternalServer,
					Message:  "Internal server error",
					HTTPCode: http.StatusInternalServerError,
				}
			}
			c.JSON(appErr.HTTPCode, errors.ErrorResponse{Error: *appErr})
		}
	}
}
