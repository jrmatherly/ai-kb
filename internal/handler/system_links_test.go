package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetSystemLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const (
		defaultDocs = "https://ai-kb.matherly.net/docs/"
		customDocs  = "https://kb.example.com/docs/"
		repo        = "https://github.com/jrmatherly/ai-kb"
	)
	cases := []struct {
		name, docsEnv, githubEnv string
		want                     SystemLinks
	}{
		{"defaults", "", "", SystemLinks{DocsURL: defaultDocs}},
		{"docs without slash", "https://kb.example.com/docs", "", SystemLinks{DocsURL: customDocs}},
		{"docs with slashes", " https://kb.example.com/docs// ", "", SystemLinks{DocsURL: customDocs}},
		{"github on", "", "true", SystemLinks{DocsURL: defaultDocs, GitHubURL: repo}},
		{"github off", "", "false", SystemLinks{DocsURL: defaultDocs}},
		{"github unparsable", "", "yes please", SystemLinks{DocsURL: defaultDocs}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(DocsURLEnv, tc.docsEnv)
			t.Setenv(GitHubLinkEnabledEnv, tc.githubEnv)
			engine := gin.New()
			engine.GET("/links", (&SystemHandler{}).GetSystemLinks)
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/links", nil))
			var got SystemLinks
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != http.StatusOK || got != tc.want {
				t.Fatalf("got %d %s, want 200 %+v", w.Code, w.Body.String(), tc.want)
			}
		})
	}
}
