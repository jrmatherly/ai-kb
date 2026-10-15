package service

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveChatModelIDRequiresConfiguredAgentModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"builtin-chat": {
					ID:   "builtin-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
		},
		// Even a valid request-level model must not hide incomplete agent config.
		SummaryModelID: "builtin-chat",
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.Error(t, err)
	assert.Empty(t, modelID)
	assert.Contains(t, err.Error(), "model_id")
}

func TestResolveChatModelIDRejectsUnavailableConfiguredAgentModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{modelsByID: map[string]*types.Model{}},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "deleted-model",
			},
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.Error(t, err)
	assert.Empty(t, modelID)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestResolveChatModelIDUsesValidConfiguredAgentModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat": {
					ID:   "agent-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "agent-chat",
			},
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "agent-chat", modelID)
}

func TestResolveChatModelIDRejectsNonChatSummaryModelOverride(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat": {
					ID:   "agent-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
				"rerank-only": {
					ID:   "rerank-only",
					Type: types.ModelTypeRerank,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "agent-chat",
			},
		},
		SummaryModelID: "rerank-only",
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "agent-chat", modelID)
}

func TestResolveChatModelIDUsesValidSummaryModelOverride(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat": {
					ID:   "agent-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
				"override-chat": {
					ID:   "override-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: "agent-1",
			Config: types.CustomAgentConfig{
				ModelID: "agent-chat",
			},
		},
		SummaryModelID: "override-chat",
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "override-chat", modelID)
}

func TestResolveChatModelIDWikiFixerFallsBackToKnowledgeBaseModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"wiki-chat": {
					ID:   "wiki-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
		knowledgeBaseService: &fakeAgentKnowledgeBaseService{
			kb: &types.KnowledgeBase{
				ID:             "wiki-kb",
				SummaryModelID: "wiki-chat",
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: types.BuiltinWikiFixerID,
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, []string{"wiki-kb"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "wiki-chat", modelID)
}

func TestResolveChatModelIDWikiFixerFallsBackToAvailableModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			availableModels: []*types.Model{
				{
					ID:   "system-chat",
					Type: types.ModelTypeKnowledgeQA,
				},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID: types.BuiltinWikiFixerID,
		},
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "system-chat", modelID)
}

// A shared agent executes in its owner's workspace; the override would pick
// any of the owner's models there, so the configured model is used instead.
func TestResolveChatModelIDIgnoresOverrideForSharedAgents(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-chat":        {ID: "agent-chat", Type: types.ModelTypeKnowledgeQA},
				"owner-other-model": {ID: "owner-other-model", Type: types.ModelTypeKnowledgeQA},
			},
		},
	}
	req := &types.QARequest{
		Session: &types.Session{},
		CustomAgent: &types.CustomAgent{
			ID:     "agent-1",
			Config: types.CustomAgentConfig{ModelID: "agent-chat"},
		},
		SummaryModelID:      "owner-other-model",
		SharedAgentReadOnly: true,
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "agent-chat", modelID)
}

// Quick-answer mode follows the same rule as agent mode: the request switch
// only opts a turn in and cannot enable search the agent turns off.
func TestResolveWebSearchEnabled(t *testing.T) {
	agent := func(enabled bool) *types.CustomAgent {
		return &types.CustomAgent{Config: types.CustomAgentConfig{WebSearchEnabled: enabled}}
	}
	assert.False(t, resolveWebSearchEnabled(&types.QARequest{CustomAgent: agent(false), WebSearchEnabled: true}))
	assert.False(t, resolveWebSearchEnabled(&types.QARequest{CustomAgent: agent(true), WebSearchEnabled: false}))
	assert.True(t, resolveWebSearchEnabled(&types.QARequest{CustomAgent: agent(true), WebSearchEnabled: true}))
	assert.True(t, resolveWebSearchEnabled(&types.QARequest{WebSearchEnabled: true}), "no agent keeps the switch")
}

// Built-in agents with an empty model slot run on the workspace's declared
// default of that type: the tenant's own default first, then the built-in one.
func TestResolveChatModelIDBuiltinAgentUsesDeclaredDefault(t *testing.T) {
	tenantDefault := &types.Model{ID: "tenant-chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true}
	builtinDefault := &types.Model{
		ID: "builtin-chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true, IsBuiltin: true,
	}
	plainChat := &types.Model{ID: "plain-chat", Type: types.ModelTypeKnowledgeQA}
	defaultRerank := &types.Model{ID: "default-rerank", Type: types.ModelTypeRerank, IsDefault: true}
	quickAnswer := func() *types.CustomAgent {
		return &types.CustomAgent{ID: types.BuiltinQuickAnswerID, IsBuiltin: true}
	}

	tests := []struct {
		name    string
		agent   *types.CustomAgent
		models  []*types.Model
		shared  bool
		want    string
		wantErr string
	}{
		{
			name:   "tenant default",
			agent:  quickAnswer(),
			models: []*types.Model{plainChat, tenantDefault},
			want:   "tenant-chat",
		},
		{
			name:   "only builtin default",
			agent:  quickAnswer(),
			models: []*types.Model{plainChat, builtinDefault},
			want:   "builtin-chat",
		},
		{
			name:   "tenant default wins over builtin default",
			agent:  quickAnswer(),
			models: []*types.Model{builtinDefault, tenantDefault},
			want:   "tenant-chat",
		},
		{
			name:    "no default declared",
			agent:   quickAnswer(),
			models:  []*types.Model{plainChat, defaultRerank},
			wantErr: "chat model is not configured: please set model_id on agent builtin-quick-answer",
		},
		{
			name:    "custom agent keeps requiring an explicit model",
			agent:   &types.CustomAgent{ID: "agent-1"},
			models:  []*types.Model{tenantDefault, builtinDefault},
			wantErr: "chat model is not configured: please set model_id on agent agent-1",
		},
		{
			name: "stale configured model is not masked",
			agent: &types.CustomAgent{
				ID: types.BuiltinQuickAnswerID, IsBuiltin: true,
				Config: types.CustomAgentConfig{ModelID: "deleted-chat"},
			},
			models:  []*types.Model{tenantDefault},
			wantErr: "configured chat model deleted-chat is unavailable for agent builtin-quick-answer",
		},
		{
			name:    "shared read-only run gets no fallback",
			agent:   quickAnswer(),
			models:  []*types.Model{tenantDefault},
			shared:  true,
			wantErr: "chat model is not configured: please set model_id on agent builtin-quick-answer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			byID := map[string]*types.Model{}
			for _, m := range tt.models {
				byID[m.ID] = m
			}
			svc := &sessionService{
				modelService: &stubModelService{modelsByID: byID, availableModels: tt.models},
			}
			req := &types.QARequest{
				Session:             &types.Session{},
				CustomAgent:         tt.agent,
				SharedAgentReadOnly: tt.shared,
			}

			modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Equal(t, "", modelID)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, modelID)
		})
	}
}

// Once the built-in agent's chat model resolves, a valid request override is
// honored as for an explicitly configured agent.
func TestResolveChatModelIDBuiltinDefaultAllowsSummaryOverride(t *testing.T) {
	tenantChat := &types.Model{ID: "tenant-chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true}
	overrideChat := &types.Model{ID: "override-chat", Type: types.ModelTypeKnowledgeQA}
	svc := &sessionService{modelService: &stubModelService{
		modelsByID:      map[string]*types.Model{"tenant-chat": tenantChat, "override-chat": overrideChat},
		availableModels: []*types.Model{tenantChat, overrideChat},
	}}
	req := &types.QARequest{
		Session:        &types.Session{},
		CustomAgent:    &types.CustomAgent{ID: types.BuiltinQuickAnswerID, IsBuiltin: true},
		SummaryModelID: "override-chat",
	}

	modelID, err := svc.resolveChatModelID(context.Background(), req, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "override-chat", modelID)
}

// The rerank requirement of a smart-reasoning agent reads the same slot rule.
func TestAgentModelSlotRerankForBuiltinSmartReasoning(t *testing.T) {
	tenantDefault := &types.Model{ID: "tenant-rerank", Type: types.ModelTypeRerank, IsDefault: true}
	builtinDefault := &types.Model{
		ID: "builtin-rerank", Type: types.ModelTypeRerank, IsDefault: true, IsBuiltin: true,
	}
	plainRerank := &types.Model{ID: "plain-rerank", Type: types.ModelTypeRerank}
	defaultChat := &types.Model{ID: "default-chat", Type: types.ModelTypeKnowledgeQA, IsDefault: true}
	smart := func(rerankID string) *types.CustomAgent {
		return &types.CustomAgent{
			ID: types.BuiltinSmartReasoningID, IsBuiltin: true,
			Config: types.CustomAgentConfig{AgentMode: types.AgentModeSmartReasoning, RerankModelID: rerankID},
		}
	}
	custom := &types.CustomAgent{
		ID: "agent-1", Config: types.CustomAgentConfig{AgentMode: types.AgentModeSmartReasoning},
	}

	tests := []struct {
		name   string
		agent  *types.CustomAgent
		models []*types.Model
		shared bool
		want   string
	}{
		{"tenant default", smart(""), []*types.Model{plainRerank, tenantDefault}, false, "tenant-rerank"},
		{"only builtin default", smart(""), []*types.Model{plainRerank, builtinDefault}, false, "builtin-rerank"},
		{"tenant default wins", smart(""), []*types.Model{builtinDefault, tenantDefault}, false, "tenant-rerank"},
		{"no default declared", smart(""), []*types.Model{plainRerank, defaultChat}, false, ""},
		{"configured id kept as is", smart("stale-rerank"), []*types.Model{tenantDefault}, false, "stale-rerank"},
		{"shared read-only run", smart(""), []*types.Model{tenantDefault}, true, ""},
		{"custom agent", custom, []*types.Model{tenantDefault}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &stubModelService{availableModels: tt.models}
			req := &types.QARequest{CustomAgent: tt.agent, SharedAgentReadOnly: tt.shared}

			modelID, err := agentModelSlot(
				context.Background(), ms, req, tt.agent.Config.RerankModelID, types.ModelTypeRerank,
			)

			require.NoError(t, err)
			assert.Equal(t, tt.want, modelID)
		})
	}
}
