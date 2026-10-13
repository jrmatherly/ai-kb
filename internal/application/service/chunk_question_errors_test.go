package service

import (
	"context"
	"testing"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type imageChunkRepo struct{ interfaces.ChunkRepository }

func (imageChunkRepo) GetChunkByID(_ context.Context, _ uint64, id string) (*types.Chunk, error) {
	return &types.Chunk{ID: id, TenantID: 1, ChunkType: types.ChunkTypeImageOCR}, nil
}

func TestRegenerateChunkQuestionsRejectsNonTextChunkAs400(t *testing.T) {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	_, err := (&knowledgeService{chunkRepo: imageChunkRepo{}}).RegenerateChunkQuestions(ctx, "img")
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, 400, appErr.HTTPCode)
	require.Equal(t, "questions can only be generated for text chunks", appErr.Message)
}
