package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func TestMemoryConsistencyPendingMustNotReplaceActive(t *testing.T) {
	s, _, tr := newMemoryHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	scope := scopeFor(t, ctx)
	old, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
		Content: "\u6211\u662f\u540e\u7aef\u5de5\u7a0b\u5e08", Origin: types.MemoryOriginManual,
	})
	require.NoError(t, err)
	proposal, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
		Content: "\u53ef\u80fd\u662f\u4ea7\u54c1\u7ecf\u7406", Origin: types.MemoryOriginExtracted, Inferred: true,
	})
	require.NoError(t, err)
	require.Equal(t, types.MemoryStatusPending, proposal.Status)
	current, err := s.repo.GetItem(ctx, scope, old.ID)
	require.NoError(t, err)
	require.Equal(t, types.MemoryStatusActive, current.Status, "unconfirmed inference must not retire a confirmed fact")
}

func TestMemoryConsistencyUpdateTargetMustWinOverChangedTopic(t *testing.T) {
	s, _, tr := newMemoryHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	scope := scopeFor(t, ctx)
	old, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u751f\u4ea7\u6570\u636e\u5e93",
		Content: "\u7ebf\u4e0a\u4f7f\u7528 MySQL",
	})
	require.NoError(t, err)
	zero := 0
	require.NoError(t, s.applyDecisions(ctx, scope, s.workspaceConfig(ctx, 1),
		transcriptSegment{lines: []transcriptLine{{
			sessionID: "s", messageID: "m",
			content: "\u8fc1\u79fb\u5230 PostgreSQL",
		}}}, []*types.MemoryItem{old}, []extractionDecision{{
			Action: "update",
			Target: &zero, Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93\u9009\u578b",
			Content: "\u7ebf\u4e0a\u5df2\u8fc1\u79fb\u5230 PostgreSQL",
		}}))
	_, total, err := s.ListItems(ctx, types.MemoryStatusActive, 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "indexed update must not leave contradictory old memory active")
}

func TestMemoryConsistencyCappedSessionsMustBeFollowedUp(t *testing.T) {
	s, tr, msg, model, q := newExtractionHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	model.response = `{"memories":[]}`
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("session-%d", i)
		msg.set(id, []*types.Message{userMessage(id, fmt.Sprintf("unique-session-marker-%d", i), base.Add(time.Duration(i)*time.Minute))})
		s.ScheduleExtraction(ctx, id, "m", "model")
	}
	drainExtractions(t, s, q)
	require.True(t, strings.Contains(model.seenTranscripts(), "unique-session-marker-3"), "fourth claimed session must survive the three-segment cap")
}

func TestMemoryConsistencyRetryMustKeepAllClaimedSessions(t *testing.T) {
	s, tr, msg, model, q := newExtractionHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	model.response = `{"memories":[]}`
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("session-%d", i)
		msg.set(id, []*types.Message{userMessage(id, fmt.Sprintf("retry-session-marker-%d", i), base.Add(time.Duration(i)*time.Minute))})
		s.ScheduleExtraction(ctx, id, "m", "model")
	}
	task := q.pop()
	require.NotNil(t, task)
	model.failNext = true
	require.Error(t, s.Handle(context.Background(), task))
	require.NoError(t, s.Handle(context.Background(), task))
	drainExtractions(t, s, q)
	require.True(t, strings.Contains(model.seenTranscripts(), "retry-session-marker-1"), "retry must retain the second drained session")
}

func TestMemoryConsistencyManualEditMustRefreshEmbedding(t *testing.T) {
	s, tr, _ := newVectorHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	scope := scopeFor(t, ctx)
	old, err := s.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Content: "\u56de\u7b54\u76f4\u63a5\u7ed9\u7ed3\u8bba",
	})
	require.NoError(t, err)
	_, err = s.UpdateItem(ctx, old.ID, "\u751f\u4ea7\u73af\u5883\u9700\u8981\u8c03\u5927\u8fde\u63a5\u6c60", 3)
	require.NoError(t, err)
	s.backfillEmbeddings(ctx, scope, s.workspaceConfig(ctx, 1))
	vectors, err := s.repo.ItemEmbeddings(ctx, scope, []string{old.ID}, "embed-1")
	require.NoError(t, err)
	require.Equal(t, []float32{0, 1, 0}, vectors[old.ID], "edit and maintenance must not leave the old semantic vector")
}

func TestMemoryConsistencyProposalConfirmationAndRejection(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		t.Run(fmt.Sprint(confirm), func(t *testing.T) {
			s, _, tr := newMemoryHarness(t)
			ctx := enabledCtx(t, tr, 1, "alice")
			scope := scopeFor(t, ctx)
			old, err := s.Remember(ctx, types.MemoryItem{
				Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
				Content: "\u6211\u662f\u5de5\u7a0b\u5e08", Origin: types.MemoryOriginManual,
			})
			require.NoError(t, err)
			proposal, err := s.Remember(ctx, types.MemoryItem{
				Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
				Content: "\u53ef\u80fd\u662f\u7ecf\u7406", Inferred: true,
			})
			require.NoError(t, err)
			require.Equal(t, old.ID, proposal.ReplacesID)
			if confirm {
				_, err = s.ConfirmItem(ctx, proposal.ID)
				require.NoError(t, err)
				_, err = s.ConfirmItem(ctx, proposal.ID)
				require.NoError(t, err, "confirmation is idempotent")
			} else {
				require.NoError(t, s.RejectItem(ctx, proposal.ID))
			}
			current, err := s.repo.GetItem(ctx, scope, old.ID)
			require.NoError(t, err)
			if confirm {
				require.Equal(t, types.MemoryStatusSuperseded, current.Status)
				require.Equal(t, proposal.ID, current.SupersededBy)
			} else {
				require.Equal(t, types.MemoryStatusActive, current.Status)
			}
			_, total, err := s.ListItems(ctx, types.MemoryStatusActive, 20, 0)
			require.NoError(t, err)
			require.Equal(t, int64(1), total)
		})
	}
}

func TestMemoryConsistencyEditInvalidatesOldProposals(t *testing.T) {
	s, _, tr := newMemoryHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	old, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
		Content: "\u6211\u662f\u5de5\u7a0b\u5e08",
	})
	require.NoError(t, err)
	proposal, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
		Content: "\u53ef\u80fd\u662f\u7ecf\u7406", Inferred: true,
	})
	require.NoError(t, err)
	_, err = s.UpdateItem(ctx, old.ID, "\u6211\u662f\u8bbe\u8ba1\u5e08", 4)
	require.NoError(t, err)
	_, err = s.ConfirmItem(ctx, proposal.ID)
	require.ErrorIs(t, err, types.ErrMemoryConflict)
	items, _, err := s.ListItems(ctx, types.MemoryStatusActive, 20, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "\u6211\u662f\u8bbe\u8ba1\u5e08", items[0].Content)
}

func TestMemoryConsistencyReplacingProposalKeepsOriginalTarget(t *testing.T) {
	s, _, tr := newMemoryHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	old, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
		Content: "\u6211\u662f\u5de5\u7a0b\u5e08",
	})
	require.NoError(t, err)
	first, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
		Content: "\u53ef\u80fd\u662f\u7ecf\u7406", Inferred: true,
	})
	require.NoError(t, err)
	second, err := s.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindProfile, Topic: "\u804c\u4e1a",
		Content: "\u53ef\u80fd\u662f\u8bbe\u8ba1\u5e08", Inferred: true,
	})
	require.NoError(t, err)
	require.Equal(t, old.ID, second.ReplacesID)
	_, err = s.ConfirmItem(ctx, first.ID)
	require.ErrorIs(t, err, types.ErrMemoryConflict)
	_, err = s.ConfirmItem(ctx, second.ID)
	require.NoError(t, err)
	_, total, err := s.ListItems(ctx, types.MemoryStatusActive, 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
}

func TestMemoryConsistencyTimestampTiesAndLargePendingQueue(t *testing.T) {
	for _, manySessions := range []bool{false, true} {
		t.Run(fmt.Sprint(manySessions), func(t *testing.T) {
			s, tr, msg, model, q := newExtractionHarness(t)
			ctx := enabledCtx(t, tr, 1, "alice")
			model.response = `{"memories":[]}`
			at := time.Now().Add(-time.Hour)
			var rows []*types.Message
			for i := 0; i < 85; i++ {
				id := fmt.Sprintf("marker-%03d", i)
				session := "single"
				if manySessions {
					session = id
				}
				row := userMessage(session, id, at)
				if manySessions {
					msg.set(session, []*types.Message{row})
				} else {
					rows = append(rows, row)
					msg.set(session, rows)
				}
				s.ScheduleExtraction(ctx, session, id, "model")
			}
			drainExtractions(t, s, q)
			var seen string
			for _, prompt := range model.prompts {
				seen += transcriptBlock(prompt)
			}
			for i := 0; i < 85; i++ {
				require.Equal(t, 1, strings.Count(seen, fmt.Sprintf("marker-%03d", i)), "each message must appear exactly once as extractable input")
			}
		})
	}
}

func TestMemoryConsistencyGapDoesNotSkipNextSegment(t *testing.T) {
	s, tr, msg, model, q := newExtractionHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	model.response = `{"memories":[]}`
	at := time.Now().Add(-24 * time.Hour)
	var rows []*types.Message
	for i := 0; i < 7; i++ {
		rows = append(rows, userMessage("s", fmt.Sprintf("gap-marker-%d", i), at.Add(time.Duration(i)*2*time.Hour)))
	}
	msg.set("s", rows)
	s.ScheduleExtraction(ctx, "s", "m", "model")
	drainExtractions(t, s, q)
	var seen string
	for _, prompt := range model.prompts {
		seen += transcriptBlock(prompt)
	}
	for i := 0; i < 7; i++ {
		require.Equal(t, 1, strings.Count(seen, fmt.Sprintf("gap-marker-%d", i)))
	}
}

func TestMemoryConsistencyLeaseAndConcurrentEnqueue(t *testing.T) {
	s, _, tr := newMemoryHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	scope := scopeFor(t, ctx)
	_, err := s.repo.EnsureSubject(ctx, scope)
	require.NoError(t, err)
	_, _, err = s.repo.EnqueuePendingSession(ctx, scope, "s", time.Minute)
	require.NoError(t, err)
	batch, err := s.repo.ClaimPendingSessions(ctx, scope, "s", "first", time.Minute)
	require.NoError(t, err)
	require.Len(t, batch.Sessions, 1)
	duplicate, err := s.repo.ClaimPendingSessions(ctx, scope, "s", "second", time.Minute)
	require.NoError(t, err)
	require.NotNil(t, duplicate)
	require.False(t, duplicate.RetryAt.IsZero())
	require.Empty(t, duplicate.Sessions)
	_, queued, err := s.repo.EnqueuePendingSession(ctx, scope, "s", time.Minute)
	require.NoError(t, err)
	require.False(t, queued)
	require.NoError(t, s.repo.CheckpointExtraction(ctx, scope, "first", batch.Sessions[0], types.MemoryMessageCursor{At: time.Now(), ID: "a"}, true))
	require.NoError(t, s.repo.FinishExtraction(ctx, scope, "first"))
	next, err := s.repo.ClaimPendingSessions(ctx, scope, "s", "next", time.Minute)
	require.NoError(t, err)
	require.Len(t, next.Sessions, 1, "a turn arriving while the batch runs must remain pending")
	require.NoError(t, s.repo.ReleaseExtractionSlot(ctx, scope, "first"))
	require.ErrorIs(t, s.repo.CheckpointExtraction(ctx, scope, "first", batch.Sessions[0], types.MemoryMessageCursor{}, true), types.ErrMemoryExtractionLeaseLost)
	require.NoError(t, s.repo.CheckpointExtraction(ctx, scope, "next", next.Sessions[0], next.Sessions[0].Cursor, true))
	require.NoError(t, s.repo.FinishExtraction(ctx, scope, "next"))
	empty, err := s.repo.ClaimPendingSessions(ctx, scope, "s", "duplicate", time.Minute)
	require.NoError(t, err)
	require.Nil(t, empty, "redelivering a finished task must be a no-op")
}

func TestMemoryConsistencyEmbeddingFailureAndStaleCompletion(t *testing.T) {
	s, tr, model := newVectorHarness(t)
	ctx := enabledCtx(t, tr, 1, "alice")
	scope := scopeFor(t, ctx)
	old, err := s.Remember(ctx, types.MemoryItem{
		Kind:    types.MemoryKindFact,
		Content: "\u56de\u7b54\u76f4\u63a5\u7ed9\u7ed3\u8bba",
	})
	require.NoError(t, err)
	model.embedder.fail = true
	_, err = s.UpdateItem(ctx, old.ID, "\u751f\u4ea7\u73af\u5883\u9700\u8981\u8c03\u5927\u8fde\u63a5\u6c60", 3)
	require.NoError(t, err)
	vectors, err := s.repo.ItemEmbeddings(ctx, scope, []string{old.ID}, "embed-1")
	require.NoError(t, err)
	require.Empty(t, vectors, "failed refresh must not leave a stale vector searchable")
	model.embedder.fail = false
	s.storeItemEmbedding(ctx, scope, s.workspaceConfig(ctx, 1), old)
	vectors, err = s.repo.ItemEmbeddings(ctx, scope, []string{old.ID}, "embed-1")
	require.NoError(t, err)
	require.Empty(t, vectors, "a slow embedding of old content must be discarded")
	require.Equal(t, 1, s.backfillEmbeddings(ctx, scope, s.workspaceConfig(ctx, 1)))
	vectors, err = s.repo.ItemEmbeddings(ctx, scope, []string{old.ID}, "embed-1")
	require.NoError(t, err)
	require.Equal(t, []float32{0, 1, 0}, vectors[old.ID])
}
