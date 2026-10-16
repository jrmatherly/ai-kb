package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/config"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/models/chat"
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

type failingQuestionChat struct{ cadenceChat }

func (failingQuestionChat) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return nil, errors.New("401 Unauthorized: invalid api key")
}

func TestGenerateQuestionsReportsModelFailureAs502(t *testing.T) {
	svc := &knowledgeService{config: &config.Config{Conversation: &config.ConversationConfig{
		GenerateQuestionsPrompt: "Write {{question_count}} questions about {{content}}",
	}}}
	_, err := svc.generateQuestionsWithContext(
		context.Background(), failingQuestionChat{}, "body", "", "", "doc", 3, "")
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok, "%v", err)
	require.Equal(t, 502, appErr.HTTPCode)
	require.Equal(t, "failed to generate questions: 401 Unauthorized: invalid api key", appErr.Message)
}

func TestGenerateQuestionsKeepsRequestDeadline(t *testing.T) {
	svc := &knowledgeService{config: &config.Config{Conversation: &config.ConversationConfig{
		GenerateQuestionsPrompt: "Write {{question_count}} questions about {{content}}",
	}}}
	_, err := svc.generateQuestionsWithContext(
		context.Background(), erroringChat{err: context.DeadlineExceeded}, "body", "", "", "doc", 3, "")
	_, ok := apperrors.IsAppError(err)
	require.False(t, ok)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
