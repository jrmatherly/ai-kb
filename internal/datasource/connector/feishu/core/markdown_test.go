package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// txt builds a text-bearing BlockText from plain content.
func txt(s string) *BlockText {
	return &BlockText{Elements: []TextElement{{TextRun: &TextRun{Content: s}}}}
}

type fakeReader struct {
	sheet            [][]string
	sheetTruncated   bool
	sheetErr         error
	bitable          [][]string
	bitableTruncated bool
	bitableErr       error
}

func (f fakeReader) readSheetRange(_ context.Context, _ string) ([][]string, bool, error) {
	return f.sheet, f.sheetTruncated, f.sheetErr
}

func (f fakeReader) readBitableRecords(_ context.Context, _ string) ([][]string, bool, error) {
	return f.bitable, f.bitableTruncated, f.bitableErr
}

func TestBlocksToMarkdown_EmbeddedSheetAndFile(t *testing.T) {
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		{BlockID: "s", BlockType: BlockTypeSheet, Sheet: &BlockTokenRef{Token: "sht_a_0"}},
		{BlockID: "img", BlockType: BlockTypeImage, Image: &BlockTokenRef{Token: "img_t"}},
		{BlockID: "f", BlockType: BlockTypeFile, File: &BlockFileRef{Token: "file_t", Name: "\u62a5\u8868.pdf"}},
	}
	fr := fakeReader{sheet: [][]string{{"\u540d\u79f0", "\u6570\u91cf"}, {"\u82f9\u679c", "3"}}}
	md, atts, err := blocksToMarkdown(context.Background(), fr, blocks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(md), "| \u540d\u79f0 | \u6570\u91cf |") || !strings.Contains(string(md), "| \u82f9\u679c | 3 |") {
		t.Errorf("sheet not inlined:\n%s", md)
	}
	if !strings.Contains(string(md), "![image]()") {
		t.Errorf("image placeholder missing:\n%s", md)
	}
	if strings.Contains(string(md), "feishu-media") {
		t.Errorf("internal media token leaked into markdown:\n%s", md)
	}
	if len(atts) != 1 || atts[0].FileToken != "file_t" || atts[0].Name != "\u62a5\u8868.pdf" {
		t.Errorf("attachments = %+v", atts)
	}
	if !strings.Contains(string(md), "📎 Attachment: \u62a5\u8868.pdf") {
		t.Errorf("attachment inline reference missing:\n%s", md)
	}
}

func TestBlocksToMarkdown_SheetTruncatedNote(t *testing.T) {
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		{BlockID: "s", BlockType: BlockTypeSheet, Sheet: &BlockTokenRef{Token: "sht_a_0"}},
	}
	fr := fakeReader{sheet: [][]string{{"h"}, {"1"}}, sheetTruncated: true}
	md, _, err := blocksToMarkdown(context.Background(), fr, blocks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(md), "Table truncated") {
		t.Errorf("want truncation note, got:\n%s", md)
	}
}

func TestBlocksToMarkdown_SheetPermissionDegrades(t *testing.T) {
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		{BlockID: "s", BlockType: BlockTypeSheet, Sheet: &BlockTokenRef{Token: "sht_a_0"}},
	}
	fr := fakeReader{sheetErr: fmt.Errorf("code=99991672 permission denied")}
	md, _, err := blocksToMarkdown(context.Background(), fr, blocks)
	if err != nil {
		t.Fatalf("should not fail on permission error, got %v", err)
	}
	if !strings.Contains(string(md), "Cannot read embedded spreadsheet") {
		t.Errorf("want degraded placeholder, got:\n%s", md)
	}
}

func TestBlocksToMarkdown_BitableInlinedAndDegrades(t *testing.T) {
	mk := func(fr fakeReader) string {
		blocks := []DocxBlock{
			{BlockID: "root", BlockType: BlockTypePage},
			{BlockID: "bt", BlockType: BlockTypeBitable, Bitable: &BlockTokenRef{Token: "bascabc_tblxyz"}},
		}
		md, _, err := blocksToMarkdown(context.Background(), fr, blocks)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		return string(md)
	}

	inlined := mk(fakeReader{bitable: [][]string{{"\u4efb\u52a1", "\u72b6\u6001"}, {"\u5199\u6587\u6863", "\u8fdb\u884c\u4e2d"}}})
	if !strings.Contains(inlined, "| \u4efb\u52a1 | \u72b6\u6001 |") || !strings.Contains(inlined, "| \u5199\u6587\u6863 | \u8fdb\u884c\u4e2d |") {
		t.Errorf("bitable not inlined:\n%s", inlined)
	}

	degraded := mk(fakeReader{bitableErr: fmt.Errorf("code=99991672 permission denied")})
	if !strings.Contains(degraded, "Cannot read embedded Base table") {
		t.Errorf("want bitable degradation note, got:\n%s", degraded)
	}
}

func TestBlocksToMarkdown_NativeTableFromCellChildren(t *testing.T) {
	// Real Feishu shape: a table_cell (block_type 32) is a container whose text
	// lives in child text blocks, not on the cell. The renderer must read cell
	// text from those children AND must not also Emit them as loose paragraphs.
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		tableBlk("t", 2, "c1", "c2", "c3", "c4"),
		cellBlk("c1"), cellBlk("c2"), cellBlk("c3"), cellBlk("c4"),
		cellTextBlk("c1", "\u59d3\u540d"), cellTextBlk("c2", "\u5206\u6570"),
		cellTextBlk("c3", "\u5f20\u4e09"), cellTextBlk("c4", "95"),
	}
	md, _, err := blocksToMarkdown(context.Background(), nil, blocks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	s := string(md)
	if !strings.Contains(s, "| \u59d3\u540d | \u5206\u6570 |") || !strings.Contains(s, "| \u5f20\u4e09 | 95 |") {
		t.Errorf("native table not rendered from cell children:\n%s", s)
	}
	// Cell text must appear ONLY inside the table, never as a stray paragraph.
	if n := strings.Count(s, "\u59d3\u540d"); n != 1 {
		t.Errorf("cell text leaked outside the table (count=%d, want 1):\n%s", n, s)
	}
}

func TestBlocksToMarkdown_AttachmentInsideTableCellStillCollected(t *testing.T) {
	// A file block nested inside a table cell must still be collected as an
	// attachment — the table "consumed" set marks a cell's text structure but
	// must NOT swallow attachment/media blocks, or embedded files silently vanish.
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		tableBlk("t", 1, "c1"),
		{BlockID: "c1", BlockType: BlockTypeTableCell, Children: []string{"f1"}},
		{BlockID: "f1", BlockType: BlockTypeFile, File: &BlockFileRef{Token: "tok-in-cell", Name: "\u5185\u5d4c.pdf"}},
	}
	_, atts, err := blocksToMarkdown(context.Background(), nil, blocks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(atts) != 1 || atts[0].FileToken != "tok-in-cell" {
		t.Fatalf("attachment nested in a table cell was not collected: %+v", atts)
	}
}

func TestMarkdownTable_RaggedRowClampedToHeader(t *testing.T) {
	// Header declares 2 columns; a data row with 4 cells (trailing populated
	// cells beyond the header range) must be clamped to 2, or the emitted table
	// has more body cells than header/separator columns → malformed GFM.
	rows := [][]string{
		{"\u540d\u79f0", "\u6570\u91cf"},
		{"\u82f9\u679c", "3", "\u591a\u4f591", "\u591a\u4f592"},
	}
	out := markdownTable(rows)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines (header, separator, 1 row), got %d:\n%s", len(lines), out)
	}
	// Every row must have the same pipe count as the 2-column header.
	want := strings.Count(lines[0], "|")
	for i, ln := range lines {
		if got := strings.Count(ln, "|"); got != want {
			t.Errorf("line %d %q has %d pipes, want %d (ragged row not clamped)", i, ln, got, want)
		}
	}
	if strings.Contains(out, "\u591a\u4f59") {
		t.Errorf("overflow cells must be dropped, got:\n%s", out)
	}
}

func TestMarkdownTable_ZeroColumnRendersNothing(t *testing.T) {
	// An embedded sheet/bitable with a header row but no columns (e.g. a bitable
	// whose fields were all deleted) yields rows == [][]string{{}}. len(rows)==1
	// passes a naive empty guard, but cols==0 would Emit a malformed GFM table
	// ("|  |" header + a bare "|" separator). It must render nothing instead.
	for _, rows := range [][][]string{
		{{}},         // one empty header row, no data
		{{}, {}, {}}, // empty header + empty data rows
	} {
		out := markdownTable(rows)
		if out != "" {
			t.Errorf("zero-column table must render nothing, got %q for rows=%+v", out, rows)
		}
	}
}

func TestBlocksToMarkdown_UnrenderableTablePreservesCellText(t *testing.T) {
	// A native table block with no column property (Table!=nil, Property==nil)
	// cannot be rendered as a Markdown table. Its cells must NOT be consumed, so
	// their text still reaches the output as loose paragraphs rather than
	// vanishing entirely.
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		// tableBlk sets Property (renderable); here we want an UNrenderable one.
		{BlockID: "t", BlockType: BlockTypeTable, Table: &BlockTable{Cells: []string{"c1"}}},
		{BlockID: "c1", BlockType: BlockTypeTableCell, Children: []string{"c1_txt"}},
		cellTextBlk("c1", "\u91cd\u8981\u5185\u5bb9"),
	}
	md, _, err := blocksToMarkdown(context.Background(), nil, blocks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(md), "\u91cd\u8981\u5185\u5bb9") {
		t.Errorf("cell text of an unrenderable table was dropped:\n%s", md)
	}
}

func TestBlocksToMarkdown_TextConstructs(t *testing.T) {
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage, Children: []string{"h", "p", "b"}},
		{BlockID: "h", BlockType: BlockTypeHeading1, Heading1: txt("\u6807\u9898")},
		{BlockID: "p", BlockType: BlockTypeText, Text: txt("\u4e00\u6bb5\u6b63\u6587")},
		{BlockID: "b", BlockType: BlockTypeBullet, Bullet: txt("\u8981\u70b9")},
	}
	md, atts, err := blocksToMarkdown(context.Background(), nil, blocks)
	if err != nil {
		t.Fatalf("blocksToMarkdown: %v", err)
	}
	if len(atts) != 0 {
		t.Errorf("want 0 attachments, got %d", len(atts))
	}
	want := "# \u6807\u9898\n\n\u4e00\u6bb5\u6b63\u6587\n\n- \u8981\u70b9\n"
	if string(md) != want {
		t.Errorf("md =\n%q\nwant\n%q", md, want)
	}
}

func TestBlocksToMarkdown_BlankDocRendersEmpty(t *testing.T) {
	// A page with no renderable children (or only empty-text/container blocks)
	// must render to empty Markdown. FetchDocxWithBlocks relies on this: an empty
	// render triggers the export fallback instead of emitting a content-less main
	// item that would wrongly ingest the login-gated wiki URL. Any File/Image
	// block writes a placeholder, so a truly empty render also implies no
	// attachment/image downdrill is lost by that fallback.
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage, Children: []string{"p"}},
		{BlockID: "p", BlockType: BlockTypeText, Text: txt("")},
	}
	md, atts, err := blocksToMarkdown(context.Background(), nil, blocks)
	if err != nil {
		t.Fatalf("blocksToMarkdown: %v", err)
	}
	if strings.TrimSpace(string(md)) != "" {
		t.Errorf("blank doc should render empty Markdown, got:\n%q", md)
	}
	if len(atts) != 0 {
		t.Errorf("blank doc should collect no attachments, got %d", len(atts))
	}
}

func TestBlocksToMarkdown_TodoAndCallout(t *testing.T) {
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		{BlockID: "t", BlockType: BlockTypeTodo, Todo: txt("\u4e70\u725b\u5976")},
		{BlockID: "c", BlockType: BlockTypeCallout, Callout: txt("\u6ce8\u610f\u4e8b\u9879")},
	}
	md, _, err := blocksToMarkdown(context.Background(), nil, blocks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(string(md), "- [ ] \u4e70\u725b\u5976") {
		t.Errorf("todo not rendered:\n%s", md)
	}
	if !strings.Contains(string(md), "> \u6ce8\u610f\u4e8b\u9879") {
		t.Errorf("callout direct text not rendered:\n%s", md)
	}
}

func TestBlocksToMarkdown_CalloutContainerNoOp(t *testing.T) {
	// A callout with no direct text (container form) must Emit nothing itself.
	blocks := []DocxBlock{
		{BlockID: "root", BlockType: BlockTypePage},
		{BlockID: "c", BlockType: BlockTypeCallout},
	}
	md, _, err := blocksToMarkdown(context.Background(), nil, blocks)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(string(md)) != "" {
		t.Errorf("empty callout should Emit nothing, got:\n%q", md)
	}
}
