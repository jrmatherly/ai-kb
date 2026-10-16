package doris

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

// TestLiveRoundTrip runs the repository against a real, disposable Doris
// cluster, which sqlmock cannot stand in for. Run it on the compose network so doris-be resolves: for
// example `docker compose --profile doris up -d`, then run the test binary in
// a container on ai-kb-network with AIKB_DORIS_TEST_ADDR=doris-fe:9030 and
// SSRF_WHITELIST_EXTRA=doris-fe,doris-be.
func TestLiveRoundTrip(t *testing.T) {
	addr := os.Getenv("AIKB_DORIS_TEST_ADDR")
	if addr == "" {
		t.Skip("set AIKB_DORIS_TEST_ADDR to run against a live Doris cluster")
	}
	ctx := context.Background()
	db, err := sql.Open("mysql", "root:@tcp("+addr+")/aikb?parseTime=true&interpolateParams=true")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	host, _, _ := strings.Cut(addr, ":")
	repo := NewDorisRetrieveEngineRepository(db, "http://"+host+":8030", "root", "", "aikb", nil)

	kb := "kb-" + uuid.NewString()
	k1, k2 := kb+"-k1", kb+"-k2"
	c1, c2 := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() { _ = repo.DeleteByKnowledgeIDList(ctx, []string{k1, k2}, 4, "") })
	require.NoError(t, repo.BatchSave(ctx, []*types.IndexInfo{
		{
			SourceID: c1, ChunkID: c1, KnowledgeID: k1, KnowledgeBaseID: kb, IsEnabled: true,
			Content: "doris upgrade smoke test",
		},
		{
			SourceID: c2, ChunkID: c2, KnowledgeID: k2, KnowledgeBaseID: kb, IsEnabled: true,
			Content: "unrelated weather report",
		},
	}, map[string]any{fieldEmbedding: map[string][]float32{c1: {1, 0, 0, 0}, c2: {0, 1, 0, 0}}}))

	// The enabled flag is written through Stream Load, which the FE redirects to the BE.
	require.NoError(t, repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{c2: false}))

	var got []*types.RetrieveResult
	require.Eventually(t, func() bool {
		got, err = repo.Retrieve(ctx, types.RetrieveParams{
			Embedding: []float32{1, 0, 0, 0}, KnowledgeBaseIDs: []string{kb}, TopK: 5,
			RetrieverType: types.VectorRetrieverType,
		})
		return err == nil && len(got) == 1 && len(got[0].Results) == 1
	}, 30*time.Second, time.Second, "the disabled chunk is filtered out")
	require.Equal(t, c1, got[0].Results[0].ChunkID)

	got, err = repo.Retrieve(ctx, types.RetrieveParams{
		Query: "upgrade smoke", KnowledgeBaseIDs: []string{kb}, TopK: 5,
		RetrieverType: types.KeywordsRetrieverType,
	})
	require.NoError(t, err)
	require.NoError(t, got[0].Error)
	require.Len(t, got[0].Results, 1)
	require.Equal(t, c1, got[0].Results[0].ChunkID)
}
