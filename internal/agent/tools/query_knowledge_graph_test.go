package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubKnowledgeBaseService struct {
	kb      *types.KnowledgeBase
	results []*types.SearchResult
	err     error
}

func (s *stubKnowledgeBaseService) CreateKnowledgeBase(context.Context, *types.KnowledgeBase) (*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

func (s *stubKnowledgeBaseService) GetKnowledgeBaseByIDOnly(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

func (s *stubKnowledgeBaseService) GetKnowledgeBasesByIDsOnly(context.Context, []string) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) FillKnowledgeBaseCounts(context.Context, *types.KnowledgeBase) error {
	return nil
}

func (s *stubKnowledgeBaseService) ListKnowledgeBases(context.Context) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) ListKnowledgeBasesByTenantID(context.Context, uint64) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) UpdateKnowledgeBase(
	context.Context,
	string,
	string,
	string,
	*types.KnowledgeBaseConfig,
	*types.VLMConfig,
) (*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) DeleteKnowledgeBase(context.Context, string) error {
	return nil
}

func (s *stubKnowledgeBaseService) TogglePinKnowledgeBase(context.Context, string) (*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) HybridSearch(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
	return s.results, s.err
}

func (s *stubKnowledgeBaseService) HybridSearchWithRerank(
	ctx context.Context, id string, params types.SearchParams,
) (*types.RetrievalResult, error) {
	results, err := s.HybridSearch(ctx, id, params)
	return &types.RetrievalResult{Results: results}, err
}

func (s *stubKnowledgeBaseService) ReadChunkImage(context.Context, *types.SearchResult) ([]byte, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) GetQueryEmbedding(context.Context, string, string) ([]float32, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) ResolveEmbeddingModelKeys(context.Context, []*types.KnowledgeBase) map[string]string {
	return nil
}

func (s *stubKnowledgeBaseService) CopyKnowledgeBase(
	context.Context,
	string,
	string,
) (*types.KnowledgeBase, *types.KnowledgeBase, error) {
	return nil, nil, nil
}

func (s *stubKnowledgeBaseService) DuplicateKnowledgeBase(
	context.Context,
	string,
) (*types.KnowledgeBase, error) {
	return nil, nil
}

func (s *stubKnowledgeBaseService) GetRepository() interfaces.KnowledgeBaseRepository {
	return nil
}

func (s *stubKnowledgeBaseService) ProcessKBDelete(context.Context, *asynq.Task) error {
	return nil
}

func TestQueryKnowledgeGraph_ReportsConfiguredEntityAndRelationTypes(t *testing.T) {
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{
			ID: "kb-1",
			ExtractConfig: &types.ExtractConfig{
				Enabled: true,
				Nodes: []*types.GraphNode{
					{Name: "\u5408\u540c"},
					{Name: "\u6cd5\u52a1\u90e8\u95e8"},
					{Name: "\u5ba1\u6279\u6d41\u7a0b"},
					{Name: "\u5408\u540c"},
					nil,
					{Name: ""},
				},
				Relations: []*types.GraphRelation{
					{Type: "\u5c5e\u4e8e"},
					{Type: "\u7ba1\u7406"},
					{Type: "\u5ba1\u6279"},
					{Type: "\u7ba1\u7406"},
					{Type: ""},
					nil,
				},
			},
		},
		results: []*types.SearchResult{
			{
				ID: "chunk-approval-1",
				Content: "\u5408\u540c\u5ba1\u6279\u6d41\u7a0b\u7531\u6cd5\u52a1\u90e8\u95e8\u4e0e\u91c7\u8d2d" +
					"\u90e8\u95e8\u5171\u540c\u7ef4\u62a4\uff0c\u6cd5\u52a1\u90e8\u95e8\u8d1f\u8d23\u5408\u89c4\u5ba1" +
					"\u67e5\u3002",
				KnowledgeID:    "doc-approval",
				KnowledgeTitle: "\u5408\u540c\u5ba1\u6279\u7ba1\u7406\u5236\u5ea6",
				Score:          0.97,
				MatchType:      types.MatchTypeEmbedding,
			},
			{
				ID: "chunk-approval-2",
				Content: "\u91c7\u8d2d\u7533\u8bf7\u63d0\u4ea4\u540e\u8fdb\u5165\u5408\u540c\u5ba1\u6279\u6d41" +
					"\u7a0b\uff0c\u5ba1\u6279\u5b8c\u6210\u540e\u5f52\u6863\u5230\u5408\u540c\u53f0\u8d26\u3002",
				KnowledgeID:    "doc-procurement",
				KnowledgeTitle: "\u91c7\u8d2d\u4e0e\u5408\u540c\u534f\u4f5c\u89c4\u8303",
				Score:          0.89,
				MatchType:      types.MatchTypeKeywords,
			},
			{
				ID: "chunk-approval-3",
				Content: "\u6cd5\u52a1\u90e8\u95e8\u7ba1\u7406\u6807\u51c6\u5408\u540c\u6a21\u677f\uff0c\u5e76" +
					"\u7ef4\u62a4\u5408\u540c\u98ce\u9669\u5ba1\u67e5\u6e05\u5355\u3002",
				KnowledgeID:    "doc-legal",
				KnowledgeTitle: "\u6cd5\u52a1\u90e8\u804c\u8d23\u8bf4\u660e",
				Score:          0.84,
				MatchType:      types.MatchTypeEmbedding,
			},
		},
	})

	args, err := json.Marshal(QueryKnowledgeGraphInput{
		KnowledgeBaseIDs: []string{"kb-1"},
		Query:            "\u5408\u540c\u5ba1\u6279\u4e0e\u6cd5\u52a1\u534f\u4f5c",
	})
	require.NoError(t, err)

	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success)
	t.Logf("tool output:\n%s", result.Output)

	assert.Contains(t, result.Output, "Entity Types (3)")
	assert.Contains(t, result.Output, "Relationship Types (3)")
	assert.NotContains(t, result.Output, "No entity types configured")
	assert.NotContains(t, result.Output, "No relationship types configured")
	assert.Contains(t, result.Output, "\u5408\u540c")
	assert.Contains(t, result.Output, "\u6cd5\u52a1\u90e8\u95e8")
	assert.Contains(t, result.Output, "\u5ba1\u6279\u6d41\u7a0b")
	assert.Contains(t, result.Output, "\u7ba1\u7406")
	assert.Contains(t, result.Output, "\u5ba1\u6279")
	assert.Contains(t, result.Output, "✓ Found 0 relations and 3 relevant chunks (deduplicated)")
	assert.Contains(t, result.Output, "Result #1:")
	assert.Contains(t, result.Output, "Result #2:")
	assert.Contains(t, result.Output, "Result #3:")
	assert.Contains(t, result.Output, "\u5408\u540c\u5ba1\u6279\u7ba1\u7406\u5236\u5ea6")

	graphConfig, ok := result.Data["graph_config"].(map[string]interface{})
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"\u5408\u540c", "\u5ba1\u6279\u6d41\u7a0b", "\u6cd5\u52a1\u90e8" +
		"\u95e8"}, graphConfig["nodes"])
	assert.ElementsMatch(t, []string{"\u5c5e\u4e8e", "\u5ba1\u6279", "\u7ba1\u7406"}, graphConfig["relations"])
}
