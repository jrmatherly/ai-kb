package memory

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

// This file is the behavioural regression set. Each case is a short sequence
// of turns across separate sessions, ending in an assertion about what the
// next turn's prompt contains. They are written against the service the chat
// path actually calls, so a change that keeps the unit tests green but breaks
// the user-visible behaviour still fails here.
//
// LoCoMo and LongMemEval are deliberately not used: their published scores are
// vendor-run and disagree by tens of points, and neither has a Chinese split
// that matches how this feature is used.

type memoryScenario struct {
	name string
	// userTurns are what the user says, in order, as if across sessions.
	userTurns []string
	// extracted is the distillation the model returns for those turns.
	extracted []map[string]any
	// laterQuery is what the user asks in a new session afterwards.
	laterQuery string
	// wantInPrompt must appear in the memory injected into that later turn.
	wantInPrompt []string
	// wantAbsent must not appear.
	wantAbsent []string
}

func TestCrossSessionMemoryScenarios(t *testing.T) {
	scenarios := []memoryScenario{
		{
			name: "\u8bb0\u4f4f\u4e2a\u4eba\u753b\u50cf\u5e76\u5728\u65b0\u4f1a\u8bdd\u91cc\u5e26\u4e0a",
			userTurns: []string{"\u6211\u662f\u505a\u533b\u7597\u5f71\u50cf\u7684\u540e\u7aef\u5de5\u7a0b\u5e08\uff0c" +
				"\u4e3b\u8981\u5199 Go"},
			extracted: []map[string]any{
				{
					"action": "add", "kind": "profile", "topic": "\u804c\u4e1a",
					"content": "\u533b\u7597\u5f71\u50cf\u65b9\u5411\u7684\u540e\u7aef\u5de5\u7a0b\u5e08\uff0c\u4e3b" +
						"\u8981\u5199 Go",
				},
			},
			laterQuery:   "\u5e2e\u6211\u8bbe\u8ba1\u4e00\u4e2a\u63a5\u53e3",
			wantInPrompt: []string{"\u533b\u7597\u5f71\u50cf", "\u540e\u7aef\u5de5\u7a0b\u5e08"},
		},
		{
			name: "\u504f\u597d\u5e38\u9a7b\uff0c\u4e0e\u95ee\u9898\u5185\u5bb9\u65e0\u5173\u4e5f\u4f1a\u5e26" +
				"\u4e0a",
			userTurns: []string{"\u4ee5\u540e\u56de\u7b54\u76f4\u63a5\u7ed9\u7ed3\u8bba\uff0c\u4e0d\u8981\u957f\u7bc7" +
				"\u94fa\u57ab"},
			extracted: []map[string]any{
				{
					"action": "add", "kind": "preference", "topic": "\u56de\u7b54\u98ce\u683c",
					"content": "\u56de\u7b54\u76f4\u63a5\u7ed9\u7ed3\u8bba\uff0c\u4e0d\u8981\u94fa\u57ab",
				},
			},
			laterQuery:   "\u4eca\u5929\u7684\u5929\u6c14\u9002\u5408\u8dd1\u6b65\u5417",
			wantInPrompt: []string{"\u76f4\u63a5\u7ed9\u7ed3\u8bba"},
		},
		{
			name: "\u4e8b\u5b9e\u6309\u95ee\u9898\u76f8\u5173\u6027\u53ec\u56de\uff0c\u4e0d\u76f8\u5173\u7684\u4e0d" +
				"\u8fdb\u4e0a\u4e0b\u6587",
			userTurns: []string{
				"\u6211\u4eec\u751f\u4ea7\u5e93\u662f PostgreSQL 17\uff0c\u8dd1\u5728\u6cd5\u5170\u514b\u798f",
				"\u524d\u7aef\u662f Vue 3 \u52a0 Vite",
			},
			extracted: []map[string]any{
				{
					"action": "add", "kind": "fact", "topic": "\u751f\u4ea7\u6570\u636e\u5e93",
					"content": "\u751f\u4ea7\u5e93\u662f PostgreSQL 17\uff0c\u90e8\u7f72\u5728\u6cd5\u5170\u514b\u798f",
				},
				{
					"action": "add", "kind": "fact", "topic": "\u524d\u7aef\u6280\u672f\u6808",
					"content": "\u524d\u7aef\u662f Vue 3 \u52a0 Vite",
				},
			},
			laterQuery:   "\u6570\u636e\u5e93\u8fde\u63a5\u6c60\u5e94\u8be5\u914d\u591a\u5927",
			wantInPrompt: []string{"PostgreSQL 17"},
			wantAbsent:   []string{"Vue 3"},
		},
		{
			name: "\u4fee\u6b63\u77db\u76fe\u4fe1\u606f\u540e\u53ea\u4fdd\u7559\u6700\u65b0\u7684",
			userTurns: []string{
				"\u6211\u4eec\u7528\u7684\u662f MySQL",
				"\u66f4\u6b63\u4e00\u4e0b\uff0c\u6211\u4eec\u4e0a\u4e2a\u6708\u5df2\u7ecf\u8fc1\u5230 PostgreSQL " +
					"\u4e86",
			},
			extracted: []map[string]any{
				{
					"action": "add", "kind": "fact", "topic": "\u5728\u7528\u7684\u6570\u636e\u5e93",
					"content": "\u7528\u7684\u662f MySQL",
				},
				{
					"action": "update", "kind": "fact", "topic": "\u5728\u7528\u7684\u6570\u636e\u5e93",
					"content": "\u5df2\u7ecf\u8fc1\u5230 PostgreSQL",
				},
			},
			laterQuery:   "\u5199\u4e00\u6bb5\u8fde\u63a5\u6570\u636e\u5e93\u7684\u793a\u4f8b\u4ee3\u7801",
			wantInPrompt: []string{"PostgreSQL"},
			wantAbsent:   []string{"MySQL"},
		},
		{
			name: "\u5728\u529e\u4e8b\u9879\u53ef\u4ee5\u8de8\u4f1a\u8bdd\u7eed\u63a5",
			userTurns: []string{"\u8fd9\u5468\u5728\u91cd\u6784\u8ba2\u5355\u670d\u52a1\u7684\u652f\u4ed8\u6d41\u7a0b" +
				"\uff0c\u8fd8\u6ca1\u5f04\u5b8c"},
			extracted: []map[string]any{
				{
					"action": "add", "kind": "task", "topic": "\u5728\u505a\u7684\u91cd\u6784",
					"content": "\u5728\u91cd\u6784\u8ba2\u5355\u670d\u52a1\u7684\u652f\u4ed8\u6d41\u7a0b\uff0c\u5c1a" +
						"\u672a\u5b8c\u6210",
				},
			},
			laterQuery:   "\u8ba2\u5355\u670d\u52a1\u90a3\u4e2a\u91cd\u6784\u63a5\u7740\u5f80\u4e0b\u600e\u4e48\u505a",
			wantInPrompt: []string{"\u652f\u4ed8\u6d41\u7a0b"},
		},
		{
			name: "\u4e8b\u60c5\u505a\u5b8c\u540e\u4e0d\u518d\u88ab\u53ec\u56de",
			userTurns: []string{
				"\u5728\u91cd\u6784\u8ba2\u5355\u670d\u52a1\u7684\u652f\u4ed8\u6d41\u7a0b",
				"\u652f\u4ed8\u6d41\u7a0b\u91cd\u6784\u5df2\u7ecf\u4e0a\u7ebf\u4e86",
			},
			extracted: []map[string]any{
				{
					"action": "add", "kind": "task", "topic": "\u5728\u505a\u7684\u91cd\u6784",
					"content": "\u5728\u91cd\u6784\u8ba2\u5355\u670d\u52a1\u7684\u652f\u4ed8\u6d41\u7a0b",
				},
				{
					"action": "delete", "kind": "task", "topic": "\u5728\u505a\u7684\u91cd\u6784",
					"content": "\u652f\u4ed8\u6d41\u7a0b\u91cd\u6784\u5df2\u5b8c\u6210",
				},
			},
			laterQuery: "\u8ba2\u5355\u670d\u52a1\u73b0\u5728\u8fd8\u6709\u4ec0\u4e48\u5728\u505a\u7684",
			wantAbsent: []string{"\u5728\u91cd\u6784\u8ba2\u5355\u670d\u52a1\u7684\u652f\u4ed8\u6d41\u7a0b"},
		},
		{
			name:      "\u4e00\u6b21\u6027\u7684\u63d0\u95ee\u4e0d\u8be5\u88ab\u8bb0\u6210\u957f\u671f\u4e8b\u5b9e",
			userTurns: []string{"Go \u7684 map \u662f\u5e76\u53d1\u5b89\u5168\u7684\u5417"},
			// A well-behaved extraction returns nothing here, which is the
			// normal outcome; the assertion is that we store nothing either.
			extracted:  nil,
			laterQuery: "Go \u7684 slice \u5e95\u5c42\u662f\u600e\u4e48\u6269\u5bb9\u7684",
			wantAbsent: []string{"map", "\u5e76\u53d1\u5b89\u5168"},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			svc, tenantRepo, messages, models, _ := newExtractionHarness(t)
			tenantRepo.set(1, &types.MemoryConfig{Enabled: true, WriteMode: types.MemoryWriteAuto})

			// Replay the turns one distillation at a time, so an update or a
			// delete sees the state its predecessor left behind.
			for i, turn := range scenario.userTurns {
				messages.messages = []*types.Message{{Role: "user", Content: turn}}
				var decisions []map[string]any
				if i < len(scenario.extracted) {
					decisions = scenario.extracted[i : i+1]
				}
				body, err := json.Marshal(map[string]any{"memories": decisions})
				require.NoError(t, err)
				models.response = string(body)

				require.NoError(t, svc.Handle(context.Background(), extractTask(t, types.MemoryExtractPayload{
					TenantID:    1,
					SubjectID:   "web_user:alice",
					SessionID:   "session-" + string(rune('a'+i)),
					MessageID:   "message-" + string(rune('a'+i)),
					ChatModelID: "conversation-model",
				})))
			}

			// A brand new session: nothing but long-term memory carries over.
			laterCtx := enabledCtx(t, tenantRepo, 1, "alice")
			prompt := svc.Recall(laterCtx, scenario.laterQuery).Prompt

			for _, want := range scenario.wantInPrompt {
				require.Contains(t, prompt, want,
					"expected the later turn to carry %q\nprompt was:\n%s", want, prompt)
			}
			for _, absent := range scenario.wantAbsent {
				require.NotContains(t, prompt, absent,
					"did not expect the later turn to carry %q\nprompt was:\n%s", absent, prompt)
			}
		})
	}
}

// TestReadPathMakesNoModelCall pins the cost promise: recall must not add a
// model call to a turn, no matter how many memories the user has.
func TestReadPathMakesNoModelCall(t *testing.T) {
	svc, tenantRepo, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	for i := 0; i < 30; i++ {
		_, err := svc.Remember(ctx, types.MemoryItem{
			Kind:    types.MemoryKindFact,
			Topic:   "\u4e8b\u5b9e" + string(rune('a'+i)),
			Content: "\u6570\u636e\u5e93\u76f8\u5173\u7684\u4e8b\u5b9e " + string(rune('a'+i)),
		})
		require.NoError(t, err)
	}

	for i := 0; i < 5; i++ {
		require.NotEmpty(t, svc.Recall(ctx, "\u6570\u636e\u5e93\u600e\u4e48\u8c03\u4f18").Prompt)
	}
	require.Zero(t, models.calls, "the read path must not call a model")
}

// TestInjectedMemoryStaysInsideItsBudget keeps a user with a large memory
// space from quietly eating the context window.
func TestInjectedMemoryStaysInsideItsBudget(t *testing.T) {
	svc, tenantRepo, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	for i := 0; i < 40; i++ {
		_, err := svc.Remember(ctx, types.MemoryItem{
			Kind:    types.MemoryKindPreference,
			Topic:   "\u504f\u597d" + string(rune('a'+i)),
			Content: strings.Repeat("\u5f88\u957f\u7684\u504f\u597d\u8bf4\u660e", 8) + string(rune('a'+i)),
		})
		require.NoError(t, err)
		_, err = svc.Remember(ctx, types.MemoryItem{
			Kind:  types.MemoryKindFact,
			Topic: "\u4e8b\u5b9e" + string(rune('a'+i)),
			Content: "\u6570\u636e\u5e93" + strings.Repeat("\u5f88\u957f\u7684\u4e8b\u5b9e\u8bf4\u660e",
				8) + string(rune('a'+i)),
		})
		require.NoError(t, err)
	}

	prompt := svc.Recall(ctx, "\u6570\u636e\u5e93\u600e\u4e48\u8c03\u4f18").Prompt
	require.NotEmpty(t, prompt)
	// Envelope wording aside, the memory content itself must fit in the two
	// declared budgets.
	require.LessOrEqual(t, len([]rune(prompt)),
		types.MemoryBlockRuneBudget+types.MemoryRecallRuneBudget+600,
		"injected memory must stay within its budget")
}
