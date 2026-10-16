package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
)

func TestErrorHandlerScrubsInternalServerErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID(), ErrorHandler())
	r.GET("/500", func(c *gin.Context) {
		_ = c.Error(apperrors.NewInternalServerError("query: pq: connection refused db-host:5432").
			WithDetails("open /data/minio/bucket: permission denied"))
	})
	r.GET("/502", func(c *gin.Context) {
		_ = c.Error(apperrors.NewBadGatewayError("Failed to test MCP service: connection refused"))
	})
	r.GET("/400", func(c *gin.Context) {
		_ = c.Error(apperrors.NewBadRequestError("name is required"))
	})

	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{
			"/500", http.StatusInternalServerError,
			`{"success":false,"error":{"code":1007,"message":"Internal server error"},"request_id":"req-1"}`,
		},
		{
			"/502", http.StatusBadGateway,
			`{"success":false,"error":{"code":1011,"message":"Failed to test MCP service: connection refused"},` +
				`"request_id":"req-1"}`,
		},
		{
			"/400", http.StatusBadRequest,
			`{"success":false,"error":{"code":1000,"message":"name is required"},"request_id":"req-1"}`,
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("X-Request-ID", "req-1")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body=%s)", w.Code, tc.status, w.Body.String())
			}
			if got := w.Body.String(); got != tc.body {
				t.Fatalf("body = %s, want %s", got, tc.body)
			}
		})
	}
}

func TestLoggerRecordsTheScrubbedCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	t.Cleanup(func() { logger.SetOutput(os.Stdout) })

	r := gin.New()
	r.Use(RequestID(), Logger(), ErrorHandler())
	r.GET("/500", func(c *gin.Context) {
		_ = c.Error(apperrors.NewInternalServerError("query: pq: connection refused db-host:5432").
			WithDetails("open /data/minio/bucket: permission denied"))
	})
	req := httptest.NewRequest(http.MethodGet, "/500", nil)
	req.Header.Set("X-Request-ID", "req-log")
	r.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	for _, want := range []string{
		"req-log", "pq: connection refused db-host:5432", "open /data/minio/bucket: permission denied",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("log is missing %q:\n%s", want, out)
		}
	}
}
