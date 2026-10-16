package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

var errWebSearchRepo = errors.New("dial tcp 10.0.0.5:5432: connection refused")

type webSearchRepoStub struct {
	interfaces.WebSearchProviderRepository
}

func (webSearchRepoStub) Create(context.Context, *types.WebSearchProviderEntity) error {
	return errWebSearchRepo
}

func (webSearchRepoStub) GetByID(_ context.Context, _ uint64, id string) (*types.WebSearchProviderEntity, error) {
	if id == "broken" {
		return nil, errWebSearchRepo
	}
	return nil, nil
}

func TestWebSearchProviderErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := webSearchRepoStub{}
	svc := service.NewWebSearchProviderService(repo)
	providers := NewWebSearchProviderHandler(repo, svc, nil)
	credentials := NewWebSearchProviderCredentialsHandler(repo, svc)
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Next()
	})
	r.POST("/web-search-providers", providers.CreateProvider)
	r.PUT("/web-search-providers/:id/credentials", credentials.Put)

	notFound := `{"success":false,"error":{"code":1003,"message":"web search provider not found"}}`
	runStatusCases(t, r, []statusCase{
		{
			"create invalid type", http.MethodPost, "/web-search-providers", `{"name":"n","provider":"altavista"}`, 400,
			`{"success":false,"error":{"code":1000,"message":"invalid provider type: altavista"}}`,
		},
		{
			"create repository failure", http.MethodPost, "/web-search-providers",
			`{"name":"n","provider":"duckduckgo"}`, 500, scrubbed500,
		},
		{
			"credentials missing provider", http.MethodPut, "/web-search-providers/gone/credentials",
			`{"api_key":"k"}`, 404, notFound,
		},
		{
			"credentials repository failure", http.MethodPut, "/web-search-providers/broken/credentials",
			`{"api_key":"k"}`, 500, scrubbed500,
		},
	})
}
