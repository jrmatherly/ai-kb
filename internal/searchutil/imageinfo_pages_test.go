package searchutil

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func pageChild(id, parentID string, chunkType types.ChunkType, page int, content string) *types.Chunk {
	return &types.Chunk{
		ID:             id,
		ParentChunkID:  parentID,
		ChunkType:      chunkType,
		IsEnabled:      true,
		Content:        content,
		SourceLocators: types.SourceLocators{{Type: types.SourceLocatorPDF, Page: page}},
	}
}

// A scanned PDF chunk holds only page images, so its page locators have no
// text to match a cited sentence against; the pages' OCR text supplies it.
func TestEnrichSearchResultsGivesScannedPagesTheirOCRText(t *testing.T) {
	repo := childrenByParentRepo{byParent: map[string][]*types.Chunk{
		"scan": {
			pageChild("cap-1", "scan", types.ChunkTypeImageCaption, 1, "A form about working at height"),
			pageChild("ocr-1", "scan", types.ChunkTypeImageOCR, 1, "\u4e00\u3001\u4f5c\u4e1a\u4eba\u5458\u5fc5\u987b\u6301\u8bc1\u4e0a\u5c97\u3002"),
			pageChild("ocr-2", "scan", types.ChunkTypeImageOCR, 2, "\u4e94\u3001\u540a\u7bee\u6bcf\u73ed\u4f5c\u4e1a\u524d\u5e94\u8fdb\u884c\u7a7a\u8f7d\u8bd5\u8fd0\u884c\u3002"),
		},
	}}
	boxed := types.SourceLocators{{Type: types.SourceLocatorPDF, Page: 3, BBox: []float64{0, 0, 1, 1}, Quote: "text"}}
	results := []*types.SearchResult{
		{ID: "scan", SourceLocators: types.SourceLocators{
			{Type: types.SourceLocatorPDF, Page: 1, Quote: "![page_1.jpg](resource://a"},
			{Type: types.SourceLocatorPDF, Page: 2},
		}},
		{ID: "text", SourceLocators: boxed},
	}

	EnrichSearchResultsImageInfo(context.Background(), repo, 1, results)

	got := results[0].SourceLocators
	if got[0].Quote != "\u4e00\u3001\u4f5c\u4e1a\u4eba\u5458\u5fc5\u987b\u6301\u8bc1\u4e0a\u5c97\u3002" || got[1].Quote != "\u4e94\u3001\u540a\u7bee\u6bcf\u73ed\u4f5c\u4e1a\u524d\u5e94\u8fdb\u884c\u7a7a\u8f7d\u8bd5\u8fd0\u884c\u3002" {
		t.Fatalf("page quotes = %q, %q", got[0].Quote, got[1].Quote)
	}
	if results[1].SourceLocators[0].Quote != "text" {
		t.Fatalf("a boxed text locator must keep its quote, got %q", results[1].SourceLocators[0].Quote)
	}
}

// A text page's chunk that already has its image info is not re-queried just
// because an embedded figure left a textless page locator.
func TestEnrichSearchResultsSkipsTextPagesWithImageInfo(t *testing.T) {
	results := []*types.SearchResult{{
		ID:        "text",
		ImageInfo: `[{"url":"local://fig.png"}]`,
		SourceLocators: types.SourceLocators{
			{Type: types.SourceLocatorPDF, Page: 2, BBox: []float64{0, 0, 1, 0.5}, Quote: "body text"},
			{Type: types.SourceLocatorPDF, Page: 2},
		},
	}}
	enrichSearchResultsImageInfo(results, func(parentIDs []string) ([]*types.Chunk, error) {
		t.Fatalf("unexpected child lookup for %v", parentIDs)
		return nil, nil
	})
}
