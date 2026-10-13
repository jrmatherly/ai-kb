package searchutil

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestAppendWithOverlap_ContiguousNoTrim(t *testing.T) {
	// The segments touch (no overlap), and next holds HTML entities so its length > EndAt-StartAt.
	// The old position formula cut too much from the start; here the whole segment must stay.
	acc := "## \u7b2c\u4e8c\u8282\n\n"
	next := "| \u5217A | \u5217B |\n| \u503c1 | \u542b\u5b9e\u4f53&#34;\u5f15\u53f7&#34;\u7684\u5185\u5bb9 |\n"
	got := AppendWithOverlap(acc, next, 0)
	want := acc + next
	if got != want {
		t.Fatalf("contiguous merge mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestAppendWithOverlap_PrependedTableHeaderSkipped(t *testing.T) {
	// Simulate the chunker repeating the header on a split table: next starts with an extra header
	// (zero-width, invisible to positions) and the real overlapping rows follow it. Position trimming
	// misaligns; text matching must dedupe correctly.
	header := "| \u52171 | \u52172 | \u52173 |\n|:---|:---|:---|\n"
	overlapRows := "| \u7b2c5\u884c | \u5185\u5bb95A | \u5185\u5bb95B |\n| \u7b2c6\u884c | \u5185\u5bb96A | " +
		"\u5185\u5bb96B |\n"
	accTail := "| \u7b2c4\u884c | \u5185\u5bb94A | \u5185\u5bb94B |\n" + overlapRows
	acc := header + "| \u7b2c1\u884c | x | \u2014 |\n" + accTail

	newRows := "| \u7b2c7\u884c | \u5185\u5bb97A | \u5185\u5bb97B |\n| \u7b2c8\u884c | \u5185\u5bb98A | " +
		"\u5185\u5bb98B |\n"
	next := header + overlapRows + newRows

	// The position overlap is roughly two rows long (an approximation is enough, it only sizes the window).
	got := AppendWithOverlap(acc, next, len([]rune(overlapRows)))
	want := acc + newRows
	if got != want {
		t.Fatalf("prepended-header merge mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestAppendWithOverlap_PlainOverlap(t *testing.T) {
	acc := "abcdefghijklmnopqrstuvwxyz0123"
	// next overlaps the "klmnopqrstuvwxyz0123" at the end of acc, then continues with new content
	next := "klmnopqrstuvwxyz0123ABCDEFG"
	got := AppendWithOverlap(acc, next, 20)
	want := "abcdefghijklmnopqrstuvwxyz0123ABCDEFG"
	if got != want {
		t.Fatalf("plain overlap mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestAppendWithOverlap_NoOverlap(t *testing.T) {
	acc := "hello world"
	next := "completely different"
	got := AppendWithOverlap(acc, next, 0)
	want := acc + next
	if got != want {
		t.Fatalf("no overlap mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestAppendWithExactOverlap_TrimsKnownOverlap(t *testing.T) {
	overlap := "shared boundary text"
	got, ok := AppendWithExactOverlap("before "+overlap, overlap+" after", len([]rune(overlap)))
	if !ok {
		t.Fatal("exact overlap should be accepted")
	}
	want := "before " + overlap + " after"
	if got != want {
		t.Fatalf("exact overlap mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestAppendWithExactOverlap_ZeroOverlapConcatenatesRepeatedText(t *testing.T) {
	// Both touching segments consist of the same periodic text (table rows / log lines). With 0 overlap
	// they must be joined as is, without a suffix search, or the repeated rows at the start of next get eaten.
	row := "| cell | cell |\n"
	acc := "\u524d\u8a00\n" + row + row
	next := row + row + row + "\u7ed3\u5c3e\n"
	got, ok := AppendWithExactOverlap(acc, next, 0)
	if !ok {
		t.Fatal("zero overlap should be accepted")
	}
	if got != acc+next {
		t.Fatalf("zero overlap must concatenate verbatim:\n got=%q\nwant=%q", got, acc+next)
	}
}

func TestAppendWithExactOverlap_RejectsMismatchedOverlap(t *testing.T) {
	// The length invariant holds but the text differs (HTML entities, a repeated header, ...): it must be rejected
	// so the caller falls back to text matching.
	if _, ok := AppendWithExactOverlap("abcdefghijkl", "XYZdefghijkl", 6); ok {
		t.Fatal("mismatched overlap should be rejected")
	}
}

func TestAppendWithExactOverlap_RejectsOverlapLongerThanBodies(t *testing.T) {
	if _, ok := AppendWithExactOverlap("short", "shorter", 99); ok {
		t.Fatal("overlap exceeding body length should be rejected")
	}
	if _, ok := AppendWithExactOverlap("short", "shorter", -1); ok {
		t.Fatal("negative overlap should be rejected")
	}
}

func TestJoinChunkContentUsesCurrentTextInsteadOfSourceOffsets(t *testing.T) {
	first := "first edited body with no original overlap"
	second := "second independently edited body"
	got := JoinChunkContent(first, second, "\n\n")
	want := first + "\n\n" + second
	if got != want {
		t.Fatalf("edited join mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestJoinChunkContentRemovesRealBoundaryOverlap(t *testing.T) {
	overlap := "shared boundary text"
	got := JoinChunkContent("before "+overlap, overlap+" after", "\n\n")
	want := "before " + overlap + " after"
	if got != want {
		t.Fatalf("overlap join mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestJoinChunkContentCollapsesExactContainment(t *testing.T) {
	outer := "prefix complete current body suffix"
	if got := JoinChunkContent(outer, "complete current body", "\n\n"); got != outer {
		t.Fatalf("contained body should be collapsed: %q", got)
	}
}

func TestMergeTextChunks_OrdersFiltersAndStitches(t *testing.T) {
	header := "| a | b |\n|:--|:--|\n"
	chunks := []*types.Chunk{
		{
			Content: header + "| r1 | x |\n| r2 | y |\n", ChunkType: types.ChunkTypeText,
			StartAt: 0, EndAt: 20, ChunkIndex: 0,
		},
		{
			// repeated header + overlap with the previous segment's r2 + new row r3
			Content: header + "| r2 | y |\n| r3 | z |\n", ChunkType: types.ChunkTypeText,
			StartAt: 10, EndAt: 40, ChunkIndex: 1,
		},
	}
	got := MergeTextChunks(chunks, "\n")
	want := header + "| r1 | x |\n| r2 | y |\n" + "| r3 | z |\n"
	if got != want {
		t.Fatalf("MergeTextChunks mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestMergeTextChunks_GapSeparator(t *testing.T) {
	chunks := []*types.Chunk{
		{Content: "first", ChunkType: types.ChunkTypeText, StartAt: 0, EndAt: 5, ChunkIndex: 0},
		{Content: "second", ChunkType: types.ChunkTypeText, StartAt: 100, EndAt: 106, ChunkIndex: 1},
	}
	got := MergeTextChunks(chunks, "\n")
	want := "first\nsecond"
	if got != want {
		t.Fatalf("gap separator mismatch:\n got=%q\nwant=%q", got, want)
	}
}

// TestAppendWithOverlap_ContiguousRealContentRepeat is a regression test:
// two chunks are strictly contiguous (positionOverlap==0), but a boilerplate
// sentence at the tail of acc reappears inside the head window of next as
// real content (the same sentence is written multiple times in the document).
// The old algorithm would enter text matching and, due to the headSlack
// floor of 320, mistake the head of next for a prepended table header and
// delete it, causing irreversible content loss. After the fix it should
// concatenate directly without trimming.
func TestAppendWithOverlap_ContiguousRealContentRepeat(t *testing.T) {
	repeat := "The system shall maintain a complete audit trail of all transactions."
	acc := "3.2 Logging Requirements\n\n" + repeat
	next := "\n\n5.1 Security Controls\n\n* Role-based access\n* Encryption at rest\n\n5.2 Compliance\n\n" + repeat + " This satisfies SOC 2."
	got := AppendWithOverlap(acc, next, 0)
	want := acc + next
	if got != want {
		t.Fatalf("contiguous real-content repeat must not trim:\n got.len=%d\nwant.len=%d\n got.tail=%q\nwant.tail=%q",
			len(got), len(want), got[len(acc)-50:], want[len(acc)-50:])
	}
}
