package memory

import (
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func item(kind, content string, importance int) *types.MemoryItem {
	return &types.MemoryItem{
		Kind:          kind,
		Content:       content,
		NormalizedKey: types.NormalizeMemoryKey("", content),
		Importance:    importance,
		ValidFrom:     time.Now(),
	}
}

func topicItem(kind, topic, content string, importance int) *types.MemoryItem {
	entry := item(kind, content, importance)
	entry.Topic = topic
	entry.NormalizedKey = types.NormalizeMemoryKey(topic, content)
	return entry
}

// TestTopicIsPartOfTheRetrievalHandle covers the common shape of an extracted
// memory: the question names the subject while the statement carries only the
// value, so matching on the statement alone would miss it.
func TestTopicIsPartOfTheRetrievalHandle(t *testing.T) {
	items := []*types.MemoryItem{
		topicItem(types.MemoryKindFact, "\u5728\u7528\u7684\u6570\u636e\u5e93",
			"\u5df2\u7ecf\u4ece MySQL \u8fc1\u5230 PostgreSQL", 3),
		topicItem(types.MemoryKindFact, "\u524d\u7aef\u6280\u672f\u6808", "\u7528\u7684\u662f Vue 3 \u52a0 Vite", 3),
	}
	selected := selectRecallItems("\u5199\u4e00\u6bb5\u8fde\u63a5\u6570\u636e\u5e93\u7684\u793a\u4f8b\u4ee3\u7801",
		items,
		types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget)
	require.NotEmpty(t, selected)
	require.Equal(t, "\u5df2\u7ecf\u4ece MySQL \u8fc1\u5230 PostgreSQL", selected[0].Content)
	require.NotContains(t, contents(selected), "\u7528\u7684\u662f Vue 3 \u52a0 Vite")
}

func contents(items []*types.MemoryItem) []string {
	out := make([]string, 0, len(items))
	for _, entry := range items {
		out = append(out, entry.Content)
	}
	return out
}

func TestSelectRecallItemsRanksChineseByTopic(t *testing.T) {
	items := []*types.MemoryItem{
		item(types.MemoryKindFact, "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17", 3),
		item(types.MemoryKindFact, "\u524d\u7aef\u6846\u67b6\u7528\u7684\u662f Vue 3", 3),
		item(types.MemoryKindTask, "\u5728\u91cd\u6784\u8ba2\u5355\u670d\u52a1\u7684\u652f\u4ed8\u6d41\u7a0b", 3),
	}
	selected := selectRecallItems("\u6570\u636e\u5e93\u6700\u8fd1\u8fde\u63a5\u8d85\u65f6\uff0c\u600e\u4e48\u6392"+
		"\u67e5", items,
		types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget)
	require.NotEmpty(t, selected)
	require.Equal(t, "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17", selected[0].Content)
	require.NotContains(t, contents(selected), "\u524d\u7aef\u6846\u67b6\u7528\u7684\u662f Vue 3")
}

func TestSelectRecallItemsMatchesEnglish(t *testing.T) {
	items := []*types.MemoryItem{
		item(types.MemoryKindFact, "The staging cluster runs in Frankfurt", 3),
		item(types.MemoryKindFact, "CI is GitHub Actions on self-hosted runners", 3),
	}
	selected := selectRecallItems("where is the staging cluster deployed", items,
		types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget)
	require.NotEmpty(t, selected)
	require.Equal(t, "The staging cluster runs in Frankfurt", selected[0].Content)
}

func TestSelectRecallItemsDropsWeakMatches(t *testing.T) {
	items := []*types.MemoryItem{
		item(types.MemoryKindFact, "\u6211\u5e73\u65f6\u7528 Go \u5199\u670d\u52a1", 3),
	}
	// The possessive particle and similar filler share nothing meaningful with the memory: a
	// weakly related memory must not be injected at all.
	require.Empty(t, selectRecallItems("\u4eca\u5929\u5929\u6c14\u600e\u4e48\u6837", items,
		types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget))
}

func TestSelectRecallItemsRespectsCountBudget(t *testing.T) {
	var items []*types.MemoryItem
	for i := 0; i < 20; i++ {
		items = append(items, item(types.MemoryKindFact,
			"\u6570\u636e\u5e93\u76f8\u5173\u7684\u4e8b\u5b9e "+string(rune('a'+i)), 3))
	}
	selected := selectRecallItems("\u6570\u636e\u5e93", items, types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget)
	require.LessOrEqual(t, len(selected), types.MemoryRecallMaxItems)
}

func TestSelectRecallItemsRespectsRuneBudget(t *testing.T) {
	var items []*types.MemoryItem
	for i := 0; i < 5; i++ {
		long := "\u6570\u636e\u5e93"
		for j := 0; j < 60; j++ {
			long += "\u5f88\u957f\u7684\u8bf4\u660e"
		}
		items = append(items, item(types.MemoryKindFact, long, 3))
	}
	selected := selectRecallItems("\u6570\u636e\u5e93", items, types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget)
	total := 0
	for _, entry := range selected {
		total += len([]rune(entry.Content))
	}
	require.LessOrEqual(t, total, types.MemoryRecallRuneBudget)
}

func TestSelectRecallItemsEmptyQuery(t *testing.T) {
	items := []*types.MemoryItem{item(types.MemoryKindFact, "\u4efb\u4f55\u4e8b\u5b9e", 3)}
	require.Empty(t, selectRecallItems("   ", items,
		types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget))
}

func TestBigramsBeatSingleCharacterNoise(t *testing.T) {
	// The character for "number" alone appears in the words for data, parameter
	// and quantity. A single ideograph must not be
	// enough to pull an unrelated memory into context.
	items := []*types.MemoryItem{
		item(types.MemoryKindFact, "\u53c2\u6570\u6821\u9a8c\u7edf\u4e00\u653e\u5728 handler \u5c42", 3),
		item(types.MemoryKindFact, "\u6570\u636e\u8fc1\u79fb\u811a\u672c\u653e\u5728 migrations \u76ee\u5f55", 3),
	}
	selected := selectRecallItems("\u6570\u636e\u8fc1\u79fb\u811a\u672c\u653e\u5728\u54ea", items,
		types.MemoryRecallMaxItems, types.MemoryRecallRuneBudget)
	require.NotEmpty(t, selected)
	require.Equal(t, "\u6570\u636e\u8fc1\u79fb\u811a\u672c\u653e\u5728 migrations \u76ee\u5f55", selected[0].Content)
}
