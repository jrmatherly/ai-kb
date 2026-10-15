package weaviate

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
	sdk "github.com/weaviate/weaviate-go-client/v5/weaviate"
)

// TestLiveRoundTrip runs the repository against a real, disposable Weaviate
// server, which the httptest fakes in the other tests cannot stand in for.
// For example: docker run --rm -p 18080:8080
// -e AUTHENTICATION_ANONYMOUS_ACCESS_ENABLED=true semitechnologies/weaviate:1.40.1,
// then AIKB_WEAVIATE_TEST_HOST=127.0.0.1:18080.
func TestLiveRoundTrip(t *testing.T) {
	host := os.Getenv("AIKB_WEAVIATE_TEST_HOST")
	if host == "" {
		t.Skip("set AIKB_WEAVIATE_TEST_HOST to run against a live Weaviate server")
	}
	ctx := context.Background()
	c, err := sdk.NewClient(sdk.Config{Host: host, Scheme: "http"})
	require.NoError(t, err)
	repo := NewWeaviateRetrieveEngineRepository(c, nil)

	kb, copyKB := "kb-"+uuid.NewString(), "kb-"+uuid.NewString()
	k1, k2, k3 := kb+"-k1", kb+"-k2", kb+"-k3"
	c1, c2, other := uuid.NewString(), uuid.NewString(), uuid.NewString()
	q1 := types.GeneratedQuestionSourceID(c1, "q1")
	t.Cleanup(func() {
		_ = repo.DeleteByKnowledgeIDList(ctx, []string{k1, k2, copyKB + "-k1"}, 4, "")
		_ = repo.DeleteByKnowledgeIDList(ctx, []string{k3}, 3, "")
	})

	// A second collection that holds neither chunk, so updates must skip it.
	require.NoError(t, repo.BatchSave(ctx, []*types.IndexInfo{{
		SourceID: other, ChunkID: other, KnowledgeID: k3, KnowledgeBaseID: kb, IsEnabled: true,
		Content: "other dimension",
	}}, map[string]any{fieldEmbedding: map[string][]float32{other: {0, 0, 1}}}))
	require.NoError(t, repo.BatchSave(ctx, []*types.IndexInfo{
		{
			SourceID: c1, ChunkID: c1, KnowledgeID: k1, KnowledgeBaseID: kb, IsEnabled: true,
			Content: "weaviate upgrade smoke test",
		},
		{
			SourceID: q1, ChunkID: c1, KnowledgeID: k1, KnowledgeBaseID: kb, IsEnabled: true,
			Content: "generated question about the upgrade",
		},
		{
			SourceID: c2, ChunkID: c2, KnowledgeID: k2, KnowledgeBaseID: kb, IsEnabled: true,
			Content: "unrelated weather report",
		},
	}, map[string]any{fieldEmbedding: map[string][]float32{c1: {1, 0, 0, 0}, q1: {0.9, 0, 0.1, 0}, c2: {0, 1, 0, 0}}}))

	require.NoError(t, repo.BatchUpdateChunkTagID(ctx, map[string]string{c1: "tag-1"}))
	require.NoError(t, repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{c1: true, c2: false}))

	retrieve := func(params types.RetrieveParams) []*types.IndexWithScore {
		got, err := repo.Retrieve(ctx, params)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.NoError(t, got[0].Error)
		return got[0].Results
	}
	vector := func(kbID string) []*types.IndexWithScore {
		return retrieve(types.RetrieveParams{
			Embedding: []float32{1, 0, 0, 0}, KnowledgeBaseIDs: []string{kbID}, TopK: 5,
			RetrieverType: types.VectorRetrieverType,
		})
	}
	sources := func(rows []*types.IndexWithScore) map[string]string {
		bySource := map[string]string{}
		for _, row := range rows {
			bySource[row.SourceID] = row.Content
		}
		return bySource
	}
	require.Equal(t, map[string]string{
		c1: "weaviate upgrade smoke test",
		q1: "generated question about the upgrade",
	}, sources(vector(kb)), "the question keeps its own row, the disabled chunk is filtered out, "+
		"updates keep the other properties")

	keyword := retrieve(types.RetrieveParams{
		Query: "upgrade smoke", KnowledgeBaseIDs: []string{kb}, TopK: 5,
		RetrieverType: types.KeywordsRetrieverType,
	})
	require.NotEmpty(t, keyword)
	require.Equal(t, c1, keyword[0].ChunkID)

	target := uuid.NewString()
	require.NoError(t, repo.CopyIndices(ctx, kb,
		map[string]string{k1: copyKB + "-k1"}, map[string]string{c1: target}, copyKB, 4, ""))
	copied := vector(copyKB)
	require.Len(t, copied, 2, "the copy carries the chunk's rows and vectors")
	require.Equal(t, target, copied[0].ChunkID)

	require.NoError(t, repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{target: false}))
	require.Empty(t, vector(copyKB), "a copied chunk can be disabled")
	require.Len(t, vector(kb), 2, "disabling the copy leaves the source alone")
}
