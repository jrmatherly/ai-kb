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

// This file covers the extraction-quality work: what the model is shown, how
// its answer is interpreted, and what is refused. Each test names the concrete
// failure it exists to prevent, because most of them were found by probing the
// running system rather than by reading the code.

// ---------------------------------------------------------------------------
// A memory the user deleted must not come back
// ---------------------------------------------------------------------------

// TestDeletedMemoryIsNotReExtracted is the one users would hit first. An
// explicit "remember X" is stored immediately, and the debounced distillation
// for that same turn reads the same message minutes later - so deleting the
// memory in between used to be undone by the system itself.
func TestDeletedMemoryIsNotReExtracted(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})

	stored, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u751f\u4ea7\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17", Origin: types.MemoryOriginExplicit,
	})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteItem(ctx, stored.ID))

	messages.set("session-1", []*types.Message{
		{
			ID: "m1", SessionID: "session-1", Role: "user",
			Content:   "\u8bb0\u4f4f\uff1a\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17",
			CreatedAt: time.Now().Add(-time.Minute),
		},
	})
	models.response = `{"memories":[{"action":"add","target":null,"kind":"fact",` +
		"\"topic\":\"\u751f\u4ea7\u6570\u636e\u5e93\",\"content\":\"\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL " +
		"17\",\"source\":1}]}"
	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	_, total, err := svc.ListItems(ctx, "", 10, 0)
	require.NoError(t, err)
	require.Zero(t, total, "a memory the user deleted must not be re-extracted")
}

// TestRewordedMemoryDoesNotComeBack is the case the fingerprint alone misses:
// distillation re-reads the same message and words the statement slightly
// differently, so it hashes differently and used to slip straight through.
func TestRewordedMemoryDoesNotComeBack(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})

	stored, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact,
		Content: "\u6211\u4eec\u7684\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL 17\uff0c\u90e8\u7f72\u5728" +
			"\u6cd5\u5170\u514b\u798f",
		Origin: types.MemoryOriginExplicit, SourceMessageID: "m1", SourceSessionID: "session-1",
	})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteItem(ctx, stored.ID))

	messages.set("session-1", []*types.Message{
		{
			ID: "m1", SessionID: "session-1", Role: "user",
			Content: "\u8bb0\u4f4f\uff1a\u6211\u4eec\u7684\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL " +
				"17\uff0c\u90e8\u7f72\u5728\u6cd5\u5170\u514b\u798f",
			CreatedAt: time.Now().Add(-time.Minute),
		},
	})
	// Same fact, different wording: a different fingerprint.
	models.response = `{"memories":[{"action":"add","target":null,"kind":"fact",` +
		"\"topic\":\"\u751f\u4ea7\u6570\u636e\u5e93\",\"content\":\"\u751f\u4ea7\u6570\u636e\u5e93\u662f PostgreSQL " +
		"17\uff0c\u90e8\u7f72\u5728\u6cd5\u5170\u514b\u798f\",\"source\":1}]}"
	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	_, total, err := svc.ListItems(ctx, "", 10, 0)
	require.NoError(t, err)
	require.Zero(t, total,
		"a re-worded restatement from the same rejected message must not come back")
}

// TestSayingItAgainLaterStillWorks keeps the suppression from turning into a
// permanent ban: the user asking again is not the system re-deriving.
func TestSayingItAgainLaterStillWorks(t *testing.T) {
	svc, tenantRepo, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	stored, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Content: "\u751f\u4ea7\u5e93\u662f PostgreSQL 17",
		Origin: types.MemoryOriginExplicit, SourceMessageID: "m1",
	})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteItem(ctx, stored.ID))

	// A later turn, a later message: this is the user asking again.
	again, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Content: "\u751f\u4ea7\u5e93\u786e\u5b9e\u662f PostgreSQL 17",
		Origin: types.MemoryOriginExplicit, SourceMessageID: "m9",
	})
	require.NoError(t, err)
	require.NotEmpty(t, again.ID)
}

func TestForgottenTopicsAreShownToTheModel(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})

	stored, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u5bb6\u5ead\u4f4f\u5740",
		Content: "\u4f4f\u5728\u676d\u5dde\u897f\u6e56\u533a",
	})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteItem(ctx, stored.ID))

	messages.set("session-1", []*types.Message{
		{ID: "m1", SessionID: "session-1", Role: "user", Content: "\u968f\u4fbf\u804a\u804a", CreatedAt: time.Now()},
	})
	models.response = `{"memories":[]}`
	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	// The fingerprint check only catches an identical restatement, so the model
	// is also told which topics were rejected.
	require.Contains(t, models.lastPrompt, "\u5bb6\u5ead\u4f4f\u5740")
	require.NotContains(t, models.lastPrompt, "\u4f4f\u5728\u676d\u5dde\u897f\u6e56\u533a",
		"a tombstone must not retain the statement the user asked to forget")
}

func TestClearLeavesTombstones(t *testing.T) {
	svc, tenantRepo, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93", Content: "\u751f\u4ea7\u5e93\u662f PostgreSQL",
	})
	require.NoError(t, err)
	_, err = svc.Clear(ctx)
	require.NoError(t, err)

	_, err = svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u6570\u636e\u5e93", Content: "\u751f\u4ea7\u5e93\u662f PostgreSQL",
	})
	require.ErrorIs(t, err, ErrPreviouslyForgotten,
		"clearing is a rejection of everything, not just a bulk delete")
}

func TestForgettingDoesNotBlockGenuinelyNewInformation(t *testing.T) {
	svc, tenantRepo, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	stored, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u5728\u7528\u7684\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u5e93\u662f MySQL",
	})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteItem(ctx, stored.ID))

	// Same topic, different statement: the user moved on, and suppressing this
	// would make deleting one memory quietly ban a subject forever.
	updated, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u5728\u7528\u7684\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u5e93\u5df2\u7ecf\u8fc1\u5230 PostgreSQL",
	})
	require.NoError(t, err)
	require.Equal(t, "\u751f\u4ea7\u5e93\u5df2\u7ecf\u8fc1\u5230 PostgreSQL", updated.Content)
}

// ---------------------------------------------------------------------------
// Provenance
// ---------------------------------------------------------------------------

// TestProvenancePointsAtTheRightMessage covers the regression that arrived with
// multi-session runs: every memory used to be attributed to the turn that
// triggered the run, which could be a different conversation entirely.
func TestProvenancePointsAtTheRightMessage(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})

	base := time.Now().Add(-time.Hour)
	messages.set("session-a", []*types.Message{
		{
			ID: "msg-a1", SessionID: "session-a", Role: "user",
			Content: "\u6211\u5728\u505a\u533b\u7597\u5f71\u50cf", CreatedAt: base,
		},
	})
	messages.set("session-b", []*types.Message{
		{
			ID: "msg-b1", SessionID: "session-b", Role: "user",
			Content: "\u987a\u4fbf\u95ee\u4e0b\u5929\u6c14", CreatedAt: base.Add(time.Second),
		},
	})
	models.response = `{"memories":[{"action":"add","target":null,"kind":"profile",` +
		"\"topic\":\"\u804c\u4e1a\",\"content\":\"\u5728\u505a\u533b\u7597\u5f71\u50cf\",\"source\":1}]}"

	svc.ScheduleExtraction(ctx, "session-a", "trigger-msg", "model-1")
	svc.ScheduleExtraction(ctx, "session-b", "trigger-msg", "model-1")
	drainExtractions(t, svc, enqueuer)

	items, _, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	for _, item := range items {
		if item.Content != "\u5728\u505a\u533b\u7597\u5f71\u50cf" {
			continue
		}
		require.Equal(t, "session-a", item.SourceSessionID)
		require.Equal(t, "msg-a1", item.SourceMessageID)
		return
	}
	t.Fatal("the extracted memory was not found")
}

func TestOutOfRangeSourceFallsBackInsideTheSegment(t *testing.T) {
	segment := transcriptSegment{lines: []transcriptLine{
		{sessionID: "s1", messageID: "m1", content: "\u7b2c\u4e00\u53e5"},
		{sessionID: "s1", messageID: "m2", content: "\u7b2c\u4e8c\u53e5"},
	}}
	bogus := 99
	resolved := extractionDecision{Source: &bogus}.resolveSource(segment)
	require.Equal(t, "m1", resolved.messageID,
		"a hallucinated line number must still land inside the right conversation")
}

// ---------------------------------------------------------------------------
// Prior context
// ---------------------------------------------------------------------------

// TestPriorContextResolvesAReferringStatement: a run sees only what is new, so
// without a lead-in a turn like "let's just use the earlier one" has nothing to resolve against.
func TestPriorContextResolvesAReferringStatement(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})
	models.response = `{"memories":[]}`

	base := time.Now().Add(-time.Hour)
	messages.set("session-1", []*types.Message{
		{
			ID: "m1", SessionID: "session-1", Role: "user",
			Content: "\u6211\u5728\u8bc4\u4f30 PostgreSQL \u548c MySQL", CreatedAt: base,
		},
	})
	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	messages.set("session-1", []*types.Message{
		{
			ID: "m1", SessionID: "session-1", Role: "user",
			Content: "\u6211\u5728\u8bc4\u4f30 PostgreSQL \u548c MySQL", CreatedAt: base,
		},
		{
			ID: "m2", SessionID: "session-1", Role: "user",
			Content: "\u5c31\u7528\u524d\u9762\u90a3\u4e2a\u5427", CreatedAt: base.Add(time.Minute),
		},
	})
	svc.ScheduleExtraction(ctx, "session-1", "m2", "model-1")
	drainExtractions(t, svc, enqueuer)

	require.Contains(t, models.lastPrompt, "\u6211\u5728\u8bc4\u4f30",
		"a referring statement needs the turn it refers to")
	require.NotContains(t, transcriptBlock(models.lastPrompt), "\u6211\u5728\u8bc4\u4f30",
		"context must not be extracted from a second time")
}

// ---------------------------------------------------------------------------
// Segmentation
// ---------------------------------------------------------------------------

// TestLongSilenceStartsANewSegment keeps one call from having to make sense of
// two unrelated situations at once.
func TestLongSilenceStartsANewSegment(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})
	models.response = `{"memories":[]}`

	base := time.Now().Add(-24 * time.Hour)
	messages.set("session-1", []*types.Message{
		{ID: "m1", SessionID: "session-1", Role: "user", Content: "\u4e0a\u5348\u804a\u7684\u4e8b", CreatedAt: base},
		{
			ID: "m2", SessionID: "session-1", Role: "user", Content: "\u665a\u4e0a\u804a\u7684\u4e8b",
			CreatedAt: base.Add(6 * time.Hour),
		},
	})
	svc.ScheduleExtraction(ctx, "session-1", "m2", "model-1")
	drainExtractions(t, svc, enqueuer)

	require.Equal(t, 2, models.calls, "a six-hour gap must split the run into two calls")
	seen := models.seenTranscripts()
	require.Contains(t, seen, "\u4e0a\u5348\u804a\u7684\u4e8b")
	require.Contains(t, seen, "\u665a\u4e0a\u804a\u7684\u4e8b")
}

func TestSeparateSessionsAreSeparateSegments(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})
	models.response = `{"memories":[]}`

	base := time.Now().Add(-time.Hour)
	messages.set("session-a", []*types.Message{
		{ID: "a1", SessionID: "session-a", Role: "user", Content: "\u4f1a\u8bddA\u7684\u8bdd", CreatedAt: base},
	})
	messages.set("session-b", []*types.Message{
		{
			ID: "b1", SessionID: "session-b", Role: "user", Content: "\u4f1a\u8bddB\u7684\u8bdd",
			CreatedAt: base.Add(time.Second),
		},
	})
	svc.ScheduleExtraction(ctx, "session-a", "a1", "model-1")
	svc.ScheduleExtraction(ctx, "session-b", "b1", "model-1")
	drainExtractions(t, svc, enqueuer)

	require.Equal(t, 2, models.calls)
	for _, prompt := range models.prompts {
		block := transcriptBlock(prompt)
		require.False(t, strings.Contains(block, "\u4f1a\u8bddA\u7684\u8bdd") && strings.Contains(block,
			"\u4f1a\u8bddB\u7684\u8bdd"),
			"two conversations must not be merged into one call")
	}
}

// TestSegmentCapStillCoversEverything: capping the calls one run makes must
// delay work, never lose it.
func TestSegmentCapStillCoversEverything(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})
	models.response = `{"memories":[]}`

	base := time.Now().Add(-100 * time.Hour)
	var transcript []*types.Message
	for i := 0; i < extractMaxSegmentsPerRun*2+1; i++ {
		transcript = append(transcript, &types.Message{
			ID: fmt.Sprintf("m%d", i), SessionID: "session-1", Role: "user",
			Content:   fmt.Sprintf("\u7b2c%d\u6bb5\u7684\u8bdd", i),
			CreatedAt: base.Add(time.Duration(i) * 3 * time.Hour),
		})
	}
	messages.set("session-1", transcript)
	svc.ScheduleExtraction(ctx, "session-1", "m0", "model-1")
	drainExtractions(t, svc, enqueuer)

	seen := models.seenTranscripts()
	for i := 0; i < extractMaxSegmentsPerRun*2+1; i++ {
		require.Contains(t, seen, fmt.Sprintf("\u7b2c%d\u6bb5\u7684\u8bdd", i),
			"segment %d was never read", i)
	}
}

// ---------------------------------------------------------------------------
// Decision handling
// ---------------------------------------------------------------------------

// TestEmptyActionIsIgnored: a truncated response used to be treated as add,
// which turned a broken model reply into a silent write.
func TestEmptyActionIsIgnored(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})
	messages.set("session-1", []*types.Message{
		{
			ID: "m1", SessionID: "session-1", Role: "user", Content: "\u968f\u4fbf\u8bf4\u70b9\u4ec0\u4e48",
			CreatedAt: time.Now(),
		},
	})
	models.response = "{\"memories\":[{\"kind\":\"fact\",\"topic\":\"t\",\"content\":\"\u67d0\u4e2a\u4e8b\u5b9e\"}," +
		"{\"action\":\"none\",\"kind\":\"fact\",\"topic\":\"t2\",\"content\":\"\u53e6\u4e00\u4e2a\u4e8b\u5b9e\"}]}"

	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	_, total, err := svc.ListItems(ctx, "", 10, 0)
	require.NoError(t, err)
	require.Zero(t, total, "only an explicit add/update/delete may write")
}

// TestUpdateAddressesTheNoteByIndex is the anti-hallucination measure: a model
// that mis-types a topic would otherwise silently create a duplicate.
func TestUpdateAddressesTheNoteByIndex(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})

	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindFact, Topic: "\u5728\u7528\u7684\u6570\u636e\u5e93",
		Content: "\u751f\u4ea7\u5e93\u662f MySQL",
	})
	require.NoError(t, err)

	messages.set("session-1", []*types.Message{
		{
			ID: "m1", SessionID: "session-1", Role: "user",
			Content: "\u6211\u4eec\u8fc1\u5230 PostgreSQL \u4e86", CreatedAt: time.Now(),
		},
	})
	// The topic is deliberately misspelled; the index is what must win.
	models.response = `{"memories":[{"action":"delete","target":0,"kind":"fact",` +
		"\"topic\":\"\u5728\u7528\u7684\u8cc7\u6599\u5eab\",\"content\":\"\u4e0d\u518d\u4f7f\u7528 " +
		"MySQL\",\"source\":1}]}"

	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	_, active, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.Zero(t, active, "delete must find the note by index despite the wrong topic")
}

func TestDuplicateTopicsInOneResponseDoNotChurn(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})
	messages.set("session-1", []*types.Message{
		{ID: "m1", SessionID: "session-1", Role: "user", Content: "\u8bf4\u4e86\u4e24\u904d", CreatedAt: time.Now()},
	})
	models.response = "{\"memories\":[\n\t\t{\"action\":\"add\",\"target\":null,\"kind\":\"fact\",\"topic\":\"\u6570" +
		"\u636e\u5e93\",\"content\":\"\u7528 PostgreSQL\",\"source\":1},\n\t\t{\"action\":\"add\",\"target\":null,\"k" +
		"ind\":\"fact\",\"topic\":\"\u6570\u636e\u5e93\",\"content\":\"\u7528 PostgreSQL 17\",\"source\":1}]}"

	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	_, superseded, err := svc.ListItems(ctx, types.MemoryStatusSuperseded, 10, 0)
	require.NoError(t, err)
	require.Zero(t, superseded, "one run must not supersede its own output")
}

// ---------------------------------------------------------------------------
// Expiry
// ---------------------------------------------------------------------------

func TestExpiredTaskLeavesTheContext(t *testing.T) {
	svc, tenantRepo, _, _, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")

	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(48 * time.Hour)
	_, err := svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindTask, Topic: "\u8fc7\u671f\u7684\u4e8b",
		Content:   "\u4e0a\u5468\u8981\u4ea4\u7684\u5468\u62a5",
		ExpiresAt: &past,
	})
	require.NoError(t, err)
	_, err = svc.Remember(ctx, types.MemoryItem{
		Kind: types.MemoryKindTask, Topic: "\u5728\u505a\u7684\u4e8b",
		Content:   "\u8fd9\u5468\u8981\u4ea4\u7684\u5468\u62a5",
		ExpiresAt: &future,
	})
	require.NoError(t, err)

	prompt := svc.Recall(ctx, "\u5468\u62a5\u7684\u4e8b\u600e\u4e48\u6837\u4e86").Prompt
	require.Contains(t, prompt, "\u8fd9\u5468\u8981\u4ea4\u7684\u5468\u62a5")
	require.NotContains(t, prompt, "\u4e0a\u5468\u8981\u4ea4\u7684\u5468\u62a5",
		"an expired task must stop being recalled")
}

func TestParseExpiryRejectsUnusableDates(t *testing.T) {
	require.Nil(t, parseExpiry(""))
	require.Nil(t, parseExpiry("null"))
	require.Nil(t, parseExpiry("\u4e0b\u5468\u4e94"))
	require.Nil(t, parseExpiry("2020-01-01"), "a date already past would be stored and archived at once")
	require.NotNil(t, parseExpiry(time.Now().Add(72*time.Hour).Format("2006-01-02")))
}

// ---------------------------------------------------------------------------
// Workspace instructions
// ---------------------------------------------------------------------------

func TestWorkspaceInstructionsReachThePrompt(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
		ExtractInstructions: "\u6c38\u8fdc\u4e0d\u8981\u8bb0\u5f55\u5ba2\u6237\u7684\u59d3\u540d",
	})
	models.response = `{"memories":[]}`
	messages.set("session-1", []*types.Message{
		{ID: "m1", SessionID: "session-1", Role: "user", Content: "\u968f\u4fbf\u804a", CreatedAt: time.Now()},
	})

	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)
	require.Contains(t, models.lastPrompt, "\u6c38\u8fdc\u4e0d\u8981\u8bb0\u5f55\u5ba2\u6237\u7684\u59d3\u540d")
}

// ---------------------------------------------------------------------------
// Structured output
// ---------------------------------------------------------------------------

func TestExtractionRequestsStructuredOutput(t *testing.T) {
	svc, tenantRepo, messages, models, enqueuer := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, ExtractDelaySeconds: 5,
	})
	models.response = `{"memories":[]}`
	messages.set("session-1", []*types.Message{
		{ID: "m1", SessionID: "session-1", Role: "user", Content: "\u968f\u4fbf\u804a", CreatedAt: time.Now()},
	})

	svc.ScheduleExtraction(ctx, "session-1", "m1", "model-1")
	drainExtractions(t, svc, enqueuer)

	require.NotEmpty(t, models.lastFormat,
		"the response schema must be sent, not just described in prose")
	require.Contains(t, string(models.lastFormat), "memories")
}

var _ = context.Background
