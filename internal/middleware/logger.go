package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

const (
	maxBodySize = 1024 * 10 // log at most 10KB of body content
	// bodyTruncatedMarker follows a truncated body in the log.
	bodyTruncatedMarker = "... [content too long, truncated]"
)

// replayedBody gives the handler back the bytes the log peek consumed, so the body it reads is identical
// to having no such middleware; Close still closes the original body so the connection is released.
type replayedBody struct {
	io.Reader
	io.Closer
}

// loggerResponseBodyWriter is a ResponseWriter that captures the response body for the logger middleware
type loggerResponseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write writes to both the buffer and the original writer.
// The buffer is capped so streaming responses such as SSE cannot grow memory without bound.
func (r loggerResponseBodyWriter) Write(b []byte) (int, error) {
	if r.body.Len() < maxBodySize {
		remaining := maxBodySize - r.body.Len()
		if len(b) <= remaining {
			r.body.Write(b)
		} else {
			r.body.Write(b[:remaining])
		}
	}
	return r.ResponseWriter.Write(b)
}

// sensitiveFieldRegex matches sensitive JSON fields (case-insensitive, covering snake_case / camelCase / PascalCase).
// $1 captures the original field name (with its quotes), so the log keeps the field name and only the value becomes
// "***".
var sensitiveFieldRegex = regexp.MustCompile(
	`(?i)("(?:new[_-]?password|old[_-]?password|password|passwd|ticket|token|access[_-]?token|` +
		`refresh[_-]?token|next[_-]?token|device[_-]?token|pending[_-]?token|pairing[_-]?link|` +
		`id[_-]?token|authorization|auth[_-]?token|api[_-]?key|` +
		`api[_-]?secret|secret[_-]?key|client[_-]?secret|private[_-]?key|secret|` +
		`authorization[_-]?url|authorization[_-]?attempt)")\s*:\s*"[^"]*"`,
)

// sanitizeBody redacts sensitive information
func sanitizeBody(body string) string {
	return sensitiveFieldRegex.ReplaceAllString(body, `$1:"***"`)
}

var sensitiveQueryFields = map[string]struct{}{
	"access_token":          {},
	"authorization_attempt": {},
	"code":                  {},
	"id_token":              {},
	"refresh_token":         {},
	"state":                 {},
	// ticket is the sandbox terminal's WebSocket handshake credential. A
	// browser cannot set Authorization on an upgrade, so it travels in the
	// query string; anyone holding it for its 2-minute TTL can open a shell
	// in the session's sandbox, which is why it must never reach a log line.
	"ticket": {},
	"token":  {},
}

// sanitizeQuery prevents OAuth authorization codes, CSRF/attempt state, and
// handshake credentials from being copied into access logs. Parsing the query
// also covers repeated and percent-encoded parameters without relying on
// fragile string replacement.
func sanitizeQuery(raw string) string {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "[invalid query omitted]"
	}
	for key := range values {
		if _, sensitive := sensitiveQueryFields[strings.ToLower(key)]; sensitive {
			values[key] = []string{"***"}
		}
	}
	return values.Encode()
}

// readRequestBody reads the request body (size-capped for the log, while handing the remaining bytes to later handlers
// unchanged)
func readRequestBody(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}

	// Check Content-Type; only JSON is logged
	contentType := c.GetHeader("Content-Type")
	if !strings.Contains(contentType, "application/json") &&
		!strings.Contains(contentType, "application/x-www-form-urlencoded") &&
		!strings.Contains(contentType, "text/") {
		return "[non-text content, skipped]"
	}

	// Reset the request body: join the peeked prefix back onto the unread remainder, so the handler still reads
	// the full data while the middleware itself keeps only the bytes the log needs.
	//
	// This used to be an unbounded io.ReadAll(c.Request.Body). Logger is registered before Auth
	// (router.go NewRouter), so it runs for every request, while every route's
	// http.MaxBytesReader (limitJSONBody / limitUploadBody / the browserskill 4 KiB
	// authorization endpoint) is installed only in the handler: the body had already been read into memory whole, so
	// those caps
	// only produced errors and did not bound memory. Routes without a cap (chat, retrieval, IM webhook,
	// MCP endpoint) had no limit at all.
	bodyBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodySize+1))
	originalBody := c.Request.Body
	c.Request.Body = &replayedBody{
		Reader: io.MultiReader(bytes.NewReader(bodyBytes), originalBody),
		Closer: originalBody,
	}
	if err != nil {
		return "[failed to read request body]"
	}

	// Body for the log (size-capped)
	truncated := len(bodyBytes) > maxBodySize
	if truncated {
		bodyBytes = bodyBytes[:maxBodySize]
	}

	bodyStr := string(bodyBytes)
	if truncated {
		bodyStr += bodyTruncatedMarker
	}

	return sanitizeBody(bodyStr)
}

// RequestID middleware adds a unique request ID to the context
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get request ID from header or generate a new one
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		safeRequestID := secutils.SanitizeForLog(requestID)
		// Set request ID in header
		c.Header("X-Request-ID", requestID)

		// Set request ID in context
		c.Set(types.RequestIDContextKey.String(), requestID)

		// Set logger in context
		requestLogger := logger.GetLogger(c)
		requestLogger = requestLogger.WithField("request_id", safeRequestID)
		c.Set(types.LoggerContextKey.String(), requestLogger)

		// Set request ID in the global context for logging
		c.Request = c.Request.WithContext(
			context.WithValue(
				context.WithValue(c.Request.Context(), types.RequestIDContextKey, requestID),
				types.LoggerContextKey, requestLogger,
			),
		)

		c.Next()
	}
}

// Logger middleware logs request details with request ID, input and output
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		isWikiStats := strings.HasPrefix(path, "/api/v1/knowledgebase/") && strings.HasSuffix(path, "/wiki/stats")
		if strings.HasPrefix(path, "/assets/") || isWikiStats {
			c.Next()
			return
		}

		// Browser traffic contains credentials, page content and screenshots.
		// Keep access metadata, but never read or buffer these request/response bodies.
		browserTraffic := strings.HasPrefix(path, "/api/v1/local-browser/") ||
			path == "/api/v1/me/browser" || strings.HasSuffix(path, "/local-browser")
		// Read the request body (before Next, because Next consumes the body)
		var requestBody string
		if !browserTraffic && (c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH") {
			requestBody = readRequestBody(c)
		}

		// Create the response body capturer
		responseBody := &bytes.Buffer{}
		responseWriter := &loggerResponseBodyWriter{
			ResponseWriter: c.Writer,
			body:           responseBody,
		}
		if !browserTraffic {
			c.Writer = responseWriter
		}

		// Process request
		c.Next()

		// Get request ID from context
		requestID, exists := c.Get(types.RequestIDContextKey.String())
		requestIDStr := "unknown"
		if exists {
			if idStr, ok := requestID.(string); ok && idStr != "" {
				requestIDStr = idStr
			}
		}
		safeRequestID := secutils.SanitizeForLog(requestIDStr)

		// Calculate latency
		latency := time.Since(start)

		// Get client IP and status code
		clientIP := c.ClientIP()
		statusCode := c.Writer.Status()
		method := c.Request.Method

		if raw != "" {
			path = path + "?" + sanitizeQuery(raw)
		}

		// Read the response body
		responseBodyStr := ""
		if responseBody.Len() > 0 {
			contentType := c.Writer.Header().Get("Content-Type")
			if strings.Contains(contentType, "text/event-stream") {
				responseBodyStr = "[SSE streaming response, skipped]"
			} else if strings.Contains(contentType, "application/json") ||
				strings.Contains(contentType, "text/") {
				bodyBytes := responseBody.Bytes()
				if len(bodyBytes) >= maxBodySize {
					responseBodyStr = string(bodyBytes[:maxBodySize]) + "... [content too long, truncated]"
				} else {
					responseBodyStr = string(bodyBytes)
				}
				responseBodyStr = sanitizeBody(responseBodyStr)
			} else {
				responseBodyStr = "[non-text content, skipped]"
			}
		}

		// Build the log message
		logMsg := logger.GetLogger(c)
		logMsg = logMsg.WithFields(map[string]interface{}{
			"request_id":  safeRequestID,
			"method":      method,
			"path":        secutils.SanitizeForLog(path),
			"status_code": statusCode,
			"size":        c.Writer.Size(),
			"latency":     latency.String(),
			"client_ip":   secutils.SanitizeForLog(clientIP),
		})

		// Add the request body (if any)
		if requestBody != "" {
			logMsg = logMsg.WithField("request_body", secutils.SanitizeForLog(requestBody))
		}

		// Add the response body (if any)
		if responseBodyStr != "" {
			logMsg = logMsg.WithField("response_body", secutils.SanitizeForLog(responseBodyStr))
		}
		if last := c.Errors.Last(); last != nil && last.Err != nil {
			logMsg = logMsg.WithField("error", secutils.SanitizeForLog(last.Err.Error()))
			// ErrorHandler drops a 500's details from the response, so this is their only record.
			if appErr, ok := apperrors.IsAppError(last.Err); ok && statusCode >= 500 && appErr.Details != nil {
				logMsg = logMsg.WithField("error_details", secutils.SanitizeForLog(fmt.Sprint(appErr.Details)))
			}
		}
		switch {
		case statusCode >= 500:
			logMsg.Error()
		case statusCode >= 400:
			logMsg.Warn()
		default:
			logMsg.Info()
		}
	}
}
