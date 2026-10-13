package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestWriteFAQMetadataXML_IncludesAnswers(t *testing.T) {
	var b strings.Builder
	meta := &types.FAQChunkMetadata{
		StandardQuestion: "\u5982\u4f55\u521b\u5efa\u77e5\u8bc6\u5e93\uff1f",
		SimilarQuestions: []string{"\u600e\u4e48\u521b\u5efa\u77e5\u8bc6\u5e93\uff1f"},
		Answers:          []string{"\u5728\u63a7\u5236\u53f0\u70b9\u51fb\u65b0\u5efa\u77e5\u8bc6\u5e93\u3002"},
	}
	writeFAQMetadataXML(&b, meta)
	out := b.String()

	for _, want := range []string{
		"<faq>", "<question>\u5982\u4f55\u521b\u5efa\u77e5\u8bc6\u5e93\uff1f</question>",
		"<similar_question>\u600e\u4e48\u521b\u5efa\u77e5\u8bc6\u5e93\uff1f</similar_question>",
		"<answer>\u5728\u63a7\u5236\u53f0\u70b9\u51fb\u65b0\u5efa\u77e5\u8bc6\u5e93\u3002</answer>", "</faq>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestWriteSimilarQuestionsXML_TruncatesWhenTooMany(t *testing.T) {
	var b strings.Builder
	questions := make([]string, 8)
	for i := range questions {
		questions[i] = fmt.Sprintf("\u76f8\u4f3c\u95ee%d", i+1)
	}
	writeSimilarQuestionsXML(&b, questions)
	out := b.String()

	if strings.Count(out, "<similar_question>") != faqMaxSimilarQuestionsDisplay {
		t.Fatalf("want %d similar_question tags, got:\n%s", faqMaxSimilarQuestionsDisplay, out)
	}
	if !strings.Contains(out, `<similar_questions_omitted count="3"`) {
		t.Fatalf("want omitted marker, got:\n%s", out)
	}
}

func TestWriteFAQEntryXML_UsesFaqNotChunk(t *testing.T) {
	chunk := &types.Chunk{
		ID:          "faq-chunk-1",
		ChunkType:   types.ChunkTypeFAQ,
		ChunkIndex:  0,
		KnowledgeID: "kb-doc-1",
		Content:     "Q: test\nSimilar Questions:\n- alt",
	}
	meta := &types.FAQChunkMetadata{
		StandardQuestion: "\u5982\u4f55\u521b\u5efa\u77e5\u8bc6\u5e93\uff1f",
		Answers:          []string{"\u70b9\u65b0\u5efa\u5373\u53ef\u3002"},
	}
	if err := chunk.SetFAQMetadata(meta); err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	writeFAQEntryXML(&b, chunk)
	out := b.String()

	if strings.Contains(out, "<chunk") {
		t.Fatalf("FAQ list output must not use <chunk>, got:\n%s", out)
	}
	for _, want := range []string{
		`<faq faq_id="faq-chunk-1"`,
		"<question>\u5982\u4f55\u521b\u5efa\u77e5\u8bc6\u5e93\uff1f</question>",
		"<answer>\u70b9\u65b0\u5efa\u5373\u53ef\u3002</answer>",
		"</faq>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
