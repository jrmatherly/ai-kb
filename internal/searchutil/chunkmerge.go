package searchutil

import (
	"sort"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// JoinChunkContent joins two current chunk bodies without relying on parser
// offsets. Exact containment is collapsed, a real suffix/prefix overlap is
// removed, and otherwise both bodies are retained with separator between
// them. The conservative fallback intentionally prefers small duplication
// over silently dropping edited content.
func JoinChunkContent(acc, next, separator string) string {
	if acc == "" {
		return next
	}
	if next == "" {
		return acc
	}
	if ContainsChunkContent(acc, next) {
		return acc
	}
	if ContainsChunkContent(next, acc) {
		return next
	}

	accRunes := []rune(acc)
	nextRunes := []rune(next)
	maxOverlap := minInt(len(accRunes), len(nextRunes))
	// Editable chunks may be much larger than parser-produced chunks. Bound
	// suffix matching so an adversarial 200 KB edit cannot turn retrieval into
	// quadratic work. Parser overlap windows are normally far below this cap;
	// larger unmatched overlap is safely retained as duplication.
	if maxOverlap > defaultSearchSpan {
		maxOverlap = defaultSearchSpan
	}
	for overlap := maxOverlap; overlap >= minOverlapRunes; overlap-- {
		if runeSlicesEqual(accRunes[len(accRunes)-overlap:], nextRunes[:overlap]) {
			return acc + string(nextRunes[overlap:])
		}
	}
	return acc + separator + next
}

// ContainsChunkContent reports whether the complete current body is safely
// represented by another body. Very short substrings are not treated as
// containment because common words and punctuation would create false drops.
func ContainsChunkContent(container, contained string) bool {
	if container == "" || contained == "" {
		return false
	}
	if container == contained {
		return true
	}
	return len([]rune(contained)) >= minOverlapRunes && strings.Contains(container, contained)
}

func runeSlicesEqual(left, right []rune) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// This file holds the shared overlap-join logic for chunk content, reused by document
// reconstruction (reconstructContent), knowledge graph content merging (graph mergeChunkContents)
// and similar paths. Chat retrieval lets users edit chunks, so it uses JoinChunkContent above,
// which needs no source offsets.
//
// Callers used to trim overlap with a position formula (offset = len(content) - (EndAt -
// lastEndAt) or similar), which assumes len([]rune(Content)) == EndAt-StartAt. Two kinds
// of data break that invariant and cause misaligned joins, lost or duplicated text:
//  1. The parent-child chunker repeats the header row on a split table. That header is zero-width
//     (start == end), positions cannot express it, and content is longer than EndAt-StartAt.
//  2. content may keep HTML entities (such as &#34; / &gt;), longer than the source span.
//
// So overlap is matched by text instead: find the first occurrence of the merged text's suffix
// in a window at the start of the next segment and continue after it. Positions (StartAt/EndAt)
// only size the search window and are no longer used for trimming.

const (
	// minOverlapRunes is the shortest suffix that takes part in matching. Shorter ones (such as a
	// table separator row |---|) match by accident, so they are ignored.
	minOverlapRunes = 12
	// defaultSearchSpan is the minimum search window, so real overlap within a range is found
	// even when positions are missing or 0.
	defaultSearchSpan = 400
)

// AppendWithOverlap appends next to acc and removes the overlap between them.
//
// positionOverlap is the overlap estimated from StartAt/EndAt (lastEnd - curStart) and only
// bounds the search window; the real overlap is matched by text, which tolerates repeated table
// headers and HTML entity length drift.
// With no text overlap the two are joined as is (no trimming): keeping text beats damaging it.
//
// positionOverlap <= 0 means the two segments are strictly adjacent or disjoint, with no overlap to remove.
// Text matching there would, with the headSlack floor of 320, wrongly match a genuine repeat
// of acc's suffix near the start of next (the same sentence appearing twice in a document) and drop
// the start of next as a repeated table header, an irreversible loss. Join directly; repeated
// headers are left to the caller's post-processing.
func AppendWithOverlap(acc, next string, positionOverlap int) string {
	if acc == "" {
		return next
	}
	if next == "" {
		return acc
	}
	if positionOverlap <= 0 {
		return acc + next
	}

	accRunes := []rune(acc)
	nextRunes := []rune(next)

	span := positionOverlap

	maxK := minInt(len(accRunes), len(nextRunes))
	if cap := maxInt(span*3, defaultSearchSpan); maxK > cap {
		maxK = cap
	}
	// How much prefix (a repeated header or other synthetic text) may be skipped before the overlap.
	headSlack := maxInt(span*2, 320)

	for k := maxK; k >= minOverlapRunes; k-- {
		needle := accRunes[len(accRunes)-k:]
		if pos := indexRunes(nextRunes, needle, headSlack); pos >= 0 {
			return acc + string(nextRunes[pos+k:])
		}
	}
	return acc + next
}

// AppendWithExactOverlap joins acc and next by the exact overlap the positions give, when the
// caller has confirmed the positions are reliable: it checks that the last overlap characters of acc equal
// the first overlap characters of next and trims exactly; overlap 0 joins directly.
//
// Unlike AppendWithOverlap it does not guess. That function searches a window for the longest suffix
// match to tolerate repeated headers and HTML entity length drift, so periodic text (tables, logs) can be
// mistaken for overlap and real content trimmed. With reliable positions the overlap is known; no search is needed.
//
// A failed check returns ok=false, and the caller decides whether to fall back to AppendWithOverlap.
func AppendWithExactOverlap(acc, next string, overlap int) (string, bool) {
	if acc == "" {
		return next, true
	}
	if next == "" {
		return acc, true
	}
	if overlap < 0 {
		return "", false
	}
	if overlap == 0 {
		return acc + next, true
	}

	accRunes := []rune(acc)
	nextRunes := []rune(next)
	if overlap > len(accRunes) || overlap > len(nextRunes) {
		return "", false
	}
	if !runeSlicesEqual(accRunes[len(accRunes)-overlap:], nextRunes[:overlap]) {
		return "", false
	}
	return acc + string(nextRunes[overlap:]), true
}

// MergeTextChunks sorts by StartAt (ties by ChunkIndex) and rebuilds the full text of several
// chunks with AppendWithOverlap. gapSep separates two segments that are not adjacent (a gap)
// (for example "\n"); an empty string joins them directly.
//
// The caller filters by type first (for example keeping only text chunks); this function ignores ChunkType.
func MergeTextChunks(chunks []*types.Chunk, gapSep string) string {
	if len(chunks) == 0 {
		return ""
	}

	sorted := make([]*types.Chunk, len(chunks))
	copy(sorted, chunks)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].StartAt == sorted[j].StartAt {
			return sorted[i].ChunkIndex < sorted[j].ChunkIndex
		}
		return sorted[i].StartAt < sorted[j].StartAt
	})

	merged := ""
	mergedEnd := -1
	for _, c := range sorted {
		if c == nil || c.Content == "" {
			continue
		}
		if merged == "" {
			merged = c.Content
			if c.EndAt > 0 {
				mergedEnd = c.EndAt
			}
			continue
		}

		// A gap, or missing positions (EndAt==0): join as a separate paragraph without trimming overlap.
		if c.StartAt > mergedEnd || c.EndAt == 0 {
			if gapSep != "" {
				merged += gapSep
			}
			merged += c.Content
			if c.EndAt > 0 {
				mergedEnd = c.EndAt
			}
			continue
		}

		// Partial overlap or touching ends: remove the overlap by text matching, then join.
		if c.EndAt > mergedEnd {
			merged = AppendWithOverlap(merged, c.Content, mergedEnd-c.StartAt)
			mergedEnd = c.EndAt
		}
		// Otherwise the previous segment fully covers this one; skip it.
	}

	return merged
}

// indexRunes returns the rune index of needle's first occurrence in haystack, starting no later than
// maxStart, or -1 when not found.
func indexRunes(haystack, needle []rune, maxStart int) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	limit := len(haystack) - len(needle)
	if maxStart < limit {
		limit = maxStart
	}
	for i := 0; i <= limit; i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
