package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestLanguageFallsBackToEnglish(t *testing.T) {
	t.Setenv("AIKB_LANGUAGE", "")
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ header, want string }{
		{"", "en-US"},
		{"ko-KR,ko;q=0.9,en;q=0.8", "ko-KR"},
	} {
		r := gin.New()
		r.Use(Language())
		var got string
		r.GET("/", func(c *gin.Context) {
			got, _ = types.LanguageFromContext(c.Request.Context())
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			req.Header.Set("Accept-Language", tc.header)
		}
		r.ServeHTTP(httptest.NewRecorder(), req)
		if got != tc.want {
			t.Errorf("Accept-Language %q: language = %q, want %q", tc.header, got, tc.want)
		}
	}
}
