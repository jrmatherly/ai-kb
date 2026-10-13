package router

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	apprepo "github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/handler"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type wikiHandlerKBService struct {
	interfaces.KnowledgeBaseService
	err error
}

func (s *wikiHandlerKBService) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return nil, s.err
}

var wikiKBRoutes = []struct{ method, path string }{
	{http.MethodGet, "/pages"},
	{http.MethodPost, "/pages"},
	{http.MethodPut, "/move-page"},
	{http.MethodGet, "/pages/a"},
	{http.MethodPut, "/pages/a"},
	{http.MethodDelete, "/pages/a"},
	{http.MethodGet, "/revisions/a"},
	{http.MethodPost, "/revert"},
	{http.MethodGet, "/folders"},
	{http.MethodPost, "/folders"},
	{http.MethodPut, "/folders/f"},
	{http.MethodDelete, "/folders/f"},
	{http.MethodGet, "/index"},
	{http.MethodGet, "/graph"},
	{http.MethodGet, "/stats"},
	{http.MethodGet, "/search?q=x"},
	{http.MethodPost, "/rebuild-links"},
	{http.MethodGet, "/lint"},
	{http.MethodPost, "/auto-fix"},
	{http.MethodGet, "/issues"},
	{http.MethodPut, "/issues/i/status"},
}

// The access guard has already resolved the KB; the handler's own lookup then
// fails (the KB was deleted in between, or the database is down). Every wiki
// route must report that with the lookup's status, not a blanket 400.
func TestWikiRoutesReportKBLookupFailureStatus(t *testing.T) {
	cases := []struct {
		name      string
		lookupErr error
		wantCode  int
		wantError string
	}{
		{"deleted knowledge base", apprepo.ErrKnowledgeBaseNotFound, http.StatusNotFound, "Knowledge base not found"},
		{"database failure", errors.New("connection refused"), http.StatusInternalServerError, "connection refused"},
	}
	for _, tc := range cases {
		wikiHandler := handler.NewWikiPageHandler(nil, &wikiHandlerKBService{err: tc.lookupErr}, nil, nil, nil)
		engine := newKBRouteTestEngine(t, 1, tenantKBLookupFixture(), &types.TenantAPIKeyScope{FullAccess: true},
			func(r *gin.RouterGroup, guards *rbacGuards) { RegisterWikiPageRoutes(r, wikiHandler, guards) })
		for _, route := range wikiKBRoutes {
			t.Run(tc.name+" "+route.method+" "+route.path, func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(route.method, "/api/v1/knowledgebase/kb-allowed/wiki"+route.path,
					strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				engine.ServeHTTP(rec, req)

				var body struct {
					Success bool `json:"success"`
					Error   struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
				require.Equal(t, tc.wantCode, rec.Code, rec.Body.String())
				require.False(t, body.Success, rec.Body.String())
				require.Equal(t, tc.wantError, body.Error.Message)
			})
		}
	}
}
