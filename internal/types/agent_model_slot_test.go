package types

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listerFunc func(context.Context) ([]*Model, error)

func (f listerFunc) ListModels(ctx context.Context) ([]*Model, error) { return f(ctx) }

func staticModels(models ...*Model) ModelLister {
	return listerFunc(func(context.Context) ([]*Model, error) { return models, nil })
}

func TestAgentModelSlot(t *testing.T) {
	tenantRerank := &Model{ID: "tenant-rerank", Type: ModelTypeRerank, IsDefault: true, Status: ModelStatusActive}
	builtinRerank := &Model{ID: "builtin-rerank", Type: ModelTypeRerank, IsDefault: true, IsBuiltin: true}
	plainRerank := &Model{ID: "plain-rerank", Type: ModelTypeRerank}
	defaultChat := &Model{ID: "default-chat", Type: ModelTypeKnowledgeQA, IsDefault: true}
	downloadingRerank := &Model{
		ID: "downloading-rerank", Type: ModelTypeRerank, IsDefault: true, Status: ModelStatusDownloading,
	}
	smart := &CustomAgent{
		ID: BuiltinSmartReasoningID, IsBuiltin: true,
		Config: CustomAgentConfig{AgentMode: AgentModeSmartReasoning},
	}
	wikiFixer := &CustomAgent{ID: BuiltinWikiFixerID, IsBuiltin: true}
	custom := &CustomAgent{ID: "agent-1", Config: CustomAgentConfig{AgentMode: AgentModeSmartReasoning}}

	tests := []struct {
		name       string
		agent      *CustomAgent
		configured string
		models     []*Model
		shared     bool
		want       string
	}{
		{"tenant default", smart, "", []*Model{plainRerank, tenantRerank}, false, "tenant-rerank"},
		{"only builtin default", smart, "", []*Model{plainRerank, builtinRerank}, false, "builtin-rerank"},
		{"tenant default wins", smart, "", []*Model{builtinRerank, tenantRerank}, false, "tenant-rerank"},
		{"no default declared", smart, "", []*Model{plainRerank, defaultChat}, false, ""},
		{"inactive default is skipped", smart, "", []*Model{downloadingRerank, builtinRerank}, false, "builtin-rerank"},
		{"configured id kept as is", smart, " stale-rerank ", []*Model{tenantRerank}, false, "stale-rerank"},
		{"custom agent", custom, "", []*Model{tenantRerank}, false, ""},
		{"wiki fixer keeps its own fallback", wikiFixer, "", []*Model{tenantRerank}, false, ""},
		// Defensive: built-in agents cannot be shared (ErrBuiltinAgentNotShareable).
		{"shared read-only run", smart, "", []*Model{tenantRerank}, true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AgentModelSlot(
				context.Background(), staticModels(tt.models...), tt.agent, tt.shared, tt.configured, ModelTypeRerank,
			)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAgentModelSlotReportsListFailure(t *testing.T) {
	failing := listerFunc(func(context.Context) ([]*Model, error) { return nil, errors.New("db down") })
	agent := &CustomAgent{ID: BuiltinQuickAnswerID, IsBuiltin: true}

	got, err := AgentModelSlot(context.Background(), failing, agent, false, "", ModelTypeKnowledgeQA)

	require.EqualError(t, err, "resolve default KnowledgeQA model for agent builtin-quick-answer: db down")
	assert.Equal(t, "", got)
}

func TestDeclaredDefaultModelIDSkipsInactiveDefaults(t *testing.T) {
	models := []*Model{
		{ID: "failed-chat", Type: ModelTypeKnowledgeQA, IsDefault: true, Status: ModelStatusDownloadFailed},
		{ID: "builtin-chat", Type: ModelTypeKnowledgeQA, IsDefault: true, IsBuiltin: true, Status: ModelStatusActive},
	}

	assert.Equal(t, "builtin-chat", DeclaredDefaultModelID(models, ModelTypeKnowledgeQA))
	assert.Equal(t, "", DeclaredDefaultModelID(models[:1], ModelTypeKnowledgeQA))
}
