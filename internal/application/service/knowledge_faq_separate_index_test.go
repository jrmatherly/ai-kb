package service

import (
	"context"
	"sort"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/models/embedding"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// faqIndexStore keeps indexed items by source ID, the way the vector stores do.
type faqIndexStore struct {
	interfaces.RetrieveEngineService
	items map[string]*types.IndexInfo
}

func (*faqIndexStore) EngineType() types.RetrieverEngineType {
	return types.PostgresRetrieverEngineType
}

func (*faqIndexStore) Support() []types.RetrieverType {
	return []types.RetrieverType{types.VectorRetrieverType}
}

func (s *faqIndexStore) BatchIndex(
	_ context.Context, _ embedding.Embedder, infos []*types.IndexInfo, _ []types.RetrieverType,
) error {
	for _, info := range infos {
		s.items[info.SourceID] = info
	}
	return nil
}

func (s *faqIndexStore) DeleteByChunkIDList(_ context.Context, ids []string, _ int, _ string) error {
	for _, id := range ids {
		for sourceID, info := range s.items {
			if info.ChunkID == id {
				delete(s.items, sourceID)
			}
		}
	}
	return nil
}

func (s *faqIndexStore) DeleteBySourceIDList(_ context.Context, ids []string, _ int, _ string) error {
	for _, id := range ids {
		delete(s.items, id)
	}
	return nil
}

func (s *faqIndexStore) contents() []string {
	out := make([]string, 0, len(s.items))
	for _, info := range s.items {
		out = append(out, info.Content)
	}
	sort.Strings(out)
	return out
}

type faqChunkUpdater struct {
	interfaces.ChunkService
	repo interfaces.ChunkRepository
}

func (u *faqChunkUpdater) UpdateChunk(ctx context.Context, chunk *types.Chunk) error {
	return u.repo.UpdateChunk(ctx, chunk)
}

func TestUpdateFAQEntrySeparateModeRemovesDroppedSimilarQuestion(t *testing.T) {
	f := newFAQWriteFixture(t)
	f.kb.EmbeddingModelID = "embed"
	f.kb.FAQConfig = &types.FAQConfig{
		IndexMode:         types.FAQIndexModeQuestionOnly,
		QuestionIndexMode: types.FAQQuestionIndexModeSeparate,
	}
	store := &faqIndexStore{items: map[string]*types.IndexInfo{}}
	f.svc.retrieveEngine = &fakeFanoutRegistry{
		byStore: map[string]interfaces.RetrieveEngineService{"write-store": store},
	}
	f.svc.modelService = parentChildModelService{embedder: parentChildEmbedder{}}
	f.svc.chunkService = &faqChunkUpdater{repo: f.chunks}

	chunk, err := f.chunks.GetChunkByID(f.ctx, 7, "one")
	require.NoError(t, err)
	require.NoError(t, chunk.SetFAQMetadata(&types.FAQChunkMetadata{
		StandardQuestion: "Reset password",
		SimilarQuestions: []string{"Forgot password", "Lost password"},
		Answers:          []string{"Use the reset link."},
	}))
	require.NoError(t, f.chunks.UpdateChunk(f.ctx, chunk))
	knowledge, err := f.svc.repo.GetKnowledgeByID(f.ctx, 7, "faq")
	require.NoError(t, err)
	require.NoError(t, f.svc.indexFAQChunks(f.ctx, f.kb, knowledge, []*types.Chunk{chunk},
		parentChildEmbedder{}, false, true))
	require.Equal(t, []string{"Forgot password", "Lost password", "Reset password"}, store.contents())

	_, err = f.svc.UpdateFAQEntry(f.ctx, "kb", 1, &types.FAQEntryPayload{
		StandardQuestion: "Reset password",
		SimilarQuestions: []string{"Lost password"},
		Answers:          []string{"Use the reset link."},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"Lost password", "Reset password"}, store.contents())
}

func TestUpdateFAQEntrySeparateModeIndexesOriginalText(t *testing.T) {
	f := newFAQWriteFixture(t)
	f.kb.EmbeddingModelID = "embed"
	f.kb.FAQConfig = &types.FAQConfig{
		IndexMode:         types.FAQIndexModeQuestionOnly,
		QuestionIndexMode: types.FAQQuestionIndexModeSeparate,
	}
	store := &faqIndexStore{items: map[string]*types.IndexInfo{}}
	f.svc.retrieveEngine = &fakeFanoutRegistry{
		byStore: map[string]interfaces.RetrieveEngineService{"write-store": store},
	}
	f.svc.modelService = parentChildModelService{embedder: parentChildEmbedder{}}
	f.svc.chunkService = &faqChunkUpdater{repo: f.chunks}

	chunk, err := f.chunks.GetChunkByID(f.ctx, 7, "one")
	require.NoError(t, err)
	require.NoError(t, chunk.SetFAQMetadata(&types.FAQChunkMetadata{
		StandardQuestion: "Reset password",
		SimilarQuestions: []string{"Lost password"},
		Answers:          []string{"Use the reset link."},
	}))
	require.NoError(t, f.chunks.UpdateChunk(f.ctx, chunk))
	knowledge, err := f.svc.repo.GetKnowledgeByID(f.ctx, 7, "faq")
	require.NoError(t, err)
	require.NoError(t, f.svc.indexFAQChunks(f.ctx, f.kb, knowledge, []*types.Chunk{chunk},
		parentChildEmbedder{}, false, true))

	_, err = f.svc.UpdateFAQEntry(f.ctx, "kb", 1, &types.FAQEntryPayload{
		StandardQuestion: "Reset my Password?",
		SimilarQuestions: []string{"Lost password", "Can't log in?"},
		Answers:          []string{"Use the reset link."},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"Can't log in?", "Lost password", "Reset my Password?"}, store.contents())

	edited, err := f.chunks.GetChunkByID(f.ctx, 7, "one")
	require.NoError(t, err)
	full, err := f.svc.buildFAQIndexInfoList(f.ctx, f.kb, edited)
	require.NoError(t, err)
	for _, info := range full {
		require.Equal(t, info.Content, store.items[info.SourceID].Content, info.SourceID)
	}
}

// Two similar questions that normalize alike share a source ID. The composite
// engine drops the second before indexing, but the storage estimate and quota
// check run on the built list, so the builder emits each key once, keeping the
// first original text as the edit path does.
func TestBuildFAQIndexInfoListSeparateModeKeepsOneItemPerNormalizedQuestion(t *testing.T) {
	svc := &knowledgeService{}
	kb := &types.KnowledgeBase{FAQConfig: &types.FAQConfig{
		IndexMode:         types.FAQIndexModeQuestionOnly,
		QuestionIndexMode: types.FAQQuestionIndexModeSeparate,
	}}
	chunk := &types.Chunk{ID: "one"}
	require.NoError(t, chunk.SetFAQMetadata(&types.FAQChunkMetadata{
		StandardQuestion: "Reset password",
		SimilarQuestions: []string{"Forgot password", " forgot password? ", "Lost password"},
		Answers:          []string{"Use the reset link."},
	}))

	infos, err := svc.buildFAQIndexInfoList(context.Background(), kb, chunk)
	require.NoError(t, err)
	contents := make([]string, 0, len(infos))
	sourceIDs := map[string]bool{}
	for _, info := range infos {
		contents = append(contents, info.Content)
		sourceIDs[info.SourceID] = true
	}
	require.Equal(t, []string{"Reset password", "Forgot password", "Lost password"}, contents)
	require.Len(t, sourceIDs, 3)
}
