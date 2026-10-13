package chunker

import (
	"strconv"
	"strings"
	"testing"
)

// buildOCRStrayBracketDoc models the OCR failure mode behind this regression:
// a full-width "【...】" (official seal) whose opening bracket was mis-read as the half-width
// '[' (closing bracket lost), followed later by an unrelated half-width
// "](...)" fragment. CommonMark forbids a link's text/destination from
// spanning a blank line, but the previous unbounded
// `\[[^\]]*\]\([^)]+\)` joined the stray '[' to the distant '](' and
// swallowed the entire document as one protected atomic span.
func buildOCRStrayBracketDoc() string {
	var sb strings.Builder
	sb.WriteString("[\u516c\u7ae0\u3011\n\n")
	for i := 0; i < 60; i++ {
		sb.WriteString("\u8fd9\u662f\u7b2c")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\u6bb5\u7528\u4e8e\u6d4b\u8bd5\u5207\u5206\u7684\u4e2d\u6587\u5185\u5bb9\uff0c\u5305\u542b" +
			"\u5e38\u89c1\u6807\u70b9\u7b26\u53f7\u3002\n\n")
	}
	sb.WriteString("\u7b2c\u4e09\u6bb5\uff08\u5317\u4eac)\n\n")
	sb.WriteString("[\u5f15\u7528](\u6765\u6e90)")
	return sb.String()
}

func maxChunkRunes(chunks []Chunk) int {
	best := 0
	for _, c := range chunks {
		if n := len([]rune(c.Content)); n > best {
			best = n
		}
	}
	return best
}

// TestSplitText_OCRStrayBracketDoesNotSwallowDocument is the core regression:
// before the fix the malformed '[' turned the whole document into a single
// protected span (~1700 runes) and SplitText returned one oversized chunk.
func TestSplitText_OCRStrayBracketDoesNotSwallowDocument(t *testing.T) {
	doc := buildOCRStrayBracketDoc()
	if !strings.Contains(doc, "[\u516c\u7ae0\u3011") || !strings.Contains(doc, "](\u6765\u6e90)") {
		t.Fatalf("fixture missing malformed-bracket markers")
	}
	const chunkSize = 128
	cfg := SplitterConfig{ChunkSize: chunkSize, ChunkOverlap: 0, Separators: []string{"\n\n", "\n", "\u3002"}}

	chunks := SplitText(doc, cfg)
	t.Logf("doc=%d runes chunkSize=%d -> chunks=%d max=%d runes",
		len([]rune(doc)), chunkSize, len(chunks), maxChunkRunes(chunks))
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d (max=%d runes)", len(chunks), maxChunkRunes(chunks))
	}
	if got := maxChunkRunes(chunks); got > 2*chunkSize {
		t.Fatalf("max chunk %d runes exceeds 2*chunkSize=%d: stray '[' swallowed the document", got, 2*chunkSize)
	}
}

// TestSplit_OCRStrayBracketDoesNotSwallowDocument exercises the strategy-aware
// entry point with the legacy tier pinned, so the protected-pattern behaviour
// is what is under test rather than the profiler's tier selection.
func TestSplit_OCRStrayBracketDoesNotSwallowDocument(t *testing.T) {
	doc := buildOCRStrayBracketDoc()
	const chunkSize = 128
	cfg := SplitterConfig{
		ChunkSize:    chunkSize,
		ChunkOverlap: 0,
		Separators:   []string{"\n\n", "\n", "\u3002"},
		Strategy:     StrategyLegacy,
	}

	chunks := Split(doc, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d (max=%d runes)", len(chunks), maxChunkRunes(chunks))
	}
	if got := maxChunkRunes(chunks); got > 2*chunkSize {
		t.Fatalf("max chunk %d runes exceeds 2*chunkSize=%d: stray '[' swallowed the document", got, 2*chunkSize)
	}
}

// TestProtectedSpans_NoCrossLineLinkOrImage asserts no protected span may
// contain a newline for the malformed document: link text and destination
// cannot span a blank line (CommonMark).
func TestProtectedSpans_NoCrossLineLinkOrImage(t *testing.T) {
	doc := buildOCRStrayBracketDoc()
	for _, s := range protectedSpans(doc) {
		seg := doc[s.start:s.end]
		if strings.Contains(seg, "\n") {
			t.Fatalf("protected span spans a newline (%d runes): %q", len([]rune(seg)), seg)
		}
	}
}

// TestProtectedSpans_MarkdownLinksAndImagesStillProtected is the positive
// guard: the newline-bounded patterns must keep recognising well-formed
// single-line Markdown links and images.
func TestProtectedSpans_MarkdownLinksAndImagesStillProtected(t *testing.T) {
	cases := []string{
		"![alt](http://x/y.png)",
		"[\u6587\u5b57](http://x.com)",
		`[link](url "title")`,
		"![](resource://abc)",
	}
	for _, c := range cases {
		spans := protectedSpans(c)
		if len(spans) == 0 {
			t.Errorf("expected %q to be protected, got no span", c)
			continue
		}
		if spans[0].start != 0 || spans[0].end != len(c) {
			t.Errorf("%q: protected span = [%d,%d), want [0,%d)", c, spans[0].start, spans[0].end, len(c))
		}
	}
}

// TestProtectedSpans_LiteralCrossLineExample documents that an isolated
// malformed bracket pair with NO complete '](...)' construct is never treated
// as a protected link/image region.
func TestProtectedSpans_MultilineLinkIntentionallyUnprotected(t *testing.T) {
	// CommonMark allows a single soft line break in link text. The protected
	// patterns still refuse it so a stray OCR '[' cannot swallow a paragraph.
	doc := "[link\ntext](http://example.com)"
	if spans := protectedSpans(doc); len(spans) != 0 {
		t.Fatalf("expected multiline link to stay unprotected, got %v", spans)
	}
}

func TestProtectedSpans_LiteralCrossLineExample(t *testing.T) {
	doc := "[\u516c\u7ae0\u3011\n\n\u7b2c\u4e00\u6bb5\u3002\n\n\u7b2c\u4e8c\u6bb5\u3002\n\n\u7b2c\u4e09\u6bb5\uff08" +
		"\u5317\u4eac)"
	if spans := protectedSpans(doc); len(spans) != 0 {
		t.Fatalf("expected no protected span, got %v", spans)
	}
}
