package service

import (
	"strings"
	"testing"
)

func TestLinkifyContent_BasicCJK(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
	}
	got, changed := linkifyContent("\u6211\u4f4f\u5728\u5317\u4eac\u5e02", refs, "")
	if !changed {
		t.Fatalf("expected change")
	}
	want := "\u6211\u4f4f\u5728[[beijing|\u5317\u4eac]]\u5e02"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLinkifyContent_LongerNameWinsOverSubstring(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
		{slug: "bupt", matchText: "\u5317\u4eac\u90ae\u7535\u5927\u5b66"},
	}
	got, changed := linkifyContent("\u6211\u5c31\u8bfb\u4e8e\u5317\u4eac\u90ae\u7535\u5927\u5b66", refs, "")
	if !changed {
		t.Fatalf("expected change")
	}
	// The longer match should win; "Beijing" must not swallow the prefix.
	if !strings.Contains(got, "[[bupt|\u5317\u4eac\u90ae\u7535\u5927\u5b66]]") {
		t.Fatalf("longer match not preferred: %q", got)
	}
	if strings.Contains(got, "[[beijing|") {
		t.Fatalf("shorter substring should not have linked: %q", got)
	}
}

func TestLinkifyContent_SkipsSingleHanRuneAutoLink(t *testing.T) {
	refs := []linkRef{
		{slug: "entity/feng", matchText: "\u98ce"},
	}
	in := "\u7a7f\u8d8a\u6cb3\u897f\u8d70\u5eca\u4f53\u9a8c\u5927\u6f20\u98ce\u6c99\u3002"
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("single Han rune must not be auto-linked: %q", got)
	}
	if got != in {
		t.Fatalf("content changed: got %q, want %q", got, in)
	}
}

func TestLinkifyContent_PreservesExplicitSingleHanRuneLink(t *testing.T) {
	refs := []linkRef{
		{slug: "entity/feng", matchText: "\u98ce"},
	}
	in := "\u5bd3\u8a00\u4e2d\uff0c[[entity/feng|\u98ce]]\u7fa1\u6155\u76ee\u3002"
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("explicit single-character link must be preserved without reinjection: %q", got)
	}
	if got != in {
		t.Fatalf("content changed: got %q, want %q", got, in)
	}
}

func TestLinkifyContent_ASCIIWordBoundary(t *testing.T) {
	refs := []linkRef{
		{slug: "ai", matchText: "AI"},
	}
	// Should NOT match "AI" inside "TRAINING" or "PAINT".
	in := "TRAINING and PAINT are words."
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("should not change, got %q", got)
	}

	// SHOULD match standalone "AI".
	in2 := "AI is cool."
	got2, changed2 := linkifyContent(in2, refs, "")
	if !changed2 {
		t.Fatalf("expected change, got %q", got2)
	}
	if !strings.HasPrefix(got2, "[[ai|AI]] ") {
		t.Fatalf("standalone AI should link: %q", got2)
	}
}

func TestLinkifyContent_SkipsExistingWikiLink(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
	}
	// Already linked - should not double-wrap.
	in := "\u6211\u4f4f\u5728[[beijing|\u5317\u4eac]]\u5e02"
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("should not change already-linked content: %q", got)
	}
}

func TestLinkifyContent_SkipsInsideFencedCode(t *testing.T) {
	refs := []linkRef{
		{slug: "go", matchText: "Go"},
	}
	in := "Prose mentions Go.\n\n```go\nfunc Go() {}\n```\nAfter code."
	got, _ := linkifyContent(in, refs, "")
	// The prose occurrence should link.
	if !strings.Contains(got, "Prose mentions [[go|Go]].") {
		t.Fatalf("prose occurrence not linked: %q", got)
	}
	// The occurrence inside the fenced block must be untouched. Since linkifyContent
	// only wraps the first eligible match, the code-block one is skipped anyway,
	// but double-check that no link was injected inside the code fence.
	if strings.Contains(got, "func [[go|Go]]") {
		t.Fatalf("should not link inside fenced code: %q", got)
	}
}

func TestLinkifyContent_SkipsInsideInlineCode(t *testing.T) {
	refs := []linkRef{
		{slug: "foo", matchText: "Foo"},
	}
	// Only occurrence is inside `Foo` - must NOT be linked.
	in := "Use the `Foo` type."
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("should not link inside inline code: %q", got)
	}

	// Second case: prose before the inline code - should link the prose one.
	in2 := "Foo is a type. Use `Foo` here."
	got2, changed2 := linkifyContent(in2, refs, "")
	if !changed2 {
		t.Fatalf("expected change, got %q", got2)
	}
	if !strings.HasPrefix(got2, "[[foo|Foo]] is") {
		t.Fatalf("prose occurrence not linked: %q", got2)
	}
	if strings.Contains(got2, "`[[foo|Foo]]`") {
		t.Fatalf("inline code occurrence should not be linked: %q", got2)
	}
}

func TestLinkifyContent_SkipsInsideMarkdownLink(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
	}
	// Occurrence inside [text](url) should be skipped.
	in := "\u53c2\u89c1 [\u5317\u4eac](https://example.com/beijing)"
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("should not link inside markdown link text: %q", got)
	}
}

func TestLinkifyContent_OnlyFirstOccurrenceWrapped(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
	}
	in := "\u5317\u4eac\u5f88\u5927\u3002\u5317\u4eac\u6709\u5f88\u591a\u4eba\u3002\u5317\u4eac\u662f\u9996\u90fd\u3002"
	got, changed := linkifyContent(in, refs, "")
	if !changed {
		t.Fatalf("expected change")
	}
	// Only first match should be wrapped.
	count := strings.Count(got, "[[beijing|\u5317\u4eac]]")
	if count != 1 {
		t.Fatalf("expected exactly 1 link, got %d: %q", count, got)
	}
	// Later occurrences remain bare.
	if strings.Count(got, "\u5317\u4eac") != 3 {
		t.Fatalf("expected 3 total Beijing (incl. one inside the link), got content %q", got)
	}
}

func TestLinkifyContent_SkipsSelfSlug(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
	}
	// Rendering on the beijing page itself - must not self-link.
	got, changed := linkifyContent("\u5317\u4eac\u662f\u9996\u90fd", refs, "beijing")
	if changed {
		t.Fatalf("should not self-link: %q", got)
	}
}

func TestLinkifyContent_SkipsWhenSlugAlreadyUsed(t *testing.T) {
	// Title matches by alias, but slug already appears via a different mention.
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
		{slug: "beijing", matchText: "\u4eac\u5e08"},
	}
	in := "\u6211\u5728[[beijing|\u4eac\u5e08]]\u51fa\u5dee\uff0c\u540e\u6765\u53bb\u4e86\u5317\u4eac\u3002"
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("should skip when slug already linked: %q", got)
	}
}

func TestLinkifyContent_MultipleRefsDisjoint(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
		{slug: "shanghai", matchText: "\u4e0a\u6d77"},
	}
	in := "\u6211\u4eca\u5929\u53bb\u5317\u4eac\uff0c\u660e\u5929\u53bb\u4e0a\u6d77\u3002"
	got, changed := linkifyContent(in, refs, "")
	if !changed {
		t.Fatalf("expected change")
	}
	if !strings.Contains(got, "[[beijing|\u5317\u4eac]]") {
		t.Fatalf("missing beijing link: %q", got)
	}
	if !strings.Contains(got, "[[shanghai|\u4e0a\u6d77]]") {
		t.Fatalf("missing shanghai link: %q", got)
	}
}

func TestLinkifyContent_EmptyOrNoMatch(t *testing.T) {
	if _, c := linkifyContent("", []linkRef{{slug: "x", matchText: "y"}}, ""); c {
		t.Fatalf("empty content must not change")
	}
	if _, c := linkifyContent("hello", nil, ""); c {
		t.Fatalf("nil refs must not change")
	}
	if _, c := linkifyContent("hello world", []linkRef{{slug: "x", matchText: "zzz"}}, ""); c {
		t.Fatalf("no match must not change")
	}
}

func TestFindFirstSafeMatch_BoundaryCases(t *testing.T) {
	// Case where the first occurrence is unsafe (in code), second is safe.
	s := "See `AI` in docs. AI rocks."
	forb, _ := computeForbiddenSpans(s)
	idx := findFirstSafeMatch(s, "AI", forb)
	// First AI is inside `...`, second is safe.
	want := strings.Index(s, "AI rocks")
	if idx != want {
		t.Fatalf("expected safe match at %d, got %d", want, idx)
	}
}

func TestComputeForbiddenSpans_FencedCode(t *testing.T) {
	s := "before\n```\nhidden\n```\nafter"
	spans, _ := computeForbiddenSpans(s)
	if len(spans) == 0 {
		t.Fatalf("expected at least one span")
	}
	// Ensure "hidden" is inside a forbidden span.
	h := strings.Index(s, "hidden")
	if !spanContains(spans, h, h+len("hidden")) {
		t.Fatalf("hidden should be forbidden, spans=%v", spans)
	}
	// "before" and "after" must NOT be forbidden.
	b := strings.Index(s, "before")
	if spanContains(spans, b, b+len("before")) {
		t.Fatalf("before should not be forbidden")
	}
	a := strings.Index(s, "after")
	if spanContains(spans, a, a+len("after")) {
		t.Fatalf("after should not be forbidden")
	}
}

func TestComputeForbiddenSpans_CollectsUsedSlugs(t *testing.T) {
	s := "Refer to [[beijing|\u5317\u4eac]] and [[shanghai]] and [[nope|x]]."
	_, used := computeForbiddenSpans(s)
	for _, slug := range []string{"beijing", "shanghai", "nope"} {
		if _, ok := used[slug]; !ok {
			t.Fatalf("expected slug %q in used set, got %v", slug, used)
		}
	}
}

func TestLinkifyContent_SkipsInsideReferenceStyleLink(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
	}
	// [text][label] form - occurrence should not be linkified.
	in := "See [\u5317\u4eac][capital] for details.\n\n[capital]: https://example.com/beijing"
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("should not link inside reference-style link or definition: %q", got)
	}
}

func TestLinkifyContent_SkipsReferenceDefinitionLine(t *testing.T) {
	refs := []linkRef{
		{slug: "example", matchText: "example"},
	}
	// The `example` inside the definition URL must not be wrapped.
	in := "[cap]: https://example.com/x"
	got, changed := linkifyContent(in, refs, "")
	if changed {
		t.Fatalf("should not link inside reference definition: %q", got)
	}
}

func TestLinkifyContent_RepeatedLinkifyIsIdempotent(t *testing.T) {
	refs := []linkRef{
		{slug: "beijing", matchText: "\u5317\u4eac"},
		{slug: "shanghai", matchText: "\u4e0a\u6d77"},
	}
	in := "\u4eca\u5929\u53bb\u5317\u4eac\uff0c\u660e\u5929\u53bb\u4e0a\u6d77\u3002"
	once, _ := linkifyContent(in, refs, "")
	twice, changed := linkifyContent(once, refs, "")
	if changed {
		t.Fatalf("second run should be a no-op, got %q", twice)
	}
	if once != twice {
		t.Fatalf("second run altered content:\nbefore: %q\nafter:  %q", once, twice)
	}
}
