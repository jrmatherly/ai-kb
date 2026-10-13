// Package providers registers the catch-all vendor for any OpenAI-compatible
// endpoint (vLLM, SGLang, Xinference, Ollama's OpenAI facade, ...).
//
// Facts (no single upstream; the settings follow the vLLM / SGLang
// conventions, https://docs.vllm.ai/en/latest/features/reasoning_outputs.html
// and https://docs.sglang.io/basic_usage/openai_api_completions.html):
//   - output cap stays `max_tokens`. vLLM now marks it deprecated in favour
//     of `max_completion_tokens` but still accepts it, while SGLang,
//     GPUStack and Xinference document only `max_tokens`, so it remains the
//     one field every runtime behind this vendor understands;
//   - thinking is switched through `chat_template_kwargs.enable_thinking`,
//     which both servers document as the per-request override. vLLM also
//     accepts a top-level `reasoning_effort` with exactly this project's
//     ladder (none | minimal | low | medium | high | xhigh | max) and
//     injects enable_thinking from it, but SGLang only says effort "follows
//     a separate normalization path", so the chat-template switch is the
//     portable choice and operators on vLLM can opt into effort per model;
//   - the gpt-5*/o1*/o3*/o4* family entries stay: a relay parked behind this
//     vendor is the common way to front OpenAI, and those families need
//     max_completion_tokens, reasoning_effort and no sampling parameters
//     whatever the hop count;
//   - no default base URL: the operator must supply one (Validate enforces
//     it) and a key is optional because local deployments run without one;
//   - no URL patterns: DetectByURL falls back to this vendor when nothing
//     else matches.
//
// Unverified: `enable_thinking` is not a portable key. vLLM documents
// `{"thinking": true}` for IBM Granite and SGLang documents the same for
// DeepSeek-V3, so a non-Qwen template will ignore the switch this vendor
// sends rather than reject it. Per-model chat-template keys are not
// expressible in the compat model.
package providers

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/models"
	"github.com/jrmatherly/ai-kb/internal/models/api"
	"github.com/jrmatherly/ai-kb/internal/types"
)

//go:embed assets/generic.svg
var genericIcon []byte

func newGenericProvider() *Definition {
	return &Definition{
		ID:              GenericID,
		Name:            "Custom (OpenAI-compatible)",
		Names:           map[string]string{"zh-CN": "\u81ea\u5b9a\u4e49 (OpenAI\u517c\u5bb9\u63a5\u53e3)"},
		Description:     "Any OpenAI-compatible endpoint (vLLM, SGLang, Xinference, Ollama, ...)",
		Descriptions:    map[string]string{"zh-CN": "\u4efb\u610f OpenAI \u517c\u5bb9\u63a5\u53e3\uff08vLLM\u3001SGLang\u3001Xinference\u3001Ollama \u7b49\uff09"},
		Icon:            genericIcon,
		API:             api.APIOpenAICompletions,
		Order:           0,
		RequiresAuth:    false,
		Auth:            AuthBearer,
		DefaultBaseURLs: map[types.ModelType]string{},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeEmbedding,
			types.ModelTypeRerank,
			types.ModelTypeVLLM,
			types.ModelTypeASR,
		},
		ExtraFields: []ExtraField{{
			Key:    models.ExtraScoreScale,
			Label:  "Rerank score scale",
			Labels: map[string]string{"zh-CN": "Rerank \u5206\u6570\u6807\u5ea6"},
			Type:   "select",
			Options: []ExtraFieldOption{
				{
					Label:  "Unbounded score (Qwen3-Reranker class)",
					Labels: map[string]string{"zh-CN": "\u65e0\u754c\u5206\u6570\uff08Qwen3-Reranker \u4e00\u7c7b\uff09"},
					Value:  "logit",
				},
				{
					Label:  "0..1 relevance (BGE class)",
					Labels: map[string]string{"zh-CN": "0~1 \u76f8\u5173\u5ea6\uff08BGE \u4e00\u7c7b\uff09"},
					Value:  "probability",
				},
			},
			Placeholder:  "match the reranker actually deployed behind this endpoint",
			Placeholders: map[string]string{"zh-CN": "\u6309\u8fd9\u4e2a\u7aef\u70b9\u540e\u9762\u5b9e\u9645\u90e8\u7f72\u7684\u91cd\u6392\u6a21\u578b\u9009\u62e9"},
			// A gateway serves whatever reranker was deployed behind it, and
			// the two families disagree. The vendor default follows the
			// documentation; the operator knows which model is actually there.
			Required:   false,
			ModelTypes: []types.ModelType{types.ModelTypeRerank},
		}, {
			Key:    models.ExtraTruncatePromptTokens,
			Label:  "Rerank prompt truncation (vLLM)",
			Labels: map[string]string{"zh-CN": "Rerank \u63d0\u793a\u8bcd\u622a\u65ad\u957f\u5ea6\uff08vLLM\uff09"},
			Type:   "number",
			// Off unless the operator asks: a runtime that does not implement
			// the extension rejects the unknown field.
			Required:     false,
			Placeholder:  "empty unless the backend rejects long documents",
			Placeholders: map[string]string{"zh-CN": "\u7559\u7a7a\uff0c\u9664\u975e\u540e\u7aef\u56e0\u6587\u6863\u8fc7\u957f\u800c\u62a5\u9519"},
			ModelTypes:   []types.ModelType{types.ModelTypeRerank},
		}},
		Compat: VendorCompat{
			// Transcriptions keeps the baseline, json by default. The
			// pre-catalog client always asked for verbose_json, which not every
			// model or server can produce: OpenAI's gpt-4o transcribers accept
			// only json, and vox-box's FunASR backend answers verbose_json with
			// a bare JSON string. A row that wants segments sets
			// {"response_format": "verbose_json"} in its compat. The language
			// hint goes as OpenAI's language form field, which a server that
			// does not implement it ignores.
			Transcriptions: api.TranscriptionsCompat{
				LanguageParam: api.Ptr(api.LanguageForm),
			},
			Embeddings: api.EmbeddingsCompat{
				// Whatever the operator runs. Everything the pre-catalog client
				// sent stays: vLLM, SGLang, TEI and Ollama's OpenAI route all
				// accept it, and dimensions still goes out only when the row
				// asks for a width.
				SendEncodingFormat:          api.Ptr(true),
				DimensionsField:             api.Ptr("dimensions"),
				AcceptsTruncatePromptTokens: api.Ptr(true),
				// Used only when the row declares image input. vLLM, the most
				// common server behind this vendor, takes images only as a
				// chat conversation in `messages`; an SGLang row switches to
				// {"image_format": "object", "image_field": "image"}.
				ImageFormat: api.Ptr(api.EmbeddingImageMessages),
			},
			Rerank: api.RerankCompat{
				// Any OpenAI-compatible endpoint an operator points here is most
				// often a vLLM or SGLang server, which is where
				// truncate_prompt_tokens comes from.
				AcceptsTruncatePromptTokens: api.Ptr(true),
			},
			OpenAICompletions: api.OpenAICompletionsCompat{
				MaxTokensField: api.Ptr("max_tokens"),
				ThinkingFormat: api.Ptr(api.ThinkingFormatChatTemplateKwargs),
			},
		},
		Validate: func(cfg *Config) error {
			if cfg == nil {
				return fmt.Errorf("config is nil")
			}
			if strings.TrimSpace(cfg.BaseURL) == "" {
				return fmt.Errorf("base URL is required for generic provider")
			}
			if strings.TrimSpace(cfg.ModelName) == "" {
				return fmt.Errorf("model name is required")
			}
			return nil
		},
	}
}
