package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/models/chat"
	"github.com/jrmatherly/ai-kb/internal/types"
)

type proseReplyChat struct{}

func (proseReplyChat) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return &types.ChatResponse{Content: "Sorry, this text has no entities or relations.", FinishReason: "stop"}, nil
}

func (proseReplyChat) ChatStream(
	context.Context, []chat.Message, *chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	return nil, errors.New("not implemented")
}
func (proseReplyChat) GetModelName() string { return "prose" }
func (proseReplyChat) GetModelID() string   { return "prose" }

// The graph-settings "try extract" button must report a prose refusal as a
// failure. Returning an empty graph would make the UI clear the user's
// few-shot nodes/relations and show "success" (#3606 review).
func TestExtractRelationsFromTextReportsModelRefusal(t *testing.T) {
	h := &InitializationHandler{config: &config.Config{
		ExtractManager: &config.ExtractManagerConfig{
			ExtractGraph: &types.PromptTemplateStructured{Description: "extract"},
		},
	}}
	res, err := h.extractRelationsFromText(context.Background(), "Contents", nil, proseReplyChat{})
	if err == nil {
		t.Fatalf("expected an error for a prose refusal, got %+v", res)
	}
}
