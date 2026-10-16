package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/access"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type routeErrorsKnowledgeRepo struct{ editErrorsKnowledgeRepo }

func (routeErrorsKnowledgeRepo) GetKnowledgeBatch(
	_ context.Context, _ uint64, ids []string,
) ([]*types.Knowledge, error) {
	var out []*types.Knowledge
	for _, id := range ids {
		if id == "k-1" {
			out = append(out, &types.Knowledge{ID: "k-1", TenantID: 1, KnowledgeBaseID: "kb"})
		}
	}
	return out, nil
}

type routeErrorsKnowledgeService struct{ interfaces.KnowledgeService }

func (routeErrorsKnowledgeService) RegenerateChunkQuestions(
	_ context.Context, chunkID string,
) ([]types.GeneratedQuestion, error) {
	switch chunkID {
	case "forbidden":
		return nil, apperrors.NewForbiddenError("No permission to modify this knowledge base")
	case "text":
		return nil, apperrors.NewBadGatewayError("failed to generate questions: model call failed")
	}
	return nil, stderrors.New("update chunk: database is locked")
}

func newChunkRouteErrorsEngine(permission types.OrgMemberRole) *gin.Engine {
	text := &types.Chunk{
		ID: "text", TenantID: 1, KnowledgeID: "k-1", KnowledgeBaseID: "kb",
		ChunkType: types.ChunkTypeText, Content: "body", IsEnabled: true, IndexStatus: "ready",
	}
	repo := &editErrorsChunkRepo{chunks: map[string]*types.Chunk{"text": text}}
	h := NewChunkHandler(
		service.NewChunkService(repo, routeErrorsKnowledgeRepo{}, editErrorsKBRepo{}, nil, nil, nil, nil, nil),
		routeErrorsKnowledgeService{})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
		ctx = (&access.KBAccess{
			KnowledgeBase:     &types.KnowledgeBase{ID: "kb", TenantID: 1},
			Caller:            types.CallerFromContext(ctx),
			EffectiveTenantID: 1,
			Permission:        permission,
		}).Context(ctx)
		c.Request = c.Request.WithContext(ctx)
	})
	engine.DELETE("/chunks/by-id/:id/questions", h.DeleteGeneratedQuestion)
	engine.PUT("/chunks/by-id/:id/questions", h.UpsertGeneratedQuestion)
	engine.POST("/chunks/by-id/:id/questions/regenerate", h.RegenerateGeneratedQuestions)
	engine.DELETE("/chunks/:knowledge_id/:id", h.DeleteChunk)
	engine.DELETE("/chunks/:knowledge_id", h.DeleteChunksByKnowledgeID)
	return engine
}

// The question and delete routes answer with the service's own status: an
// AppError keeps its code and anything else is a 500.
func TestChunkQuestionAndDeleteRoutesKeepServiceStatus(t *testing.T) {
	cases := []struct {
		name       string
		permission types.OrgMemberRole
		method     string
		path, body string
		wantCode   int
		wantMsg    string
	}{
		{
			"edit unknown question", types.OrgRoleEditor, http.MethodPut, "/chunks/by-id/text/questions",
			`{"question_id":"nope","question":"q?"}`, http.StatusNotFound, "question not found",
		},
		{
			"delete question from a chunk without questions", types.OrgRoleEditor, http.MethodDelete,
			"/chunks/by-id/text/questions", `{"question_id":"nope"}`,
			http.StatusNotFound, "no generated questions found for chunk text",
		},
		{
			"edit question without write access", types.OrgRoleViewer, http.MethodPut, "/chunks/by-id/text/questions",
			`{"question":"q?"}`, http.StatusForbidden, "No permission to modify this knowledge base",
		},
		{
			"delete question without write access", types.OrgRoleViewer, http.MethodDelete,
			"/chunks/by-id/text/questions", `{"question_id":"q"}`,
			http.StatusForbidden, "No permission to modify this knowledge base",
		},
		{
			"regenerate without write access", types.OrgRoleEditor, http.MethodPost,
			"/chunks/by-id/forbidden/questions/regenerate", "",
			http.StatusForbidden, "No permission to modify this knowledge base",
		},
		{
			"regenerate when the model fails", types.OrgRoleEditor, http.MethodPost,
			"/chunks/by-id/text/questions/regenerate", "", http.StatusBadGateway,
			"failed to generate questions: model call failed",
		},
		{
			"regenerate when the repository fails", types.OrgRoleEditor, http.MethodPost,
			"/chunks/by-id/db/questions/regenerate", "", http.StatusInternalServerError, "Internal server error",
		},
		{
			"delete chunk without write access", types.OrgRoleViewer, http.MethodDelete, "/chunks/k-1/text", "",
			http.StatusForbidden, "No permission to modify this knowledge base",
		},
		{
			"delete chunks of an unknown document", types.OrgRoleEditor, http.MethodDelete, "/chunks/missing", "",
			http.StatusNotFound, "knowledge not found",
		},
		{
			"delete chunks without write access", types.OrgRoleViewer, http.MethodDelete, "/chunks/k-1", "",
			http.StatusForbidden, "No permission to modify this knowledge base",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newChunkRouteErrorsEngine(tc.permission).ServeHTTP(rec, req)

			var body struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
			require.Equal(t, tc.wantCode, rec.Code, rec.Body.String())
			require.Equal(t, tc.wantMsg, body.Error.Message)
		})
	}
}
