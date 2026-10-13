package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWikiErrorsEngine(t *testing.T) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}, &types.WikiPage{}, &types.WikiPageRevision{}))

	repo := repository.NewWikiPageRepository(db)
	for _, p := range []*types.WikiPage{
		{ID: "page-a", Slug: "concept/a", Title: "A", PageType: types.WikiPageTypeConcept},
		{ID: "page-index", Slug: "index", Title: "Index", PageType: types.WikiPageTypeIndex},
	} {
		p.TenantID, p.KnowledgeBaseID, p.Status, p.Version = 1, "kb-1", types.WikiPageStatusPublished, 1
		require.NoError(t, repo.Create(context.Background(), p))
	}
	h := &WikiPageHandler{
		wikiService: service.NewWikiPageService(repo, nil, nil, nil, nil, nil),
		kbService: &stubKBService{get: func(_ context.Context, id string) (*types.KnowledgeBase, error) {
			kb := &types.KnowledgeBase{ID: id, TenantID: 1}
			kb.IndexingStrategy.WikiEnabled = true
			return kb, nil
		}},
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler())
	engine.Use(func(c *gin.Context) { c.Set(types.TenantIDContextKey.String(), uint64(1)) })
	engine.POST("/knowledgebase/:kb_id/wiki/pages", h.CreatePage)
	engine.PUT("/knowledgebase/:kb_id/wiki/move-page", h.MovePage)
	engine.POST("/knowledgebase/:kb_id/wiki/folders", h.CreateFolder)
	engine.GET("/knowledgebase/:kb_id/wiki/graph", h.GetGraph)
	engine.GET("/knowledgebase/:kb_id/wiki/index", h.GetIndex)
	return engine
}

// Caller mistakes on the wiki routes must surface as 4xx with the service's
// message, not as a 500.
func TestWikiHandlersMapCallerMistakesTo4xx(t *testing.T) {
	engine := newWikiErrorsEngine(t)
	cases := []struct {
		name, method, path, body string
		wantCode                 int
		wantError                string
	}{
		{
			"create page without slug", http.MethodPost, "/knowledgebase/kb-1/wiki/pages", `{"title":"x"}`,
			http.StatusBadRequest, "wiki page slug is required",
		},
		{
			"create page in unknown folder", http.MethodPost, "/knowledgebase/kb-1/wiki/pages",
			`{"slug":"concept/b","title":"B","folder_id":"nope"}`,
			http.StatusNotFound, `wiki page references unknown folder "nope"`,
		},
		{
			"move page to unknown folder", http.MethodPut, "/knowledgebase/kb-1/wiki/move-page",
			`{"slug":"concept/a","folder_id":"nope"}`,
			http.StatusNotFound, `wiki page references unknown folder "nope"`,
		},
		{
			"create folder with blank name", http.MethodPost, "/knowledgebase/kb-1/wiki/folders", `{"name":"  "}`,
			http.StatusBadRequest, "folder name is required",
		},
		{
			"create folder with separator", http.MethodPost, "/knowledgebase/kb-1/wiki/folders", `{"name":"a/b"}`,
			http.StatusBadRequest, `folder name "a/b" must not contain a path separator`,
		},
		{
			"ego graph on unknown center", http.MethodGet, "/knowledgebase/kb-1/wiki/graph?mode=ego&center=missing", "",
			http.StatusNotFound, `ego center slug "missing" not found`,
		},
		{
			"index with bad cursor", http.MethodGet, "/knowledgebase/kb-1/wiki/index?cursor=abc", "",
			http.StatusBadRequest, `invalid cursor "abc"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
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
