package memory

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

// A model asked to name a topic will not name it the same way twice. Counting
// the raw string is therefore not a small inaccuracy - it is the difference
// between the feature working and the feature silently never promoting
// anything. These tests use the drift that actually shows up in practice.

func TestTopicKeyIgnoresCosmeticDifferences(t *testing.T) {
	same := [][2]string{
		{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406", "\u95e8\u5e97\u7684\u6392\u73ed\u7ba1\u7406"},
		{"PostgreSQL \u8fde\u63a5\u6c60", "PostgreSQL \u8fde\u63a5\u6c60\u95ee\u9898"},
		{"postgresql\u8fde\u63a5\u6c60", "PostgreSQL \u8fde\u63a5\u6c60"},
		{"\u6570\u636e\u5e93\u8fc1\u79fb", "\u6570\u636e\u5e93\u7684\u8fc1\u79fb"},
	}
	for _, pair := range same {
		require.Equal(t, types.NormalizeTopicKey(pair[0]), types.NormalizeTopicKey(pair[1]),
			"%q and %q are the same subject", pair[0], pair[1])
	}

	different := [][2]string{
		{"PostgreSQL \u8fde\u63a5\u6c60", "PostgreSQL \u5907\u4efd\u6062\u590d"},
		{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406", "\u6210\u4eba\u9a6c\u62c9\u677e\u62a5\u540d"},
	}
	for _, pair := range different {
		require.NotEqual(t, types.NormalizeTopicKey(pair[0]), types.NormalizeTopicKey(pair[1]),
			"%q and %q are different subjects", pair[0], pair[1])
	}
}

// The old key sorted and de-duplicated characters, which is why one extra
// character produced a different topic. Order has to survive.
func TestTopicKeyIsNotACharacterBag(t *testing.T) {
	require.NotEqual(t,
		types.NormalizeTopicKey("\u4e0a\u6d77\u5230\u5317\u4eac"),
		types.NormalizeTopicKey("\u5317\u4eac\u5230\u4e0a\u6d77"),
	)
}

func TestRephrasedTopicStillCountsTowardsTheSameSubject(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, InterestThreshold: 3,
	})
	scope := scopeFor(t, ctx)

	// Three sightings, three different wordings, one subject. The first is an
	// exact match after normalisation, the second is close enough for the
	// bigram tier; neither needs a model.
	require.Empty(t, svc.ObserveQuestionTopics(ctx, []string{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"}))
	require.Empty(t, svc.ObserveQuestionTopics(ctx, []string{"\u95e8\u5e97\u7684\u6392\u73ed\u7ba1\u7406"}))
	promoted := svc.ObserveQuestionTopics(ctx, []string{"\u8fde\u9501\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"})

	require.Equal(t, []string{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"}, promoted,
		"three wordings of one subject must reach the threshold together")

	stats, err := svc.repo.TopTopics(context.Background(), scope, 10)
	require.NoError(t, err)
	require.Len(t, stats, 1, "one subject, one row")
	require.Equal(t, 3, stats[0].Hits)
	require.Equal(t, "\u95e8\u5e97\u6392\u73ed\u7ba1\u7406", stats[0].Topic,
		"the label stays the one it was first recorded under, so the list does not churn")
	require.True(t, stats[0].Aliases.Has("\u8fde\u9501\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"),
		"the other wordings are kept, both as an audit trail and as a fast path")
}

func TestDifferentSubjectsAreStillCountedApart(t *testing.T) {
	svc, _, tenantRepo := newMemoryHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto, InterestThreshold: 2,
	})
	scope := scopeFor(t, ctx)

	svc.ObserveQuestionTopics(ctx, []string{"PostgreSQL \u8fde\u63a5\u6c60"})
	svc.ObserveQuestionTopics(ctx, []string{"PostgreSQL \u5907\u4efd\u6062\u590d"})

	stats, err := svc.repo.TopTopics(context.Background(), scope, 10)
	require.NoError(t, err)
	require.Len(t, stats, 2,
		"sharing a product name is not being about the same thing")
	for _, stat := range stats {
		require.Equal(t, 1, stat.Hits)
	}
}

// Merging two subjects that are not the same corrupts the count that decides
// what becomes a memory, and it is invisible once done. The cheap tier is
// therefore held well above where two labels merely look alike.
func TestLooseMatchingIsConservative(t *testing.T) {
	existing := []*types.MemoryTopicStat{
		{
			Topic:         "\u95e8\u5e97\u6392\u73ed\u7ba1\u7406",
			NormalizedKey: types.NormalizeTopicKey("\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"),
		},
	}
	require.NotNil(t, matchTopicLoosely("\u8fde\u9501\u95e8\u5e97\u6392\u73ed\u7ba1\u7406", existing))
	require.Nil(t, matchTopicLoosely("\u5e97\u5458\u73ed\u6b21\u5b89\u6392", existing),
		"a synonym is not something character overlap can decide; that is the model's job")
	require.Nil(t, matchTopicLoosely("\u4f9b\u5e94\u5546\u7ed3\u7b97\u6d41\u7a0b", existing))
}

// Short labels carry too little information for overlap to mean anything, so
// they skip the cheap tier rather than produce a false merge.
func TestShortLabelsDoNotMatchLoosely(t *testing.T) {
	existing := []*types.MemoryTopicStat{
		{Topic: "\u6392\u73ed", NormalizedKey: types.NormalizeTopicKey("\u6392\u73ed")},
	}
	require.Nil(t, matchTopicLoosely("\u6e38\u620f", existing))
	require.False(t, types.TopicIsSpecificEnoughToMatchLoosely("\u6392\u73ed"))
}

func TestAliasGivesAnExactMatchNextTime(t *testing.T) {
	existing := []*types.MemoryTopicStat{
		{
			Topic:         "\u95e8\u5e97\u6392\u73ed\u7ba1\u7406",
			NormalizedKey: types.NormalizeTopicKey("\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"),
			Aliases:       types.MemoryTopicAliases{"\u5e97\u5458\u73ed\u6b21\u5b89\u6392"},
		},
	}
	// A synonym the model decided on once must not be re-adjudicated forever.
	require.NotNil(t, matchTopicExactly("\u5e97\u5458\u73ed\u6b21\u5b89\u6392", existing))
	require.NotNil(t, matchTopicExactly("\u5e97\u5458\u7684\u73ed\u6b21\u5b89\u6392", existing))
}

func TestTwoNewWordingsInOneRunBecomeOneTopic(t *testing.T) {
	resolutions := []topicResolution{
		{Surface: "\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"},
		{Surface: "\u95e8\u5e97\u7684\u6392\u73ed\u7ba1\u7406"},
	}
	collapseNewTopicsWithinRun(resolutions)
	require.Equal(t, resolutions[0].Surface, resolutions[1].Surface,
		"one run must not create two rows it then has to keep apart forever")
}

// A synonym is not something character overlap can decide. "staff rota planning"
// and "store shift scheduling" (in Chinese) share one bigram out of fourteen, so the only tier
// that can resolve them is the model - and once it has, the answer is stored as
// an alias so it is never asked again.
func TestASynonymIsResolvedByTheModelAndThenRemembered(t *testing.T) {
	svc, tenantRepo, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto,
		ExtractModelID: "model-1", InterestThreshold: 3,
	})
	scope := scopeFor(t, ctx)
	models.responseFor = map[string]string{
		"You maintain the list of subjects one person keeps coming back to": `{"resolutions":[{"index":0,
			"same_as":0}]}`,
	}

	svc.ObserveQuestionTopics(ctx, []string{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"})
	callsBefore := models.callCount()

	svc.ObserveQuestionTopics(ctx, []string{"\u5e97\u5458\u73ed\u6b21\u5b89\u6392"})
	require.Greater(t, models.callCount(), callsBefore,
		"nothing cheaper could have decided this, so the model must have been asked")

	stats, err := svc.repo.TopTopics(context.Background(), scope, 10)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, 2, stats[0].Hits)
	require.True(t, stats[0].Aliases.Has("\u5e97\u5458\u73ed\u6b21\u5b89\u6392"))

	// Third sighting of the same synonym: the alias now answers it, so the
	// model is not consulted again.
	callsBefore = models.callCount()
	promoted := svc.ObserveQuestionTopics(ctx, []string{"\u5e97\u5458\u73ed\u6b21\u5b89\u6392"})
	require.Equal(t, callsBefore, models.callCount(),
		"a decision the model already made must not be paid for twice")
	require.Equal(t, []string{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"}, promoted)
}

// The model gets a veto, not a free hand: if it says two subjects are distinct,
// they stay distinct, and it is never asked about labels an earlier tier
// already resolved.
func TestTheModelCanDeclineToMergeTopics(t *testing.T) {
	svc, tenantRepo, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto,
		ExtractModelID: "model-1", InterestThreshold: 3,
	})
	scope := scopeFor(t, ctx)
	models.responseFor = map[string]string{
		"You maintain the list of subjects one person keeps coming back to": `{"resolutions":[{"index":0,
			"same_as":null}]}`,
	}

	svc.ObserveQuestionTopics(ctx, []string{"PostgreSQL \u8fde\u63a5\u6c60"})
	svc.ObserveQuestionTopics(ctx, []string{"PostgreSQL \u5907\u4efd\u6062\u590d"})

	stats, err := svc.repo.TopTopics(context.Background(), scope, 10)
	require.NoError(t, err)
	require.Len(t, stats, 2)
}

// A model that answers about a label it was not asked about must not be able to
// overwrite a match a more reliable tier already made.
func TestAdjudicationCannotOverrideACheaperTier(t *testing.T) {
	svc, tenantRepo, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto,
		ExtractModelID: "model-1", InterestThreshold: 5,
	})
	scope := scopeFor(t, ctx)
	models.responseFor = map[string]string{
		"You maintain the list of subjects one person keeps coming back to": `{"resolutions":[{"index":0,"same_as":0},
			{"index":1,"same_as":0}]}`,
	}

	svc.ObserveQuestionTopics(ctx, []string{"PostgreSQL \u8fde\u63a5\u6c60"})
	// One label matches by alias-free exact key, one is genuinely new. Only the
	// new one is up for adjudication.
	svc.ObserveQuestionTopics(ctx, []string{
		"PostgreSQL \u8fde\u63a5\u6c60\u95ee\u9898",
		"\u5b8c\u5168\u65e0\u5173\u7684\u56ed\u827a\u8bdd\u9898",
	})

	stats, err := svc.repo.TopTopics(context.Background(), scope, 10)
	require.NoError(t, err)
	require.Len(t, stats, 1, "the model merged the one label it was asked about")
	require.Equal(t, 3, stats[0].Hits)
}

// The label a merge leaves behind used to be whichever wording arrived first,
// which is arbitrary and not cosmetic: interests are fed to the query rewriter
// as this person's vocabulary. When one of the two names is plainly better, the
// model may say so.
func TestAMergeCanAdoptTheBetterName(t *testing.T) {
	svc, tenantRepo, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto,
		ExtractModelID: "model-1", InterestThreshold: 2,
	})
	scope := scopeFor(t, ctx)
	models.responseFor = map[string]string{
		"You maintain the list of subjects one person keeps coming back to": "{\"resolutions\":[{\"index\":0,\"same_a" +
			"s\":0,\"label\":\"\u6301\u7eed\u96c6\u6210\u6d41\u6c34\u7ebf\"}]}",
	}

	svc.ObserveQuestionTopics(ctx, []string{"CI \u6d41\u6c34\u7ebf"})
	promoted := svc.ObserveQuestionTopics(ctx, []string{"\u6301\u7eed\u96c6\u6210\u6d41\u6c34\u7ebf"})

	stats, err := svc.repo.TopTopics(context.Background(), scope, 10)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "\u6301\u7eed\u96c6\u6210\u6d41\u6c34\u7ebf", stats[0].Topic, "the fuller name should win")
	require.Equal(t, 2, stats[0].Hits, "renaming must not lose the count")
	require.True(t, stats[0].Aliases.Has("CI \u6d41\u6c34\u7ebf"),
		"the old label is what earlier sightings were counted under; dropping it makes that "+
			"wording look new again")
	require.False(t, stats[0].Aliases.Has("\u6301\u7eed\u96c6\u6210\u6d41\u6c34\u7ebf"),
		"a subject must not be listed as an alias of itself")
	require.Equal(t, []string{"\u6301\u7eed\u96c6\u6210\u6d41\u6c34\u7ebf"}, promoted)
}

// A model asked what two labels have in common will reach for something broader
// every time. Left unchecked, each merge widens the subject until it is an
// umbrella that means nothing - the exact failure this feature was just fixed
// for, arriving through a different door.
func TestAMergeCannotMakeTheSubjectVaguer(t *testing.T) {
	svc, tenantRepo, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto,
		ExtractModelID: "model-1", InterestThreshold: 5,
	})
	scope := scopeFor(t, ctx)
	models.responseFor = map[string]string{
		"You maintain the list of subjects one person keeps coming back to": "{\"resolutions\":[{\"index\":0,\"same_a" +
			"s\":0,\"label\":\"\u6392\u73ed\"}]}",
	}

	svc.ObserveQuestionTopics(ctx, []string{"\u95e8\u5e97\u6392\u73ed\u7ba1\u7406"})
	svc.ObserveQuestionTopics(ctx, []string{"\u5e97\u5458\u73ed\u6b21\u5b89\u6392"})

	stats, err := svc.repo.TopTopics(context.Background(), scope, 10)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "\u95e8\u5e97\u6392\u73ed\u7ba1\u7406", stats[0].Topic,
		"the merge stands, but the label may not become a category")
	require.Equal(t, 2, stats[0].Hits)
}

func TestProposedLabelsAreJudgedOnDirectionNotNovelty(t *testing.T) {
	canonical, incoming := "PostgreSQL \u8fde\u63a5\u6c60", "PostgreSQL \u8fde\u63a5\u6c60\u8c03\u4f18"

	require.True(t, types.TopicLabelIsAnImprovement(canonical, incoming, "PostgreSQL \u8fde\u63a5\u6c60\u8c03\u4f18"),
		"more specific is the direction a label is allowed to move in")
	require.False(t, types.TopicLabelIsAnImprovement(canonical, incoming, "\u6570\u636e\u5e93"),
		"a category is not a better name for the same subject")
	require.False(t, types.TopicLabelIsAnImprovement(canonical, incoming, "PostgreSQL"),
		"dropping what makes the subject specific is generalising")
	require.False(t, types.TopicLabelIsAnImprovement(canonical, incoming,
		"\u8fd0\u7ef4\u76f8\u5173\u7684\u4e00\u4e9b\u8bdd\u9898"),
		"a name grounded in neither label is an invention, not a merge")
	require.False(t, types.TopicLabelIsAnImprovement(canonical, incoming, canonical),
		"proposing the name it already has is not a rename")
}

// A promoted interest is a row the user can see and edit. Renaming the subject
// behind it has to keep the two in step, but must not overwrite wording the
// user chose.
func TestRenamingASubjectDoesNotOverwriteAnEditedInterest(t *testing.T) {
	svc, tenantRepo, _, models, _ := newExtractionHarness(t)
	ctx := enabledCtx(t, tenantRepo, 1, "alice")
	tenantRepo.set(1, &types.MemoryConfig{
		Enabled: true, WriteMode: types.MemoryWriteAuto,
		ExtractModelID: "model-1", InterestThreshold: 1,
	})
	models.responseFor = map[string]string{
		"You maintain the list of subjects one person keeps coming back to": "{\"resolutions\":[{\"index\":0,\"same_a" +
			"s\":0,\"label\":\"\u6301\u7eed\u96c6\u6210\u6d41\u6c34\u7ebf\"}]}",
	}

	svc.ObserveQuestionTopics(ctx, []string{"CI \u6d41\u6c34\u7ebf"})
	items, _, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	_, err = svc.UpdateItem(ctx, items[0].ID, "\u6211\u81ea\u5df1\u5199\u7684\u8bf4\u6cd5", 4)
	require.NoError(t, err)

	svc.ObserveQuestionTopics(ctx, []string{"\u6301\u7eed\u96c6\u6210\u6d41\u6c34\u7ebf"})

	after, _, err := svc.ListItems(ctx, types.MemoryStatusActive, 10, 0)
	require.NoError(t, err)
	require.Equal(t, "\u6211\u81ea\u5df1\u5199\u7684\u8bf4\u6cd5", after[0].Content,
		"the user's own wording outranks a better generated one")
}
