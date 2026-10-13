package chatpipeline

import (
	"context"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
)

func TestExtractKeywordsSegmentsChineseQuery(t *testing.T) {
	// "how to configure a vector database"; expects the tokens "configure", "vector", "database".
	keywords := extractKeywords("\u5982\u4f55\u914d\u7f6e\u5411\u91cf\u6570\u636e\u5e93")

	assert.NotContains(t, keywords, "\u5982\u4f55\u914d\u7f6e\u5411\u91cf\u6570\u636e\u5e93")
	assert.Contains(t, keywords, "\u914d\u7f6e")
	assert.Contains(t, keywords, "\u5411\u91cf")
	assert.Contains(t, keywords, "\u6570\u636e\u5e93")
}

func TestTokenizePreservesMixedLanguageBoundaries(t *testing.T) {
	// "RAG how to configure PostgreSQL" with no spaces.
	tokens := tokenize("RAG\u5982\u4f55\u914d\u7f6ePostgreSQL")

	assert.Contains(t, tokens, "RAG")
	assert.Contains(t, tokens, "\u914d\u7f6e")
	assert.Contains(t, tokens, "PostgreSQL")
	assert.NotContains(t, tokens, "RAG\u5982\u4f55\u914d\u7f6e")
}

func TestExtractKeywordsDropsSingleRuneChineseTokens(t *testing.T) {
	// "He came to the NetEase Hangzhou research building"; expects "NetEase" and "building".
	keywords := extractKeywords("\u4ed6\u6765\u5230\u4e86\u7f51\u6613\u676d\u7814\u5927\u53a6")

	assert.Contains(t, keywords, "\u7f51\u6613")
	assert.Contains(t, keywords, "\u5927\u53a6")
	for _, keyword := range keywords {
		assert.Greater(t, utf8.RuneCountInString(keyword), 1, "unexpected single-rune keyword %q", keyword)
	}
}

func TestExpandQueriesBuildsChineseKeywordVariant(t *testing.T) {
	expansions := (&PluginSearch{}).expandQueries(context.Background(), &types.ChatManage{
		PipelineState: types.PipelineState{RewriteQuery: "\u5982\u4f55\u914d\u7f6e\u5411\u91cf\u6570\u636e\u5e93"},
	})

	var foundKeywordVariant bool
	for _, expansion := range expansions {
		fields := strings.Fields(expansion)
		if containsToken(fields, "\u914d\u7f6e") &&
			containsToken(fields, "\u5411\u91cf") &&
			containsToken(fields, "\u6570\u636e\u5e93") {
			foundKeywordVariant = true
			break
		}
	}

	assert.True(t, foundKeywordVariant, "expected a Chinese keyword expansion with segmented terms, got %v", expansions)
}

func containsToken(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type recordingExpansionKBService struct {
	interfaces.KnowledgeBaseService
	mu     sync.Mutex
	params []types.SearchParams
}

func (s *recordingExpansionKBService) HybridSearch(
	_ context.Context, _ string, params types.SearchParams,
) ([]*types.SearchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.params = append(s.params, params)
	return nil, nil
}

// Expansion variants only add keyword recall, so they must not re-embed and
// repeat the vector search; with keyword matching off there is nothing to do.
func TestRunQueryExpansionSearchesKeywordsOnly(t *testing.T) {
	kbService := &recordingExpansionKBService{}
	plugin := &PluginSearch{knowledgeBaseService: kbService}
	cm := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			EmbeddingTopK: 10,
			SearchTargets: types.SearchTargets{{KnowledgeBaseID: "kb-1", Type: types.SearchTargetTypeKnowledgeBase}},
		},
		PipelineState: types.PipelineState{RewriteQuery: "\u5982\u4f55\u914d\u7f6e\u5411\u91cf\u6570\u636e\u5e93" +
			"\u7684\u7d22\u5f15\u53c2\u6570"},
	}

	plugin.runQueryExpansion(context.Background(), cm)
	if len(kbService.params) == 0 {
		t.Fatal("expected expansion searches")
	}
	for _, p := range kbService.params {
		if !p.DisableVectorMatch || p.DisableKeywordsMatch {
			t.Fatalf("expansion search params = %+v, want keyword-only", p)
		}
	}

	kbService.params = nil
	cm.DisableKeywordsMatch = true
	plugin.runQueryExpansion(context.Background(), cm)
	if len(kbService.params) != 0 {
		t.Fatalf("expansion ran %d searches with keyword matching off", len(kbService.params))
	}
}
