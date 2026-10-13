package im

import (
	"strings"
	"testing"
)

func TestStripThinkBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "no think blocks", input: "Hello, world!", want: "Hello, world!"},
		{name: "empty", input: "", want: ""},
		{
			name:  "single block before answer",
			input: "<think>reasoning</think>The answer is 42.",
			want:  "The answer is 42.",
		},
		{
			name: "multiline think with tools",
			input: "<think>\nLet me search the knowledge base first\nCalling Text Pattern Search...\nKeyword search: " +
				"\"civilization\"\n</think>\n\nCivilization VI is a strategy game.",
			want: "Civilization VI is a strategy game.",
		},
		{
			name:  "multiple blocks",
			input: "<think>first</think>Part 1. <think>second</think>Part 2.",
			want:  "Part 1. Part 2.",
		},
		{name: "only think block", input: "<think>just thinking</think>", want: ""},
		{
			name:  "unclosed think block",
			input: "<think>still streaming",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripThinkBlocks(tt.input)
			if got != tt.want {
				t.Fatalf("StripThinkBlocks() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatIMDisplayContent_intermediate_showsThinkingStyled(t *testing.T) {
	raw := "<think>\nAnalyzing the user's question\nCalling Knowledge Search...\n</think>\n\n"
	got := FormatIMDisplayContent(raw, StreamDisplayIntermediate)

	if !strings.Contains(got, "Thought process") {
		t.Fatalf("intermediate display should include thinking header, got: %q", got)
	}
	if !strings.Contains(got, "Analyzing the user's question") {
		t.Fatalf("intermediate display should include thinking body, got: %q", got)
	}
	if !strings.Contains(got, "Knowledge Search") {
		t.Fatalf("intermediate display should include tool progress, got: %q", got)
	}
	if strings.Contains(got, "<think>") {
		t.Fatalf("intermediate display must not leak raw think tags, got: %q", got)
	}
}

func TestFormatIMDisplayContent_intermediate_inProgressThink(t *testing.T) {
	raw := "<think>\nReasoning\nCalling Text Pattern Search...\n"
	got := FormatIMDisplayContent(raw, StreamDisplayIntermediate)

	if !strings.Contains(got, "Thought process") {
		t.Fatalf("open think block should show thinking header, got: %q", got)
	}
	if strings.Contains(got, "<think>") {
		t.Fatalf("must not leak raw tags, got: %q", got)
	}
}

func TestFormatIMDisplayContent_intermediate_showsAnswerPreview(t *testing.T) {
	raw := "<think>\nSearching\n</think>\n\nCivilization VI is a turn-based strategy game."
	got := FormatIMDisplayContent(raw, StreamDisplayIntermediate)

	if !strings.Contains(got, "Civilization VI is a turn-based strategy game") {
		t.Fatalf("intermediate display should preview answer after think block, got: %q", got)
	}
}

func TestFormatIMDisplayContent_final_stripsThinkingAndTools(t *testing.T) {
	raw := "<think>\nLet me search the knowledge base first\nCalling Text Pattern Search...\nKeyword search: " +
		"\"civilization\"\n</think>\n\nCivilization VI is a strategy game."
	got := FormatIMDisplayContent(raw, StreamDisplayFinal)

	want := "Civilization VI is a strategy game."
	if got != want {
		t.Fatalf("final display = %q, want %q", got, want)
	}
}

func TestFormatIMDisplayContent_final_plainAnswerUnchanged(t *testing.T) {
	raw := "This is the final answer."
	got := FormatIMDisplayContent(raw, StreamDisplayFinal)
	if got != raw {
		t.Fatalf("final display = %q, want %q", got, raw)
	}
}

func TestFormatIMDisplayContent_final_ragPipelineHidden(t *testing.T) {
	raw := "<think>\nUnderstanding query...\nQuery understood\nSearching knowledge base...\nSearched knowledge base: " +
		"\"query\" · Found 3 result(s)\n</think>\n\nAccording to the knowledge base, the answer is A."
	got := FormatIMDisplayContent(raw, StreamDisplayFinal)

	if strings.Contains(got, "Query understood") || strings.Contains(got, "Searched knowledge base") {
		t.Fatalf("final display must not contain RAG pipeline steps, got: %q", got)
	}
	if got != "According to the knowledge base, the answer is A." {
		t.Fatalf("final display = %q", got)
	}
}

func TestFormatIMAgentIntermediate_answerFirstBeforeTools(t *testing.T) {
	parts := IMStreamParts{
		Mode:       IMStreamModeAgent,
		LiveAnswer: "OK, let me search the knowledge base first.",
	}
	got := FormatIMIntermediateFromParts(parts, true)
	if got != "OK, let me search the knowledge base first." {
		t.Fatalf("should stream as plain answer, got: %q", got)
	}
	if strings.Contains(got, "Thought process") {
		t.Fatal("think header must not appear while answer is live")
	}
}

func TestFormatIMAgentIntermediate_retractIntoThinkOnTools(t *testing.T) {
	parts := IMStreamParts{
		Mode:       IMStreamModeAgent,
		AgentInner: "OK, let me search the knowledge base first.\n",
		AgentToolSteps: []IMToolStep{
			{ToolName: "grep_chunks", Pending: true},
			{ToolName: "knowledge_search", Success: true, Arguments: map[string]any{"query": "Civilization VI"}},
		},
	}
	got := FormatIMIntermediateFromParts(parts, true)
	if !strings.Contains(got, "Thought process") {
		t.Fatalf("after tool retract should show think block, got: %q", got)
	}
	if !strings.Contains(got, "OK, let me search the knowledge base first") {
		t.Fatalf("retracted preamble should be inside think, got: %q", got)
	}
	if !strings.Contains(got, "Text Pattern Search") {
		t.Fatalf("tool lines should be inside think, got: %q", got)
	}
	if !strings.Contains(got, "Civilization VI") {
		t.Fatalf("tool query should be inside think, got: %q", got)
	}
}

func TestFormatIMAgentIntermediate_newAnswerAfterTools(t *testing.T) {
	parts := IMStreamParts{
		Mode:       IMStreamModeAgent,
		AgentInner: "OK, let me search\n",
		AgentToolSteps: []IMToolStep{
			{ToolName: "knowledge_search", Success: true, Arguments: map[string]any{"query": "Civilization VI"}},
		},
		LiveAnswer: "According to the search results, Civilization VI is…",
	}
	got := FormatIMIntermediateFromParts(parts, true)
	if !strings.Contains(got, "According to the search results, Civilization VI is…") {
		t.Fatalf("should still stream live answer, got: %q", got)
	}
	if !strings.Contains(got, "Thought process") {
		t.Fatalf("think block should stay visible above answer, got: %q", got)
	}
	if !strings.Contains(got, "Civilization VI") {
		t.Fatalf("tool query should remain in think block, got: %q", got)
	}
}

func TestBuildIMStreamRaw_agentInProgress_mergesToolsAndNarrativeIntoThink(t *testing.T) {
	parts := IMStreamParts{
		Mode:       IMStreamModeAgent,
		AgentInner: "The user asks about Civilization VI again\n",
		AgentToolSteps: []IMToolStep{
			{ToolName: "grep_chunks", Pending: true},
			{ToolName: "knowledge_search", Success: true},
		},
	}
	got := FormatIMIntermediateFromParts(parts, true)

	if !strings.Contains(got, "Text Pattern Search") {
		t.Fatalf("tool progress should be inside think block, got: %q", got)
	}
	if !strings.Contains(got, "Thought process") {
		t.Fatalf("agent tooling phase should show Thought process, got: %q", got)
	}
}

func TestFormatIMQuickQA_separatesPipelineAndThinking(t *testing.T) {
	parts := IMStreamParts{
		Mode: IMStreamModeQuickQA,
		PipelineToolSteps: []IMToolStep{
			{ToolName: "query_understand", Pending: true},
			{ToolName: "knowledge_search", Success: true, Arguments: map[string]any{"query": "Civilization VI"}},
		},
		ReasoningInner: "Analyzing the question intent…",
	}
	got := FormatIMIntermediateFromParts(parts, false)

	if strings.Contains(got, "Thought process") {
		t.Fatalf("quick QA should not use agent Thought process header, got: %q", got)
	}
	if !strings.Contains(got, "> 💭 **Thinking**") {
		t.Fatalf("quick QA reasoning should use separate Thinking section, got: %q", got)
	}
	if !strings.Contains(got, "Analyzing the question intent") {
		t.Fatalf("reasoning body missing, got: %q", got)
	}
	if !strings.Contains(got, "Understanding query") {
		t.Fatalf("pipeline steps missing, got: %q", got)
	}
	if !strings.Contains(got, "Civilization VI") {
		t.Fatalf("pipeline query missing, got: %q", got)
	}
}

func TestFormatIMQuickQA_collapsesToAnswerWhenStreaming(t *testing.T) {
	parts := IMStreamParts{
		Mode: IMStreamModeQuickQA,
		PipelineToolSteps: []IMToolStep{
			{ToolName: "query_understand", Success: true},
			{ToolName: "knowledge_search", Success: true},
		},
		Answer: "Civilization VI is a turn-based strategy game.",
	}
	got := FormatIMIntermediateFromParts(parts, false)
	if got != "Civilization VI is a turn-based strategy game." {
		t.Fatalf("quick QA should collapse to answer preview, got: %q", got)
	}
}

func TestFormatIMFinalFromParts_agentAnswerOnly(t *testing.T) {
	parts := IMStreamParts{
		Mode:       IMStreamModeAgent,
		AgentInner: "OK, let me search\n",
		AgentToolSteps: []IMToolStep{
			{ToolName: "grep_chunks", Pending: true},
			{ToolName: "knowledge_search", Success: true},
		},
		LiveAnswer: "should not appear in the final message",
		Answer:     "Civilization VI is a strategy game.",
	}
	got := FormatIMFinalFromParts(parts)
	if got != "Civilization VI is a strategy game." {
		t.Fatalf("final should be answer-only, got: %q", got)
	}
	if strings.Contains(got, "Thought process") {
		t.Fatalf("final must not include collapsed think header, got: %q", got)
	}
}

func TestFormatIMFinalFromParts_usesAnswerOnly(t *testing.T) {
	parts := IMStreamParts{
		Mode:              IMStreamModeQuickQA,
		PipelineToolSteps: []IMToolStep{{ToolName: "query_understand", Success: true}},
		ReasoningInner:    "Reasoning in progress",
		AgentToolSteps:    []IMToolStep{{ToolName: "grep_chunks", Pending: true}},
		Answer:            "Civilization VI is a strategy game.",
	}
	got := FormatIMFinalFromParts(parts)
	if got != "Civilization VI is a strategy game." {
		t.Fatalf("final display = %q", got)
	}
}

func TestIsRAGPipelineToolName_matchesWeb(t *testing.T) {
	for _, name := range []string{"query_understand", "knowledge_search"} {
		if !IsRAGPipelineToolName(name) {
			t.Fatalf("%q should be a RAG pipeline tool", name)
		}
	}
	if IsRAGPipelineToolName("grep_chunks") {
		t.Fatal("grep_chunks is agent tool, not RAG pipeline progress tool")
	}
}
