package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// WebSearchHandler handles legacy web search related requests
type WebSearchHandler struct{}

// NewWebSearchHandler creates a new web search handler
func NewWebSearchHandler() *WebSearchHandler {
	return &WebSearchHandler{}
}

// GetProviders returns the list of available web search provider types.
//
// GetProviders godoc
// @Summary      List available web search providers
// @Description  Return every registered web search provider, with metadata
// @Tags         Web Search
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Provider list"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /web-search/providers [get]
func (h *WebSearchHandler) GetProviders(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.GetWebSearchProviderTypes(),
	})
}
