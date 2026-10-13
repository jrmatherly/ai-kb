package sourceloc

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func runeIndex(s, sub string) int {
	i := strings.Index(s, sub)
	if i < 0 {
		return -1
	}
	return len([]rune(s[:i]))
}

func TestRemapperFollowsRewrites(t *testing.T) {
	oldText := "\u7b2c\u4e00\u6bb5\r\n\r\n<table><tr><td>a</td></tr></table>\r\n\r\n![img](images/x.png)\r\n\r\n" +
		"\u6700\u540e\u4e00\u6bb5\u6587\u5b57"
	newText := "\u7b2c\u4e00\u6bb5\n\n| a |\n| --- |\n\n![img](https://cdn.example/x.png)\n\n\u6700\u540e\u4e00\u6bb5" +
		"\u6587\u5b57"
	r := NewRemapper(oldText, newText)

	assert.Equal(t, 0, r.Map(0))
	assert.Equal(t, runeIndex(newText, "\u6700\u540e"), r.Map(runeIndex(oldText, "\u6700\u540e")))
	assert.Equal(t, runeIndex(newText, "\u6bb5\u6587\u5b57")+1, r.Map(runeIndex(oldText, "\u6bb5\u6587\u5b57")+1))
	assert.Equal(t, len([]rune(newText)), r.Map(len([]rune(oldText))))

	// Offsets inside the rewritten table stay inside the rewritten table.
	tableOld := runeIndex(oldText, "<table>")
	mapped := r.Map(tableOld + 5)
	assert.GreaterOrEqual(t, mapped, runeIndex(newText, "| a |"))
	assert.Less(t, mapped, runeIndex(newText, "![img]"))
}

func TestRemapBlocksDropsCollapsedBlocks(t *testing.T) {
	blocks := []types.SourceBlock{{Start: 0, End: 3, Locator: types.SourceLocator{Type: "pdf", Page: 1}}}
	out := RemapBlocks(blocks, "abc\ndef", "abc\ndef")
	require.Len(t, out, 1)
	assert.Equal(t, 3, out[0].End)
}

// A scanned PDF is only page images and blank lines, so no line survives the
// image URL rewrite; each page must still land on its own line.
func TestRemapBlocksKeepsRewrittenLinesOnTheirPages(t *testing.T) {
	var oldLines, newLines []string
	var blocks []types.SourceBlock
	offset := 0
	for page := 1; page <= 300; page++ {
		line := fmt.Sprintf("![doc_page_%d.jpg](images/doc_page_%d.jpg)", page, page)
		if page > 1 {
			offset += 2
		}
		blocks = append(blocks, types.SourceBlock{
			Start: offset, End: offset + len(line),
			Locator: types.SourceLocator{Type: types.SourceLocatorPDF, Page: page},
		})
		offset += len(line)
		oldLines = append(oldLines, line)
		newLines = append(newLines, fmt.Sprintf("![doc_page_%d.jpg](local://1/images/0123456789abcdef.jpg)", page))
	}
	oldText, newText := strings.Join(oldLines, "\n\n"), strings.Join(newLines, "\n\n")
	idx := NewIndex(newText, RemapBlocks(blocks, oldText, newText))
	for i, line := range newLines {
		at := strings.Index(newText, line) + strings.Index(line, "local://")
		locs := idx.LocatorsAt(utf8.RuneCountInString(newText[:at]))
		require.Len(t, locs, 1, "page %d", i+1)
		assert.Equal(t, i+1, locs[0].Page)
	}
}

func TestAlignPlacesUnitsInOrder(t *testing.T) {
	md := "# \u516c\u8def\u5de5\u7a0b\n\n8.14.11 \u7d22\u5939\u4e0e\u540a\u7d22\u65bd\u5de5\u5e94\u7b26\u5408\u4e0b" +
		"\u5217\u89c4\u5b9a\uff1a\n\n1. \u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b\uff0c" +
		"\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef\u3002\n\n" +
		"![\u56fe](data:image/png;base64,QUJDREVGR0g=)\n\n| \u5217 | \u503c |\n| --- | --- |\n| \u9ad8\u5ea6 | 12m |\n"
	units := []Unit{
		{Text: "\u516c\u8def\u5de5\u7a0b", Locator: types.SourceLocator{Type: "docx", Block: 1}},
		{Text: "8.14.11 \u7d22\u5939\u4e0e\u540a\u7d22\u65bd\u5de5\u5e94\u7b26\u5408\u4e0b\u5217\u89c4\u5b9a" +
			":", Locator: types.SourceLocator{Type: "docx", Block: 2}},
		{Text: "\u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b,\u5e94\u51cf\u5c0f\u732b\u9053" +
			"\u9762\u5c42\u5f00\u5b54\u9762\u79ef\u3002", Locator: types.SourceLocator{Type: "docx", Block: 3}},
		{Text: "\u4e0d\u5b58\u5728\u7684\u6bb5\u843d\u5185\u5bb9\u4e0d\u4f1a\u88ab\u653e" +
			"\u7f6e", Locator: types.SourceLocator{Type: "docx", Block: 4}},
		{Text: "\u5217 \u503c", Locator: types.SourceLocator{Type: "docx", Block: 5}},
		{Text: "\u9ad8\u5ea6 12m", Locator: types.SourceLocator{Type: "docx", Block: 5}},
	}
	blocks := Align(md, units)
	require.Len(t, blocks, 5)
	assert.Equal(t, 1, blocks[0].Locator.Block)
	assert.Equal(t, runeIndex(md, "8.14.11"), blocks[1].Start)
	assert.Equal(t, blocks[1].End, blocks[2].Start)
	assert.Equal(t, runeIndex(md, "1. \u5728\u6ee1\u8db3"), blocks[2].Start, "list numbering stays with its paragraph")
	assert.Equal(t, 5, blocks[4].Locator.Block)
	assert.Equal(t, len([]rune(md)), blocks[4].End)
}

func TestIndexLocatorsFoldsAndQuotes(t *testing.T) {
	md := "\u7b2c\u4e00\u9875\u6b63\u6587\u3002\n\n\u7b2c\u4e8c\u9875\u6b63\u6587\u5f00\u59cb\n\n\u7b2c\u4e8c\u9875" +
		"\u6b63\u6587\u7ee7\u7eed\n\n\u7b2c\u4e09\u9875"
	p1 := runeIndex(md, "\u7b2c\u4e8c\u9875\u6b63\u6587\u5f00\u59cb")
	p2 := runeIndex(md, "\u7b2c\u4e8c\u9875\u6b63\u6587\u7ee7\u7eed")
	p3 := runeIndex(md, "\u7b2c\u4e09\u9875")
	blocks := []types.SourceBlock{
		{Start: 0, End: p1, Locator: types.SourceLocator{Type: "pdf", Page: 1}},
		{Start: p1, End: p2, Locator: types.SourceLocator{Type: "pdf", Page: 2}},
		{Start: p2, End: p3, Locator: types.SourceLocator{Type: "pdf", Page: 2}},
		{Start: p3, End: len([]rune(md)), Locator: types.SourceLocator{
			Type: "pdf", Page: 3, BBox: []float64{0.1, 0.1, 0.9, 0.2},
		}},
	}
	idx := NewIndex(md, blocks)
	locs := idx.Locators(p1+2, len([]rune(md)))
	require.Len(t, locs, 2)
	assert.Equal(t, 2, locs[0].Page)
	assert.Equal(t, "\u9875\u6b63\u6587\u5f00\u59cb \u7b2c\u4e8c\u9875\u6b63\u6587\u7ee7\u7eed", locs[0].Quote)
	assert.Equal(t, 3, locs[1].Page)
	assert.Equal(t, []float64{0.1, 0.1, 0.9, 0.2}, locs[1].BBox)
	assert.Nil(t, idx.Locators(5, 5))
}

func TestAppendLocatorMergesSheetRows(t *testing.T) {
	var list types.SourceLocators
	list = AppendLocator(list, types.SourceLocator{Type: "sheet", Sheet: "S1", RowStart: 3, RowEnd: 3, Quote: "a"})
	list = AppendLocator(list, types.SourceLocator{Type: "sheet", Sheet: "S1", RowStart: 4, RowEnd: 4, Quote: "b"})
	list = AppendLocator(list, types.SourceLocator{Type: "sheet", Sheet: "S1", RowStart: 9, RowEnd: 9, Quote: "c"})
	require.Len(t, list, 2)
	assert.Equal(t, 3, list[0].RowStart)
	assert.Equal(t, 4, list[0].RowEnd)
	assert.Equal(t, "a b", list[0].Quote)
}

func TestMergeLocatorsDedupes(t *testing.T) {
	a := types.SourceLocators{{Type: "slide", Slide: 1}}
	b := types.SourceLocators{{Type: "slide", Slide: 1}, {Type: "slide", Slide: 2}}
	assert.Len(t, MergeLocators(a, b), 2)
}

func TestCleanQuote(t *testing.T) {
	got := CleanQuote("\u89c1 ![\u56fe1](https://x/y.png) \u548c [\u94fe\u63a5](https://a.b)\n<b>\u52a0\u7c97</b>  " +
		"<!-- Slide number: 3 -->")
	assert.Equal(t, "\u89c1 \u548c \u94fe\u63a5 \u52a0\u7c97", got)
}

func TestTextBlocks(t *testing.T) {
	content := "# \u6807\u9898\n\n\u7b2c\u4e00\u6bb5\n\u7b2c\u4e00\u6bb5\u7eed\n\n\n\u7b2c\u4e8c\u6bb5"
	blocks := TextBlocks(content)
	require.Len(t, blocks, 3)
	assert.Equal(t, 0, blocks[0].Start)
	assert.Equal(t, runeIndex(content, "\u7b2c\u4e00\u6bb5"), blocks[1].Start)
	assert.Equal(t, runeIndex(content, "\u7b2c\u4e8c\u6bb5"), blocks[2].Start)
	assert.Equal(t, len([]rune(content)), blocks[2].End)
	assert.Equal(t, blocks[1].Start, blocks[1].Locator.Start)
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestDocxUnits(t *testing.T) {
	doc := "<?xml version=\"1.0\"?>\n<w:document xmlns:w=\"http://schemas.openxmlformats.org/wordprocessingml/2006/ma" +
		"in\"><w:body>\n<w:p><w:r><w:t>\u6807\u9898</w:t></w:r></w:p>\n<w:sdt><w:sdtPr><w:alias " +
		"w:val=\"x\"/></w:sdtPr><w:sdtContent>\n<w:p><w:r><w:t>\u63a7\u4ef6\u5185</w:t></w:r><w:r><w:delText>\u5220" +
		"\u9664</w:delText></w:r></w:p>\n</w:sdtContent></w:sdt>\n<w:tbl><w:tr><w:tc><w:p><w:r><w:t>A1</w:t></w:r></w" +
		":p></w:tc><w:tc><w:p><w:r><w:t>B1</w:t></w:r></w:p></w:tc></w:tr>\n<w:tr><w:tc><w:p><w:r><w:t>A2</w:t></w:r>" +
		"</w:p></w:tc></w:tr></w:tbl>\n<w:p/>\n<w:p><w:r><w:t xml:space=\"preserve\">\u672b\u6bb5 " +
		"</w:t></w:r><w:r><w:tab/><w:t>\u6587\u5b57</w:t></w:r></w:p>\n<w:sectPr/></w:body></w:document>"
	units, err := docxUnits(zipOf(t, map[string]string{"word/document.xml": doc}))
	require.NoError(t, err)
	var got []string
	for _, u := range units {
		got = append(got, u.Text+"@"+string(rune('0'+u.Locator.Block)))
	}
	assert.Equal(t, []string{"\u6807\u9898@1", "\u63a7\u4ef6\u5185@2", "A1 B1@3", "A2@3", "\u672b\u6bb5  " +
		"\u6587\u5b57@5"}, got)
}

func TestPptxUnits(t *testing.T) {
	const (
		pNS = `xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
		rNS = `xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
	)
	files := map[string]string{
		"ppt/presentation.xml": `<p:presentation ` + pNS + ` ` + rNS + `><p:sldIdLst>` +
			`<p:sldId id="256" r:id="rId3"/><p:sldId id="257" r:id="rId2"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships>` +
			`<Relationship Id="rId2" Type="http://x/slide" Target="slides/slide1.xml"/>` +
			`<Relationship Id="rId3" Type="http://x/slide" Target="slides/slide2.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": "<p:sld xmlns:a=\"a\"><a:p><a:r><a:t>\u7b2c\u4e8c\u5f20</a:t></a:r></a:p></p:sld>",
		"ppt/slides/slide2.xml": "<p:sld xmlns:a=\"a\"><a:p><a:r><a:t>\u5c01\u9762</a:t></a:r>" +
			"<a:r><a:t>\u6807\u9898</a:t></a:r></a:p><a:p/></p:sld>",
		"ppt/slides/_rels/slide2.xml.rels": `<Relationships><Relationship Id="rId1" ` +
			`Type="http://x/notesSlide" Target="../notesSlides/notesSlide1.xml"/></Relationships>`,
		"ppt/notesSlides/notesSlide1.xml": "<p:notes xmlns:a=\"a\"><a:p><a:r><a:t>\u5907\u6ce8</a:t></a:r></a:p></p:n" +
			"otes>",
	}
	units, err := pptxUnits(zipOf(t, files))
	require.NoError(t, err)
	require.Len(t, units, 3)
	assert.Equal(t, "\u5c01\u9762\u6807\u9898", units[0].Text)
	assert.Equal(t, 1, units[0].Locator.Slide)
	assert.Equal(t, "\u5907\u6ce8", units[1].Text)
	assert.Equal(t, 1, units[1].Locator.Slide)
	assert.Equal(t, 2, units[2].Locator.Slide)
}

func TestXlsxUnits(t *testing.T) {
	f := excelize.NewFile()
	require.NoError(t, f.SetCellValue("Sheet1", "A1", "\u540d\u79f0"))
	require.NoError(t, f.SetCellValue("Sheet1", "B1", "\u6570\u91cf"))
	require.NoError(t, f.SetCellValue("Sheet1", "A3", "\u87ba\u6813"))
	require.NoError(t, f.SetCellValue("Sheet1", "B3", 12))
	_, err := f.NewSheet("\u6c47\u603b")
	require.NoError(t, err)
	require.NoError(t, f.SetCellValue("\u6c47\u603b", "A2", "\u5408\u8ba1"))
	buf, err := f.WriteToBuffer()
	require.NoError(t, err)

	units, err := xlsxUnits(buf.Bytes())
	require.NoError(t, err)
	require.Len(t, units, 3)
	assert.Equal(t, "\u87ba\u6813 12", units[1].Text)
	assert.Equal(t, 3, units[1].Locator.RowStart)
	assert.Equal(t, "\u6c47\u603b", units[2].Locator.Sheet)
	assert.Equal(t, 2, units[2].Locator.RowStart)

	md := "## Sheet1\n| \u540d\u79f0 | \u6570\u91cf |\n| --- | --- |\n| \u87ba\u6813 | 12 |\n\n## \u6c47\u603b\n| " +
		"\u5408\u8ba1 |\n"
	blocks := Align(md, units)
	require.Len(t, blocks, 3)
	assert.Equal(t, runeIndex(md, "| \u87ba\u6813"), blocks[1].Start, "a table row starts at its line")
}

func TestEpubUnits(t *testing.T) {
	files := map[string]string{
		"META-INF/container.xml": `<container><rootfiles>` +
			`<rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf": `<package><manifest><item id="c1" href="ch1.xhtml"/>` +
			`<item id="c2" href="text/ch2.xhtml"/></manifest>` +
			`<spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`,
		"OEBPS/ch1.xhtml": "<html><head><title>x</title></head><body><h1>\u7b2c\u4e00\u7ae0</h1><p>\u5f00\u7bc7<" +
			"b>\u6b63\u6587</b></p></body></html>",
		"OEBPS/text/ch2.xhtml": "<html><body><h2>\u7b2c\u4e8c\u7ae0</h2><p>\u540e\u6587</p><script>var " +
			"a=1</script></body></html>",
	}
	units, err := epubUnits(zipOf(t, files))
	require.NoError(t, err)
	require.Len(t, units, 4)
	assert.Equal(t, "\u5f00\u7bc7 \u6b63\u6587", units[1].Text)
	assert.Equal(t, "\u7b2c\u4e00\u7ae0", units[1].Locator.Title)
	assert.Equal(t, 2, units[3].Locator.Section)
}

func TestCSVUnits(t *testing.T) {
	units, err := csvUnits([]byte("name,qty\nbolt,12\n\nnut,3\n"))
	require.NoError(t, err)
	require.Len(t, units, 3)
	assert.Equal(t, 3, units[2].Locator.RowStart)
}
