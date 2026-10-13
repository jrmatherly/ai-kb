package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestLLMToolOutputsUseMarkdownImages(t *testing.T) {
	imageInfo, err := json.Marshal([]types.ImageInfo{
		{
			URL:     "resource://AbCdEfGhIjKlMnOpQrStUv",
			Caption: "\u76ee\u6807\u8bf4\u8bdd\u4eba\u63d0\u53d6\u6d41\u7a0b\u56fe",
			OCRText: "\u8f93\u5165\n\u8f93\u51fa",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	chunk := &types.Chunk{
		ID:         "chunk-1",
		ChunkIndex: 0,
		ChunkType:  types.ChunkTypeText,
		Content:    "\u6d4b\u8bd5\u9636\u6bb5\u4e3b\u8981\u6d41\u7a0b",
		ImageInfo:  string(imageInfo),
	}

	readOutput := (&ReadDocumentTool{}).buildOutput(
		&types.Knowledge{ID: "knowledge-1", Title: "\u6d4b\u8bd5\u6587\u6863"}, 1, []readChunkRow{{chunk: chunk}}, "",
	)
	enrichedOutput := enrichChunkContent(chunk)
	for name, output := range map[string]string{
		"read_document":        readOutput,
		"enrich_chunk_content": enrichedOutput,
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(output, "![\u76ee\u6807\u8bf4\u8bdd\u4eba\u63d0\u53d6\u6d41\u7a0b\u56fe](resource://AbCdEfGhIjKlMnOpQrStUv)") {
				t.Fatalf("expected Markdown image in tool output:\n%s", output)
			}
			if strings.Contains(output, "<image") || strings.Contains(output, "<caption>") || strings.Contains(output, "<ocr_text>") {
				t.Fatalf("tool output leaked legacy image XML:\n%s", output)
			}
		})
	}
}
