package im

import (
	"strings"
	"testing"
)

func TestFormatIMToolLine_pendingWithQuery(t *testing.T) {
	line := FormatIMToolLine(IMToolStep{
		ToolName:  "knowledge_search",
		Pending:   true,
		Arguments: map[string]any{"query": "Civilization VI"},
	})
	if line != "Calling Knowledge Search..." {
		t.Fatalf("pending line = %q", line)
	}
}

func TestFormatIMToolLine_searchDoneWithQueryAndSummary(t *testing.T) {
	line := FormatIMToolLine(IMToolStep{
		ToolName: "knowledge_search",
		Success:  true,
		Arguments: map[string]any{
			"query": "Civilization VI",
		},
		Data: map[string]interface{}{
			"results":   []interface{}{map[string]interface{}{}, map[string]interface{}{}, map[string]interface{}{}},
			"kb_counts": map[string]interface{}{"a": 1, "b": 1},
		},
	})
	if !strings.Contains(line, "Searched knowledge base: \"Civilization VI\"") {
		t.Fatalf("title missing query: %q", line)
	}
	if !strings.Contains(line, "Found 3 result(s) from 2 file(s)") {
		t.Fatalf("summary missing: %q", line)
	}
}

func TestFormatIMToolLine_grepPatterns(t *testing.T) {
	line := FormatIMToolLine(IMToolStep{
		ToolName: "grep_chunks",
		Success:  true,
		Arguments: map[string]any{
			"patterns": []any{"civilization", "strategy"},
		},
		Data: map[string]interface{}{
			"total_matches":  float64(5),
			"document_count": float64(2),
		},
	})
	if line != "Keyword search: \"civilization, strategy\" · Found 5 matching chunk(s) across 2 document(s)" {
		t.Fatalf("grep line = %q", line)
	}
}

func TestFormatIMRagPipelineLine_queryUnderstand(t *testing.T) {
	pending := FormatIMRagPipelineLine(IMToolStep{
		ToolName: "query_understand",
		Pending:  true,
	})
	if pending != "Understanding query..." {
		t.Fatalf("pending = %q", pending)
	}
	done := FormatIMRagPipelineLine(IMToolStep{
		ToolName: "query_understand",
		Success:  true,
	})
	if done != "Query understood" {
		t.Fatalf("done = %q", done)
	}
}

func TestFormatIMRagPipelineLine_searchWithQuery(t *testing.T) {
	line := FormatIMRagPipelineLine(IMToolStep{
		ToolName:  "knowledge_search",
		Pending:   true,
		Arguments: map[string]any{"query": "iFLYTEK Open Platform"},
	})
	if line != "Searching knowledge base: \"iFLYTEK Open Platform\"" {
		t.Fatalf("line = %q", line)
	}
}

func TestFormatIMRagPipelineLine_webSearchWithQuery(t *testing.T) {
	line := FormatIMRagPipelineLine(IMToolStep{
		ToolName:  "knowledge_search",
		Pending:   true,
		Arguments: map[string]any{"query": "Ren Suxi concert", "search_source": "web"},
	})
	if line != "Searching the web: \"Ren Suxi concert\"" {
		t.Fatalf("line = %q", line)
	}
}

func TestIMGetQueryText_joinsUniqueQueries(t *testing.T) {
	got := imGetQueryText(map[string]any{
		"query":   "foo",
		"queries": []any{"foo", "bar"},
	})
	if got != "foo, bar" {
		t.Fatalf("query text = %q", got)
	}
}

func TestFormatIMToolLine_writeSandboxPendingShowsDiffStat(t *testing.T) {
	line := FormatIMToolLine(IMToolStep{
		ToolName: "write_sandbox_file",
		Pending:  true,
		Arguments: map[string]any{
			"path":          "/workspace/output/a.py",
			"added_lines":   12,
			"removed_lines": 0,
		},
	})
	if line != "Write sandbox file: \"/workspace/output/a.py\"... +12" {
		t.Fatalf("pending write line = %q", line)
	}
}
