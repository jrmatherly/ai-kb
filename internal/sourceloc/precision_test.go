package sourceloc

import (
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func TestAlignNeverAttributesMissingBlockToPreviousPage(t *testing.T) {
	md := "\u7b2c\u4e00\u6bb5\u6b63\u786e\u6587\u672c\u3002\n\u7b2c\u4e8c\u6bb5\u5b9e\u9645\u5185\u5bb9\u6ca1\u6709" +
		"\u5339\u914d\u3002\n\u7b2c\u4e09\u6bb5\u6b63\u786e\u6587\u672c\u3002"
	units := []Unit{
		{Text: "\u7b2c\u4e00\u6bb5\u6b63\u786e\u6587\u672c", Locator: types.SourceLocator{Type: "pdf", Page: 1}},
		{Text: "\u7b2c\u4e8c\u6bb5OCR\u9519\u8bef\u5185\u5bb9", Locator: types.SourceLocator{Type: "pdf", Page: 2}},
		{Text: "\u7b2c\u4e09\u6bb5\u6b63\u786e\u6587\u672c", Locator: types.SourceLocator{Type: "pdf", Page: 3}},
	}
	index := NewIndex(md, Align(md, units))
	at := runeIndex(md, "\u7b2c\u4e8c\u6bb5")
	require.Empty(t, index.Locators(at, at+5))
	require.Equal(t, 3, index.Locators(runeIndex(md, "\u7b2c\u4e09\u6bb5"), len([]rune(md)))[0].Page)
}

func TestAlignRequiresFullUnitNotCommonPrefix(t *testing.T) {
	prefix := strings.Repeat("\u5171\u540c\u524d\u7f00", 12)
	require.Empty(t,
		Align(prefix+"\u9a8c\u6536\u4e0d\u5408\u683c",
			[]Unit{{
				Text: prefix + "\u9a8c\u6536\u5408\u683c",
				Locator: types.SourceLocator{
					Type: "pdf",
					Page: 1,
				},
			}}))
}

func TestAlignPreservesNumericMeaning(t *testing.T) {
	for _, pair := range [][2]string{{"\u6570\u503c15\u6beb\u7c73", "\u6570\u503c1.5\u6beb\u7c73"}, {"\u6e29\u5ea65" +
		"\u5ea6", "\u6e29\u5ea6-5\u5ea6"}, {"\u6bd4\u4f8b5", "\u6bd4\u4f8b5%"}, {"\u503c≤5", "\u503c≥5"}} {
		require.Empty(t,
			Align(pair[0],
				[]Unit{{
					Text: pair[1],
					Locator: types.SourceLocator{
						Type: "pdf",
						Page: 1,
					},
				}}),
			pair)
	}
}

func TestAlignRejectsUnaccountedDuplicate(t *testing.T) {
	require.Empty(t,
		Align("\u76ee\u5f55\uff1a\u8bbe\u5907\u9a8c\u6536\u5408\u683c\u3002\n\u6b63\u6587\uff1a\u8bbe\u5907\u9a8c"+
			"\u6536\u5408\u683c\u3002",
			[]Unit{{
				Text: "\u8bbe\u5907\u9a8c\u6536\u5408\u683c",
				Locator: types.SourceLocator{
					Type: "pdf",
					Page: 9,
				},
			}}))
	units := []Unit{
		{
			Text: "\u76f8\u540c\u6bb5\u843d\u5185\u5bb9",
			Locator: types.SourceLocator{
				Type: "pdf",
				Page: 1,
			},
		},
		{
			Text: "\u76f8\u540c\u6bb5\u843d\u5185\u5bb9",
			Locator: types.SourceLocator{
				Type: "pdf",
				Page: 2,
			},
		},
	}
	blocks := Align("\u76f8\u540c\u6bb5\u843d\u5185\u5bb9\n\u76f8\u540c\u6bb5\u843d\u5185\u5bb9", units)
	require.Len(t, blocks, 2)
	require.Equal(t, 2, blocks[1].Locator.Page)
}

func TestRemapNeverInterpolatesChangedText(t *testing.T) {
	old := "\u7b2c\u4e00\u6bb5\n\u5df2\u901a\u8fc7\u9a8c\u6536\n\u672b\u6bb5"
	blocks := []types.SourceBlock{{Start: 4, End: 9, Locator: types.SourceLocator{Type: "pdf", Page: 2}}}
	require.Empty(t, RemapBlocks(blocks, old, "\u7b2c\u4e00\u6bb5\n\u672a\u901a\u8fc7\u68c0\u67e5\n\u672b\u6bb5"))
}

func TestMergeKeepsDistinctExcerptsOfSameRegion(t *testing.T) {
	a := types.SourceLocator{Type: "pdf", Page: 2, BBox: []float64{0, 0, 1, 1}, Quote: "\u7b2c\u4e00\u53e5"}
	b := a
	b.Quote = "\u53e6\u4e00\u53e5"
	require.Len(t, MergeLocators(types.SourceLocators{a}, types.SourceLocators{a, b}), 2)
}

func TestLocatorsPreserveLongEvidenceAndAllRegions(t *testing.T) {
	text := strings.Repeat("\u5b8c\u6574\u539f\u6587", 100) + "\u672b\u5c3e\u5f15\u7528\u7684\u5173\u952e\u7ed3\u8bba"
	index := NewIndex(text,
		[]types.SourceBlock{{
			Start: 0,
			End:   len([]rune(text)),
			Locator: types.SourceLocator{
				Type: "pdf",
				Page: 1,
			},
		}})
	require.Equal(t, text, index.Locators(0, len([]rune(text)))[0].Quote)
	var blocks []types.SourceBlock
	for i := 0; i < 80; i++ {
		blocks = append(blocks,
			types.SourceBlock{
				Start: i,
				End:   i + 1,
				Locator: types.SourceLocator{
					Type: "pdf",
					Page: i + 1,
				},
			})
	}
	locs := NewIndex(strings.Repeat("\u6587", 80), blocks).Locators(0, 80)
	require.Len(t, locs, 80)
	require.Len(t, MergeLocators(locs[:40], locs[40:]), 80)
}

func TestFoldingCannotMixRevisionsOrMappingQuality(t *testing.T) {
	first := types.SourceLocator{Type: "pdf", Page: 1, Mapping: "exact", SourceHash: "original", Quote: "\u7b2c\u4e00" +
		"\u6bb5"}
	second := first
	second.Mapping = ""
	require.Len(t, AppendLocator(types.SourceLocators{first}, second), 2)
	second = first
	second.SourceHash = "replacement"
	require.Len(t, AppendLocator(types.SourceLocators{first}, second), 2)
}

func TestAlignRejectsPartialNumericValues(t *testing.T) {
	for _, pair := range [][2]string{{"\u589e\u957f\u4e3a5%", "\u589e\u957f\u4e3a5"}, {"115", "15"}, {"15.5", "1" +
		"5"}, {"-15", "15"}} {
		require.Empty(t,
			Align(pair[0], []Unit{{Text: pair[1], Locator: types.SourceLocator{Type: "pdf", Page: 1}}}),
			pair,
		)
	}
}

func TestChunkWithUnmappedEvidenceIsMarkedPartial(t *testing.T) {
	md := "\u5339\u914d\u7684\u539f\u6587\u3002\n\u672a\u5339\u914d\u7684\u5173\u952e\u7ed3\u8bba\u3002"
	index := NewIndex(md, Align(md, []Unit{{Text: "\u5339\u914d\u7684\u539f" +
		"\u6587", Locator: types.SourceLocator{Type: "pdf", Page: 1}}}))
	locs := index.Locators(0, len([]rune(md)))
	require.Len(t, locs, 1)
	require.True(t, locs[0].Partial)
	require.False(t, index.Locators(0, 5)[0].Partial)
}

func TestAlignmentDecodesTableEntitiesWithoutLosingOffsets(t *testing.T) {
	md := "<table><tr><td>\u9650\u503c</td><td>&lt;1.5</td></tr></table>"
	loc := types.SourceLocator{Type: "pdf", Page: 2}
	blocks := Align(md, []Unit{{Text: "\u9650\u503c <1.5", Locator: loc}})
	require.Len(t, blocks, 1)
	quotes := NewIndex(md, blocks).Locators(0, len([]rune(md)))
	require.False(t, quotes[0].Partial)
	require.Equal(t, "\u9650\u503c <1.5", quotes[0].Quote)
	remapped := RemapBlocks(blocks, md, "|\u9650\u503c|<1.5|")
	require.Len(t, remapped, 1)
	require.Equal(t, 2, remapped[0].Locator.Page)
}

func TestImagePlaceholderDoesNotInvalidateVerifiedTextLocators(t *testing.T) {
	md := "\u7b2c\u4e00\u6bb5\u5b8c\u6574\u7684\u539f\u6587\u3002\n\n![descript](resource://image)\n\n\u7b2c\u4e8c" +
		"\u6bb5\u5b8c\u6574\u7684\u539f\u6587\u3002"
	blocks := Align(md, []Unit{
		{Text: "\u7b2c\u4e00\u6bb5\u5b8c\u6574\u7684\u539f\u6587\u3002", Locator: types.SourceLocator{Type: "doc" +
			"x", Block: 1}},
		{Text: "\u7b2c\u4e8c\u6bb5\u5b8c\u6574\u7684\u539f\u6587\u3002", Locator: types.SourceLocator{Type: "doc" +
			"x", Block: 3}},
	})
	locs := NewIndex(md, blocks).Locators(0, len([]rune(md)))
	require.Len(t, locs, 2)
	for _, loc := range locs {
		require.False(t, loc.Partial)
	}
}
