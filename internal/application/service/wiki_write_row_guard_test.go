package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ledger is the shape that regressed in production: the rows ARE the data, so a
// rewrite that drops some of them shrinks the page. The first column names the
// holder, and the guard keys a row on exactly that column.
func ledger(title string, holders ...string) string {
	body := "# " + title + "\n\n## \u6301\u8bc1\u4eba\u5458\u660e\u7ec6\n\n" +
		"| \u59d3\u540d | \u8bc1\u4e66\u7f16\u53f7 | \u6709\u6548\u671f\u6b62 |\n| --- | --- | --- |\n"
	for _, holder := range holders {
		body += "| " + holder + " | CERT-2020 | 2099-01-01 |\n"
	}
	return body
}

func TestWikiWriteMissingRowIdentities(t *testing.T) {
	full := ledger("PMP", "\u5f20\u4e09", "\u674e\u56db", "\u738b\u4e94")
	cases := []struct {
		name      string
		existing  string
		rewritten string
		want      []string
	}{
		{"identical body keeps every row", full, full, nil},
		{
			"reordered rows are all still there",
			full, ledger("PMP", "\u738b\u4e94", "\u5f20\u4e09", "\u674e\u56db"), nil,
		},
		{
			"emphasis, padding and case changes are not lost rows",
			full, ledger("PMP", "**\u5f20\u4e09**", " \u674e\u56db ", "\u738b\u4e94"), nil,
		},
		{
			"a renamed header is not a lost row",
			full,
			"# PMP \u8bc1\u4e66\n\n## \u6301\u8bc1\u4eba\u5458\u660e\u7ec6\n\n" +
				"| \u6301\u8bc1\u4eba | \u8bc1\u4e66\u7f16\u53f7 | \u6709\u6548\u671f\u81f3 |\n| --- | --- | --- |\n" +
				"| \u5f20\u4e09 | CERT-2020 | 2099-01-01 |\n| \u674e\u56db | CERT-2020 | 2099-01-01 |\n" +
				"| \u738b\u4e94 | CERT-2020 | 2099-01-01 |\n",
			nil,
		},
		{
			"an added row is not a loss",
			full, ledger("PMP", "\u5f20\u4e09", "\u674e\u56db", "\u738b\u4e94", "\u8d75\u516d"), nil,
		},
		{"prose-only pages have nothing to guard", "just prose", "shorter prose", nil},
		{
			"one dropped row is reported",
			full, ledger("PMP", "\u5f20\u4e09", "\u674e\u56db"),
			[]string{"\u738b\u4e94"},
		},
		{
			"a truncated rewrite reports every missing row",
			full, ledger("PMP", "\u5f20\u4e09"),
			[]string{"\u674e\u56db", "\u738b\u4e94"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want,
				wikiWriteMissingRowIdentities(testCase.existing, testCase.rewritten))
		})
	}
}

// Duplicate identities are compared as a set, not as a multiset: two stored
// rows whose first non-empty cell is the same are ONE identity, so a rewrite
// that keeps it once has kept it — merging them into one row is not a loss.
// Only when the identity no longer appears at all is it reported, and then
// once, not once per stored row.
func TestWikiWriteMissingRowIdentitiesComparesIdentitiesAsASet(t *testing.T) {
	existing := ledger("\u8bc1\u4e66", "\u5f20\u4e09", "\u5f20\u4e09")
	require.Empty(t, wikiWriteMissingRowIdentities(existing, ledger("\u8bc1\u4e66", "\u5f20\u4e09")))
	require.Equal(t, []string{"\u5f20\u4e09"}, wikiWriteMissingRowIdentities(existing, ledger("\u8bc1\u4e66")))
}

// The ratio denominator is the number of DISTINCT identities: repeating a
// holder on the stored page must not dilute the share of identities lost, or a
// page full of duplicates could lose whole subjects under the threshold.
func TestWikiWriteUniqueRowIdentityCountCountsEachIdentityOnce(t *testing.T) {
	require.Equal(t, 0, wikiWriteUniqueRowIdentityCount("just prose"))
	require.Equal(t, 1, wikiWriteUniqueRowIdentityCount(ledger("\u8bc1\u4e66", "\u5f20\u4e09", "\u5f20\u4e09")))
	repeated := ledger("\u8bc1\u4e66", "\u5f20\u4e09", "\u5f20\u4e09", "\u674e\u56db", "\u738b\u4e94", "\u674e\u56db")
	require.Equal(t, 3, wikiWriteUniqueRowIdentityCount(repeated))
}

// The threshold boundary is exact: a loss of exactly 20% is allowed, anything
// above it is not, and an empty stored identity set has no ratio at all.
func TestWikiWriteRowIdentityLossRatioBoundary(t *testing.T) {
	require.Equal(t, 0.0, wikiWriteRowIdentityLossRatio(2, 0), "an empty identity set cannot lose anything")
	require.LessOrEqual(t, wikiWriteRowIdentityLossRatio(1, 5), wikiWriteIngestRowIdentityLossLimit)
	require.Greater(t, wikiWriteRowIdentityLossRatio(1, 4), wikiWriteIngestRowIdentityLossLimit)
}

// The row above a `| --- |` delimiter names columns rather than an entity, and
// every table in the page is surveyed: a rewrite that keeps one table intact
// while emptying another is still a truncation.
func TestWikiWriteTableRowIdentitiesSkipsHeadersAndCoversEveryTable(t *testing.T) {
	content := "| \u59d3\u540d | \u8bc1\u4e66\u7f16\u53f7 |\n| --- | --- |\n| \u5f20\u4e09 | CERT-1 |\n\nprose\n\n" +
		"| \u9879\u76ee | \u91d1\u989d |\n| --- | --- |\n| \u5dee\u65c5 | 100 |\n| \u4f1a\u52a1 | 200 |\n"
	require.Equal(t, []string{"\u5f20\u4e09", "\u5dee\u65c5", "\u4f1a\u52a1"}, wikiWriteTableRowIdentities(content))

	rewritten := "| \u59d3\u540d | \u8bc1\u4e66\u7f16\u53f7 |\n| --- | --- |\n| \u5f20\u4e09 | CERT-1 |\n\nprose\n\n" +
		"| \u9879\u76ee | \u91d1\u989d |\n| --- | --- |\n| \u5dee\u65c5 | 100 |\n"
	require.Equal(t, []string{"\u4f1a\u52a1"}, wikiWriteMissingRowIdentities(content, rewritten))
}

// A row whose first cell is empty still names an entity in its next non-empty
// cell, so it is counted rather than silently ignored.
func TestWikiWriteRowIdentityFallsBackToTheNextNonEmptyCell(t *testing.T) {
	content := "| \u59d3\u540d | \u8bc1\u4e66\u7f16\u53f7 |\n| --- | --- |\n|  | CERT-1 |\n"
	require.Equal(t, []string{"cert-1"}, wikiWriteTableRowIdentities(content))
}

// prose that merely mentions a pipe is not a table, so it cannot be read as one
// losing rows.
func TestWikiWriteTableRowIdentitiesIgnoresLonePipesInProse(t *testing.T) {
	require.Empty(t, wikiWriteTableRowIdentities("a | b\n\nc"))
}

func newWikiWriteGuardHarness(t *testing.T) (interfaces.WikiPageService, *types.WikiPage) {
	t.Helper()
	return newWikiWriteGuardStore(t, ledger("PMP", "\u5f20\u4e09", "\u674e\u56db", "\u738b\u4e94"))
}

// newWikiWriteGuardStore builds the same isolated page store as
// newWikiWriteGuardHarness, with a caller-chosen stored body.
func newWikiWriteGuardStore(t *testing.T, content string) (interfaces.WikiPageService, *types.WikiPage) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}, &types.WikiPage{}, &types.WikiPageRevision{}))

	ctx := context.Background()
	svc := NewWikiPageService(repository.NewWikiPageRepository(db), nil, nil, nil, nil, nil)
	page, err := svc.CreatePage(ctx, &types.WikiPage{
		TenantID: 1, KnowledgeBaseID: "kb-row-guard", Slug: "entity/pmp",
		Title: "PMP", PageType: types.WikiPageTypeEntity,
		Content: content,
	})
	require.NoError(t, err)
	return svc, page
}

// ledgerHolders builds n distinct ledger identities.
func ledgerHolders(n int) []string {
	names := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		names = append(names, fmt.Sprintf("\u6301\u8bc1\u4eba%02d", i))
	}
	return names
}

// An ingest rewrite that loses more than the threshold is refused with an
// explicit signal, and the stored page survives the refusal untouched.
func TestUpdatePageRefusesAnIngestRewriteAboveTheLossLimit(t *testing.T) {
	// Four stored identities, one gone: 25% > 20%.
	svc, page := newWikiWriteGuardStore(t, ledger("PMP", ledgerHolders(4)...))
	ctx := context.Background()

	truncated := *page
	truncated.Content = ledger("PMP", ledgerHolders(4)[:3]...)
	kept, err := svc.UpdatePage(ctx, &truncated)
	require.ErrorIs(t, err, ErrWikiWriteDroppedTableRows)
	require.Nil(t, kept, "a refused write has no updated page to hand back")

	stored, err := svc.GetPageBySlug(ctx, "kb-row-guard", "entity/pmp")
	require.NoError(t, err)
	require.Equal(t, page.Content, stored.Content, "the stored body must survive the refusal")
	require.Equal(t, page.Version, stored.Version, "a refusal is not an edit")
}

// The tolerance is a strictly-greater-than limit: losing exactly one identity
// in five (20%) is written, and the version advances like any other edit.
func TestUpdatePageWritesAnIngestRewriteAtTheLossLimit(t *testing.T) {
	svc, page := newWikiWriteGuardStore(t, ledger("PMP", ledgerHolders(5)...))
	rewritten := *page
	rewritten.Content = ledger("PMP", ledgerHolders(5)[:4]...)

	updated, err := svc.UpdatePage(context.Background(), &rewritten)
	require.NoError(t, err)
	require.Equal(t, rewritten.Content, updated.Content)
	require.Equal(t, page.Version+1, updated.Version)
}

// Below the limit is written too: one identity gone out of ten is 10%.
func TestUpdatePageWritesAnIngestRewriteBelowTheLossLimit(t *testing.T) {
	svc, page := newWikiWriteGuardStore(t, ledger("PMP", ledgerHolders(10)...))
	rewritten := *page
	rewritten.Content = ledger("PMP", ledgerHolders(10)[:9]...)

	updated, err := svc.UpdatePage(context.Background(), &rewritten)
	require.NoError(t, err)
	require.Equal(t, rewritten.Content, updated.Content)
	require.Equal(t, page.Version+1, updated.Version)
}

// An ingest rewrite that keeps every stored identity while de-duplicating a
// repeated row is a normal write: identities are compared as a set, so merging
// the two Zhang San rows into one loses nothing. Under occurrence counting this was
// a refusal on every batch.
func TestUpdatePageWritesAnIngestRewriteThatMergesDuplicateRows(t *testing.T) {
	svc, page := newWikiWriteGuardStore(t, ledger("\u8bc1\u4e66", "\u5f20\u4e09", "\u5f20\u4e09", "\u674e\u56db",
		"\u738b\u4e94"))
	rewritten := *page
	rewritten.Content = ledger("\u8bc1\u4e66", "\u5f20\u4e09", "\u674e\u56db", "\u738b\u4e94")

	updated, err := svc.UpdatePage(context.Background(), &rewritten)
	require.NoError(t, err)
	require.Equal(t, rewritten.Content, updated.Content)
	require.Equal(t, page.Version+1, updated.Version)
}

// The ratio denominator counts distinct identities, not rows: three stored rows
// naming Zhang San are one identity, so losing Li Si from that page is a loss of 1/3,
// not of 1/5. Counting rows here would have let a whole subject disappear.
func TestUpdatePageWeighsDuplicateIdentitiesOnceInTheLossRatio(t *testing.T) {
	svc, page := newWikiWriteGuardStore(t, ledger("\u8bc1\u4e66", "\u5f20\u4e09", "\u5f20\u4e09", "\u5f20\u4e09",
		"\u674e\u56db", "\u738b\u4e94"))
	rewritten := *page
	rewritten.Content = ledger("\u8bc1\u4e66", "\u5f20\u4e09", "\u738b\u4e94")

	kept, err := svc.UpdatePage(context.Background(), &rewritten)
	require.ErrorIs(t, err, ErrWikiWriteDroppedTableRows)
	require.Nil(t, kept)

	stored, err := svc.GetPageBySlug(context.Background(), "kb-row-guard", "entity/pmp")
	require.NoError(t, err)
	require.Equal(t, page.Content, stored.Content)
}

// The agent's whole-page writer is told the write was refused, instead of being
// told it edited a page it never touched.
func TestUpdatePageReportsTheRefusalToTheAgentWriter(t *testing.T) {
	svc, page := newWikiWriteGuardHarness(t)
	ctx := types.WithWikiEditSource(context.Background(), types.WikiEditSourceAgent)

	truncated := *page
	truncated.Content = ledger("PMP", "\u5f20\u4e09")
	_, err := svc.UpdatePage(ctx, &truncated)
	require.ErrorIs(t, err, ErrWikiWriteDroppedTableRows)

	stored, err := svc.GetPageBySlug(context.Background(), "kb-row-guard", "entity/pmp")
	require.NoError(t, err)
	require.Equal(t, page.Content, stored.Content)
}

// The agent gets no tolerance at all: a rewrite that loses exactly the share
// ingest would accept is still refused, because the tool can hand the refusal
// back to the model and ask it to re-emit the page.
func TestUpdatePageRefusesForTheAgentAtTheIngestLossLimit(t *testing.T) {
	svc, page := newWikiWriteGuardStore(t, ledger("PMP", ledgerHolders(5)...))
	truncated := *page
	truncated.Content = ledger("PMP", ledgerHolders(5)[:4]...)

	// The agent's write loses 1 of 5 identities (exactly the ingest limit).
	_, err := svc.UpdatePage(
		types.WithWikiEditSource(context.Background(), types.WikiEditSourceAgent), &truncated)
	require.ErrorIs(t, err, ErrWikiWriteDroppedTableRows)

	// The same body is accepted when the ingest pipeline writes it.
	updated, err := svc.UpdatePage(context.Background(), &truncated)
	require.NoError(t, err)
	require.Equal(t, truncated.Content, updated.Content)
	require.Equal(t, page.Version+1, updated.Version)
}

// Deliberate shrinkages go through: a human deleting rows, a revert to an older
// (possibly shorter) revision, and a caller that marked the write.
func TestUpdatePageAllowsDeliberateShrinkage(t *testing.T) {
	cases := []struct {
		name string
		ctx  context.Context
	}{
		{"user edit", types.WithWikiEditSource(context.Background(), types.WikiEditSourceUser)},
		{"revert", types.WithWikiEditSource(context.Background(), types.WikiEditSourceRevert)},
		{"explicitly allowed", types.WithWikiShrinkAllowed(context.Background())},
		{"retract round", types.WithWikiShrinkAllowed(
			types.WithWikiEditSource(context.Background(), types.WikiEditSourcePipeline))},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			svc, page := newWikiWriteGuardHarness(t)
			truncated := *page
			truncated.Content = ledger("PMP", "\u5f20\u4e09")
			updated, err := svc.UpdatePage(testCase.ctx, &truncated)
			require.NoError(t, err)
			require.Equal(t, truncated.Content, updated.Content)
			require.Equal(t, page.Version+1, updated.Version)
		})
	}
}

// A rewrite that keeps every row but reflows the table is an ordinary edit.
func TestUpdatePageWritesARewriteThatKeepsEveryRow(t *testing.T) {
	svc, page := newWikiWriteGuardHarness(t)
	rewritten := *page
	rewritten.Content = ledger("PMP \u8bc1\u4e66", "\u674e\u56db", "**\u5f20\u4e09**", "\u738b\u4e94", "\u8d75\u516d")

	updated, err := svc.UpdatePage(context.Background(), &rewritten)
	require.NoError(t, err)
	require.Equal(t, rewritten.Content, updated.Content)
	require.Equal(t, page.Version+1, updated.Version)
}
