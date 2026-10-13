package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/access"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type editErrorsChunkRepo struct {
	interfaces.ChunkRepository
	chunks map[string]*types.Chunk
}

func (r *editErrorsChunkRepo) GetChunkByID(_ context.Context, _ uint64, id string) (*types.Chunk, error) {
	chunk, ok := r.chunks[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copyOfChunk := *chunk
	return &copyOfChunk, nil
}

func (r *editErrorsChunkRepo) ListChunkByParentID(context.Context, uint64, string) ([]*types.Chunk, error) {
	return nil, nil
}

func (r *editErrorsChunkRepo) GetChunkRevision(
	context.Context, uint64, string, int,
) (*types.ChunkRevision, error) {
	return &types.ChunkRevision{}, gorm.ErrRecordNotFound
}

type editErrorsKnowledgeRepo struct{ interfaces.KnowledgeRepository }

func (editErrorsKnowledgeRepo) GetKnowledgeByID(context.Context, uint64, string) (*types.Knowledge, error) {
	return &types.Knowledge{ID: "k-1", TenantID: 1, KnowledgeBaseID: "kb"}, nil
}

type editErrorsKBRepo struct {
	interfaces.KnowledgeBaseRepository
}

func (editErrorsKBRepo) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return &types.KnowledgeBase{ID: "kb", TenantID: 1}, nil
}

func newChunkEditErrorsEngine() *gin.Engine {
	text := &types.Chunk{
		ID: "text", TenantID: 1, KnowledgeID: "k-1", KnowledgeBaseID: "kb",
		ChunkType: types.ChunkTypeText, Content: "body", IsEnabled: true, IndexStatus: "ready",
	}
	image := *text
	image.ID, image.ChunkType = "image", types.ChunkTypeImageOCR
	repo := &editErrorsChunkRepo{chunks: map[string]*types.Chunk{"text": text, "image": &image}}
	h := NewChunkHandler(
		service.NewChunkService(repo, editErrorsKnowledgeRepo{}, editErrorsKBRepo{}, nil, nil, nil, nil, nil), nil)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = (&access.KBAccess{
			KnowledgeBase:     &types.KnowledgeBase{ID: "kb", TenantID: 1},
			Caller:            types.CallerFromContext(ctx),
			EffectiveTenantID: 1,
			Permission:        types.OrgRoleEditor,
		}).Context(ctx)
		c.Request = c.Request.WithContext(ctx)
	})
	engine.PUT("/chunks/:knowledge_id/:id", h.UpdateChunk)
	engine.POST("/chunks/:knowledge_id/:id/revert", h.RevertChunk)
	return engine
}

// A rejected chunk edit is the caller's mistake: PUT and revert both answer
// 400 with the service's message.
func TestChunkEditRejectionsReturn400(t *testing.T) {
	engine := newChunkEditErrorsEngine()
	cases := []struct {
		name, method, path, body, wantMessage string
	}{
		{
			"non-text chunk", http.MethodPut, "/chunks/k-1/image", `{"content":"x"}`,
			"only text chunks can be edited",
		},
		{
			"empty content", http.MethodPut, "/chunks/k-1/text", `{"content":"   "}`,
			"chunk content cannot be empty",
		},
		{
			"oversized content", http.MethodPut, "/chunks/k-1/text",
			`{"content":"` + strings.Repeat("a", 200001) + `"}`, "chunk content exceeds 200000 bytes",
		},
		{
			"new image", http.MethodPut, "/chunks/k-1/text", `{"content":"body ![x](resource://new)"}`,
			"adding images to an existing chunk is not supported: resource://new",
		},
		{
			"revert to an unknown revision", http.MethodPost, "/chunks/k-1/text/revert", `{"revision":7}`,
			"chunk revision 7 not found",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			var body struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Equal(t, tc.wantMessage, body.Error.Message)
		})
	}
}
