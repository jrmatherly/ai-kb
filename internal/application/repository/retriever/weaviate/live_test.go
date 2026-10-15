package weaviate

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
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

	kb, copyKB, cjkKB := "kb-"+uuid.NewString(), "kb-"+uuid.NewString(), "kb-"+uuid.NewString()
	k1, k2, k3, kCJK := kb+"-k1", kb+"-k2", kb+"-k3", cjkKB+"-k"
	c1, c2, other, cCJK := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	q1 := types.GeneratedQuestionSourceID(c1, "q1")
	t.Cleanup(func() {
		_ = repo.DeleteByKnowledgeIDList(ctx, []string{k1, k2, kCJK, copyKB + "-k1", copyKB + "-k2"}, 4, "")
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

	// "The knowledge base supports Chinese retrieval" plus "database" in Katakana.
	cjkText := "\u77e5\u8bc6\u5e93\u652f\u6301\u4e2d\u6587\u68c0\u7d22 \u30c7\u30fc\u30bf\u30d9\u30fc\u30b9"
	require.NoError(t, repo.BatchSave(ctx, []*types.IndexInfo{{
		SourceID: cCJK, ChunkID: cCJK, KnowledgeID: kCJK, KnowledgeBaseID: cjkKB, IsEnabled: true, Content: cjkText,
	}}, map[string]any{fieldEmbedding: map[string][]float32{cCJK: {0, 0, 0, 1}}}))
	for _, query := range []string{
		"\u4e2d\u6587",             // "Chinese", a two-character word inside the run
		"\u4e2d\u6587\u68c0\u7d22", // "Chinese retrieval"
		"\u5e93",                   // one character inside the run
		"\u30c7\u30fc\u30bf",       // "data" in Katakana
	} {
		hits := retrieveKeywords(t, repo, query, cjkKB)
		require.Len(t, hits, 1, "keyword query %q", query)
		require.Equal(t, cCJK, hits[0].ChunkID)
	}
	require.Empty(t, retrieveKeywords(t, repo, "\u5929\u6c14", cjkKB), "an absent word does not match") // "weather"

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

	keyword := retrieveKeywords(t, repo, "Upgrade smoke", kb)
	require.NotEmpty(t, keyword)
	require.Equal(t, c1, keyword[0].ChunkID)

	target, disabledTarget := uuid.NewString(), uuid.NewString()
	require.NoError(t, repo.CopyIndices(ctx, kb,
		map[string]string{k1: copyKB + "-k1", k2: copyKB + "-k2"},
		map[string]string{c1: target, c2: disabledTarget}, copyKB, 4, ""))
	copied := vector(copyKB)
	require.Len(t, copied, 2, "the copy carries the chunk's rows and vectors, and the disabled chunk stays disabled")
	require.Equal(t, target, copied[0].ChunkID)

	require.NoError(t, repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{target: false}))
	require.Empty(t, vector(copyKB), "a copied chunk can be disabled")
	require.Len(t, vector(kb), 2, "disabling the copy leaves the source alone")
}

func retrieveKeywords(
	t *testing.T, repo interfaces.RetrieveEngineRepository, query, kbID string,
) []*types.IndexWithScore {
	t.Helper()
	got, err := repo.Retrieve(context.Background(), types.RetrieveParams{
		Query: query, KnowledgeBaseIDs: []string{kbID}, TopK: 5, RetrieverType: types.KeywordsRetrieverType,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NoError(t, got[0].Error)
	return got[0].Results
}
