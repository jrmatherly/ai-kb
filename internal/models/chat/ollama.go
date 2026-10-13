package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/models/api"
	"github.com/jrmatherly/ai-kb/internal/models/utils/ollama"
	"github.com/jrmatherly/ai-kb/internal/types"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
	ollamaapi "github.com/ollama/ollama/api"
)

// OllamaChat implements chat on Ollama
type OllamaChat struct {
	modelName     string
	modelID       string
	ollamaService *ollama.OllamaService
}

// NewOllamaChat creates an Ollama chat instance
func NewOllamaChat(config *ChatConfig, ollamaService *ollama.OllamaService) (*OllamaChat, error) {
	return &OllamaChat{
		modelName:     config.ModelName,
		modelID:       config.ModelID,
		ollamaService: ollamaService,
	}, nil
}

// convertMessages converts messages to the Ollama API format
func (c *OllamaChat) convertMessages(messages []Message) []ollamaapi.Message {
	ollamaMessages := make([]ollamaapi.Message, 0, len(messages))
	for _, msg := range messages {
		msg = api.NeutralizeMessageSpecialTokens(msg)
		msgOllama := ollamaapi.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolCalls: c.toolCallFrom(msg.ToolCalls),
		}
		if msg.Role == "tool" {
			msgOllama.ToolName = msg.Name
		}
		// The agent builds multimodal turns as MultiContent parts, the shape
		// every remote protocol prefers, and only the older callers fill
		// Images. Reading just Images dropped both the text and the pictures
		// of such a turn: Ollama answered about nothing, without an error.
		images := msg.Images
		if len(msg.MultiContent) > 0 {
			// Copy first: appending to the caller's slice could write into
			// its spare capacity and leak an image into another message.
			images = append([]string(nil), msg.Images...)
			var text strings.Builder
			for _, part := range msg.MultiContent {
				switch part.Type {
				case "text":
					text.WriteString(part.Text)
				case "image_url":
					if part.ImageURL != nil && part.ImageURL.URL != "" {
						images = append(images, part.ImageURL.URL)
					}
				}
			}
			if msgOllama.Content == "" {
				msgOllama.Content = text.String()
			}
		}
		if len(images) > 0 && msg.Role == "user" {
			for _, imgURL := range images {
				if imgData := resolveImageForOllama(imgURL); imgData != nil {
					msgOllama.Images = append(msgOllama.Images, imgData)
				}
			}
		}
		ollamaMessages = append(ollamaMessages, msgOllama)
	}
	return ollamaMessages
}

// resolveImageForOllama resolves an image URL into raw bytes for Ollama.
// Handles local serving paths (/files/...), data URIs, and remote HTTP URLs.
func resolveImageForOllama(imageURL string) ollamaapi.ImageData {
	if data := api.ResolveImageURLForOllama(imageURL); data != nil {
		return data
	}
	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		if err := secutils.ValidateURLForSSRF(imageURL); err != nil {
			return nil
		}
		client := secutils.NewSSRFSafeHTTPClient(secutils.SSRFSafeHTTPClientConfig{
			Timeout:      30 * time.Second,
			MaxRedirects: 5,
		})
		resp, err := client.Get(imageURL)
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024))
		if err != nil {
			return nil
		}
		return data
	}
	return nil
}

// buildChatRequest builds the chat request parameters
func (c *OllamaChat) buildChatRequest(messages []Message, opts *ChatOptions, isStream bool) *ollamaapi.ChatRequest {
	// Set the stream flag
	streamFlag := isStream

	// Build the request parameters
	chatReq := &ollamaapi.ChatRequest{
		Model:    c.modelName,
		Messages: c.convertMessages(messages),
		Stream:   &streamFlag,
		Options:  make(map[string]interface{}),
	}

	// Add the optional parameters
	if opts != nil {
		chatReq.Options["temperature"] = opts.Temperature
		if opts.TopP > 0 {
			chatReq.Options["top_p"] = opts.TopP
		}
		if budget := opts.CompletionBudget(); budget > 0 {
			chatReq.Options["num_predict"] = budget
		}
		if level, requested := opts.Reasoning(); requested {
			// Ollama accepts a boolean switch; graded levels only exist for a
			// few models, so they collapse to on/off here.
			chatReq.Think = &ollamaapi.ThinkValue{Value: level.Enabled()}
		}
		if len(opts.Format) > 0 {
			chatReq.Format = opts.Format
		}
		if len(opts.Tools) > 0 {
			chatReq.Tools = c.toolFrom(opts.Tools)
		}
	}

	return chatReq
}

// Chat runs a non-streaming chat
func (c *OllamaChat) Chat(ctx context.Context, messages []Message, opts *ChatOptions) (*types.ChatResponse, error) {
	// Make sure the model is available
	if err := c.ensureModelAvailable(ctx); err != nil {
		return nil, err
	}

	// Build the request parameters
	chatReq := c.buildChatRequest(messages, opts, false)

	// Log the request
	logger.GetLogger(ctx).Infof("sending chat request to model %s", c.modelName)

	var responseContent string
	var toolCalls []types.LLMToolCall
	var promptTokens, completionTokens int
	var finishReason string

	// Send the request with the Ollama client
	err := c.ollamaService.Chat(ctx, chatReq, func(resp ollamaapi.ChatResponse) error {
		responseContent = resp.Message.Content
		// When Content is empty but Thinking is not (e.g. a reasoning model without the
		// thinking parameter set), fall back to Thinking
		if responseContent == "" && resp.Message.Thinking != "" {
			responseContent = resp.Message.Thinking
		}
		toolCalls = c.toolCallTo(resp.Message.ToolCalls)
		// done_reason is "stop" / "length" and so on; without passing it through, the caller cannot tell a refusal from
		// truncation at a full num_predict (graph extraction uses it to decide between skipping and retrying).
		if resp.DoneReason != "" {
			finishReason = resp.DoneReason
		}

		// Get the token counts. eval_count is already the answer's token count, without the prompt
		// (https://github.com/ollama/ollama/blob/main/docs/api.md), so prompt_eval_count must not be
		// subtracted again; the streaming branch always used it directly, and this matches it.
		if resp.EvalCount > 0 {
			promptTokens = resp.PromptEvalCount
			completionTokens = resp.EvalCount
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("chat request failed: %w", err)
	}

	usage := types.TokenUsage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
	usage.MarkPromptCacheUnsupported()
	api.LogUsage(ctx, c.modelName, &usage)

	return &types.ChatResponse{
		Content:      responseContent,
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

// ChatStream runs a streaming chat
func (c *OllamaChat) ChatStream(
	ctx context.Context,
	messages []Message,
	opts *ChatOptions,
) (<-chan types.StreamResponse, error) {
	// Make sure the model is available
	if err := c.ensureModelAvailable(ctx); err != nil {
		return nil, err
	}

	// Build the request parameters
	chatReq := c.buildChatRequest(messages, opts, true)

	// Log the request
	logger.GetLogger(ctx).Infof("sending streaming chat request to model %s", c.modelName)

	// Create the streaming response channel
	streamChan := make(chan types.StreamResponse)

	// Start a goroutine to handle the streaming response
	go func() {
		defer close(streamChan)

		var thinking api.ThinkingEmitter
		err := c.ollamaService.Chat(ctx, chatReq, func(resp ollamaapi.ChatResponse) error {
			// Send thinking content (reasoning models such as Qwen3 and DeepSeek)
			if resp.Message.Thinking != "" {
				thinking.Emit(streamChan, resp.Message.Thinking)
			}

			if resp.Message.Content != "" {
				// When the thinking phase ends, send the thinking-complete event
				thinking.Finish(streamChan)
				streamChan <- types.StreamResponse{
					ResponseType: types.ResponseTypeAnswer,
					Content:      resp.Message.Content,
					Done:         false,
				}
			}

			if len(resp.Message.ToolCalls) > 0 {
				streamChan <- types.StreamResponse{
					ResponseType: types.ResponseTypeToolCall,
					ToolCalls:    c.toolCallTo(resp.Message.ToolCalls),
					Done:         false,
				}

				// Ollama returns tool calls as complete objects (not incremental deltas).
				// Log this so we can trace non-streaming thought delivery.
				for _, tc := range resp.Message.ToolCalls {
					if tc.Function.Name == "thinking" {
						argsBytes, _ := json.Marshal(tc.Function.Arguments)
						logger.Warnf(ctx, "[Ollama Stream] Tool %q arrived non-incrementally (%d bytes args), "+
							"thought will not be token-streamed to frontend",
							tc.Function.Name, len(argsBytes))
					}
				}

				for _, tc := range resp.Message.ToolCalls {
					argsMap := tc.Function.Arguments.ToMap()
					switch tc.Function.Name {
					case "thinking":
						if thought, ok := argsMap["thought"].(string); ok && thought != "" {
							streamChan <- types.StreamResponse{
								ResponseType: types.ResponseTypeThinking,
								Content:      thought,
								Done:         false,
								Data: map[string]interface{}{
									"source":       "thinking_tool",
									"tool_call_id": tooli2s(tc.Function.Index),
								},
							}
						}
					}
				}
			}

			if resp.Done {
				var usage *types.TokenUsage
				if resp.PromptEvalCount > 0 || resp.EvalCount > 0 {
					usage = &types.TokenUsage{
						PromptTokens:     resp.PromptEvalCount,
						CompletionTokens: resp.EvalCount,
						TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
					}
					usage.MarkPromptCacheUnsupported()
				}
				api.LogUsage(ctx, c.modelName, usage)
				streamChan <- types.StreamResponse{
					ResponseType: types.ResponseTypeAnswer,
					Done:         true,
					Usage:        usage,
				}
			}

			return nil
		})
		if err != nil {
			logger.GetLogger(ctx).Errorf("streaming chat request failed: %v", err)
			// Send the error response
			streamChan <- types.StreamResponse{
				ResponseType: types.ResponseTypeError,
				Content:      err.Error(),
				Done:         true,
			}
		}
	}()

	return streamChan, nil
}

// Make sure the model is available
func (c *OllamaChat) ensureModelAvailable(ctx context.Context) error {
	logger.GetLogger(ctx).Infof("ensuring model %s is available", c.modelName)
	return c.ollamaService.EnsureModelAvailable(ctx, c.modelName)
}

// GetModelName returns the model name
func (c *OllamaChat) GetModelName() string {
	return c.modelName
}

// GetModelID returns the model ID
func (c *OllamaChat) GetModelID() string {
	return c.modelID
}

// toolFrom converts this package's Tool to an Ollama Tool
func (c *OllamaChat) toolFrom(tools []Tool) ollamaapi.Tools {
	if len(tools) == 0 {
		return nil
	}
	ollamaTools := make(ollamaapi.Tools, 0, len(tools))
	for _, tool := range tools {
		function := ollamaapi.ToolFunction{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
		}
		if len(tool.Function.Parameters) > 0 {
			_ = json.Unmarshal(tool.Function.Parameters, &function.Parameters)
		}

		ollamaTools = append(ollamaTools, ollamaapi.Tool{
			Type:     tool.Type,
			Function: function,
		})
	}
	return ollamaTools
}

// toolCallFrom converts this package's ToolCall to an Ollama ToolCall
func (c *OllamaChat) toolCallFrom(toolCalls []ToolCall) []ollamaapi.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	ollamaToolCalls := make([]ollamaapi.ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		args := ollamaapi.NewToolCallFunctionArguments()
		if tc.Function.Arguments != "" {
			_ = args.UnmarshalJSON([]byte(tc.Function.Arguments))
		}
		ollamaToolCalls = append(ollamaToolCalls, ollamaapi.ToolCall{
			Function: ollamaapi.ToolCallFunction{
				Index:     tools2i(tc.ID),
				Name:      tc.Function.Name,
				Arguments: args,
			},
		})
	}
	return ollamaToolCalls
}

// toolCallTo converts an Ollama ToolCall to this package's ToolCall
func (c *OllamaChat) toolCallTo(ollamaToolCalls []ollamaapi.ToolCall) []types.LLMToolCall {
	if len(ollamaToolCalls) == 0 {
		return nil
	}
	toolCalls := make([]types.LLMToolCall, 0, len(ollamaToolCalls))
	for _, tc := range ollamaToolCalls {
		argsBytes, _ := json.Marshal(tc.Function.Arguments)
		toolCalls = append(toolCalls, types.LLMToolCall{
			ID:   tooli2s(tc.Function.Index),
			Type: "function",
			Function: types.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: string(argsBytes),
			},
		})
	}
	return toolCalls
}

func tooli2s(i int) string {
	return strconv.Itoa(i)
}

func tools2i(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}
