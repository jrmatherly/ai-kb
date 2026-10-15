package service

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/config"
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
			// Defensive: built-in agents cannot be shared (ErrBuiltinAgentNotShareable).
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

// Quick Answer reranks with the declared default when its rerank slot is
// empty; without one the pipeline keeps skipping rerank.
func TestApplyAgentOverridesResolvesBuiltinRerankSlot(t *testing.T) {
	defaultRerank := &types.Model{ID: "default-rerank", Type: types.ModelTypeRerank, IsDefault: true}
	plainRerank := &types.Model{ID: "plain-rerank", Type: types.ModelTypeRerank}
	quickAnswer := &types.CustomAgent{ID: types.BuiltinQuickAnswerID, IsBuiltin: true}

	tests := []struct {
		name   string
		agent  *types.CustomAgent
		models []*types.Model
		want   string
	}{
		{"builtin uses the declared default", quickAnswer, []*types.Model{plainRerank, defaultRerank}, "default-rerank"},
		{"builtin without a default skips rerank", quickAnswer, []*types.Model{plainRerank}, ""},
		{"custom agent keeps an empty slot", &types.CustomAgent{ID: "agent-1"}, []*types.Model{defaultRerank}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &sessionService{cfg: &config.Config{}, modelService: &stubModelService{availableModels: tt.models}}
			cm := &types.ChatManage{}

			svc.applyAgentOverridesToChatManage(context.Background(), tt.agent, false, cm)

			assert.Equal(t, tt.want, cm.RerankModelID)
		})
	}
}
