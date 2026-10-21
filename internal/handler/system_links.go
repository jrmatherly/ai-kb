package handler

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/envlist"
	"github.com/jrmatherly/ai-kb/internal/logger"
)

const (
	// DocsURLEnv holds the docs base URL the UI links to.
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

var invalidDocsURLWarning sync.Once

// DocsURL returns DocsURLEnv, or DefaultDocsURL when it is unset or not an
// absolute http(s) URL, ending in exactly one slash because the UI appends
// page paths to it.
func DocsURL() string {
	raw := strings.TrimSpace(os.Getenv(DocsURLEnv))
	if raw == "" {
		raw = DefaultDocsURL
	} else if u, err := url.Parse(raw); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		invalidDocsURLWarning.Do(func() {
			logger.Warnf(context.Background(), "[System] %s is not an absolute http(s) URL, using %s: %q",
				DocsURLEnv, DefaultDocsURL, raw)
		})
		raw = DefaultDocsURL
	}
	return strings.TrimRight(raw, "/") + "/"
}

// GitHubLinkEnabled reports GitHubLinkEnabledEnv; unset or unparsable is false.
func GitHubLinkEnabled() bool {
	return envlist.Bool(GitHubLinkEnabledEnv)
}

// CurrentSystemLinks is the GET /system/links body.
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
