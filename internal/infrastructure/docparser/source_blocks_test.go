package docparser

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/docreader/proto"
	"github.com/jrmatherly/ai-kb/internal/types"
)

const minerUContentListJSON = "[\n {\"type\":\"text\",\"text\":\"8.14.11 \u7d22\u5939\u4e0e\u540a\u7d22\u65bd\u5de5\u5e94\u7b26\u5408\u4e0b\u5217\u89c4\u5b9a\uff1a\",\"text_level\":2,\"page_idx\":56,\"bbox\":[80,100,900,120]},\n {\"type\":\"text\",\"text\":\"1 \u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b\uff0c\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef\u3002\",\"page_idx\":56,\"bbox\":[80,130,920,170]},\n {\"type\":\"table\",\"table_body\":\"<table><tr><td>\u9ad8\u5ea6</td><td>12m</td></tr></table>\",\n  \"table_caption\":[\"\u8868 1\"],\"page_idx\":57,\"bbox\":[50,10,950,2000]}\n]"

func TestMinerUSourceBlocks(t *testing.T) {
	md := "## 8.14.11 \u7d22\u5939\u4e0e\u540a\u7d22\u65bd\u5de5\u5e94\u7b26\u5408\u4e0b\u5217\u89c4\u5b9a\uff1a\n\n1 \u5728\u6ee1\u8db3\u65bd\u5de5\u9700\u8981\u7684\u524d\u63d0\u4e0b\uff0c\u5e94\u51cf\u5c0f\u732b\u9053\u9762\u5c42\u5f00\u5b54\u9762\u79ef\u3002\n\n\u8868 1\n\n| \u9ad8\u5ea6 | 12m |\n| --- | --- |\n"
	for name, raw := range map[string][]byte{
		"array":  []byte(minerUContentListJSON),
		"string": mustJSON(t, minerUContentListJSON),
	} {
		t.Run(name, func(t *testing.T) {
			blocks := minerUSourceBlocks(md, raw, "pdf")
			require.Len(t, blocks, 3)
			assert.Equal(t, 57, blocks[0].Locator.Page)
			assert.InDeltaSlice(t, []float64{0.08, 0.1, 0.9, 0.12}, blocks[0].Locator.BBox, 1e-9)
			assert.Equal(t, 58, blocks[2].Locator.Page)
			assert.Nil(t, blocks[2].Locator.BBox, "out-of-range boxes are dropped")
		})
	}
	assert.Nil(t, minerUSourceBlocks(md, []byte(minerUContentListJSON), "docx"))
}

func TestMinerUContentListFromZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"doc/doc.md":                     "x",
		"doc/doc_content_list.json":      "[1]",
		"doc/auto/doc_content_list.json": "[2]",
	} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, _ = w.Write([]byte(body))
	}
	require.NoError(t, zw.Close())
	assert.Equal(t, "[1]", string(minerUContentListFromZip(buf.Bytes())))
}

func TestPaddleOCRVLPageUnits(t *testing.T) {
	var page paddleOCRVLPage
	require.NoError(t, json.Unmarshal([]byte("{\n\t\t\"markdown\":{\"text\":\"\u7b2c\u4e00\u6bb5\\n\\n\u7b2c\u4e8c\u6bb5\"},\n\t\t\"prunedResult\":{\"width\":1000,\"height\":2000,\"parsing_res_list\":[\n\t\t\t{\"block_label\":\"text\",\"block_content\":\"\u7b2c\u4e00\u6bb5\",\"block_bbox\":[100,200,900,400]}]}\n\t}"), &page))
	units := paddleOCRVLPageUnits(3, page)
	require.Len(t, units, 1)
	assert.Equal(t, []float64{0.1, 0.1, 0.9, 0.2}, units[0].Locator.BBox)
	assert.Equal(t, 3, units[0].Locator.Page)

	page.PrunedResult = nil
	units = paddleOCRVLPageUnits(4, page)
	require.Len(t, units, 2)
	assert.Nil(t, units[1].Locator.BBox)
	assert.True(t, strings.Contains(units[1].Text, "\u7b2c\u4e8c\u6bb5"))
}

func TestSourceBlocksFromProto(t *testing.T) {
	blocks := sourceBlocksFromProto([]*proto.SourceBlock{
		{Start: 0, End: 5, LocatorJson: `{"type":"pdf","page":2,"bbox":[0.1,0.2,0.3,0.4]}`},
		{Start: 5, End: 5, LocatorJson: `{"type":"pdf","page":2}`},
		{Start: 5, End: 9, LocatorJson: `not json`},
	})
	require.Len(t, blocks, 1)
	assert.Equal(t, types.SourceLocator{Type: "pdf", Page: 2, BBox: []float64{0.1, 0.2, 0.3, 0.4}}, blocks[0].Locator)
}

func mustJSON(t *testing.T, s string) []byte {
	t.Helper()
	b, err := json.Marshal(s)
	require.NoError(t, err)
	return b
}
