package milvus

import (
	"testing"

	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/stretchr/testify/require"
)

func TestDetectAnalyzerName(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "Chinese",
			// Chinese sentence about multi-head attention
			text: "Transformer\u4e2d\u591a\u5934\u6ce8\u610f\u529b\u7684\u4e09\u79cd\u4f7f\u7528\u65b9\u5f0f" +
				"\u53ca\u89e3\u7801\u5668\u81ea\u6ce8\u610f\u529b\u63a9\u7801\u539f\u56e0",
			want: milvusAnalyzerChinese,
		},
		{name: "English", text: "How does multi-head attention work", want: milvusAnalyzerEnglish},
		// "AI attention"
		{name: "ChineseWithTechnicalTerm", text: "AI \u6ce8\u610f\u529b", want: milvusAnalyzerChinese},
		// "Tokyo settings"
		{name: "JapaneseKanjiWithKana", text: "\u6771\u4eac\u90fd\u306e\u8a2d\u5b9a", want: milvusAnalyzerDefault},
		// "Seoul settings"
		{name: "KoreanHangul", text: "\uc11c\uc6b8 \uc124\uc815", want: milvusAnalyzerDefault},
		{name: "Symbols", text: "12345 --", want: milvusAnalyzerDefault},
		{name: "Empty", text: "", want: milvusAnalyzerDefault},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, detectAnalyzerName(tt.text))
		})
	}
}

func TestMultiAnalyzerParams(t *testing.T) {
	params := multiAnalyzerParams()

	require.Equal(t, fieldLanguage, params["by_field"])
	require.Equal(t, map[string]string{
		"type": milvusAnalyzerEnglish,
	}, params["analyzers"].(map[string]any)[milvusAnalyzerEnglish])
	require.Equal(t, map[string]string{
		"type": milvusAnalyzerChinese,
	}, params["analyzers"].(map[string]any)[milvusAnalyzerChinese])
	require.Equal(t, map[string]string{
		"tokenizer": "icu",
	}, params["analyzers"].(map[string]any)[milvusAnalyzerDefault])
}

func TestNormalizeAnalyzerName(t *testing.T) {
	require.Equal(t, milvusAnalyzerEnglish, normalizeAnalyzerName(" EN "))
	require.Equal(t, milvusAnalyzerChinese, normalizeAnalyzerName("zh"))
	require.Equal(t, milvusAnalyzerDefault, normalizeAnalyzerName("unknown"))
}

func TestAnalyzerNameForUpsert(t *testing.T) {
	require.Equal(t, milvusAnalyzerDefault, analyzerNameForUpsert(nil))
	require.Equal(t, milvusAnalyzerChinese, analyzerNameForUpsert(&MilvusVectorEmbedding{
		Content: "AI \u6ce8\u610f\u529b", // "AI attention" in Chinese
	}))
	require.Equal(t, milvusAnalyzerEnglish, analyzerNameForUpsert(&MilvusVectorEmbedding{
		Content:  "AI \u6ce8\u610f\u529b",
		Language: "en",
	}))
	require.Equal(t, milvusAnalyzerEnglish, analyzerNameForUpsert(&MilvusVectorEmbedding{
		Content:  "hello",
		Language: "  ",
	}))
}

func TestAnalyzerModeFromSchema(t *testing.T) {
	legacySchema := entity.NewSchema().WithField(
		entity.NewField().WithName(fieldContent).WithDataType(entity.FieldTypeVarChar),
	)
	require.Equal(t, collectionAnalyzerLegacy, analyzerModeFromSchema(legacySchema))

	multilingualSchema := entity.NewSchema().WithField(
		entity.NewField().WithName(fieldContent).
			WithDataType(entity.FieldTypeVarChar).
			WithMultiAnalyzerParams(multiAnalyzerParams()),
	).WithField(
		entity.NewField().WithName(fieldLanguage).WithDataType(entity.FieldTypeVarChar),
	)
	require.Equal(t, collectionAnalyzerMulti, analyzerModeFromSchema(multilingualSchema))
}
