package milvus

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/types"
	client "github.com/milvus-io/milvus/client/v3/milvusclient"
	"github.com/stretchr/testify/require"
)

// TestLiveRoundTrip runs the repository against a real, disposable Milvus
// server, which the fake gRPC servers in the other tests cannot stand in for.
// For example: docker compose -p aikbtest --profile milvus up -d milvus, then
// AIKB_MILVUS_TEST_ADDR=127.0.0.1:19530.
func TestLiveRoundTrip(t *testing.T) {
	addr := os.Getenv("AIKB_MILVUS_TEST_ADDR")
	if addr == "" {
		t.Skip("set AIKB_MILVUS_TEST_ADDR to run against a live Milvus server")
	}
	ctx := context.Background()
	c, err := client.New(ctx, &client.ClientConfig{Address: addr})
	require.NoError(t, err)
	defer func() { _ = c.Close(ctx) }()

	repo := NewMilvusRetrieveEngineRepository(c, nil)
	kb := "kb-live-" + time.Now().Format("150405.000000")
	k1, k2 := kb+"-k1", kb+"-k2"
	items := []*types.IndexInfo{
		{
			SourceID: "s1", ChunkID: "c1", KnowledgeID: k1, KnowledgeBaseID: kb, IsEnabled: true,
			Content: "milvus upgrade smoke test",
		},
		{
			SourceID: "s2", ChunkID: "c2", KnowledgeID: k2, KnowledgeBaseID: kb, IsEnabled: true,
			Content: "unrelated weather report",
		},
	}
	t.Cleanup(func() { _ = repo.DeleteByKnowledgeIDList(ctx, []string{k1, k2}, 4, "") })
	vectors := map[string][]float32{"s1": {1, 0, 0, 0}, "s2": {0, 1, 0, 0}}
	require.NoError(t, repo.BatchSave(ctx, items, map[string]any{fieldEmbedding: vectors}))

	// Milvus searches with bounded consistency, so fresh inserts become
	// visible shortly after the write returns.
	var got []*types.RetrieveResult
	require.Eventually(t, func() bool {
		got, err = repo.Retrieve(ctx, types.RetrieveParams{
			Embedding: []float32{1, 0, 0, 0}, KnowledgeBaseIDs: []string{kb}, TopK: 2,
			RetrieverType: types.VectorRetrieverType,
		})
		return err == nil && len(got) > 0 && len(got[0].Results) == 2
	}, 30*time.Second, time.Second)
	require.Equal(t, "c1", got[0].Results[0].ChunkID)

	got, err = repo.Retrieve(ctx, types.RetrieveParams{
		Query: "upgrade smoke", KnowledgeBaseIDs: []string{kb}, TopK: 2,
		RetrieverType: types.KeywordsRetrieverType,
	})
	require.NoError(t, err)
	require.NoError(t, got[0].Error)
	require.NotEmpty(t, got[0].Results)
	require.Equal(t, "c1", got[0].Results[0].ChunkID)
}
