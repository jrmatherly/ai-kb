package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
)

func TestErrorHandlerKeepsStatusOfWrappedAppError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ErrorHandler())
	r.GET("/thing", func(c *gin.Context) {
		_ = c.Error(fmt.Errorf("lookup: %w", apperrors.NewNotFoundError("thing not found")))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/thing", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", w.Code, w.Body.String())
	}
	want := `{"success":false,"error":{"code":1003,"message":"thing not found"}}`
	if got := w.Body.String(); got != want {
		t.Fatalf("body = %s\nwant   %s", got, want)
	}
	if strings.Contains(w.Body.String(), "lookup") {
		t.Fatalf("body leaks the wrap text: %s", w.Body.String())
	}
}
