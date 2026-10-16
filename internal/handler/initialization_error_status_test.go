package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/models/chat"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type failingChat struct{ proseReplyChat }

func (failingChat) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return nil, errors.New("upstream returned 503")
}

type chatModelLookupStub struct {
	interfaces.ModelService
}

func (chatModelLookupStub) GetChatModel(_ context.Context, id string) (chat.Chat, error) {
	switch id {
	case "missing":
		return nil, service.ErrModelNotFound
	case "failing":
		return failingChat{}, nil
	}
	return nil, errors.New("dial tcp 10.0.0.5:5432: connection refused")
}

func TestChatModelLookupErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &InitializationHandler{
		modelService: chatModelLookupStub{},
		config: &config.Config{ExtractManager: &config.ExtractManagerConfig{
			ExtractGraph: &types.PromptTemplateStructured{Description: "extract"},
			FabriText:    &config.FebriText{WithTag: "tags %s", WithNoTag: "no tags"},
		}},
	}
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	r.POST("/extract/text-relation", h.ExtractTextRelations)
	r.POST("/extract/fabri-text", h.FabriText)

	notFound := `{"success":false,"error":{"code":1000,"message":"Failed to get model: model not found"}}`
	relations := func(model string) string { return `{"text":"t","tags":["Person"],"model_id":"` + model + `"}` }
	fabri := func(model string) string { return `{"model_id":"` + model + `"}` }
	runStatusCases(t, r, []statusCase{
		{"relations unknown model", http.MethodPost, "/extract/text-relation", relations("missing"), 400, notFound},
		{"relations lookup failure", http.MethodPost, "/extract/text-relation", relations("db"), 500, scrubbed500},
		{"fabri unknown model", http.MethodPost, "/extract/fabri-text", fabri("missing"), 400, notFound},
		{"fabri lookup failure", http.MethodPost, "/extract/fabri-text", fabri("db"), 500, scrubbed500},
		{
			"fabri chat failure", http.MethodPost, "/extract/fabri-text", fabri("failing"), 502,
			`{"success":false,"error":{"code":1011,"message":"failed to generate fabri text: upstream returned 503"}}`,
		},
	})
}
