package session

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
)

type modelListStub struct {
	interfaces.ModelService
	models []*types.Model
}

func (s *modelListStub) ListModels(context.Context) ([]*types.Model, error) { return s.models, nil }

func (s *modelListStub) GetModelByID(_ context.Context, id string) (*types.Model, error) {
	for _, m := range s.models {
		if m.ID == id {
			return m, nil
		}
	}
	return nil, nil
}

// The message, its title and memory extraction record the model the answer
// runs on, so a built-in agent without model_id records the declared default.
func TestMessageChatModelID(t *testing.T) {
	h := &Handler{modelService: &modelListStub{models: []*types.Model{
		{ID: "default-chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true},
		{ID: "other-chat", Type: types.ModelTypeKnowledgeQA},
		{ID: "a-rerank", Type: types.ModelTypeRerank},
	}}}
	quickAnswer := &types.CustomAgent{ID: types.BuiltinQuickAnswerID, IsBuiltin: true}
	custom := &types.CustomAgent{ID: "agent-1", Config: types.CustomAgentConfig{ModelID: "other-chat"}}
	ctx := context.Background()

	assert.Equal(t, "default-chat", h.messageChatModelID(ctx, quickAnswer, false, ""))
	assert.Equal(t, "other-chat", h.messageChatModelID(ctx, quickAnswer, false, "other-chat"))
	assert.Equal(t, "default-chat", h.messageChatModelID(ctx, quickAnswer, false, "a-rerank"), "invalid override")
	assert.Equal(t, "other-chat", h.messageChatModelID(ctx, custom, false, ""))
	assert.Equal(t, "", h.messageChatModelID(ctx, &types.CustomAgent{ID: "agent-2"}, false, ""))
	assert.Equal(t, "other-chat", h.messageChatModelID(ctx, nil, false, "other-chat"), "no agent keeps the override")
}
