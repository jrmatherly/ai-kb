// Package providers registers Zhipu AI's BigModel open platform.
//
// Facts (chat completions reference,
// https://docs.bigmodel.cn/api-reference/%E6%A8%A1%E5%9E%8B-api/%E5%AF%B9%E8%AF%9D%E8%A1%A5%E5%85%A8):
//   - chat completions live at POST /paas/v4/chat/completions under
//     https://open.bigmodel.cn/api, so the base URL is
//     https://open.bigmodel.cn/api/paas/v4;
//   - output cap is `max_tokens`; the reference lists no
//     `max_completion_tokens`. GLM-5.3 / 5.2 / 5.1 / 5 / 4.7 / 4.6 cap at
//     128K output, the GLM-4.5 series at 96K;
//   - thinking is switched with `thinking: {"type": "enabled"|"disabled"}`
//     (default enabled, GLM-4.5 and later only). The sibling
//     `clear_thinking` defaults to true, i.e. the platform strips
//     `reasoning_content` from previous turns unless it is set to false, so
//     replaying it is accepted but normally a no-op;
//   - `reasoning_effort` takes none | minimal | low | medium | high | xhigh |
//     max, defaults to max, and is only honoured by "GLM-5.2 and later models":
//     GLM-5.1, GLM-5, GLM-4.7, GLM-4.6 and GLM-4.5 ignore it, so those
//     entries turn it off. The vendor-level map spells every rung, including
//     off -> "none";
//   - per-model effort ladders narrow that set: GLM-5.3 and GLM-5.3-Flash
//     only take low / high / max, and GLM-5.2 maps none|minimal to "no
//     thinking", low|medium to high and xhigh to max. That folding is the
//     platform's own, so GLM-5.2 keeps sending "minimal" and "low"
//     verbatim: rewriting minimal to "low" here would turn the weakest rung
//     into the strongest one, because the platform then folds it to high;
//   - GLM-5.3 and GLM-5.3-Flash/FlashX always think: `thinking.type` "can only
//     be enabled", so both carry "off": null
//     (https://docs.bigmodel.cn/cn/guide/models/text/glm-5.3,
//     https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash);
//   - `tool_choice` is documented as "defaults to auto and only supports auto", so none /
//     required / named-function are never sent;
//   - `response_format` takes text | json_object (no json_schema);
//     temperature is [0.0, 1.0] and top_p is [0.01, 1.0], both two decimals;
//   - usage reports `prompt_tokens_details.cached_tokens`;
//   - the platform also exposes an Anthropic Messages facade at
//     https://open.bigmodel.cn/api/anthropic (/v1/messages under it), which
//     Resolve switches to when the base URL ends with /anthropic;
//   - embeddings: embedding-3 defaults to 2048 dimensions (256 / 512 / 1024 /
//     2048 selectable), embedding-2 is fixed at 1024;
//   - rerank is POST https://open.bigmodel.cn/api/paas/v4/rerank with model
//     id "rerank".
//
// unverified: the reference does not document `stream_options`,
// `parallel_tool_calls` or `seed`; the protocol defaults (include_usage sent,
// parallel tool calls and seed allowed) are kept because nothing states they
// are rejected.
//
// unverified: glm-5v-turbo is a vision model and the "GLM-5.2 and later" rule
// does not name it, so its supports_reasoning_effort:false is kept as-is.
//
// unverified: context windows, max output tokens and prices of individual
// entries beyond the GLM-5 / 5.2 / 5.3 / 4.7 model pages, and the
// glm-4.7-flash zero price.
package providers

import (
	_ "embed"

	"github.com/jrmatherly/ai-kb/internal/models/api"
	"github.com/jrmatherly/ai-kb/internal/types"
)

//go:embed assets/zhipu.svg
var zhipuIcon []byte

// ZhipuID is the provider identifier stored on model rows.
const ZhipuID = "zhipu"

// ZhipuBaseURL is the OpenAI-compatible chat / embedding endpoint.
const ZhipuBaseURL = "https://open.bigmodel.cn/api/paas/v4"

// ZhipuRerankBaseURL is the rerank endpoint.
const ZhipuRerankBaseURL = "https://open.bigmodel.cn/api/paas/v4/rerank"

// ZhipuAnthropicBaseURL is the Anthropic Messages compatibility facade.
const ZhipuAnthropicBaseURL = "https://open.bigmodel.cn/api/anthropic"

func newZhipuProvider() *Definition {
	return &Definition{
		ID:    ZhipuID,
		Name:  "Zhipu BigModel",
		Names: map[string]string{"zh-CN": "\u667a\u8c31 BigModel"},
		Description: "glm-5.3, glm-5.2, glm-4.7, embedding-3, rerank; " +
			"Anthropic-compatible facade at " + ZhipuAnthropicBaseURL,
		Descriptions: map[string]string{
			"zh-CN": "glm-5.3, glm-5.2, glm-4.7, embedding-3, rerank\uff1bAnthropic \u517c\u5bb9\u63a5\u53e3 " + ZhipuAnthropicBaseURL,
		},
		Website:      "https://open.bigmodel.cn",
		Icon:         zhipuIcon,
		API:          api.APIOpenAICompletions,
		Order:        11,
		RequiresAuth: true,
		Auth:         AuthBearer,
		URLPatterns:  []string{"open.bigmodel.cn", "zhipu"},
		DefaultBaseURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: ZhipuBaseURL,
			types.ModelTypeEmbedding:   ZhipuBaseURL,
			types.ModelTypeRerank:      ZhipuRerankBaseURL,
			types.ModelTypeVLLM:        ZhipuBaseURL,
		},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeEmbedding,
			types.ModelTypeRerank,
			types.ModelTypeVLLM,
			types.ModelTypeASR,
		},
		Compat: VendorCompat{
			Transcriptions: api.TranscriptionsCompat{
				// Speech to text reference:
				// https://docs.bigmodel.cn/api-reference/%E6%A8%A1%E5%9E%8B-api/%E8%AF%AD%E9%9F%B3%E8%BD%AC%E6%96%87%E6%9C%AC
				// POST {base}/audio/transcriptions, multipart file + model
				// (glm-asr-2512), answering {text}. The file is ".wav / .mp3",
				// at most 25 MB and 30 seconds — the duration cannot be checked
				// here without decoding, so a longer file is the vendor's error.
				MaxFileBytes: api.Ptr(25 << 20),
				// No language field; prompt and hotwords are its only hints.
				Formats: []string{"wav", "mp3"},
			},
			Embeddings: api.EmbeddingsCompat{
				// Text embeddings reference:
				// https://docs.bigmodel.cn/api-reference/%E6%A8%A1%E5%9E%8B-api/%E6%96%87%E6%9C%AC%E5%B5%8C%E5%85%A5
				// model,
				// input, dimensions — no encoding_format. embedding-2 is fixed at
				// 1024 and its entry turns dimensions off; embedding-3 takes at
				// most 64 inputs per request.
				DimensionsField: api.Ptr("dimensions"),
			},
			Rerank: api.RerankCompat{
				// The reference gives a maximum length of 4096 characters for the query and for each
				// document, and caps documents at 128 per request.
				SendReturnDocs:   api.Ptr(true),
				MaxDocuments:     api.Ptr(128),
				MaxQueryChars:    api.Ptr(4096),
				MaxDocumentChars: api.Ptr(4096),
			},
			OpenAICompletions: api.OpenAICompletionsCompat{
				MaxTokensField:          api.Ptr("max_tokens"),
				ThinkingFormat:          api.Ptr(api.ThinkingFormatThinkingType),
				SupportsReasoningEffort: api.Ptr(true),
				PromptCacheAccounting:   api.Ptr(true),
				// "defaults to auto and only supports auto": none / required / named function
				// are rejected.
				ToolChoiceModes: []string{"auto"},
			},
		},
		ThinkingLevels: api.ThinkingLevelMap{
			api.ReasoningOff:     api.StringPtr("none"),
			api.ReasoningMinimal: api.StringPtr("minimal"),
			api.ReasoningLow:     api.StringPtr("low"),
			api.ReasoningMedium:  api.StringPtr("medium"),
			api.ReasoningHigh:    api.StringPtr("high"),
			api.ReasoningXHigh:   api.StringPtr("xhigh"),
			api.ReasoningMax:     api.StringPtr("max"),
		},
	}
}
