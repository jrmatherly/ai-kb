package chunker

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitText_BasicASCII(t *testing.T) {
	text := "Hello world. This is a test."
	cfg := SplitterConfig{ChunkSize: 100, ChunkOverlap: 0, Separators: []string{". "}}
	chunks := SplitText(text, cfg)
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	combined := ""
	for _, c := range chunks {
		combined += c.Content
	}
	if combined != text {
		t.Errorf("combined content mismatch:\n  got:  %q\n  want: %q", combined, text)
	}
}

func TestSplitText_ChineseText_StartEndAreRuneOffsets(t *testing.T) {
	// Each Chinese character is 3 bytes in UTF-8 but 1 rune.
	// This test ensures Start/End are rune offsets, not byte offsets.
	text := "\u4f60\u597d\u4e16\u754c\u8fd9\u662f\u4e00\u4e2a\u6d4b\u8bd5\u6587\u672c\u7528\u4e8e\u68c0\u9a8c\u5206" +
		"\u5272\u4f4d\u7f6e"
	runeCount := utf8.RuneCountInString(text)
	byteCount := len(text)
	if runeCount == byteCount {
		t.Fatal("test requires multi-byte characters")
	}

	cfg := SplitterConfig{ChunkSize: 100, ChunkOverlap: 0, Separators: []string{"\n"}}
	chunks := SplitText(text, cfg)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	c := chunks[0]
	if c.Start != 0 {
		t.Errorf("Start: got %d, want 0", c.Start)
	}
	if c.End != runeCount {
		t.Errorf("End: got %d, want %d (runeCount); byteCount would be %d",
			c.End, runeCount, byteCount)
	}
}

func TestSplitText_ChineseMultiChunk_StartEndConsistency(t *testing.T) {
	// Build a long Chinese text that will be split into multiple chunks.
	line := "\u8fd9\u662f\u4e00\u6bb5\u4e2d\u6587\u5185\u5bb9\u7528\u4e8e\u6d4b\u8bd5\u5206\u5272\u529f\u80fd\u662f" +
		"\u5426\u6b63\u786e\u3002"
	text := strings.Repeat(line+"\n\n", 20)
	text = strings.TrimRight(text, "\n")

	cfg := SplitterConfig{ChunkSize: 30, ChunkOverlap: 5, Separators: []string{"\n\n", "\n", "\u3002"}}
	chunks := SplitText(text, cfg)

	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	textRunes := []rune(text)
	for i, c := range chunks {
		contentRunes := []rune(c.Content)
		contentRuneLen := len(contentRunes)

		// End - Start must equal the rune length of the content
		spanLen := c.End - c.Start
		if spanLen != contentRuneLen {
			t.Errorf("chunk[%d]: End(%d) - Start(%d) = %d, but rune len of content = %d",
				i, c.End, c.Start, spanLen, contentRuneLen)
		}

		// Start must be non-negative and End must not exceed total rune count
		if c.Start < 0 {
			t.Errorf("chunk[%d]: Start is negative: %d", i, c.Start)
		}
		if c.End > len(textRunes) {
			t.Errorf("chunk[%d]: End %d exceeds total rune count %d", i, c.End, len(textRunes))
		}

		// Content from rune slice must match the chunk content
		if c.Start >= 0 && c.End <= len(textRunes) {
			sliced := string(textRunes[c.Start:c.End])
			if sliced != c.Content {
				t.Errorf("chunk[%d]: content mismatch via rune slice:\n  got:  %q\n  want: %q",
					i, sliced, c.Content)
			}
		}
	}
}

func TestSplitText_MixedChineseAndASCII(t *testing.T) {
	text := "Hello\u4f60\u597dWorld\u4e16\u754cTest\u6d4b\u8bd5"
	cfg := SplitterConfig{ChunkSize: 100, ChunkOverlap: 0, Separators: []string{"\n"}}
	chunks := SplitText(text, cfg)

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	expectedRuneLen := utf8.RuneCountInString(text)
	if c.End-c.Start != expectedRuneLen {
		t.Errorf("End(%d) - Start(%d) = %d, want rune len %d (byte len would be %d)",
			c.End, c.Start, c.End-c.Start, expectedRuneLen, len(text))
	}
}

func TestSplitText_ProtectedPattern_ChineseContext(t *testing.T) {
	// Test protected markdown images in Chinese context.
	text := "\u8fd9\u662f\u524d\u9762\u7684\u4e2d\u6587\u5185\u5bb9\u3002![\u56fe\u7247\u63cf\u8ff0](http://example.c" +
		"om/img.png)\u8fd9\u662f\u540e\u9762\u7684\u4e2d\u6587\u5185\u5bb9\u3002"
	cfg := SplitterConfig{ChunkSize: 200, ChunkOverlap: 0, Separators: []string{"\u3002"}}
	chunks := SplitText(text, cfg)

	textRunes := []rune(text)
	for i, c := range chunks {
		if c.Start < 0 || c.End > len(textRunes) {
			t.Errorf("chunk[%d]: out of rune range [%d, %d), total runes %d",
				i, c.Start, c.End, len(textRunes))
			continue
		}
		sliced := string(textRunes[c.Start:c.End])
		if sliced != c.Content {
			t.Errorf("chunk[%d]: rune-slice mismatch:\n  sliced: %q\n  content: %q",
				i, sliced, c.Content)
		}
	}
}

func TestSplitText_SimulateMergeSlicing(t *testing.T) {
	// Simulate what merge.go:104-106 does to ensure it won't panic.
	// This is the exact pattern that caused the production crash.
	line := "\u8fd9\u662f\u7b2c\u4e00\u6bb5\u5185\u5bb9\u7528\u4e8e\u6a21\u62df\u77e5\u8bc6\u5e93\u95ee\u7b54\u7684" +
		"\u6587\u672c"
	text := line + "\n\n" + line + "\n\n" + line

	cfg := SplitterConfig{ChunkSize: 25, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)
	if len(chunks) < 2 {
		t.Fatalf("need at least 2 chunks for overlap test, got %d", len(chunks))
	}

	for i := 1; i < len(chunks); i++ {
		prev := chunks[i-1]
		curr := chunks[i]

		if curr.Start > prev.End {
			continue // non-overlapping, no merge needed
		}

		// This is the exact merge.go logic:
		contentRunes := []rune(curr.Content)
		offset := len(contentRunes) - (curr.End - prev.End)

		if offset < 0 {
			t.Fatalf("chunk[%d] merge panic: offset=%d < 0 (contentRunes=%d, curr.End=%d, prev.End=%d)",
				i, offset, len(contentRunes), curr.End, prev.End)
		}
		if offset > len(contentRunes) {
			t.Fatalf("chunk[%d] merge panic: offset=%d > len(contentRunes)=%d",
				i, offset, len(contentRunes))
		}

		_ = string(contentRunes[offset:])
	}
}

// TestSplitText_RecursiveSeparators_NoOversizeChunks exposes the regression
// where after picking the first separator that yields >1 piece, sub-pieces
// that are still larger than ChunkSize were not split further with the next
// separator. Real-world docs with one paragraph break followed by a long
// run of newline-separated lines must still be honored.
func TestSplitText_RecursiveSeparators_NoOversizeChunks(t *testing.T) {
	// One paragraph break, then 50 short newline-separated lines forming
	// ~1500 chars in the second paragraph.
	body := strings.Repeat("This is one fairly short line of text.\n", 50)
	text := "lead paragraph that is short.\n\n" + body
	cfg := SplitterConfig{
		ChunkSize:    300,
		ChunkOverlap: 30,
		Separators:   []string{"\n\n", "\n", ". "},
	}
	chunks := SplitText(text, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// No chunk should exceed roughly 1.5x ChunkSize — recursive splitting
	// at the next-priority separator should keep this bounded.
	maxAllowed := cfg.ChunkSize * 3 / 2
	for i, c := range chunks {
		l := len([]rune(c.Content))
		if l > maxAllowed {
			t.Errorf("chunk %d is %d runes, > 1.5x ChunkSize (%d) — recursive split missing", i, l, maxAllowed)
		}
	}
}

func TestSplitText_Empty(t *testing.T) {
	chunks := SplitText("", DefaultConfig())
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

func TestSplitText_SingleCharChinese(t *testing.T) {
	text := "\u4f60"
	cfg := SplitterConfig{ChunkSize: 10, ChunkOverlap: 0, Separators: []string{"\n"}}
	chunks := SplitText(text, cfg)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Start != 0 || chunks[0].End != 1 {
		t.Errorf("expected [0,1), got [%d,%d)", chunks[0].Start, chunks[0].End)
	}
}

func TestSplitText_LaTeXBlockInChinese(t *testing.T) {
	text := "\u524d\u9762\u7684\u6587\u5b57$$E=mc^2$$\u540e\u9762\u7684\u6587\u5b57"
	cfg := SplitterConfig{ChunkSize: 200, ChunkOverlap: 0, Separators: []string{"\n"}}
	chunks := SplitText(text, cfg)

	textRunes := []rune(text)
	for i, c := range chunks {
		spanLen := c.End - c.Start
		contentRuneLen := utf8.RuneCountInString(c.Content)
		if spanLen != contentRuneLen {
			t.Errorf("chunk[%d]: span %d != rune len %d", i, spanLen, contentRuneLen)
		}
		if c.End > len(textRunes) {
			t.Errorf("chunk[%d]: End %d > total runes %d", i, c.End, len(textRunes))
		}
	}
}

func TestSplitText_CodeBlockInChinese(t *testing.T) {
	text := "\u4e2d\u6587\u63cf\u8ff0\n```python\nprint('hello')\n```\n\u7ee7\u7eed\u4e2d\u6587"
	cfg := SplitterConfig{ChunkSize: 200, ChunkOverlap: 0, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	textRunes := []rune(text)
	for i, c := range chunks {
		if c.Start < 0 || c.End > len(textRunes) {
			t.Errorf("chunk[%d]: out of range [%d,%d), total %d", i, c.Start, c.End, len(textRunes))
			continue
		}
		sliced := string(textRunes[c.Start:c.End])
		if sliced != c.Content {
			t.Errorf("chunk[%d]: rune-slice mismatch:\n  sliced: %q\n  content: %q",
				i, sliced, c.Content)
		}
	}
}

func TestSplitText_OverlapChunks_NonNegativeStart(t *testing.T) {
	// When overlap is used, start of the next chunk could go before 0 if broken.
	text := strings.Repeat("\u4e2d\u6587\u6d4b\u8bd5\u5185\u5bb9\uff0c", 50)
	cfg := SplitterConfig{ChunkSize: 20, ChunkOverlap: 5, Separators: []string{"\uff0c"}}
	chunks := SplitText(text, cfg)

	for i, c := range chunks {
		if c.Start < 0 {
			t.Errorf("chunk[%d]: negative Start %d", i, c.Start)
		}
		if c.End < c.Start {
			t.Errorf("chunk[%d]: End %d < Start %d", i, c.End, c.Start)
		}
	}
}

func TestFindSemanticOverlapBoundary_PriorityThenEarliest(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "paragraph outranks earlier sentence and line",
			text: "\u7b2c\u4e00\u53e5\u3002\u7b2c\u4e8c\u53e5\n\u666e\u901a\u6362\u884c\u540e\u7684\u5185\u5bb9\n\n" +
				"\u6700\u540e\u4e00\u6bb5",
			want: "\u6700\u540e\u4e00\u6bb5",
		},
		{
			name: "earliest sentence wins within same priority",
			text: "\u7b2c\u4e00\u53e5\u3002\u7b2c\u4e8c\u53e5\u3002\u7b2c\u4e09\u53e5",
			want: "\u7b2c\u4e8c\u53e5\u3002\u7b2c\u4e09\u53e5",
		},
		{
			name: "windows paragraph break",
			text: "\u7b2c\u4e00\u6bb5\u3002\r\n\r\n\u7b2c\u4e8c\u6bb5",
			want: "\u7b2c\u4e8c\u6bb5",
		},
		{
			name: "line outranks earlier sentence",
			text: "\u7b2c\u4e00\u53e5\u3002\u7b2c\u4e00\u884c\n\u7b2c\u4e8c\u884c",
			want: "\u7b2c\u4e8c\u884c",
		},
		{
			name: "windows line break",
			text: "\u7b2c\u4e00\u53e5\u3002\u7b2c\u4e00\u884c\r\n\u7b2c\u4e8c\u884c",
			want: "\u7b2c\u4e8c\u884c",
		},
		{
			name: "chinese question mark",
			text: "\u7b2c\u4e00\u53e5\uff1f\u7b2c\u4e8c\u53e5",
			want: "\u7b2c\u4e8c\u53e5",
		},
		{
			name: "chinese exclamation mark",
			text: "\u7b2c\u4e00\u53e5\uff01\u7b2c\u4e8c\u53e5",
			want: "\u7b2c\u4e8c\u53e5",
		},
		{
			name: "english period requires following space",
			text: "First sentence. Second sentence",
			want: "Second sentence",
		},
		{
			name: "english question mark requires following space",
			text: "Question? Answer",
			want: "Answer",
		},
		{
			name: "english exclamation mark requires following space",
			text: "Warning! Continue",
			want: "Continue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			end, ok := findSemanticOverlapBoundary(tt.text)
			if !ok {
				t.Fatal("expected semantic overlap boundary")
			}
			got := string([]rune(tt.text)[end:])
			if got != tt.want {
				t.Fatalf("overlap tail = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindSemanticOverlapBoundary_NoConfiguredSemanticSeparator(t *testing.T) {
	for _, text := range []string{
		"\u6ca1\u6709\u4efb\u4f55\u8bed\u4e49\u5206\u9694\u7b26\u7684\u8fde\u7eed\u6587\u672c",
		"\u53ea\u6709\uff0c\u9017\u53f7\uff1b\u5206\u53f7\uff1a\u5192\u53f7",
		"version1.2 remains one unit",
		"address 192.168.1.1 remains one unit",
		"see https://ex.com?q=1&foo=bar",
	} {
		if end, ok := findSemanticOverlapBoundary(text); ok {
			t.Errorf("findSemanticOverlapBoundary(%q) returned boundary at %d", text, end)
		}
	}
}

func TestFindSemanticOverlapBoundary_IgnoresProtectedContent(t *testing.T) {
	for _, text := range []string{
		"\u4ee3\u7801\u662f `fmt.Println(\"hello. world\")` \u540e\u7eed\u5185\u5bb9",
		"\u4ee3\u7801\u5757 ```go\nfmt.Println(\"hello. world\")\n``` \u540e\u7eed\u5185\u5bb9",
		"\u516c\u5f0f $$x. y$$ \u540e\u7eed\u5185\u5bb9",
	} {
		if end, ok := findSemanticOverlapBoundary(text); ok {
			t.Errorf("findSemanticOverlapBoundary(%q) returned protected boundary at %d", text, end)
		}
	}
}

func TestFindSemanticOverlapBoundary_FiltersEligibilityBeforePriorityAndPosition(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		minEnd int
		want   string
	}{
		{
			name:   "earlier ineligible sentence does not hide eligible sentence",
			text:   "a\u3002bc\u3002XYZ",
			minEnd: 4,
			want:   "XYZ",
		},
		{
			name:   "ineligible paragraph does not outrank eligible sentence",
			text:   "\n\nx? tail",
			minEnd: 3,
			want:   "tail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			end, ok := findSemanticOverlapBoundaryEndingAtOrAfter(tt.text, tt.minEnd)
			if !ok {
				t.Fatal("expected eligible semantic overlap boundary")
			}
			if got := string([]rune(tt.text)[end:]); got != tt.want {
				t.Fatalf("overlap tail = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComputeOverlap_FindsBoundaryInsideLargeUnit(t *testing.T) {
	text := "abcdefgh\u3002\u5c3e\u5df4\u5185\u5bb9"
	unit := splitUnit{text: text, start: 100, end: 100 + len([]rune(text))}

	overlap, overlapLen := computeOverlap([]splitUnit{unit}, 8, 32, 8)
	if got := unitsText(overlap); got != "\u5c3e\u5df4\u5185\u5bb9" {
		t.Fatalf("overlap = %q, want %q", got, "\u5c3e\u5df4\u5185\u5bb9")
	}
	if overlapLen != len([]rune("\u5c3e\u5df4\u5185\u5bb9")) {
		t.Fatalf("overlapLen = %d, want %d", overlapLen, len([]rune("\u5c3e\u5df4\u5185\u5bb9")))
	}
	if len(overlap) != 1 || overlap[0].start != 109 || overlap[0].end != 113 {
		t.Fatalf("overlap source span = %+v, want [109,113)", overlap)
	}
}

func TestComputeOverlap_NoBoundaryMeansNoOverlap(t *testing.T) {
	text := "abcdefgh\u5c3e\u5df4\u5185\u5bb9"
	unit := splitUnit{text: text, start: 0, end: len([]rune(text))}

	overlap, overlapLen := computeOverlap([]splitUnit{unit}, 8, 32, 8)
	if len(overlap) != 0 || overlapLen != 0 {
		t.Fatalf("expected no overlap, got %q (%d)", unitsText(overlap), overlapLen)
	}
}

func TestComputeOverlap_RespectsNextChunkCapacity(t *testing.T) {
	text := "abcdefgh\u3002\u5c3e\u5df4"
	unit := splitUnit{text: text, start: 0, end: len([]rune(text))}

	overlap, overlapLen := computeOverlap([]splitUnit{unit}, 10, 8, 5)
	if got := unitsText(overlap); got != "\u5c3e\u5df4" {
		t.Fatalf("overlap = %q, want %q", got, "\u5c3e\u5df4")
	}
	if overlapLen+5 > 8 {
		t.Fatalf("overlap plus next content exceeds chunk size: %d + %d > %d", overlapLen, 5, 8)
	}
}

func TestComputeOverlap_LookbehindBoundaryEligibility(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "single rune separator ending at minus one is eligible",
			text: "abcd\u3002WXYZ",
			want: "WXYZ",
		},
		{
			name: "longest separator ending at minus one is eligible",
			text: "\r\n\r\nWXYZ",
			want: "WXYZ",
		},
		{
			name: "separator ending before minus one is ineligible",
			text: "a\u3002bcWXYZ",
			want: "",
		},
		{
			name: "separator crossing original window start is eligible",
			text: "abc. WXY",
			want: "WXY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit := splitUnit{text: tt.text, start: 0, end: len([]rune(tt.text))}
			overlap, overlapLen := computeOverlap([]splitUnit{unit}, 4, 20, 4)
			if got := unitsText(overlap); got != tt.want {
				t.Fatalf("overlap = %q, want %q", got, tt.want)
			}
			if overlapLen != len([]rune(tt.want)) {
				t.Fatalf("overlapLen = %d, want %d", overlapLen, len([]rune(tt.want)))
			}
			if overlapLen > 4 {
				t.Fatalf("overlap exceeds configured limit: %d > 4", overlapLen)
			}
		})
	}
}

func TestSplitText_OverlapDoesNotBreakURLQuery(t *testing.T) {
	line1 := strings.Repeat("a", 35)
	text := line1 + "\n" + "https://ex.com?q=1 tail\n" + "more content here"
	cfg := SplitterConfig{ChunkSize: 40, ChunkOverlap: 10, Separators: []string{"\n\n", "\n"}}

	chunks := SplitText(text, cfg)
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d: %#v", len(chunks), chunks)
	}
	if strings.HasPrefix(chunks[2].Content, "q=1") {
		t.Fatalf("overlap broke at URL query string: chunk[2]=%q", chunks[2].Content)
	}
}

func TestBuildUnitsWithProtection_InlineCodeNotSplit(t *testing.T) {
	text := "intro `code.here` suffix"
	units := buildUnitsWithProtection(text, protectedSpans(text), []string{"\n"}, 0)

	for _, u := range units {
		if strings.Contains(u.text, "code.here") && !strings.Contains(u.text, "`code.here`") {
			t.Fatalf("inline code period split unit: %q", u.text)
		}
	}
}

func TestSplitText_SemanticOverlapInsideParagraph(t *testing.T) {
	first := strings.Repeat("\u7532", 15) + "\u3002\u5c3e\u5df4"
	text := first + "\n\n" + "\u540e\u7eed\u5185\u5bb9"
	cfg := SplitterConfig{
		ChunkSize:    20,
		ChunkOverlap: 10,
		Separators:   []string{"\n\n", "\n", "\u3002"},
	}

	chunks := SplitText(text, cfg)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %#v", len(chunks), chunks)
	}
	if !strings.HasPrefix(chunks[1].Content, "\u5c3e\u5df4\n\n") {
		t.Fatalf("second chunk should start at the sentence boundary inside the large unit, got %q", chunks[1].Content)
	}
	if got := chunks[1].End - chunks[1].Start; got != len([]rune(chunks[1].Content)) {
		t.Fatalf("position invariant broken: span=%d content=%d", got, len([]rune(chunks[1].Content)))
	}
}

func TestBuildUnitsWithProtection_RuneOffsets(t *testing.T) {
	text := "\u4f60\u597d\u4e16\u754c"
	units := buildUnitsWithProtection(text, nil, []string{"\n"}, 0)

	if len(units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(units))
	}

	u := units[0]
	expectedRuneLen := 4 // 4 Chinese characters
	byteLen := len(text) // 12 bytes

	if u.start != 0 {
		t.Errorf("start: got %d, want 0", u.start)
	}
	if u.end != expectedRuneLen {
		t.Errorf("end: got %d, want %d (rune len); byte len is %d", u.end, expectedRuneLen, byteLen)
	}
}

func TestBuildUnitsWithProtection_WithProtectedSpan(t *testing.T) {
	text := "\u524d\u9762![alt](url)\u540e\u9762"
	protected := protectedSpans(text)
	units := buildUnitsWithProtection(text, protected, []string{"\n"}, 0)

	textRunes := []rune(text)
	for i, u := range units {
		contentRuneLen := utf8.RuneCountInString(u.text)
		spanLen := u.end - u.start
		if spanLen != contentRuneLen {
			t.Errorf("unit[%d] %q: span %d != rune len %d (byte len %d)",
				i, u.text, spanLen, contentRuneLen, len(u.text))
		}
		if u.start < 0 || u.end > len(textRunes) {
			t.Errorf("unit[%d]: out of range [%d,%d), total runes %d",
				i, u.start, u.end, len(textRunes))
		}
	}
}

func TestSplitBySeparators(t *testing.T) {
	tests := []struct {
		text       string
		separators []string
		wantParts  int
	}{
		{"a\n\nb\n\nc", []string{"\n\n"}, 5},
		{"abc", []string{"\n"}, 1},
		{"a\nb\nc", []string{"\n"}, 5},
		{"", []string{"\n"}, 1},
	}

	for _, tt := range tests {
		parts := splitBySeparators(tt.text, tt.separators, 0)
		if len(parts) != tt.wantParts {
			t.Errorf("splitBySeparators(%q, %v): got %d parts %v, want %d",
				tt.text, tt.separators, len(parts), parts, tt.wantParts)
		}
	}
}

func TestExtractImageRefs(t *testing.T) {
	text := "hello ![alt1](url1) world ![alt2](url2) end"
	refs := ExtractImageRefs(text)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].OriginalRef != "url1" || refs[0].AltText != "alt1" {
		t.Errorf("ref[0] mismatch: %+v", refs[0])
	}
	if refs[1].OriginalRef != "url2" || refs[1].AltText != "alt2" {
		t.Errorf("ref[1] mismatch: %+v", refs[1])
	}
}

func TestSplitText_LargeChineseDocument(t *testing.T) {
	// Simulate a real document with paragraphs of Chinese text.
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "\u7b2c%d\u6bb5\uff1a\u8fd9\u662f\u4e00\u6bb5\u7528\u4e8e\u6d4b\u8bd5\u7684\u4e2d"+
			"\u6587\u5185\u5bb9\uff0c\u5305\u542b\u5404\u79cd\u5e38\u89c1\u7684\u6c49\u5b57\u548c\u6807\u70b9\u7b26"+
			"\u53f7\u3002", i)
		sb.WriteString("\n\n")
	}
	text := sb.String()

	cfg := SplitterConfig{ChunkSize: 50, ChunkOverlap: 10, Separators: []string{"\n\n", "\n", "\u3002"}}
	chunks := SplitText(text, cfg)

	textRunes := []rune(text)
	for i, c := range chunks {
		contentRuneLen := utf8.RuneCountInString(c.Content)
		spanLen := c.End - c.Start
		if spanLen != contentRuneLen {
			t.Errorf("chunk[%d]: End(%d)-Start(%d)=%d != runeLen(%d)",
				i, c.End, c.Start, spanLen, contentRuneLen)
		}
		if c.Start < 0 {
			t.Errorf("chunk[%d]: negative Start %d", i, c.Start)
		}
		if c.End > len(textRunes) {
			t.Errorf("chunk[%d]: End %d > total runes %d", i, c.End, len(textRunes))
		}
		if c.Start >= 0 && c.End <= len(textRunes) {
			sliced := string(textRunes[c.Start:c.End])
			if sliced != c.Content {
				t.Errorf("chunk[%d]: content mismatch via rune-slice", i)
			}
		}
	}

	// Simulate merge.go logic on all overlapping chunk pairs
	for i := 1; i < len(chunks); i++ {
		prev := chunks[i-1]
		curr := chunks[i]
		if curr.Start > prev.End {
			continue
		}
		contentRunes := []rune(curr.Content)
		offset := len(contentRunes) - (curr.End - prev.End)
		if offset < 0 || offset > len(contentRunes) {
			t.Fatalf("chunk[%d] merge would panic: offset=%d, contentRunes=%d, curr.End=%d, prev.End=%d",
				i, offset, len(contentRunes), curr.End, prev.End)
		}
	}
}

// ---------------------------------------------------------------------------
// Table header prepending tests
// ---------------------------------------------------------------------------

func TestSplitText_TableHeaderPrependedToChunks(t *testing.T) {
	// A markdown table large enough to span multiple chunks.
	// Each chunk after the first should have the header row + separator prepended.
	text := "" +
		"\u524d\u9762\u7684\u6587\u5b57\n\n" +
		"| \u59d3\u540d | \u5e74\u9f84 | \u57ce\u5e02 |\n" +
		"| --- | --- | --- |\n" +
		"| \u5f20\u4e09 | 25 | \u5317\u4eac |\n" +
		"| \u674e\u56db | 30 | \u4e0a\u6d77 |\n" +
		"| \u738b\u4e94 | 28 | \u5e7f\u5dde |\n" +
		"| \u8d75\u516d | 35 | \u6df1\u5733 |\n" +
		"| \u5b59\u4e03 | 22 | \u676d\u5dde |\n" +
		"| \u5468\u516b | 40 | \u6210\u90fd |\n" +
		"\n\u540e\u9762\u7684\u6587\u5b57"

	tableHeader := "| \u59d3\u540d | \u5e74\u9f84 | \u57ce\u5e02 |\n| --- | --- | --- |\n"

	cfg := SplitterConfig{ChunkSize: 60, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}

	// Find chunks that contain table row data but not the original header position.
	// These should have the header prepended.
	headerPrependCount := 0
	for _, c := range chunks {
		if strings.Contains(c.Content, "| \u674e\u56db") || strings.Contains(c.Content, "| \u738b\u4e94") ||
			strings.Contains(c.Content, "| \u8d75\u516d") || strings.Contains(c.Content, "| \u5b59\u4e03") ||
			strings.Contains(c.Content, "| \u5468\u516b") {
			if !strings.Contains(c.Content, "| \u5f20\u4e09") {
				// This is a chunk with table rows but not the first row;
				// it should have the header prepended.
				if !strings.HasPrefix(c.Content, tableHeader) {
					t.Errorf("chunk (seq=%d) has table rows but is missing prepended header:\n%s",
						c.Seq, c.Content)
				} else {
					headerPrependCount++
				}
			}
		}
	}

	if headerPrependCount == 0 {
		t.Error("expected at least one chunk to have prepended table header, found none")
		for i, c := range chunks {
			t.Logf("chunk[%d] (seq=%d, start=%d, end=%d):\n%s", i, c.Seq, c.Start, c.End, c.Content)
		}
	}
}

func TestSplitText_NoHeaderForNonTableContent(t *testing.T) {
	// Ensure header prepending doesn't affect non-table content.
	text := strings.Repeat("\u8fd9\u662f\u4e00\u6bb5\u666e\u901a\u7684\u4e2d\u6587\u6587\u672c\uff0c\u4e0d\u5305"+
		"\u542b\u4efb\u4f55\u8868\u683c\u3002\n\n", 10)

	cfg := SplitterConfig{ChunkSize: 30, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	textRunes := []rune(text)
	for i, c := range chunks {
		contentRuneLen := utf8.RuneCountInString(c.Content)
		spanLen := c.End - c.Start
		if spanLen != contentRuneLen {
			t.Errorf("chunk[%d]: span %d != rune len %d (no table, should be exact)", i, spanLen, contentRuneLen)
		}
		if c.End > len(textRunes) {
			t.Errorf("chunk[%d]: End %d exceeds total runes %d", i, c.End, len(textRunes))
		}
	}
}

func TestSplitText_TableHeaderEndedByEmptyLine(t *testing.T) {
	// After the table ends (empty line), subsequent chunks should NOT have the header.
	text := "" +
		"| A | B |\n" +
		"| --- | --- |\n" +
		"| 1 | 2 |\n" +
		"| 3 | 4 |\n" +
		"\n" +
		"\u8fd9\u662f\u8868\u683c\u4e4b\u540e\u7684\u666e\u901a\u6587\u672c\u5185\u5bb9\uff0c\u4e0d\u5e94\u8be5\u5305" +
		"\u542b\u8868\u5934\u3002\n" +
		"\u66f4\u591a\u7684\u666e\u901a\u6587\u672c\u5185\u5bb9\u7528\u4e8e\u586b\u5145\u3002"

	cfg := SplitterConfig{ChunkSize: 40, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	for _, c := range chunks {
		hasTableRow := strings.Contains(c.Content, "| A | B |") || strings.Contains(c.Content, "|")
		hasPlainText := strings.Contains(c.Content, "\u8fd9\u662f\u8868\u683c\u4e4b"+
			"\u540e") || strings.Contains(c.Content, "\u66f4\u591a\u7684\u666e\u901a")
		if hasPlainText && !hasTableRow {
			// This chunk is purely post-table text; should NOT have table header
			if strings.Contains(c.Content, "| --- |") {
				t.Errorf("post-table chunk should not contain table header:\n%s", c.Content)
			}
		}
	}
}

func TestHeaderTracker_BasicLifecycle(t *testing.T) {
	ht := newHeaderTracker()

	// Before table: no headers
	ht.update("Some regular text")
	if h := ht.getHeaders(); h != "" {
		t.Errorf("expected no headers before table, got %q", h)
	}

	// Table header unit
	ht.update("| A | B |\n| --- | --- |\n")
	if h := ht.getHeaders(); h == "" {
		t.Error("expected active header after table header unit")
	}

	// Table row: header should stay active
	ht.update("| 1 | 2 |\n")
	if h := ht.getHeaders(); h == "" {
		t.Error("header should remain active during table rows")
	}

	// Empty line: header should end
	ht.update("\n")
	if h := ht.getHeaders(); h != "" {
		t.Errorf("header should be cleared after empty line, got %q", h)
	}

	// New table can be tracked after the old one ended
	ht.update("| X | Y |\n| --- | --- |\n")
	if h := ht.getHeaders(); h == "" {
		t.Error("expected new header to be tracked after previous table ended")
	}
}

func TestHeaderTracker_EmptyHeaderRowRewrite(t *testing.T) {
	// Some converters (e.g., MarkItDown) produce tables with empty header rows:
	//   ||
	//   | --- | --- |
	//   | real col A | real col B |
	// The tracker should rewrite the header to be a proper Markdown table header:
	//   | real col A | real col B |
	//   | --- | --- |
	ht := newHeaderTracker()

	// Empty header row + separator
	ht.update("||\n| --- | --- | --- |\n")
	h := ht.getHeaders()
	if h == "" {
		t.Fatal("expected active header after empty header unit")
	}
	t.Logf("after empty header unit (pending): %q", h)

	// First data row → becomes the real column names
	ht.update("| \u6d4b\u8bd5\u7528\u4f8b ID | \u6d4b\u8bd5\u6a21\u5757 | \u5907\u6ce8 |\n")
	h = ht.getHeaders()
	t.Logf("after rewrite: %q", h)

	if !strings.Contains(h, "\u6d4b\u8bd5\u7528\u4f8b ID") {
		t.Errorf("rewritten header should contain column names, got:\n%s", h)
	}
	if strings.Contains(h, "||") {
		t.Errorf("rewritten header should NOT contain empty '||' row, got:\n%s", h)
	}
	if !strings.Contains(h, "---") {
		t.Errorf("rewritten header should contain separator, got:\n%s", h)
	}
	// Column names should come BEFORE the separator
	colIdx := strings.Index(h, "\u6d4b\u8bd5\u7528\u4f8b ID")
	sepIdx := strings.Index(h, "---")
	if colIdx > sepIdx {
		t.Errorf("column names should appear before separator in rewritten header:\n%s", h)
	}

	// Subsequent data rows should NOT be absorbed
	ht.update("| TC-001 | \u6a21\u5757A | \u5907\u6ce81 |\n")
	h2 := ht.getHeaders()
	if strings.Contains(h2, "TC-001") {
		t.Errorf("header should NOT include subsequent data rows, got:\n%s", h2)
	}

	// Table end
	ht.update("\n")
	if ht.getHeaders() != "" {
		t.Error("header should be cleared after empty line")
	}
}

func TestHeaderTracker_NormalHeaderNoExtension(t *testing.T) {
	// A table with proper column names in the header row should NOT be extended.
	ht := newHeaderTracker()

	ht.update("| \u59d3\u540d | \u5e74\u9f84 |\n| --- | --- |\n")
	h := ht.getHeaders()
	if h == "" {
		t.Fatal("expected active header")
	}

	// First data row should NOT be absorbed into the header
	ht.update("| \u5f20\u4e09 | 25 |\n")
	h2 := ht.getHeaders()
	if strings.Contains(h2, "\u5f20\u4e09") {
		t.Errorf("normal header should not absorb data rows, got:\n%s", h2)
	}
}

func TestSplitText_EmptyHeaderRowPrepend(t *testing.T) {
	// Simulate MarkItDown output: empty header row, real column names in first data row.
	text := "" +
		"\u524d\u8a00\n\n" +
		"||\n" +
		"| --- | --- | --- |\n" +
		"| \u7528\u4f8bID | \u6a21\u5757 | \u6b65\u9aa4 |\n" +
		"| TC-001 | A | \u6b65\u9aa41 |\n" +
		"| TC-002 | B | \u6b65\u9aa42 |\n" +
		"| TC-003 | C | \u6b65\u9aa43 |\n" +
		"| TC-004 | D | \u6b65\u9aa44 |\n" +
		"\n" +
		"\u7ed3\u5c3e"

	cfg := SplitterConfig{ChunkSize: 80, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	t.Logf("total chunks: %d", len(chunks))
	for i, c := range chunks {
		t.Logf("chunk[%d] seq=%d start=%d end=%d:\n%s", i, c.Seq, c.Start, c.End, c.Content)
	}

	for _, c := range chunks {
		hasLaterRow := strings.Contains(c.Content, "TC-002") ||
			strings.Contains(c.Content, "TC-003") ||
			strings.Contains(c.Content, "TC-004")
		if hasLaterRow && !strings.Contains(c.Content, "TC-001") {
			// Should have column names prepended
			if !strings.Contains(c.Content, "\u7528\u4f8bID") {
				t.Errorf("chunk with data rows should have real column names prepended:\n%s", c.Content)
			}
			// Should NOT have the empty || row
			lines := strings.Split(c.Content, "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				isOnlyPipes := trimmed != "" && func() bool {
					for _, r := range trimmed {
						if r != '|' && r != ' ' {
							return false
						}
					}
					return true
				}()
				if isOnlyPipes {
					t.Errorf("chunk should NOT contain empty pipe row %q:\n%s", trimmed, c.Content)
					break
				}
			}
		}

		// No line should appear as a duplicate in any chunk
		lines := strings.Split(strings.TrimRight(c.Content, "\n"), "\n")
		seen := make(map[string]int)
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.Contains(trimmed, "---") {
				continue
			}
			seen[trimmed]++
			if seen[trimmed] > 1 {
				t.Errorf("line appears %d times in chunk (seq=%d): %q", seen[trimmed], c.Seq, trimmed)
			}
		}
	}

	// Verify restoration still works
	restored := restoreTextFromChunks(chunks)
	if restored != text {
		t.Errorf("restoration failed for empty-header table\n  original: %q\n  restored: %q", text, restored)
	}
}

func TestHeaderTracker_ColumnMismatchEndsTable(t *testing.T) {
	ht := newHeaderTracker()
	ht.update("| Name | Game | Fame | Blame |\n| --- | --- | --- | --- |\n")
	if ht.getHeaders() == "" {
		t.Fatal("expected active table header")
	}
	ht.update("| Sinple | Table |\n")
	if h := ht.getHeaders(); h != "" {
		t.Fatalf("2-col row should end 4-col table header, still active:\n%s", h)
	}
}

func TestTableRowColumnCount_EscapedPipeIsCellText(t *testing.T) {
	cases := []struct {
		row  string
		want int
	}{
		{"| a | b | c |", 3},
		{`| a \| b | c |`, 2},
		{"| `x \\| y` | c |", 2},
		{`| a \\| b | c |`, 3}, // an escaped backslash leaves the pipe a delimiter
		{`| a \\\| b | c |`, 2},
		{`| C:\temp\x | c |`, 2}, // a backslash before another character escapes nothing
	}
	for _, tc := range cases {
		if got := tableRowColumnCount(tc.row); got != tc.want {
			t.Errorf("tableRowColumnCount(%q) = %d, want %d", tc.row, got, tc.want)
		}
	}
}

func TestHeaderTracker_EscapedPipeRowKeepsTableHeader(t *testing.T) {
	ht := newHeaderTracker()
	ht.update("| Name | Note | City |\n| --- | --- | --- |\n")
	ht.update("| Lee | x \\| y | Shanghai |\n")
	if ht.getHeaders() == "" {
		t.Fatal("a 3-col row with an escaped pipe in a cell ended the 3-col table header")
	}
}

func TestSplitText_EscapedPipeRowKeepsTableHeaderInLaterChunks(t *testing.T) {
	text := "" +
		"\u524d\u9762\u7684\u6587\u5b57\n\n" +
		"| \u59d3\u540d | \u5907\u6ce8 | \u57ce\u5e02 |\n" +
		"| --- | --- | --- |\n" +
		"| \u5f20\u4e09 | a | \u5317\u4eac |\n" +
		"| \u674e\u56db | x \\| y | \u4e0a\u6d77 |\n" +
		"| \u738b\u4e94 | b | \u5e7f\u5dde |\n" +
		"| \u8d75\u516d | c | \u6df1\u5733 |\n" +
		"| \u5b59\u4e03 | d | \u676d\u5dde |\n" +
		"| \u5468\u516b | e | \u6210\u90fd |\n" +
		"\n\u540e\u9762\u7684\u6587\u5b57"
	tableHeader := "| \u59d3\u540d | \u5907\u6ce8 | \u57ce\u5e02 |\n| --- | --- | --- |\n"

	cfg := SplitterConfig{ChunkSize: 60, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	checked := 0
	for _, c := range chunks {
		for _, row := range []string{"| \u738b\u4e94", "| \u8d75\u516d", "| \u5b59\u4e03", "| \u5468\u516b"} {
			if strings.Contains(c.Content, row) {
				checked++
				if !strings.HasPrefix(c.Content, tableHeader) {
					t.Errorf("chunk (seq=%d) with rows after the escaped-pipe row is missing the header:\n%s",
						c.Seq, c.Content)
				}
				break
			}
		}
	}
	if checked == 0 {
		t.Fatal("expected chunks holding the rows after the escaped-pipe row")
	}
}

func TestHeaderTracker_ParagraphBreakEndsOnNextUnit(t *testing.T) {
	ht := newHeaderTracker()
	ht.update("| Name | Game | Fame | Blame |\n| --- | --- | --- | --- |\n")
	ht.update("| Russell Wilson | Football | High | Tacky uniform |\n\n")
	if h := ht.getHeaders(); h == "" {
		t.Fatal("paragraph break alone should not clear header yet")
	}
	if !ht.pendingTableBreak {
		t.Fatal("expected pendingTableBreak after row ending with \\n\\n")
	}
	ht.update("| Sinple | Table |\n")
	if h := ht.getHeaders(); h != "" {
		t.Fatalf("next table row should clear previous header, got %q", h)
	}
	if !ht.headerEndedThisUnit {
		t.Fatal("expected flush signal when new table starts after paragraph break")
	}
}

func TestSplitText_EnTablesNoCrossTableHeader(t *testing.T) {
	text := "## A table, with and without a header row\n\n" +
		"| Name | Game | Fame | Blame |\n" +
		"| --- | --- | --- | --- |\n" +
		"| Lebron James | Basketball | Very High | Leaving Cleveland |\n" +
		"| Ryan Braun | Baseball | Moderate | Steroids |\n" +
		"| Russell Wilson | Football | High | Tacky uniform |\n\n" +
		"| Sinple | Table |\n" +
		"| Without | Header |\n\n" +
		"| Simple  Multiparagraph | Table  Full |\n" +
		"| Of  Paragraphs | In each  Cell. |\n"

	cfg := SplitterConfig{ChunkSize: 200, ChunkOverlap: 20, Separators: []string{"\n\n", "\n", "\u3002"}}
	chunks := SplitText(text, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	for i, c := range chunks {
		hasSinple := strings.Contains(c.Content, "| Sinple | Table |")
		hasSimple := strings.Contains(c.Content, "| Simple  Multiparagraph |")
		if hasSinple || hasSimple {
			if strings.Contains(c.Content, "| Name | Game | Fame | Blame |") {
				t.Errorf("chunk[%d] must not carry table-1 header into later tables:\n%s", i, c.Content)
			}
		}
	}
}

func TestSplitText_MultipleTablesInDocument(t *testing.T) {
	text := "" +
		"\u7b2c\u4e00\u4e2a\u8868\u683c\uff1a\n\n" +
		"| \u540d\u79f0 | \u503c |\n" +
		"| --- | --- |\n" +
		"| A | 1 |\n" +
		"| B | 2 |\n" +
		"| C | 3 |\n" +
		"\n" +
		"\u4e2d\u95f4\u7684\u6587\u5b57\n\n" +
		"| \u9879\u76ee | \u72b6\u6001 |\n" +
		"| --- | --- |\n" +
		"| X | \u5b8c\u6210 |\n" +
		"| Y | \u8fdb\u884c\u4e2d |\n" +
		"| Z | \u672a\u5f00\u59cb |\n" +
		"\n" +
		"\u7ed3\u5c3e\u6587\u5b57"

	cfg := SplitterConfig{ChunkSize: 50, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	// Verify that if a chunk has rows from table 2, it has table 2's header, not table 1's.
	for _, c := range chunks {
		if strings.Contains(c.Content, "| Y |") && !strings.Contains(c.Content, "| X |") {
			if !strings.Contains(c.Content, "| \u9879\u76ee | \u72b6\u6001 |") {
				t.Errorf("chunk with table-2 rows should have table-2 header:\n%s", c.Content)
			}
			if strings.Contains(c.Content, "| \u540d\u79f0 | \u503c |") {
				t.Errorf("chunk with table-2 rows should NOT have table-1 header:\n%s", c.Content)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Start/End restoration tests — verify original text can be reconstructed
// ---------------------------------------------------------------------------

// restoreTextFromChunks reconstructs the original text using only chunk
// Start/End positions. For chunks with prepended headers, the header is a
// "virtual" prefix whose length = runeLen(Content) - (End - Start).
// The original text portion is the last (End-Start) runes of Content.
func restoreTextFromChunks(chunks []Chunk) string {
	if len(chunks) == 0 {
		return ""
	}

	// Sort by End (ascending), then Start (ascending) — same order as Python restore_text
	sorted := make([]Chunk, len(chunks))
	copy(sorted, chunks)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			if sorted[j].End < sorted[j-1].End ||
				(sorted[j].End == sorted[j-1].End && sorted[j].Start < sorted[j-1].Start) {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			} else {
				break
			}
		}
	}

	var result []rune
	lastEnd := 0

	for _, c := range sorted {
		if c.End <= lastEnd {
			continue // fully contained in a previously processed chunk
		}

		contentRunes := []rune(c.Content)
		spanLen := c.End - c.Start
		headerLen := len(contentRunes) - spanLen
		if headerLen < 0 {
			headerLen = 0
		}
		// originalPortion is the text[Start:End] part, excluding any prepended header
		originalPortion := contentRunes[headerLen:]

		// Only take the portion after lastEnd (skip overlap)
		newStart := 0
		if lastEnd > c.Start {
			newStart = lastEnd - c.Start
		}
		if newStart < len(originalPortion) {
			result = append(result, originalPortion[newStart:]...)
		}

		lastEnd = c.End
	}

	return string(result)
}

func TestSplitText_RestoreTextNoTable(t *testing.T) {
	// Plain Chinese text without any tables — baseline restoration check.
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, "\u7b2c%d\u6bb5\uff1a\u8fd9\u662f\u4e00\u6bb5\u7528\u4e8e\u6d4b\u8bd5\u7684\u4e2d"+
			"\u6587\u5185\u5bb9\uff0c\u5305\u542b\u5404\u79cd\u6807\u70b9\u7b26\u53f7\u3002", i)
		sb.WriteString("\n\n")
	}
	text := sb.String()

	cfg := SplitterConfig{ChunkSize: 50, ChunkOverlap: 10, Separators: []string{"\n\n", "\n", "\u3002"}}
	chunks := SplitText(text, cfg)

	restored := restoreTextFromChunks(chunks)
	if restored != text {
		t.Errorf("restoration failed for plain text\n  original len: %d\n  restored len: %d",
			len([]rune(text)), len([]rune(restored)))
		// Find first difference
		orig := []rune(text)
		rest := []rune(restored)
		minLen := len(orig)
		if len(rest) < minLen {
			minLen = len(rest)
		}
		for i := 0; i < minLen; i++ {
			if orig[i] != rest[i] {
				t.Errorf("first diff at rune %d: orig=%q rest=%q", i, string(orig[i]), string(rest[i]))
				break
			}
		}
	}
}

func TestSplitText_RestoreTextWithTable(t *testing.T) {
	// Document with a table large enough to span multiple chunks.
	// Use a 2-column table (shorter header ~24 runes) + ChunkSize 80 so the
	// header can be prepended (header 24 + row ~16 = 40 < 80).
	text := "" +
		"\u8fd9\u662f\u6587\u6863\u524d\u8a00\u90e8\u5206\u7684\u5185\u5bb9\u3002\n\n" +
		"| \u59d3\u540d | \u57ce\u5e02 |\n" +
		"| --- | --- |\n" +
		"| \u5f20\u4e09 | \u5317\u4eac |\n" +
		"| \u674e\u56db | \u4e0a\u6d77 |\n" +
		"| \u738b\u4e94 | \u5e7f\u5dde |\n" +
		"| \u8d75\u516d | \u6df1\u5733 |\n" +
		"| \u5b59\u4e03 | \u676d\u5dde |\n" +
		"| \u5468\u516b | \u6210\u90fd |\n" +
		"| \u5434\u4e5d | \u6b66\u6c49 |\n" +
		"| \u90d1\u5341 | \u5357\u4eac |\n" +
		"\n" +
		"\u8fd9\u662f\u8868\u683c\u4e4b\u540e\u7684\u6587\u5b57\u5185\u5bb9\u3002\n" +
		"\u8fd9\u91cc\u8fd8\u6709\u66f4\u591a\u7684\u666e\u901a\u6bb5\u843d\u3002"

	cfg := SplitterConfig{ChunkSize: 80, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	t.Logf("total chunks: %d", len(chunks))
	for i, c := range chunks {
		contentRunes := []rune(c.Content)
		spanLen := c.End - c.Start
		headerLen := len(contentRunes) - spanLen
		t.Logf("chunk[%d] seq=%d start=%d end=%d span=%d contentLen=%d headerPrepend=%d\n  content: %q",
			i, c.Seq, c.Start, c.End, spanLen, len(contentRunes), headerLen, c.Content)
	}

	// 1. Verify basic position invariants
	textRunes := []rune(text)
	for i, c := range chunks {
		if c.Start < 0 {
			t.Errorf("chunk[%d]: Start %d < 0", i, c.Start)
		}
		if c.End > len(textRunes) {
			t.Errorf("chunk[%d]: End %d > total runes %d", i, c.End, len(textRunes))
		}
		if c.End < c.Start {
			t.Errorf("chunk[%d]: End %d < Start %d", i, c.End, c.Start)
		}
	}

	// 2. Verify text[Start:End] matches the non-header portion of Content
	for i, c := range chunks {
		contentRunes := []rune(c.Content)
		spanLen := c.End - c.Start
		headerLen := len(contentRunes) - spanLen
		if headerLen < 0 {
			t.Errorf("chunk[%d]: Content rune len (%d) < span len (%d) — impossible",
				i, len(contentRunes), spanLen)
			continue
		}

		if c.Start >= 0 && c.End <= len(textRunes) {
			originalSlice := string(textRunes[c.Start:c.End])
			contentSuffix := string(contentRunes[headerLen:])
			if originalSlice != contentSuffix {
				t.Errorf("chunk[%d]: text[%d:%d] != Content[%d:]"+
					"\n  text slice:     %q"+
					"\n  content suffix: %q",
					i, c.Start, c.End, headerLen, originalSlice, contentSuffix)
			}
		}
	}

	// 3. Restore original text and compare
	restored := restoreTextFromChunks(chunks)
	if restored != text {
		t.Errorf("restoration FAILED"+
			"\n  original rune len: %d"+
			"\n  restored rune len: %d",
			len(textRunes), len([]rune(restored)))
		orig := []rune(text)
		rest := []rune(restored)
		minLen := len(orig)
		if len(rest) < minLen {
			minLen = len(rest)
		}
		for i := 0; i < minLen; i++ {
			if orig[i] != rest[i] {
				lo := i - 20
				if lo < 0 {
					lo = 0
				}
				hi := i + 20
				if hi > minLen {
					hi = minLen
				}
				t.Errorf("first diff at rune %d:\n  orig context: %q\n  rest context: %q",
					i, string(orig[lo:hi]), string(rest[lo:hi]))
				break
			}
		}
	} else {
		t.Log("restoration OK — original text perfectly reconstructed from Start/End")
	}

	// 4. Verify full coverage — Start/End spans must cover [0, len(textRunes))
	covered := make([]bool, len(textRunes))
	for _, c := range chunks {
		for p := c.Start; p < c.End && p < len(textRunes); p++ {
			covered[p] = true
		}
	}
	for i, v := range covered {
		if !v {
			t.Errorf("rune position %d is not covered by any chunk", i)
			break
		}
	}
}

func TestSplitText_RestoreTextWithMultipleTables(t *testing.T) {
	text := "" +
		"\u524d\u8a00\n\n" +
		"| A | B |\n| --- | --- |\n" +
		"| 1 | 2 |\n| 3 | 4 |\n| 5 | 6 |\n| 7 | 8 |\n" +
		"\n\u4e2d\u95f4\u6587\u5b57\n\n" +
		"| X | Y |\n| --- | --- |\n" +
		"| a | b |\n| c | d |\n| e | f |\n" +
		"\n\u7ed3\u5c3e"

	cfg := SplitterConfig{ChunkSize: 50, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	chunks := SplitText(text, cfg)

	// Restore and compare
	restored := restoreTextFromChunks(chunks)
	if restored != text {
		t.Errorf("multi-table restoration failed\n  original: %q\n  restored: %q", text, restored)
	}

	// Verify text[Start:End] matches content suffix for every chunk
	textRunes := []rune(text)
	for i, c := range chunks {
		contentRunes := []rune(c.Content)
		spanLen := c.End - c.Start
		headerLen := len(contentRunes) - spanLen
		if headerLen < 0 {
			t.Errorf("chunk[%d]: headerLen < 0", i)
			continue
		}
		if c.End <= len(textRunes) {
			if string(textRunes[c.Start:c.End]) != string(contentRunes[headerLen:]) {
				t.Errorf("chunk[%d]: text[Start:End] mismatch with content suffix", i)
			}
		}
	}
}

func TestSplitText_RestoreTextWithOverlap(t *testing.T) {
	// Larger overlap to stress the overlap+header interaction.
	text := "" +
		"| \u52171 | \u52172 | \u52173 |\n" +
		"| --- | --- | --- |\n" +
		"| \u6570\u636eA1 | \u6570\u636eA2 | \u6570\u636eA3 |\n" +
		"| \u6570\u636eB1 | \u6570\u636eB2 | \u6570\u636eB3 |\n" +
		"| \u6570\u636eC1 | \u6570\u636eC2 | \u6570\u636eC3 |\n" +
		"| \u6570\u636eD1 | \u6570\u636eD2 | \u6570\u636eD3 |\n" +
		"| \u6570\u636eE1 | \u6570\u636eE2 | \u6570\u636eE3 |\n" +
		"| \u6570\u636eF1 | \u6570\u636eF2 | \u6570\u636eF3 |\n" +
		"\n" +
		"\u8868\u540e\u6587\u672c\u3002"

	for _, overlap := range []int{0, 3, 10, 20} {
		t.Run(fmt.Sprintf("overlap=%d", overlap), func(t *testing.T) {
			cfg := SplitterConfig{ChunkSize: 60, ChunkOverlap: overlap, Separators: []string{"\n\n", "\n"}}
			chunks := SplitText(text, cfg)

			restored := restoreTextFromChunks(chunks)
			if restored != text {
				t.Errorf("restoration failed with overlap=%d\n  orig len=%d  rest len=%d",
					overlap, len([]rune(text)), len([]rune(restored)))
				for i, c := range chunks {
					t.Logf("  chunk[%d] start=%d end=%d content=%q", i, c.Start, c.End, c.Content)
				}
			}
		})
	}
}

func TestSplitTextParentChild_WithTableHeaders(t *testing.T) {
	text := "" +
		"\u524d\u8a00\n\n" +
		"| \u5217A | \u5217B |\n" +
		"| --- | --- |\n" +
		"| \u6570\u636e1 | \u6570\u636e2 |\n" +
		"| \u6570\u636e3 | \u6570\u636e4 |\n" +
		"| \u6570\u636e5 | \u6570\u636e6 |\n" +
		"| \u6570\u636e7 | \u6570\u636e8 |\n" +
		"\n" +
		"\u7ed3\u5c3e"

	parentCfg := SplitterConfig{ChunkSize: 200, ChunkOverlap: 0, Separators: []string{"\n\n", "\n"}}
	childCfg := SplitterConfig{ChunkSize: 40, ChunkOverlap: 5, Separators: []string{"\n\n", "\n"}}
	result := SplitTextParentChild(text, parentCfg, childCfg)

	if len(result.Children) == 0 {
		t.Fatal("expected child chunks")
	}

	// Verify child chunk positions don't exceed parent document
	textRunes := []rune(text)
	for i, child := range result.Children {
		if child.Start < 0 {
			t.Errorf("child[%d]: negative Start %d", i, child.Start)
		}
		if child.End > len(textRunes) {
			t.Errorf("child[%d]: End %d exceeds text rune count %d", i, child.End, len(textRunes))
		}
	}
}
