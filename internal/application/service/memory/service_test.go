package memory

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// memoryDBSeq keeps each harness database distinct: t.Name() repeats under
// -count>1 and a shared-cache database lives while gorm holds a connection.
var memoryDBSeq atomic.Uint64

// newMemoryHarness builds a service over a real SQLite database. The write
// path is mostly about what ends up in the database after a conflict, so a
// mocked repository would assert the wrong thing.
func newMemoryHarness(t *testing.T) (*Service, *gorm.DB, *stubTenantRepo) {
	t.Helper()
	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), memoryDBSeq.Add(1))),
		&gorm.Config{Logger: logger.Discard},
	)
	require.NoError(t, err)
	// Recall records usage on a background goroutine. A shared-cache SQLite
	// file rejects a concurrent writer, so pin the pool to one connection and
	// let the driver serialize instead of failing the next read.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&types.MemorySubject{}, &types.MemoryItem{}, &types.MemoryTombstone{},
		&types.MemoryTopicStat{}, &types.MemoryDocAffinity{},
		&types.MemoryItemEmbedding{}, &types.MemoryExtractionSession{}))

	tenantRepo := &stubTenantRepo{
		configs: map[uint64]*types.MemoryConfig{},
	}
	svc := &Service{
		repo:       repository.NewMemoryRepository(db),
		tenantRepo: tenantRepo,
	}
	return svc, db, tenantRepo
}

// enabledCtx returns a request context for one principal in one workspace,
// with memory switched on for that workspace.
func enabledCtx(t *testing.T, tenantRepo *stubTenantRepo, tenantID uint64, userID string) context.Context {
	t.Helper()
	tenantRepo.set(tenantID, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, EmbeddingModelID: "embed-1",
	})
	ctx := context.WithValue(t.Context(), types.TenantIDContextKey, tenantID)
	return types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: userID})
}

func TestRememberStoresAndRecalls(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindPreference,
		Content: "\u56de\u7b54\u8bf7\u76f4\u63a5\u7ed9\u7ed3\u8bba\uff0c\u4e0d\u8981\u94fa\u57ab", Importance: 4,
	})
	require.NoError(t, err)

	recall := svc.Recall(ctx, "\u5e2e\u6211\u770b\u770b\u8fd9\u4e2a\u62a5\u9519")
	require.Contains(t, recall.Prompt, "\u56de\u7b54\u8bf7\u76f4\u63a5\u7ed9\u7ed3\u8bba")
	require.Len(t, recall.Items, 1)
}

// TestExplicitMemoryIsAlwaysAvailable pins the rule that a user who said
// "remember this" gets it back regardless of how they phrase the next
// question. Leaving it to lexical matching means the one memory the user
// deliberately asked for is the one most likely to go missing.
func TestExplicitMemoryIsAlwaysAvailable(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Content: "my editor is Neovim and my terminal is WezTerm",
		Origin:  types.MemoryOriginExplicit,
	})
	require.NoError(t, err)

	// Not one word in common with the memory.
	recall := svc.Recall(ctx, "what tools do I use daily")
	require.Contains(t, recall.Prompt, "Neovim")
	require.Len(t, recall.Items, 1)
}

// TestExtractedFactStillNeedsAQueryMatch is the counterpart: memory the user
// never asked for must stay out of context unless it is relevant, or the block
// grows without bound.
func TestExtractedFactStillNeedsAQueryMatch(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Topic:   "\u7f16\u8f91\u5668",
		Content: "\u7528\u7684\u7f16\u8f91\u5668\u662f Neovim",
		Origin:  types.MemoryOriginExtracted,
	})
	require.NoError(t, err)

	require.Empty(t, svc.Recall(ctx, "\u5e2e\u6211\u7b97\u4e00\u4e0b\u8fd9\u4e2a\u6708\u7684\u8d26").Prompt)
	require.Contains(t, svc.Recall(ctx, "\u7f16\u8f91\u5668\u600e\u4e48\u914d\u7f6e").Prompt, "Neovim")
}

// TestRecalledItemIsNotListedTwice guards the seam between the resident block
// and query matching: an explicit fact is in both candidate sets.
func TestRecalledItemIsNotListedTwice(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17",
		Origin:  types.MemoryOriginExplicit,
	})
	require.NoError(t, err)

	recall := svc.Recall(ctx, "\u6570\u636e\u5e93\u8fde\u63a5\u6c60\u914d\u591a\u5927")
	require.Len(t, recall.Items, 1, "the same memory must not be reported twice")
	require.Equal(t, 1, strings.Count(recall.Prompt, "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17"))
}

func TestRecallMatchesSituationalItemsByQuery(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact,
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17\uff0c\u90e8\u7f72\u5728\u6cd5\u5170\u514b" +
			"\u798f",
	})
	require.NoError(t, err)
	_, err = svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Content: "\u524d\u7aef\u7528 Vue 3 \u52a0 Vite",
	})
	require.NoError(t, err)

	recall := svc.Recall(ctx, "\u6570\u636e\u5e93\u8fde\u63a5\u8d85\u65f6\u5e94\u8be5\u600e\u4e48\u6392\u67e5")
	require.Contains(t, recall.Prompt, "PostgreSQL 17")
	// An unrelated fact must stay out: injecting it would spend context and
	// invite the model to use it.
	require.NotContains(t, recall.Prompt, "Vue 3")
}

func TestContradictionSupersedesRatherThanDeletes(t *testing.T) {
	svc, db, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	first, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u5728\u7528\u7684\u6570\u636e\u5e93",
		Content: "\u6211\u4eec\u7528\u7684\u662f MySQL",
	})
	require.NoError(t, err)
	second, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u5728\u7528\u7684\u6570\u636e\u5e93",
		Content: "\u6211\u4eec\u5df2\u7ecf\u8fc1\u5230 PostgreSQL",
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)

	var old types.MemoryItem
	require.NoError(t, db.First(&old, "id = ?", first.ID).Error)
	require.Equal(t, types.MemoryStatusSuperseded, old.Status,
		"the outdated statement must be superseded, not left active")
	require.Equal(t, second.ID, old.SupersededBy)
	require.NotNil(t, old.InvalidAt, "a superseded item must record when it stopped being true")
	require.Equal(t, "\u6211\u4eec\u7528\u7684\u662f MySQL", old.Content,
		"history must stay readable, so the old content is preserved")

	recall := svc.Recall(ctx, "\u6211\u4eec\u7684\u6570\u636e\u5e93\u662f\u4ec0\u4e48")
	require.Contains(t, recall.Prompt, "PostgreSQL")
	require.NotContains(t, recall.Prompt, "MySQL")
}

func TestRepeatedIdenticalStatementDoesNotChurn(t *testing.T) {
	svc, db, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	first, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4f4d", Content: "\u540e\u7aef\u5de5\u7a0b\u5e08",
	})
	require.NoError(t, err)
	second, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4f4d", Content: "\u540e\u7aef\u5de5\u7a0b\u5e08",
	})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "an unchanged statement must not create a new row")

	var count int64
	require.NoError(t, db.Model(&types.MemoryItem{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

// TestRestatedFactDoesNotDuplicate covers the shape seen in real use: the user
// says "remember X" and the background distillation later produces the same
// fact with slightly different wording. They carry different topic keys, so
// without a containment check the user's memory list shows the same thing
// twice.
func TestRestatedFactDoesNotDuplicate(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	explicit, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact,
		Content: "\u6211\u4eec\u7684\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17\uff0c\u90e8\u7f72\u5728\u6cd5" +
			"\u5170\u514b\u798f",
		Origin: types.MemoryOriginExplicit,
	})
	require.NoError(t, err)

	// The distillation restates it more tersely and under its own topic.
	extracted, err := svc.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Topic:   "\u751f\u4ea7\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17\uff0c\u90e8\u7f72\u5728\u6cd5\u5170\u514b\u798f",
		Origin:  types.MemoryOriginExtracted,
	})
	require.NoError(t, err)
	require.Equal(t, explicit.ID, extracted.ID,
		"a restatement contained in an existing memory must not create a second row")

	_, total, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}

// TestMoreSpecificRestatementSupersedes is the other direction: when the new
// statement contains the old one it carries strictly more information, so it
// replaces it instead of sitting beside it.
func TestMoreSpecificRestatementSupersedes(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	short, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u751f\u4ea7\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17",
	})
	require.NoError(t, err)
	long, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact,
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17\uff0c\u90e8\u7f72\u5728\u6cd5\u5170\u514b" +
			"\u798f",
	})
	require.NoError(t, err)
	require.NotEqual(t, short.ID, long.ID)

	items, total, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t,
		"\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17\uff0c\u90e8\u7f72\u5728\u6cd5\u5170\u514b\u798f",
		items[0].Content)
}

// TestDifferentFactsSharingWordsAreKept guards the containment rule from being
// too eager: two genuinely different statements must both survive.
func TestDifferentFactsSharingWordsAreKept(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u751f\u4ea7\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17",
	})
	require.NoError(t, err)
	_, err = svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u6d4b\u8bd5\u6570\u636e\u5e93",
		Content: "\u6d4b\u8bd5\u6570\u636e\u5e93\u662f PostgreSQL 15",
	})
	require.NoError(t, err)

	_, total, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
}

func TestMemoriesAreIsolatedAcrossSubjectsAndWorkspaces(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)

	aliceInOne := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(aliceInOne, types.MemoryItem{
		Kind: types.MemoryKindProfile, Content: "\u7231\u4e3d\u4e1d\u5728\u505a\u533b\u7597\u5f71\u50cf\u9879\u76ee",
	})
	require.NoError(t, err)

	// Same workspace, different person.
	bobInOne := enabledCtx(t, tenantRepo, 1, "bob")
	require.Empty(t, svc.Recall(bobInOne, "\u6211\u5728\u505a\u4ec0\u4e48\u9879\u76ee").Prompt,
		"another user in the same workspace must not see the memory")

	// Same person, different workspace: the agreed scope is (workspace,
	// principal), so work memories do not follow someone across workspaces.
	aliceInTwo := enabledCtx(t, tenantRepo, 2, "alice")
	require.Empty(t, svc.Recall(aliceInTwo, "\u6211\u5728\u505a\u4ec0\u4e48\u9879\u76ee").Prompt,
		"the same user in another workspace must not see the memory")

	require.Contains(t, svc.Recall(aliceInOne, "\u6211\u5728\u505a\u4ec0\u4e48\u9879\u76ee").Prompt,
		"\u533b\u7597\u5f71\u50cf")
}

func TestListItemsIsScopedToTheCaller(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	aliceCtx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(aliceCtx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Content: "\u7231\u4e3d\u4e1d\u7684\u79d8\u5bc6",
	})
	require.NoError(t, err)

	bobCtx := enabledCtx(t, tenantRepo, 1, "bob")
	items, total, err := svc.ListItems(bobCtx, "", 50, 0)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, items)
}

func TestDeleteAnotherUsersMemoryIsNotFound(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	aliceCtx := enabledCtx(t, tenantRepo, 1, "alice")
	item, err := svc.Remember(aliceCtx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Content: "\u7231\u4e3d\u4e1d\u7684\u79d8\u5bc6",
	})
	require.NoError(t, err)

	bobCtx := enabledCtx(t, tenantRepo, 1, "bob")
	require.ErrorIs(t, svc.DeleteItem(bobCtx, item.ID), ErrItemNotFound)

	// And the item survives the attempt.
	require.Contains(t, svc.Recall(aliceCtx, "\u7231\u4e3d\u4e1d\u7684\u79d8\u5bc6").Prompt,
		"\u7231\u4e3d\u4e1d\u7684\u79d8\u5bc6")
}

func TestWorkspaceSwitchOffDisablesReadAndWrite(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{Kind: types.MemoryKindFact, Content: "\u8bb0\u4f4f\u7684\u4e1c\u897f"})
	require.NoError(t, err)

	tenantRepo.set(1, &types.MemoryConfig{Enabled: false})
	require.Empty(t, svc.Recall(ctx, "\u8bb0\u4f4f\u7684\u4e1c\u897f").Prompt)
	_, err = svc.Remember(ctx, types.MemoryItem{Kind: types.MemoryKindFact, Content: "\u65b0\u7684\u4e1c\u897f"})
	require.ErrorIs(t, err, ErrMemoryDisabled)
}

func TestUserSwitchOffDisablesReadAndWrite(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{Kind: types.MemoryKindFact, Content: "\u8bb0\u4f4f\u7684\u4e1c\u897f"})
	require.NoError(t, err)

	require.NoError(t, svc.SetEnabled(ctx, false))
	require.Empty(t, svc.Recall(ctx, "\u8bb0\u4f4f\u7684\u4e1c\u897f").Prompt)
	_, err = svc.Remember(ctx, types.MemoryItem{Kind: types.MemoryKindFact, Content: "\u65b0\u7684\u4e1c\u897f"})
	require.ErrorIs(t, err, ErrMemoryDisabled)

	// Turning it back on restores what was stored: an opt out pauses memory,
	// it does not erase it.
	require.NoError(t, svc.SetEnabled(ctx, true))
	require.Contains(t, svc.Recall(ctx, "\u8bb0\u4f4f\u7684\u4e1c\u897f").Prompt, "\u8bb0\u4f4f\u7684\u4e1c\u897f")
}

func TestAgentOptOutDisablesRecall(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindPreference,
		Content: "\u53ea\u8981\u4e2d\u6587\u56de\u7b54",
	})
	require.NoError(t, err)

	disabled := false
	agentCtx := types.ApplyAgentMemoryPreference(ctx, &disabled)
	require.Empty(t, svc.Recall(agentCtx, "\u5e2e\u6211\u5199\u4e2a\u51fd\u6570").Prompt)
	require.Contains(t, svc.Recall(ctx, "\u5e2e\u6211\u5199\u4e2a\u51fd\u6570").Prompt,
		"\u53ea\u8981\u4e2d\u6587\u56de\u7b54")
}

func TestRecallWithoutPrincipalIsEmpty(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	tenantRepo.set(1, &types.MemoryConfig{Enabled: true})
	ctx := context.WithValue(t.Context(), types.TenantIDContextKey, uint64(1))
	require.Empty(t, svc.Recall(ctx, "\u4efb\u4f55\u95ee\u9898").Prompt,
		"a request with no principal has no memory space to read")
}

func TestCapacityCapArchivesLowestRanked(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{Enabled: true, WriteMode: types.MemoryWriteAuto, MaxItems: 3})

	// The important one is written first so recency alone would evict it.
	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u5173\u952e\u4e8b\u5b9e", Content: "\u6700\u91cd\u8981\u7684\u4e8b\u5b9e",
		Importance: 5,
	})
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err := svc.Remember(ctx, types.MemoryItem{
			Kind:       types.MemoryKindFact,
			Topic:      fmt.Sprintf("\u6b21\u8981\u4e8b\u5b9e-%d", i),
			Content:    fmt.Sprintf("\u6b21\u8981\u4e8b\u5b9e %d", i),
			Importance: 1,
		})
		require.NoError(t, err)
	}

	active, total, err := svc.ListItems(ctx, types.MemoryStatusActive, 50, 0)
	require.NoError(t, err)
	require.Equal(t, int64(3), total, "the cap must be enforced")

	var kept []string
	for _, item := range active {
		kept = append(kept, item.Content)
	}
	require.Contains(t, kept, "\u6700\u91cd\u8981\u7684\u4e8b\u5b9e", "importance must outrank recency")

	// Overflow is archived, not deleted, so it stays visible in the manager.
	_, archivedTotal, err := svc.ListItems(ctx, types.MemoryStatusArchived, 50, 0)
	require.NoError(t, err)
	require.Equal(t, int64(3), archivedTotal)
}

func TestClearForgetsEverything(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	for i := 0; i < 3; i++ {
		_, err := svc.Remember(ctx, types.MemoryItem{
			Kind: types.MemoryKindFact, Topic: fmt.Sprintf("k%d", i), Content: fmt.Sprintf("\u4e8b\u5b9e %d", i),
		})
		require.NoError(t, err)
	}
	removed, err := svc.Clear(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), removed)

	require.Empty(t, svc.Recall(ctx, "\u4e8b\u5b9e").Prompt)
	settings, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.Zero(t, settings.ItemCount)

	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, InterestThreshold: 3,
	})
	svc.ObserveQuestionTopics(ctx, []string{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"})
	_, err = svc.Clear(ctx)
	require.NoError(t, err)
	topics, total, err := svc.ListTopics(ctx, 10, 0)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, topics)
}

// TestClearTombstonesLiveMemoriesFirst pins which memories get the bounded
// rejection budget. max_items caps active memories only, so superseded and
// archived rows accumulate without limit and a long-lived store easily holds
// more rows than MaxMemoryTombstones. Spending the budget on whatever happened
// to be newest left the memory the user was actually being served with no
// tombstone, free to be written straight back.
func TestClearTombstonesLiveMemoriesFirst(t *testing.T) {
	svc, db, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	// The one memory still in use, deliberately the oldest row in the store.
	require.NoError(t, db.Create(&types.MemoryItem{
		ID: "live-1", TenantID: 1, SubjectID: "web_user:alice",
		Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93", Content: "\u751f\u4ea7\u5e93\u662f PostgreSQL",
		NormalizedKey: "\u6570\u636e\u5e93|\u751f\u4ea7\u5e93\u662f postgresql", Status: types.MemoryStatusActive,
		ValidFrom: time.Now().Add(-24 * time.Hour),
	}).Error)

	// Enough newer dead rows to exhaust the budget on their own.
	for i := 0; i < types.MaxMemoryTombstones+10; i++ {
		require.NoError(t, db.Create(&types.MemoryItem{
			ID: fmt.Sprintf("dead-%d", i), TenantID: 1, SubjectID: "web_user:alice",
			Kind: types.MemoryKindFact, Content: fmt.Sprintf("\u65e7\u7684\u8bf4\u6cd5 %d", i),
			NormalizedKey: fmt.Sprintf("\u65e7|%d", i), Status: types.MemoryStatusSuperseded,
			ValidFrom: time.Now(),
		}).Error)
	}

	_, err := svc.Clear(ctx)
	require.NoError(t, err)

	_, err = svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93", Content: "\u751f\u4ea7\u5e93\u662f PostgreSQL",
	})
	require.ErrorIs(t, err, ErrPreviouslyForgotten,
		"the memory that was in use must keep its tombstone, not be crowded out by dead rows")
}

func TestGetSettingsReportsMergedState(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{Kind: types.MemoryKindFact, Content: "\u4e00\u6761\u8bb0\u5fc6"})
	require.NoError(t, err)

	settings, err := svc.GetSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.WorkspaceEnabled)
	require.True(t, settings.UserEnabled)
	require.True(t, settings.Effective)
	require.Equal(t, 1, settings.ItemCount)

	require.NoError(t, svc.SetEnabled(ctx, false))
	settings, err = svc.GetSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.WorkspaceEnabled)
	require.False(t, settings.UserEnabled)
	require.False(t, settings.Effective, "either switch being off must make the effective state off")
}

func TestUpdateItemMarksMemoryAsManual(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	item, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindPreference, Content: "\u559c\u6b22\u5f88\u957f\u7684\u89e3\u91ca",
	})
	require.NoError(t, err)

	updated, err := svc.UpdateItem(ctx, item.ID, "\u559c\u6b22\u7b80\u77ed\u7684\u89e3\u91ca", 5)
	require.NoError(t, err)
	require.Equal(t, "\u559c\u6b22\u7b80\u77ed\u7684\u89e3\u91ca", updated.Content)
	require.Equal(t, 5, updated.Importance)
	require.Equal(t, types.MemoryOriginManual, updated.Origin,
		"a corrected memory must be marked manual so extraction does not undo it")

	require.Contains(t, svc.Recall(ctx, "\u968f\u4fbf\u95ee\u70b9\u4ec0\u4e48").Prompt,
		"\u559c\u6b22\u7b80\u77ed\u7684\u89e3\u91ca")
}

func TestResidentBlockSurvivesCacheLoss(t *testing.T) {
	svc, db, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Content: "\u5728\u505a\u533b\u7597\u5f71\u50cf",
	})
	require.NoError(t, err)

	// Simulate a row written before a block-rebuild failure.
	require.NoError(t, db.Model(&types.MemorySubject{}).
		Where("1 = 1").Update("block_text", "").Error)

	require.Contains(t, svc.Recall(ctx, "\u968f\u4fbf\u95ee").Prompt, "\u5728\u505a\u533b\u7597\u5f71\u50cf",
		"an empty block cache must not silently drop the user's memories")
}

// An interest is a standing property of the person, so it has to be present
// whatever they ask. The case that forced this: "what am I focused on" shares
// no words with "Xiaowei SDK device onboarding" (in Chinese), so query matching can never reach it, and
// the assistant claimed to know nothing about a memory the user could see
// listed in the memory manager.
func TestInterestIsPresentRegardlessOfTheQuestion(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.CreateItem(ctx, types.MemoryKindInterest, "\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165", 3)
	require.NoError(t, err)

	require.Contains(t, svc.Recall(ctx, "\u6211\u5173\u6ce8\u54ea\u4e9b\u4e8b\u60c5\uff1f").Prompt,
		"\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165")
	require.Contains(t, svc.Recall(ctx, "\u6211\u5173\u6ce8\u54ea\u4e9b\u4e8b\u60c5\uff1f").Prompt, "Long-term focus")
	require.Contains(t, svc.Recall(ctx, "\u4eca\u5929\u5929\u6c14\u600e\u4e48\u6837").Prompt,
		"\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165")
}

// Being injected and being reported are different things. An interest that is
// present only because the cap left room is standing background; listing it as
// a memory this answer recalled would put something unrelated to the question
// on the chat timeline every single turn.
func TestUnrelatedInterestIsInjectedButNotReported(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	_, err := svc.CreateItem(ctx, types.MemoryKindInterest, "\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165", 3)
	require.NoError(t, err)

	unrelated := svc.Recall(ctx, "\u4eca\u5929\u5929\u6c14\u600e\u4e48\u6837")
	require.Contains(t, unrelated.Prompt, "\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165")
	require.Empty(t, unrelated.Items)

	related := svc.Recall(ctx, "\u5c0f\u5faeSDK\u600e\u4e48\u63a5\u5165\u8bbe\u5907")
	require.Contains(t, related.Prompt, "\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165")
	require.Len(t, related.Items, 1, "an interest the question matches is a real recall")
}

// Relevance does not decide whether interests appear, it decides which ones
// survive the cap.
func TestInterestsBeyondTheCapAreChosenByRelevance(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	topics := []string{
		"\u533b\u5b66\u5f71\u50cf\u5206\u5272", "\u6570\u636e\u5e93\u8c03\u4f18",
		"\u524d\u7aef\u6784\u5efa\u901f\u5ea6", "\u6307\u6807\u76d1\u63a7\u544a\u8b66",
		"\u65e5\u5fd7\u91c7\u96c6\u94fe\u8def", "\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165",
	}
	for _, topic := range topics {
		_, err := svc.CreateItem(ctx, types.MemoryKindInterest, topic, 3)
		require.NoError(t, err)
	}

	recall := svc.Recall(ctx, "\u5c0f\u5faeSDK\u600e\u4e48\u63a5\u5165\u8bbe\u5907")
	require.Contains(t, recall.Prompt, "\u5c0f\u5faeSDK\u8bbe\u5907\u63a5\u5165",
		"the interest the question is about must not be the one dropped by the cap")

	var injected int
	for _, topic := range topics {
		if strings.Contains(recall.Prompt, topic) {
			injected++
		}
	}
	require.Equal(t, types.MemoryResidentInterestMaxItems, injected,
		"the block carries at most the cap, not every interest ever promoted")
}

func TestCreateItemFromManagerGoesThroughTheWritePath(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.CreateItem(ctx, types.MemoryKindPreference, "  \u56de\u7b54\u8bf7\u7528\u4e2d\u6587  \n", 0)
	require.NoError(t, err)

	items, _, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "\u56de\u7b54\u8bf7\u7528\u4e2d\u6587", items[0].Content,
		"manual input must be sanitized like any other")
	require.Equal(t, types.MemoryOriginManual, items[0].Origin)
	require.False(t, strings.Contains(items[0].Content, "\n"))
}
