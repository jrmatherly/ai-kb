package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// DocsURLEnv is the public documentation base URL the UI links to.
	DocsURLEnv = "AIKB_DOCS_URL"
	// DefaultDocsURL is the public docs site, used when DocsURLEnv is unset.
	DefaultDocsURL = "https://ai-kb.matherly.net/docs/"
	// GitHubLinkEnabledEnv opts the UI into links to the source repository.
	GitHubLinkEnabledEnv = "AIKB_GITHUB_LINK_ENABLED"
	// GitHubURL is the source repository the GitHub links open.
	GitHubURL = "https://github.com/jrmatherly/ai-kb"
)

// SystemLinks is returned by GET /system/links. An empty string means the UI
// shows no link.
type SystemLinks struct {
	DocsURL   string `json:"docs_url"`
	GitHubURL string `json:"github_url"`
}

// DocsURL returns DocsURLEnv, or DefaultDocsURL when unset, ending in exactly
// one slash because the UI appends page paths to it.
func DocsURL() string {
	raw := strings.TrimSpace(os.Getenv(DocsURLEnv))
	if raw == "" {
		raw = DefaultDocsURL
	}
	return strings.TrimRight(raw, "/") + "/"
}

// GitHubLinkEnabled reports GitHubLinkEnabledEnv; unset or unparsable is false.
func GitHubLinkEnabled() bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(os.Getenv(GitHubLinkEnabledEnv)))
	return err == nil && parsed
}

// CurrentSystemLinks reads both variables at call time.
func CurrentSystemLinks() SystemLinks {
	links := SystemLinks{DocsURL: DocsURL()}
	if GitHubLinkEnabled() {
		links.GitHubURL = GitHubURL
	}
	return links
}

// GetSystemLinks godoc
// @Summary      Get the deployment's external links
// @Description  Returns the documentation base URL and the source repository URL the UI links to. Unauthenticated, so
// @Description  the login page can read it; github_url is empty unless AIKB_GITHUB_LINK_ENABLED is true
// @Tags         System
// @Produce      json
// @Success      200  {object}  SystemLinks
// @Router       /system/links [get]
func (*SystemHandler) GetSystemLinks(c *gin.Context) {
	c.JSON(http.StatusOK, CurrentSystemLinks())
}
