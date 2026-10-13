package reranking

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestGetEnrichedPassageKeepsQuestionsFromEarlierRevision(t *testing.T) {
	metadata, err := json.Marshal(types.DocumentChunkMetadata{
		GeneratedQuestionsRevision: 1,
		GeneratedQuestions: []types.GeneratedQuestion{{
			ID: "old", Question: "question generated before the edit",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	passage := EnrichedPassage(context.Background(), &types.SearchResult{
		Content:       "edited chunk body",
		ChunkMetadata: types.JSON(metadata),
	})
	if !strings.Contains(passage, "question generated before the edit") {
		t.Fatalf("earlier generated question was excluded from rerank passage: %q", passage)
	}
}

func TestGetEnrichedPassageKeepsCodeAndMathCandidates(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "code only",
			content: "```go\nfunc answer() int { return 42 }\n```",
			want:    "func answer() int { return 42 }",
		},
		{
			name:    "math only",
			content: "$$\nE = mc^2\n$$",
			want:    "E = mc^2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passage := EnrichedPassage(context.Background(), &types.SearchResult{Content: tt.content})
			if strings.TrimSpace(passage) == "" {
				t.Fatal("semantic-only candidate was removed from the rerank passage")
			}
			if !strings.Contains(passage, tt.want) {
				t.Fatalf("rerank passage %q does not preserve %q", passage, tt.want)
			}
		})
	}
}

func TestCleanPassageForRerank(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "plain text unchanged",
			input:  "\u8fd9\u662f\u4e00\u6bb5\u666e\u901a\u7684\u6587\u672c\u5185\u5bb9",
			expect: "\u8fd9\u662f\u4e00\u6bb5\u666e\u901a\u7684\u6587\u672c\u5185\u5bb9",
		},
		{
			name:   "remove markdown images",
			input:  "\u524d\u6587 ![\u56fe\u7247\u8bf4\u660e](https://example.com/img.png) \u540e\u6587",
			expect: "\u524d\u6587  \u540e\u6587",
		},
		{
			name:   "convert markdown links to text",
			input:  "\u8bf7\u53c2\u8003 [\u5b98\u65b9\u6587\u6863](https://docs.example.com) \u4e86\u89e3\u8be6\u60c5",
			expect: "\u8bf7\u53c2\u8003 \u5b98\u65b9\u6587\u6863 \u4e86\u89e3\u8be6\u60c5",
		},
		{
			name:   "remove standalone URLs",
			input:  "\u8bbf\u95ee https://example.com/path?q=1&b=2 \u83b7\u53d6\u66f4\u591a\u4fe1\u606f",
			expect: "\u8bbf\u95ee  \u83b7\u53d6\u66f4\u591a\u4fe1\u606f",
		},
		{
			name:   "unwrap code blocks",
			input:  "\u793a\u4f8b\u4ee3\u7801\uff1a\n```python\nprint('hello')\n```\n\u4ee5\u4e0a\u662f\u793a\u4f8b",
			expect: "\u793a\u4f8b\u4ee3\u7801\uff1a\nprint('hello')\n\u4ee5\u4e0a\u662f\u793a\u4f8b",
		},
		{
			name:   "unwrap LaTeX blocks",
			input:  "\u516c\u5f0f\u5982\u4e0b $$E=mc^2$$ \u5176\u4e2dE\u662f\u80fd\u91cf",
			expect: "\u516c\u5f0f\u5982\u4e0b E=mc^2 \u5176\u4e2dE\u662f\u80fd\u91cf",
		},
		{
			name:   "remove table separator rows and convert data rows",
			input:  "| \u540d\u79f0 | \u503c |\n| --- | --- |\n| A | 1 |",
			expect: "\u540d\u79f0, \u503c\n\nA, 1",
		},
		{
			name:   "strip heading markers",
			input:  "## \u7b2c\u4e8c\u7ae0 \u6982\u8ff0\n### 2.1 \u80cc\u666f",
			expect: "\u7b2c\u4e8c\u7ae0 \u6982\u8ff0\n2.1 \u80cc\u666f",
		},
		{
			name:   "strip blockquote markers",
			input:  "> \u8fd9\u662f\u4e00\u6bb5\u5f15\u7528\n> \u7b2c\u4e8c\u884c\u5f15\u7528",
			expect: "\u8fd9\u662f\u4e00\u6bb5\u5f15\u7528\n\u7b2c\u4e8c\u884c\u5f15\u7528",
		},
		{
			name:   "unwrap bold and italic",
			input:  "\u8fd9\u662f **\u52a0\u7c97** \u548c *\u659c\u4f53* \u4ee5\u53ca ***\u7c97\u659c\u4f53*** \u6587\u672c",
			expect: "\u8fd9\u662f \u52a0\u7c97 \u548c \u659c\u4f53 \u4ee5\u53ca \u7c97\u659c\u4f53 \u6587\u672c",
		},
		{
			name:   "strip list markers",
			input:  "- \u9879\u76ee\u4e00\n- \u9879\u76ee\u4e8c\n1. \u6709\u5e8f\u4e00\n2. \u6709\u5e8f\u4e8c",
			expect: "\u9879\u76ee\u4e00\n\u9879\u76ee\u4e8c\n\u6709\u5e8f\u4e00\n\u6709\u5e8f\u4e8c",
		},
		{
			name:   "remove HTML tags",
			input:  "\u6587\u672c<br>\u6362\u884c<div class=\"test\">\u5185\u5bb9</div>\u7ed3\u5c3e",
			expect: "\u6587\u672c\u6362\u884c\u5185\u5bb9\u7ed3\u5c3e",
		},
		{
			name:   "collapse excessive newlines",
			input:  "\u6bb5\u843d\u4e00\n\n\n\n\n\u6bb5\u843d\u4e8c",
			expect: "\u6bb5\u843d\u4e00\n\n\u6bb5\u843d\u4e8c",
		},
		{
			name:   "combined real-world passage",
			input:  "## \u4ea7\u54c1\u4ecb\u7ecd\n\n\u8fd9\u662f\u4e00\u4e2a **\u91cd\u8981\u7684** \u4ea7\u54c1\u3002\u8be6\u89c1 [\u4ea7\u54c1\u9875\u9762](https://example.com/product)\u3002\n\n![\u4ea7\u54c1\u622a\u56fe](images/product.png)\n\n> \u7528\u6237\u8bc4\u4ef7\uff1a\u975e\u5e38\u597d\u7528\n\n- \u529f\u80fd\u4e00\n- \u529f\u80fd\u4e8c\n\n" + "```json\n{\"key\": \"value\"}\n```",
			expect: "\u4ea7\u54c1\u4ecb\u7ecd\n\n\u8fd9\u662f\u4e00\u4e2a \u91cd\u8981\u7684 \u4ea7\u54c1\u3002\u8be6\u89c1 \u4ea7\u54c1\u9875\u9762\u3002\n\n\u7528\u6237\u8bc4\u4ef7\uff1a\u975e\u5e38\u597d\u7528\n\n\u529f\u80fd\u4e00\n\u529f\u80fd\u4e8c\n\n{\"key\": \"value\"}",
		},
		{
			name:   "convert table data rows to plain text",
			input:  "| col1 | col2 | col3 |",
			expect: "col1, col2, col3",
		},
		{
			name:   "multi-row table fully converted",
			input:  "| Header1 | Header2 |\n| --- | --- |\n| data1 | data2 |\n| data3 | data4 |",
			expect: "Header1, Header2\n\ndata1, data2\ndata3, data4",
		},
		{
			name:   "table-only passage becomes empty after separator removal",
			input:  "| --- | --- |",
			expect: "",
		},
		{
			name:   "whitespace-only after cleaning",
			input:  "   \n\n   ",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanPassage(tt.input)
			if got != tt.expect {
				t.Errorf("CleanPassage():\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestModelPassage_includesHeadingBreadcrumb(t *testing.T) {
	t.Parallel()
	got := ModelPassage(context.Background(), &types.SearchResult{
		Content: "7 \u5929\u5185\u53ef\u7533\u8bf7", KnowledgeTitle: "\u552e\u540e\u624b\u518c", ContextHeader: "\u9000\u6b3e\u653f\u7b56 > \u7533\u8bf7\u6761\u4ef6",
		ChunkType: string(types.ChunkTypeText),
	})
	if got != "\u552e\u540e\u624b\u518c\n\n\u9000\u6b3e\u653f\u7b56 > \u7533\u8bf7\u6761\u4ef6\n\n7 \u5929\u5185\u53ef\u7533\u8bf7" {
		t.Fatalf("passage = %q", got)
	}
}
