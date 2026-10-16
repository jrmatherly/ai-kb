package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type summaryRegenerateStub struct {
	interfaces.KnowledgeService
}

func (summaryRegenerateStub) GetKnowledgeByIDOnly(_ context.Context, id string) (*types.Knowledge, error) {
	return &types.Knowledge{ID: id, TenantID: 1, KnowledgeBaseID: "kb-1"}, nil
}

func (summaryRegenerateStub) GetKnowledgeByID(_ context.Context, id string) (*types.Knowledge, error) {
	return &types.Knowledge{ID: id, TenantID: 1, KnowledgeBaseID: "kb-1"}, nil
}

func (summaryRegenerateStub) RegenerateKnowledgeSummary(_ context.Context, id string) (*types.Knowledge, error) {
	if id == "authored" {
		return nil, apperrors.NewConflictError("summary generation is already running")
	}
	return nil, errors.New("enqueue summary task: dial tcp 10.0.0.6:6379: connection refused")
}

func TestRegenerateKnowledgeSummaryErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &KnowledgeHandler{kgService: summaryRegenerateStub{}}
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "u-test")
		c.Next()
	})
	r.POST("/knowledge/:id/summary", h.RegenerateKnowledgeSummary)

	runStatusCases(t, r, []statusCase{
		{
			"app error passes through", http.MethodPost, "/knowledge/authored/summary", "", 409,
			`{"success":false,"error":{"code":1005,"message":"summary generation is already running"}}`,
		},
		{"plain error is scrubbed", http.MethodPost, "/knowledge/plain/summary", "", 500, scrubbed500},
	})
}
