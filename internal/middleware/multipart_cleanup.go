package middleware

import "github.com/gin-gonic/gin"

// MultipartFormCleanup deletes the files multipart parsing wrote to the system temp directory once the request ends.
// Input: the Gin request context c; output: none. Requests that parsed no multipart form, or kept it in memory, delete
// nothing.
// Register it before every route so it covers uploads that succeeded, failed, or recovered from a panic.
func MultipartFormCleanup() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			// Go sets MultipartForm once ParseMultipartForm spills to disk.
			// RemoveAll deletes only the multipart temp files it created itself, never persisted business files.
			if c.Request != nil && c.Request.MultipartForm != nil {
				_ = c.Request.MultipartForm.RemoveAll()
			}
		}()
		c.Next()
	}
}
