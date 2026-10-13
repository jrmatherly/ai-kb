package session

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

// handleFor builds a syntactically valid 22-character resource handle for
// artifact i, so tests exercise the same ParseResourcePath path as production.
func handleFor(i int) string {
	base := fmt.Sprintf("art%d", i)
	return base + strings.Repeat("x", types.ResourceHandleLength-len(base))
}

func refFor(i int) string {
	return types.BuildResourcePath(handleFor(i))
}

// artifactsFixture builds artifacts whose storage URL is a catalog handle -
// the normal deployment. Use artifactsWithoutCatalog for the degraded case.
func artifactsFixture(names ...string) types.MessageArtifacts {
	list := make(types.MessageArtifacts, 0, len(names))
	for i, name := range names {
		list = append(list, types.MessageArtifact{FileName: name, URL: refFor(i)})
	}
	return list
}

func artifactsWithoutCatalog(names ...string) types.MessageArtifacts {
	list := make(types.MessageArtifacts, 0, len(names))
	for _, name := range names {
		list = append(list, types.MessageArtifact{FileName: name, URL: "local://7/exports/" + name})
	}
	return list
}

func TestRewriteArtifactReferences(t *testing.T) {
	artifacts := artifactsFixture(
		"\u5e02\u573a\u753b\u50cf\u8bc4\u5206_e7edba.html", // CJK name: "market profile score"
		"concept_ranking.csv",
		"trend.png",
		"\u817e\u8baf\u63a7\u80a1(00700) \u6210\u4ea4\u91cf_838ccc.html", // CJK name: "Tencent Holdings(00700) volume"
	)

	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "file name with spaces and parentheses",
			content: "![Volume](sandbox:\u817e\u8baf\u63a7\u80a1(00700) \u6210\u4ea4\u91cf_838ccc.html)",
			want:    "![Volume](" + refFor(3) + ")",
		},
		{
			name:    "file name with spaces and parentheses, no prefix",
			content: "![Volume](\u817e\u8baf\u63a7\u80a1(00700) \u6210\u4ea4\u91cf_838ccc.html)",
			want:    "![Volume](" + refFor(3) + ")",
		},
		{
			name:    "bare file name in image",
			content: "![Market profile score](\u5e02\u573a\u753b\u50cf\u8bc4\u5206_e7edba.html)",
			want:    "![Market profile score](" + refFor(0) + ")",
		},
		{
			name:    "sandbox prefix",
			content: "![Score](sandbox:\u5e02\u573a\u753b\u50cf\u8bc4\u5206_e7edba.html)",
			want:    "![Score](" + refFor(0) + ")",
		},
		{
			name:    "sandbox scheme with slashes",
			content: "![Score](sandbox://trend.png)",
			want:    "![Score](" + refFor(2) + ")",
		},
		{
			name:    "directory prefix is dropped",
			content: "[Ranking](/workspace/output/concept_ranking.csv)",
			want:    "[Ranking](" + refFor(1) + ")",
		},
		{
			name:    "percent-encoded name",
			content: "![Score](%E5%B8%82%E5%9C%BA%E7%94%BB%E5%83%8F%E8%AF%84%E5%88%86_e7edba.html)",
			want:    "![Score](" + refFor(0) + ")",
		},
		{
			name:    "title is preserved",
			content: `![Score](trend.png "Trend")`,
			want:    `![Score](` + refFor(2) + ` "Trend")`,
		},
		{
			name:    "ordinary link with a colliding name is not rewritten",
			content: "See [notes](trend.png)",
			want:    "See [notes](trend.png)",
		},
		{
			name:    "sandbox-prefixed link is rewritten even when not an image",
			content: "Data in [table](sandbox:concept_ranking.csv)",
			want:    "Data in [table](" + refFor(1) + ")",
		},
		{
			name:    "already-rewritten reference is left alone",
			content: "![Score](" + refFor(0) + ")",
			want:    "![Score](" + refFor(0) + ")",
		},
		{
			name:    "prose parentheses are not link destinations",
			content: "Tencent Holdings(00700) volume is shown below.",
			want:    "Tencent Holdings(00700) volume is shown below.",
		},
		{
			name:    "unknown file name untouched",
			content: "![Other](missing.html)",
			want:    "![Other](missing.html)",
		},
		{
			name:    "http url untouched",
			content: "![Remote](https://example.com/trend.png)",
			want:    "![Remote](https://example.com/trend.png)",
		},
		{
			name:    "knowledge base image untouched",
			content: "![Resource](resource://abcdefghijklmnopqrstuv)",
			want:    "![Resource](resource://abcdefghijklmnopqrstuv)",
		},
		{
			name:    "fenced code untouched",
			content: "```\n![Score](trend.png)\n```",
			want:    "```\n![Score](trend.png)\n```",
		},
		{
			name:    "inline code untouched",
			content: "Write `![Score](trend.png)` and you are done",
			want:    "Write `![Score](trend.png)` and you are done",
		},
		{
			name:    "plain prose untouched",
			content: "Generated two files, trend.png and concept_ranking.csv.",
			want:    "Generated two files, trend.png and concept_ranking.csv.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := rewriteArtifactReferences(tc.content, artifacts); got != tc.want {
				t.Fatalf("rewriteArtifactReferences() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRewriteArtifactReferencesMixedContent(t *testing.T) {
	artifacts := artifactsFixture("chart.html", "data.csv")
	content := "## Chart\n\n![Chart](chart.html)\n\nData in [table](sandbox:data.csv), " +
		"external [doc](https://example.com/chart.html) is untouched."
	want := "## Chart\n\n![Chart](" + refFor(0) + ")\n\nData in [table](" + refFor(1) + "), " +
		"external [doc](https://example.com/chart.html) is untouched."

	if got := rewriteArtifactReferences(content, artifacts); got != want {
		t.Fatalf("rewriteArtifactReferences() = %q, want %q", got, want)
	}
}

// A knowledge-base image and a skill artifact routinely appear in the same
// answer. Only the artifact is rebound; the existing reference must survive
// byte for byte, since it already is the canonical form.
func TestRewriteArtifactReferencesKeepsExistingResourceImages(t *testing.T) {
	artifacts := artifactsFixture("chart.html")
	kbImage := types.BuildResourcePath(strings.Repeat("Z", types.ResourceHandleLength))
	content := "![Retrieved image](" + kbImage + ")\n\n![Chart](chart.html)"
	want := "![Retrieved image](" + kbImage + ")\n\n![Chart](" + refFor(0) + ")"

	if got := rewriteArtifactReferences(content, artifacts); got != want {
		t.Fatalf("rewriteArtifactReferences() = %q, want %q", got, want)
	}
}

// Without a resource catalog there is no durable handle, so references are
// normalized to the chat-only sandbox form rather than leaking a storage path.
func TestRewriteArtifactReferencesWithoutCatalog(t *testing.T) {
	artifacts := artifactsWithoutCatalog("chart.html")
	got := rewriteArtifactReferences("![Chart](chart.html)", artifacts)
	if want := "![Chart](sandbox:chart.html)"; got != want {
		t.Fatalf("rewriteArtifactReferences() = %q, want %q", got, want)
	}
	if strings.Contains(got, "local://") {
		t.Fatalf("storage path leaked into content: %q", got)
	}
}

func TestRewriteArtifactReferencesNoArtifacts(t *testing.T) {
	content := "![Chart](chart.html)"
	if got := rewriteArtifactReferences(content, nil); got != content {
		t.Fatalf("rewriteArtifactReferences() = %q, want unchanged", got)
	}
}

func TestArtifactRefByNameKeepsFirstDuplicate(t *testing.T) {
	byName := artifactRefByName(artifactsFixture("a.html", "a.html", "b.html"))
	if byName["a.html"] != refFor(0) {
		t.Fatalf("duplicate name resolved to %q, want %q", byName["a.html"], refFor(0))
	}
	if byName["b.html"] != refFor(2) {
		t.Fatalf("b.html resolved to %q, want %q", byName["b.html"], refFor(2))
	}
}

func TestReferencedArtifactsMatchesNamesAndHandles(t *testing.T) {
	artifacts := artifactsFixture("report.pptx", "chart.html", "data.csv")

	cases := []struct {
		name    string
		content string
		want    types.MessageArtifacts
	}{
		{
			name:    "sandbox-prefixed name",
			content: "Generated ![Report](sandbox:report.pptx)",
			want:    types.MessageArtifacts{artifacts[0]},
		},
		{
			name:    "bare name in an image",
			content: "![Chart](chart.html)",
			want:    types.MessageArtifacts{artifacts[1]},
		},
		{
			name:    "output path in an ordinary link",
			content: "[Data](./output/data.csv)",
			want:    types.MessageArtifacts{artifacts[2]},
		},
		{
			name:    "canonical handle",
			content: "![Report](" + refFor(0) + ")",
			want:    types.MessageArtifacts{artifacts[0]},
		},
		{
			name:    "prose mention is not a reference",
			content: "Generated two files, report.pptx and chart.html.",
			want:    nil,
		},
		{
			name:    "bare name in an ordinary link is not a reference",
			content: "See [notes](report.pptx)",
			want:    nil,
		},
		{
			name:    "unknown name",
			content: "![Other](missing.pptx)",
			want:    nil,
		},
		{
			name:    "code sample is not a reference",
			content: "```\n![Report](sandbox:report.pptx)\n```",
			want:    nil,
		},
		{
			// Go regexp.Split does not interleave the matched fences, so
			// walking parts with i+=2 would skip the segment after the first
			// fence and miss a real citation that rewrite still rewrites.
			name:    "reference after a code fence",
			content: "![Rep](sandbox:report.pptx)\n\n```\n![No](sandbox:chart.html)\n```\n\n![Fig](sandbox:chart.html)",
			want:    types.MessageArtifacts{artifacts[0], artifacts[1]},
		},
		{
			name:    "multiple references keep candidate order",
			content: "![Data](data.csv)\n\n![Report](sandbox:report.pptx)",
			want:    types.MessageArtifacts{artifacts[0], artifacts[2]},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, referencedArtifacts(tc.content, artifacts))
		})
	}
}

// A regenerated file shadows the older version of itself: the name resolves to
// the first candidate, while an explicit handle still reaches the old one.
func TestReferencedArtifactsPrefersTheFirstCandidate(t *testing.T) {
	old := types.MessageArtifact{FileName: "deck.pptx", URL: refFor(0)}
	fresh := types.MessageArtifact{FileName: "deck.pptx", URL: refFor(1)}
	candidates := types.MessageArtifacts{fresh, old}

	require.Equal(t, types.MessageArtifacts{fresh},
		referencedArtifacts("![deck](sandbox:deck.pptx)", candidates))
	require.Equal(t, types.MessageArtifacts{old},
		referencedArtifacts("![deck]("+old.URL+")", candidates))
}

// KnownArtifacts is oldest-first. Reverse it before merging so a hash-skipped
// name binds the latest file; this turn still shadows that latest file.
func TestMergeArtifactListsPrefersThisTurnThenLatestKnown(t *testing.T) {
	old := types.MessageArtifact{FileName: "deck.pptx", URL: refFor(0)}
	latest := types.MessageArtifact{FileName: "deck.pptx", URL: refFor(1)}
	fresh := types.MessageArtifact{FileName: "deck.pptx", URL: refFor(2)}
	known := types.MessageArtifacts{old, latest}

	require.Equal(t, types.MessageArtifacts{latest},
		referencedArtifacts("![deck](sandbox:deck.pptx)",
			mergeArtifactLists(nil, artifactsNewestFirst(known))))
	require.Equal(t, types.MessageArtifacts{fresh},
		referencedArtifacts("![deck](sandbox:deck.pptx)",
			mergeArtifactLists(types.MessageArtifacts{fresh}, artifactsNewestFirst(known))))
}

func TestArtifactsNewestFirstDoesNotMutateInput(t *testing.T) {
	in := artifactsFixture("a.html", "b.html")
	got := artifactsNewestFirst(in)
	require.Equal(t, "b.html", got[0].FileName)
	require.Equal(t, "a.html", got[1].FileName)
	require.Equal(t, "a.html", in[0].FileName)
}
