package docparser

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

// Contract fixture for DocVortex MiddleJson 2.0. Bboxes are normalized
// fractions, page indices are original source indices, and captions are
// child blocks. This is deliberately not a legacy content_list payload.
const middleFixture = "{\n  \"schema\": \"docvortex.middle\",\n  \"schema_version\": \"2.0\",\n  \"is_full_document\": true,\n  \"metadata\": {\n    \"file_suffix\": \"pdf\",\n    \"producer\": {\n      \"name\": \"mineru\",\n      \"version\": \"4.0.0\"\n    }\n  },\n  \"pages\": [\n    {\n      \"page_idx\": 4,\n      \"blocks\": [\n        {\n          \"type\": \"text\",\n          \"index\": 0,\n          \"bbox\": [\n            0.1,\n            0.2,\n            0.9,\n            0.3\n          ],\n          \"content\": [\n            {\n              \"type\": \"text\",\n              \"content\": \"\u8bbe\u5907\"\n            },\n            {\n              \"type\": \"hyperlink\",\n              \"url\": \"https://example.org\",\n              \"content\": [\n                {\n                  \"type\": \"text\",\n                  \"content\": \"\u9a8c\u6536\u5408\u683c\"\n                }\n              ]\n            }\n          ]\n        },\n        {\n          \"type\": \"table\",\n          \"index\": 1,\n          \"bbox\": [\n            0.1,\n            0.4,\n            0.9,\n            0.8\n          ],\n          \"content\": [\n            {\n              \"type\": \"table_caption\",\n              \"bbox\": [\n                0.1,\n                0.4,\n                0.9,\n                0.45\n              ],\n              \"content\": [\n                {\n                  \"type\": \"text\",\n                  \"content\": \"\u8bbe\u5907\u53c2\u6570\u8868\"\n                }\n              ]\n            },\n            {\n              \"type\": \"table_body\",\n              \"bbox\": [\n                0.1,\n                0.46,\n                0.9,\n                0.8\n              ],\n              \"content\": \"<table><tr><td>\u9650\u503c</td><td>1.5\u6beb\u7c73</td></tr></table>\"\n            }\n          ]\n        }\n      ]\n    }\n  ]\n}"

func TestMinerUMiddleNormalizedGeometryAndNestedBlocks(t *testing.T) {
	blocks := minerUSourceBlocks("\u8bbe\u5907\u9a8c\u6536\u5408\u683c\n\n\u8bbe\u5907\u53c2\u6570\u8868\n\n|\u9650\u503c|1.5\u6beb\u7c73|", []byte(middleFixture), "pdf")
	require.Len(t, blocks, 3)
	require.Equal(t, 5, blocks[0].Locator.Page)
	require.Equal(t, []float64{0.1, 0.2, 0.9, 0.3}, blocks[0].Locator.BBox)
	require.Equal(t, "mineru:2:4:0", blocks[0].Locator.SourceID)
	require.Equal(t, "exact", blocks[0].Locator.Mapping)
	require.Equal(t, []float64{0.1, 0.46, 0.9, 0.8}, blocks[2].Locator.BBox)
}

func TestMinerUMiddleZIPPreferredToLegacy(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, body := range map[string]string{
		"markdown.md":           "\u8bbe\u5907\u9a8c\u6536\u5408\u683c",
		"middle_json.json":      middleFixture,
		"doc_content_list.json": "[]",
	} {
		w, err := z.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, z.Close())
	require.JSONEq(t, middleFixture, string(minerUContentListFromZip(buf.Bytes())))
}

func TestMinerUMiddleRejectsUnknownSchemaAndInvalidIndices(t *testing.T) {
	for _, mutate := range []func(map[string]any){
		func(d map[string]any) { d["schema"] = "docvortex.model" },
		func(d map[string]any) { d["schema_version"] = "9.0" },
		func(d map[string]any) { delete(d["pages"].([]any)[0].(map[string]any), "page_idx") },
		func(d map[string]any) { d["pages"].([]any)[0].(map[string]any)["page_idx"] = -1 },
	} {
		var d map[string]any
		require.NoError(t, json.Unmarshal([]byte(middleFixture), &d))
		mutate(d)
		raw, err := json.Marshal(d)
		require.NoError(t, err)
		require.Empty(t, minerUSourceBlocks("\u8bbe\u5907\u9a8c\u6536\u5408\u683c", raw, "pdf"))
	}
}

func TestMinerULegacyMissingPageIsNotPageOne(t *testing.T) {
	require.Empty(t, minerUSourceBlocks("\u8bbe\u5907\u9a8c\u6536\u5408\u683c", []byte("[{\"type\":\"text\",\"text\":\"\u8bbe\u5907\u9a8c\u6536\u5408\u683c\"}]"), "pdf"))
}

func TestMinerUBoxesRejectNonFiniteAndWrongUnits(t *testing.T) {
	for _, box := range [][]float64{{0, 0, math.NaN(), 1}, {0, 0, math.Inf(1), 1}, {0, 0, 900, 600}, {.9, 0, .1, 1}} {
		require.False(t, normalizedMinerUBox(box))
	}
	_, ok := minerUBBox([]float64{0, 0, math.NaN(), 100})
	require.False(t, ok)
}

func TestMinerULegacyZIPWithUnsupportedMiddleKeepsContentList(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	legacy := "[{\"type\":\"text\",\"text\":\"\u8bbe\u5907\u9a8c\u6536\u5408\u683c\",\"page_idx\":1,\"bbox\":[100,200,900,300]}]"
	for name, body := range map[string]string{
		"doc_middle_json.json":  `{"pdf_info":[]}`,
		"doc_content_list.json": legacy,
	} {
		w, err := z.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, z.Close())
	blocks := minerUSourceBlocks("\u8bbe\u5907\u9a8c\u6536\u5408\u683c", minerUContentListFromZip(buf.Bytes()), "pdf")
	require.Len(t, blocks, 1)
	require.Equal(t, 2, blocks[0].Locator.Page)
	require.Equal(t, []float64{.1, .2, .9, .3}, blocks[0].Locator.BBox)
}
