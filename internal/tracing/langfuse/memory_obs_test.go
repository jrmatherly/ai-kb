package langfuse

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestSummarizeMemoryRecallOutput_includesMetaAndItems(t *testing.T) {
	out := SummarizeMemoryRecallOutput(map[string]interface{}{
		"matched": 1,
		"mode":    "lexical_only",
	}, []*types.MemoryItem{
		{ID: "m1", Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93", Content: "\u751f\u4ea7\u7528 PostgreSQL " +
			"17"},
	})

	if out["matched"] != 1 || out["mode"] != "lexical_only" {
		t.Fatalf("meta not preserved: %#v", out)
	}
	items := out["recalled_items"].([]map[string]interface{})
	if len(items) != 1 || items[0]["id"] != "m1" {
		t.Fatalf("unexpected recalled_items: %#v", items)
	}
}
