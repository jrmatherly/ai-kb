package tools

import (
	"regexp"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func faqChunkForSnippetTest(t *testing.T) *types.Chunk {
	t.Helper()
	chunk := &types.Chunk{
		ChunkType: types.ChunkTypeFAQ,
		Content: "Q: \u79bb\u804c\u539f\u56e0\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49\nSimilar Questions:\n- " +
			"\u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49\n- \u8bbe\u7f6e\u79bb\u804c\u539f\u56e0",
	}
	meta := &types.FAQChunkMetadata{
		StandardQuestion: "\u79bb\u804c\u539f\u56e0\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49",
		SimilarQuestions: []string{"\u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49", "\u8bbe\u7f6e\u79bb\u804c\u539f" +
			"\u56e0"},
		Answers: []string{"\u53ef\u4ee5\uff0c\u5728\u7cfb\u7edf\u8bbe\u7f6e\u4e2d\u81ea\u5b9a\u4e49\u79bb" +
			"\u804c\u539f\u56e0\u3002"},
	}
	if err := chunk.SetFAQMetadata(meta); err != nil {
		t.Fatalf("SetFAQMetadata: %v", err)
	}
	return chunk
}

func TestFaqMatchSnippet_StandardQuestionAndMetadataAnswer(t *testing.T) {
	chunk := faqChunkForSnippetTest(t)
	re := regexp.MustCompile("(?i)\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49")
	snippet := faqMatchSnippet(chunk, []*regexp.Regexp{re})

	if !strings.Contains(snippet, "Q: \u79bb\u804c\u539f\u56e0\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49") {
		t.Fatalf("want standard question in snippet, got: %s", snippet)
	}
	if !strings.Contains(snippet, "A: \u53ef\u4ee5\uff0c\u5728\u7cfb\u7edf\u8bbe\u7f6e\u4e2d") {
		t.Fatalf("want metadata answer in snippet, got: %s", snippet)
	}
	for _, unwanted := range []string{"Similar Questions", "\u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49", "\u8bbe" +
		"\u7f6e\u79bb\u804c\u539f\u56e0"} {
		if strings.Contains(snippet, unwanted) {
			t.Errorf("snippet should not include similar-question noise %q: %s", unwanted, snippet)
		}
	}
}

func TestFaqMatchSnippetFromQueries_StandardQuestionAndMetadataAnswer(t *testing.T) {
	meta := &types.FAQChunkMetadata{
		StandardQuestion: "\u79bb\u804c\u539f\u56e0\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49",
		SimilarQuestions: []string{"\u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49", "\u8bbe\u7f6e\u79bb\u804c\u539f" +
			"\u56e0"},
		Answers: []string{"\u53ef\u4ee5\uff0c\u5728\u7cfb\u7edf\u8bbe\u7f6e\u4e2d\u81ea\u5b9a\u4e49\u79bb" +
			"\u804c\u539f\u56e0\u3002"},
	}
	snippet := faqMatchSnippetFromQueries(meta, []string{"\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49"})

	if !strings.Contains(snippet, "Q: \u79bb\u804c\u539f\u56e0\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49") {
		t.Fatalf("want standard question, got: %s", snippet)
	}
	if !strings.Contains(snippet, "A: \u53ef\u4ee5\uff0c\u5728\u7cfb\u7edf\u8bbe\u7f6e\u4e2d") {
		t.Fatalf("want metadata answer, got: %s", snippet)
	}
	if strings.Contains(snippet, "Similar Questions") {
		t.Fatalf("should not include similar-question list: %s", snippet)
	}
}

func TestFaqMatchSnippetFromQueries_SimilarQuestionHitShowsMatchedVariant(t *testing.T) {
	meta := &types.FAQChunkMetadata{
		StandardQuestion: "\u79bb\u804c\u539f\u56e0\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49",
		SimilarQuestions: []string{"\u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49", "\u8bbe\u7f6e\u79bb\u804c\u539f" +
			"\u56e0"},
		Answers: []string{"\u53ef\u4ee5\u81ea\u5b9a\u4e49\u3002"},
	}
	snippet := faqMatchSnippetFromQueries(meta, []string{"\u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49"})

	if !strings.Contains(snippet, "Q: \u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49") {
		t.Fatalf("want matched similar question, got: %s", snippet)
	}
}

func TestFaqMatchSnippet_SimilarQuestionHitShowsMatchedVariant(t *testing.T) {
	chunk := faqChunkForSnippetTest(t)
	re := regexp.MustCompile("(?i)\u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49")
	snippet := faqMatchSnippet(chunk, []*regexp.Regexp{re})

	if !strings.Contains(snippet, "Q: \u79bb\u804c\u539f\u56e0\u81ea\u5b9a\u4e49") {
		t.Fatalf("want matched similar question, got: %s", snippet)
	}
	if strings.Contains(snippet, "\u662f\u5426\u53ef\u4ee5\u81ea\u5b9a\u4e49") {
		t.Fatalf("should not show standard question when similar question matched: %s", snippet)
	}
}

func TestExtractChunkMatchSnippet_NonFAQUsesBodyContext(t *testing.T) {
	content := strings.Repeat("\u524d\u7f6e\u3002", 50) + "TARGET" + strings.Repeat("\u540e\u7eed\u3002", 50)
	chunk := &types.Chunk{ChunkType: types.ChunkTypeText, Content: content}
	re := regexp.MustCompile(`TARGET`)
	snippet := extractChunkMatchSnippet(chunk, []*regexp.Regexp{re})

	if !strings.Contains(snippet, "TARGET") {
		t.Fatalf("missing match: %s", snippet)
	}
	if !strings.Contains(snippet, "\u524d\u7f6e") || !strings.Contains(snippet, "\u540e\u7eed") {
		t.Fatalf("expected expanded body context: %s", snippet)
	}
}

// A Chinese question has no spaces; it is segmented into words so the
// snippet lands on the matching passage instead of the chunk's opening.
func TestExtractSnippetForChineseQuery(t *testing.T) {
	content := strings.Repeat("\u65e0\u5173\u7684\u5f00\u5934\u5185\u5bb9"+
		"\u3002", 60) +
		"\u9000\u6b3e\u7533\u8bf7\u9700\u8981\u5728\u6536\u8d27\u540e\u4e03\u5929\u5185\u63d0\u4ea4\u3002" +
		strings.Repeat("\u5176\u4ed6\u5185\u5bb9\u3002", 20)
	snippet := extractSnippetForQueries(content, []string{"\u9000\u6b3e\u7533\u8bf7\u7684\u65f6\u9650\u662f\u591a" +
		"\u4e45\uff1f"})
	if !strings.Contains(snippet, "\u9000\u6b3e\u7533\u8bf7") {
		t.Fatalf("snippet did not reach the matching passage: %q", snippet)
	}
}

func TestSearchQueryTokensSkipsStopwords(t *testing.T) {
	tokens := searchQueryTokens([]string{"How does the engine start?"})
	for _, tok := range tokens {
		if tok == "the" || tok == "does" || tok == "how" {
			t.Fatalf("stopword %q kept: %v", tok, tokens)
		}
	}
}
