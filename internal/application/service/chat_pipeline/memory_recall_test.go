package chatpipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jrmatherly/ai-kb/internal/event"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubMemoryService returns a fixed recall so the test can assert what the
// pipeline does with it, rather than re-testing the memory service.
type stubMemoryService struct {
	interfaces.MemoryService

	recall     interfaces.MemoryRecall
	lastQuery  string
	recallCall int
}

func (s *stubMemoryService) Recall(_ context.Context, query string) interfaces.MemoryRecall {
	s.recallCall++
	s.lastQuery = query
	return s.recall
}

func (s *stubMemoryService) ScheduleExtraction(context.Context, string, string, string) {}

func (s *stubMemoryService) Handle(context.Context, *asynq.Task) error { return nil }

func newMemoryRecallPlugin(memoryService interfaces.MemoryService) *PluginMemoryRecall {
	return NewPluginMemoryRecall(NewEventManager(), memoryService)
}

// TestMemoryReachesTheMessagesSentToTheModel is the assertion that matters:
// recalling memory is pointless if it never lands in the request. It walks the
// recall stage and then the same message assembly the completion plugins use.
func TestMemoryReachesTheMessagesSentToTheModel(t *testing.T) {
	memoryService := &stubMemoryService{
		recall: interfaces.MemoryRecall{
			Prompt: types.WrapMemoryForPrompt("Preferences:\n- Lead every answer with the conclusion", ""),
			Items: []*types.MemoryItem{
				{ID: "m1", Kind: types.MemoryKindPreference, Content: "Lead every answer with the conclusion"},
			},
		},
	}
	plugin := newMemoryRecallPlugin(memoryService)

	chatManage := &types.ChatManage{}
	chatManage.Query = "Can you look at this error for me"
	chatManage.UserContent = "Can you look at this error for me"
	chatManage.SummaryConfig.Prompt = "You are an assistant."

	nextCalled := false
	err := plugin.OnEvent(t.Context(), types.MEMORY_RECALL, chatManage, func() *PluginError {
		nextCalled = true
		return nil
	})
	require.Nil(t, err)
	require.True(t, nextCalled, "the recall stage must never stop the pipeline")
	require.Equal(t, "Can you look at this error for me", memoryService.lastQuery)

	messages := prepareMessagesWithHistory(chatManage)
	require.NotEmpty(t, messages)
	require.Equal(t, "system", messages[0].Role)
	require.Contains(t, messages[0].Content, "Lead every answer with the conclusion",
		"the recalled memory must be present in the system message")
	require.Contains(t, messages[0].Content, "<user_memory>")
	require.True(t, strings.HasPrefix(messages[0].Content, "You are an assistant."),
		"memory must be appended after the configured prompt, not replace it")
}

func TestMemoryIsAbsentWhenNothingRecalled(t *testing.T) {
	plugin := newMemoryRecallPlugin(&stubMemoryService{})

	chatManage := &types.ChatManage{}
	chatManage.Query = "Just asking something"
	chatManage.SummaryConfig.Prompt = "You are an assistant."

	require.Nil(t, plugin.OnEvent(t.Context(), types.MEMORY_RECALL, chatManage, func() *PluginError { return nil }))
	require.Empty(t, chatManage.MemoryPrompt)
	require.Empty(t, chatManage.UsedMemories)

	messages := prepareMessagesWithHistory(chatManage)
	require.NotContains(t, messages[0].Content, "<user_memory>")
}

func TestMemoryRecallEmitsWhatTheAnswerSaw(t *testing.T) {
	memoryService := &stubMemoryService{
		recall: interfaces.MemoryRecall{
			Prompt: types.WrapMemoryForPrompt("About the user:\n- Works on medical imaging", ""),
			Items: []*types.MemoryItem{
				{ID: "m1", Kind: types.MemoryKindProfile, Content: "Works on medical imaging"},
			},
		},
	}
	plugin := newMemoryRecallPlugin(memoryService)

	bus := event.NewEventBus()
	var received types.UsedMemories
	bus.On(event.EventMemoryRecalled, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.MemoryRecalledData)
		require.True(t, ok)
		received, ok = data.Memories.(types.UsedMemories)
		require.True(t, ok)
		return nil
	})

	chatManage := &types.ChatManage{}
	chatManage.Query = "Pick up where we left off"
	chatManage.EventBus = bus.AsEventBusInterface()

	require.Nil(t, plugin.OnEvent(t.Context(), types.MEMORY_RECALL, chatManage, func() *PluginError { return nil }))

	// The chat UI promises "these are the memories this answer saw", so the
	// streamed list has to be the same one that was injected.
	require.Len(t, received, 1)
	require.Equal(t, "m1", received[0].ID)
	require.Equal(t, "Works on medical imaging", received[0].Content)
	require.Equal(t, received, chatManage.UsedMemories)
}

func TestMemoryRecallToleratesNoService(t *testing.T) {
	plugin := newMemoryRecallPlugin(nil)
	chatManage := &types.ChatManage{}
	chatManage.SummaryConfig.Prompt = "You are an assistant."
	require.Nil(t, plugin.OnEvent(t.Context(), types.MEMORY_RECALL, chatManage, func() *PluginError { return nil }))
	require.Empty(t, chatManage.MemoryPrompt)
}

func TestMemoryRecallStageIsRegisteredInThePipeline(t *testing.T) {
	// A stage nobody runs is the failure mode this whole feature has had
	// before, so assert the plugin declares the event the assembler adds.
	plugin := newMemoryRecallPlugin(&stubMemoryService{})
	require.Equal(t, []types.EventType{types.MEMORY_RECALL}, plugin.ActivationEvents())
}
