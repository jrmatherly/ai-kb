package service

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// Standard mode reports is_active while an ingest batch holds an in-flight slot.
func TestWikiStatsIsActiveWhileIngestBatchRuns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}, &types.WikiPage{}, &types.WikiPageIssue{}))
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	pages := NewWikiPageService(repository.NewWikiPageRepository(db), nil, nil, nil, rdb, nil)
	ingest := &wikiIngestService{redisClient: rdb}
	ctx := context.Background()

	stats, err := pages.GetStats(ctx, "kb-1")
	require.NoError(t, err)
	require.False(t, stats.IsActive)

	release, granted := ingest.reserveInflightSlot(ctx, "kb-1", wikiInflightDefault)
	require.True(t, granted)
	stats, err = pages.GetStats(ctx, "kb-1")
	require.NoError(t, err)
	require.True(t, stats.IsActive)

	release()
	stats, err = pages.GetStats(ctx, "kb-1")
	require.NoError(t, err)
	require.False(t, stats.IsActive)
}

// Lite mode has no Redis; is_active follows the in-process per-KB batch lock.
func TestWikiStatsIsActiveWhileLiteBatchRuns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.WikiFolder{}, &types.WikiPage{}, &types.WikiPageIssue{}))

	pages := NewWikiPageService(repository.NewWikiPageRepository(db), nil, nil, nil, nil, nil)
	ingest := NewWikiIngestService(pages, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).(*wikiIngestService)
	ctx := context.Background()

	stats, err := pages.GetStats(ctx, "kb-1")
	require.NoError(t, err)
	require.False(t, stats.IsActive)

	_, held := ingest.liteLocks.LoadOrStore("kb-1", struct{}{})
	require.False(t, held)
	stats, err = pages.GetStats(ctx, "kb-1")
	require.NoError(t, err)
	require.True(t, stats.IsActive)

	ingest.liteLocks.Delete("kb-1")
	stats, err = pages.GetStats(ctx, "kb-1")
	require.NoError(t, err)
	require.False(t, stats.IsActive)
}
