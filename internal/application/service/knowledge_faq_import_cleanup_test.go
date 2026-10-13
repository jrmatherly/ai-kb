package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type cleanupIndexSpy struct {
	writeIndexSpy
	deleted []string
}

func (s *cleanupIndexSpy) DeleteByChunkIDList(_ context.Context, ids []string, _ int, _ string) error {
	s.deleted = append(s.deleted, ids...)
	return nil
}

func TestDeleteUnindexedFAQChunksDeletesOnlyOwnedOrStaleChunks(t *testing.T) {
	f := newFAQWriteFixture(t)
	index := &cleanupIndexSpy{}
	f.svc.retrieveEngine = &fakeFanoutRegistry{
		byStore: map[string]interfaces.RetrieveEngineService{*f.kb.VectorStoreID: index},
	}
	f.svc.modelService = kbCleanupModelService{}

	for _, chunk := range []*types.Chunk{
		{ID: "import-stored", Status: int(types.ChunkStatusStored)},
		{ID: "create-indexing", Status: int(types.ChunkStatusStored)},
		{ID: "import-indexed", Status: int(types.ChunkStatusIndexed)},
		{ID: "dead-writer", Status: int(types.ChunkStatusStored), UpdatedAt: time.Now().Add(-3 * time.Hour)},
	} {
		chunk.TenantID, chunk.KnowledgeBaseID, chunk.KnowledgeID = 7, "kb", "faq"
		chunk.ChunkType, chunk.IsEnabled = types.ChunkTypeFAQ, true
		require.NoError(t, f.db.Create(chunk).Error)
	}
	payload := &types.FAQImportPayload{TaskID: "task", TenantID: 7, KBID: "kb", KnowledgeID: "faq"}
	require.NoError(t, f.svc.saveFAQImportProgress(f.ctx, &types.FAQImportProgress{
		TaskID: "task", PendingChunkIDs: []string{"import-stored", "import-indexed"},
	}))

	require.NoError(t, f.svc.deleteUnindexedFAQChunks(f.ctx, payload, f.kb))

	var left []string
	require.NoError(t, f.db.Model(&types.Chunk{}).Where("knowledge_id = ?", "faq").Order("id").Pluck("id", &left).Error)
	require.Equal(t, []string{"create-indexing", "import-indexed", "one"}, left)
	require.ElementsMatch(t, []string{"import-stored", "dead-writer"}, index.deleted)
	progress, err := f.svc.GetFAQImportProgress(f.ctx, "task")
	require.NoError(t, err)
	require.Empty(t, progress.PendingChunkIDs)
}

// A retry rebuilds the task progress, so it must carry over the validation result and the batch left to clean up.
func TestFAQImportRetryKeepsValidationAndCleansPendingBatch(t *testing.T) {
	f := newFAQWriteFixture(t)
	f.svc.modelService = embedFailureModelService{err: errors.New("embedding down")}
	f.svc.tenantRepo = &processSyncTenantRepo{tenant: &types.Tenant{ID: 7}}
	require.NoError(t, f.db.Create(&types.Chunk{
		ID: "import-stored", TenantID: 7, KnowledgeBaseID: "kb", KnowledgeID: "faq",
		ChunkType: types.ChunkTypeFAQ, Status: int(types.ChunkStatusStored),
	}).Error)
	failed := []types.FAQFailedEntry{{Index: 1, Reason: "duplicate"}}
	require.NoError(t, f.svc.saveFAQImportProgress(f.ctx, &types.FAQImportProgress{
		TaskID: "task", ValidEntryIndices: []int{0}, FailedCount: 1, FailedEntries: failed,
		PendingChunkIDs: []string{"import-stored"},
	}))
	entries := []types.FAQEntryPayload{
		{StandardQuestion: "kept", Answers: []string{"a"}},
		{StandardQuestion: "dropped", Answers: []string{"b"}},
	}
	payload := types.FAQImportPayload{TenantID: 7, KBID: "kb", KnowledgeID: "faq", TaskID: "task", Entries: entries}

	err := f.svc.ProcessFAQImport(context.Background(), asynq.NewTask(types.TypeFAQImport, mustWriteJSON(t, payload)))
	require.Error(t, err)

	progress, err := f.svc.GetFAQImportProgress(f.ctx, "task")
	require.NoError(t, err)
	require.Equal(t, []int{0}, progress.ValidEntryIndices)
	require.Equal(t, 1, progress.FailedCount)
	require.Empty(t, progress.PendingChunkIDs)
	var count int64
	require.NoError(t, f.db.Model(&types.Chunk{}).Where("id = ?", "import-stored").Count(&count).Error)
	require.Zero(t, count)
}
