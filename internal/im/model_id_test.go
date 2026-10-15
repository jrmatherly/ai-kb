package im

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

type modelListStub struct {
	interfaces.ModelService
	models []*types.Model
}

func (s *modelListStub) ListModels(context.Context) ([]*types.Model, error) { return s.models, nil }

// IM titles and last-request state use the model the answer runs on.
func TestAgentChatModelID(t *testing.T) {
	s := &Service{modelService: &modelListStub{models: []*types.Model{
		{ID: "builtin-chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true, IsBuiltin: true},
	}}}
	ctx := context.Background()

	cases := []struct {
		agent *types.CustomAgent
		want  string
	}{
		{&types.CustomAgent{ID: types.BuiltinQuickAnswerID, IsBuiltin: true}, "builtin-chat"},
		{&types.CustomAgent{ID: "agent-1", Config: types.CustomAgentConfig{ModelID: "model-1"}}, "model-1"},
		{&types.CustomAgent{ID: "agent-2"}, ""},
		{nil, ""},
	}
	for _, c := range cases {
		if got := s.agentChatModelID(ctx, c.agent); got != c.want {
			t.Fatalf("agentChatModelID(%v) = %q, want %q", c.agent, got, c.want)
		}
	}
}
