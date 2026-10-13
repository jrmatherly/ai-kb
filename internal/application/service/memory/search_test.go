package memory

import (
	"fmt"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

// The reason search exists at all: recall is ranked once, against the question
// the user opened with. An agent that works its way from that question to a
// different sub-problem is holding memories chosen for a query it has left
// behind, and nothing in the turn's budget can fix that.
func TestSearchFindsWhatTheOpeningQuestionDidNotMatch(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	for _, item := range []types.MemoryItem{
		{
			Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93",
			Content: "\u751f\u4ea7\u6570\u636e\u5e93\u7528\u7684\u662f PostgreSQL",
		},
		{
			Kind: types.MemoryKindFact, Topic: "\u90e8\u7f72",
			Content: "\u90e8\u7f72\u8d70\u7684\u662f\u84dd\u7eff\u53d1\u5e03",
		},
	} {
		_, err := svc.Remember(ctx, item)
		require.NoError(t, err)
	}

	// The turn opened with a database question, so that is what recall ranked
	// against and the deployment memory is nowhere in the prompt.
	recall := svc.Recall(ctx, "\u5e2e\u6211\u770b\u770b\u6570\u636e\u5e93\u8fde\u63a5\u6c60\u7684\u914d\u7f6e")
	require.Contains(t, recall.Prompt, "PostgreSQL")
	require.NotContains(t, recall.Prompt, "\u84dd\u7eff\u53d1\u5e03")

	// Several iterations later the agent is looking at deployment instead.
	result := svc.SearchMemory(ctx, "\u90e8\u7f72\u65b9\u5f0f", 10)
	require.True(t, result.Available)
	require.Len(t, result.Items, 1)
	require.Equal(t, "\u90e8\u7f72\u8d70\u7684\u662f\u84dd\u7eff\u53d1\u5e03", result.Items[0].Content)
}

// The other half of the gap: recall admits five situational items no matter
// how many matched, because it is paid for on every turn. A search is paid for
// only when the model asked for it, so it can afford to answer properly.
func TestSearchReachesPastTheFiveItemTurnBudget(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	for i := 0; i < 8; i++ {
		_, err := svc.Remember(ctx, types.MemoryItem{
			Kind:    types.MemoryKindFact,
			Topic:   fmt.Sprintf("\u914d\u7f6e\u9879-%d", i),
			Content: fmt.Sprintf("\u7f51\u5173\u914d\u7f6e\u7b2c %d \u9879\u5df2\u7ecf\u8c03\u8fc7", i),
		})
		require.NoError(t, err)
	}

	recall := svc.Recall(ctx, "\u7f51\u5173\u914d\u7f6e")
	require.Len(t, recall.Items, types.MemoryRecallMaxItems)

	result := svc.SearchMemory(ctx, "\u7f51\u5173\u914d\u7f6e", 8)
	require.True(t, result.Available)
	require.Len(t, result.Items, 8)
}

// Resident kinds are in the block rather than in situational recall, and the
// block has its own rune budget. Search covers them too, or the tool would be
// unable to answer "what do you know about my preferences" for anyone whose
// block is full.
func TestSearchCoversTheResidentKindsToo(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindPreference, Topic: "\u4ee3\u7801\u98ce\u683c",
		Content: "\u4ee3\u7801\u6ce8\u91ca\u7edf\u4e00\u7528\u82f1\u6587",
	})
	require.NoError(t, err)

	result := svc.SearchMemory(ctx, "\u4ee3\u7801\u6ce8\u91ca", 10)
	require.True(t, result.Available)
	require.Len(t, result.Items, 1)
	require.Equal(t, types.MemoryKindPreference, result.Items[0].Kind)
}

// A statement a later one replaced is exactly what the supersede machinery
// exists to keep out of an answer. Reaching it through search would undo that
// and hand the model a fact the user has already corrected.
func TestSearchDoesNotResurrectReplacedMemories(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u751f\u4ea7\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u7528\u7684\u662f MySQL",
	})
	require.NoError(t, err)
	_, err = svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u751f\u4ea7\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u5df2\u7ecf\u8fc1\u5230 PostgreSQL",
	})
	require.NoError(t, err)

	result := svc.SearchMemory(ctx, "\u751f\u4ea7\u6570\u636e\u5e93", 10)
	require.True(t, result.Available)
	require.Len(t, result.Items, 1)
	require.Contains(t, result.Items[0].Content, "PostgreSQL")
}

// "Switched off" and "nothing stored" have to stay distinguishable all the way
// out to the caller. Collapsing them would have the agent tell someone who
// disabled memory that it remembers nothing about them.
func TestSearchTellsDisabledApartFromEmpty(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u7528\u7684\u662f PostgreSQL",
	})
	require.NoError(t, err)

	empty := svc.SearchMemory(ctx, "\u5b8c\u5168\u65e0\u5173\u7684\u9898\u76ee", 10)
	require.True(t, empty.Available, "memory is on, this user simply has no match")
	require.Empty(t, empty.Items)

	disabled := false
	off := svc.SearchMemory(types.ApplyAgentMemoryPreference(ctx, &disabled), "\u6570\u636e\u5e93", 10)
	require.False(t, off.Available, "an agent opting out must not be able to search either")
	require.Empty(t, off.Items)
}

// MemoryAvailable is what lets a caller decide not to offer a memory feature
// at all. It has to track every switch the read path honours, or the agent
// would keep being handed a tool that can only report that memory is off.
func TestMemoryAvailableTracksAllThreeSwitches(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	require.True(t, svc.MemoryAvailable(ctx))

	// The user's own toggle in My memory.
	require.NoError(t, svc.SetEnabled(ctx, false))
	require.False(t, svc.MemoryAvailable(ctx), "the user opted out")
	require.NoError(t, svc.SetEnabled(ctx, true))
	require.True(t, svc.MemoryAvailable(ctx))

	// The agent handling this request.
	disabled := false
	require.False(t, svc.MemoryAvailable(types.ApplyAgentMemoryPreference(ctx, &disabled)))

	// The workspace setting.
	tenantRepo.set(1, &types.MemoryConfig{Enabled: false})
	require.False(t, svc.MemoryAvailable(ctx), "the workspace switched memory off")
}

// The predicate and the search must never disagree: anything that reports
// available has to be searchable, and anything unavailable has to say so
// rather than come back looking like an empty store.
func TestMemoryAvailableAgreesWithSearch(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u7528\u7684\u662f PostgreSQL",
	})
	require.NoError(t, err)

	require.Equal(t, svc.MemoryAvailable(ctx), svc.SearchMemory(ctx, "\u6570\u636e\u5e93", 10).Available)

	require.NoError(t, svc.SetEnabled(ctx, false))
	require.Equal(t, svc.MemoryAvailable(ctx), svc.SearchMemory(ctx, "\u6570\u636e\u5e93", 10).Available)
	require.False(t, svc.SearchMemory(ctx, "\u6570\u636e\u5e93", 10).Available)
}

func TestSearchWithoutPrincipalIsUnavailable(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	tenantRepo.set(1, &types.MemoryConfig{Enabled: true})
	ctx := t.Context()

	result := svc.SearchMemory(ctx, "\u6570\u636e\u5e93", 10)
	require.False(t, result.Available,
		"a request with no principal has no memory space to search")
}

func TestSearchClampsAnAbsurdLimit(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	for i := 0; i < types.MemorySearchMaxItems+10; i++ {
		_, err := svc.Remember(ctx, types.MemoryItem{
			Kind:    types.MemoryKindFact,
			Topic:   fmt.Sprintf("\u7f51\u5173-%d", i),
			Content: fmt.Sprintf("\u7f51\u5173\u914d\u7f6e\u7b2c %d \u9879\u5df2\u7ecf\u8c03\u8fc7", i),
		})
		require.NoError(t, err)
	}

	result := svc.SearchMemory(ctx, "\u7f51\u5173\u914d\u7f6e", 10_000)
	require.True(t, result.Available)
	require.LessOrEqual(t, len(result.Items), types.MemorySearchMaxItems)
}
