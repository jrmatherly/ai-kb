// Package providers registers Volcengine Ark (Doubao) through its
// OpenAI-compatible chat endpoint.
//
// Facts (https://www.volcengine.com/docs/82379/1494384 = Chat API reference,
// https://docs.volcengine.com/docs/ark/deep-thinking and
// https://docs.volcengine.com/docs/ark/base-url-and-authentication):
//   - the data-plane base URL is https://ark.cn-beijing.volces.com/api/v3
//     (chat completions at /chat/completions) with Bearer API-key auth; AK/SK
//     signing is the alternative and is what the managed rerank service uses;
//   - output cap stays `max_completion_tokens` ("controls the maximum model output length (including
//     the model answer and the chain-of-thought length)"), which explicitly "cannot be set together with
//     the max_tokens field". Its documented range is [1, 65536];
//     `max_tokens` defaults to 4096 and covers the answer only;
//   - thinking is switched with `thinking: {"type": ...}` where the type is
//     enabled | disabled | auto - Ark does accept "auto" ("the model decides whether
//     deep thinking is needed"), although only doubao-seed-1-6-250615 lists it as
//     supported today. Every model tagged deep thinking defaults to enabled;
//   - `reasoning_effort` takes none | minimal | low | medium | high | xhigh |
//     max and "every model that supports the field accepts all 7 levels"; each model then maps
//     the rungs it does not implement onto equivalents (Seed 2.x folds
//     xhigh/max into high, glm-5-3-flash folds them into max, and on most
//     models `minimal` switches thinking off). The vendor map therefore
//     passes all seven through verbatim;
//   - doubao-seed-1-6-flash-250828 and doubao-seed-1-6-vision-250815 are
//     absent from the reasoning_effort table, so those entries turn it off;
//   - glm-5-3-flash-260828 "always thinks; disabling thinking is no longer supported", so it carries
//     "off": null;
//   - temperature is [0, 2] but is "fixed at 1; a manually set value is ignored" on
//     doubao-seed-2-0-pro-260215 and doubao-seed-2-0-lite-260215;
//   - tool_choice takes none / auto / required / a named function, and
//     parallel_tool_calls (default true, false only on doubao-seed-1.6 and
//     later), response_format (text | json_object | json_schema, beta) and
//     stream_options.include_usage are all documented, so the protocol
//     defaults stand. glm-5-2-260617, deepseek-v4-pro-ga-260813 and
//     deepseek-v4-flash-ga-260731 return streaming usage even without
//     include_usage;
//   - usage reports `prompt_tokens_details.cached_tokens` (implicit cache);
//     explicit prefix / session caching is a separate Context Cache API and
//     is not driven from here;
//   - in tool-calling turns Ark also returns `encrypted_content` next to
//     `reasoning_content` and asks for it to be replayed; omitting it is not
//     an error but degrades multi-turn agent quality;
//   - embeddings post to /api/v3/embeddings/multimodal with `dimensions`
//     defaulting to 2048, and answer one fused vector for the whole input
//     (https://docs.volcengine.com/docs/ark/multimodal-vectorization-api).
//     It is the only embedding API the current docs list, and the vectorization model
//     list names only the two doubao-embedding-vision snapshots. The text
//     endpoint, /api/v3/embeddings in the OpenAI shape, now sits under
//     the retired documentation archive, so doubao-embedding-large-text was
//     removed from models.json; rows that still name a doubao-embedding text
//     model are routed to that endpoint by pattern;
//   - rerank is the VikingDB Knowledge Base service signed with AK/SK at
//     https://api-knowledgebase.mlp.cn-beijing.volces.com/api/knowledge/service/rerank
//     (the API key field carries the access key; the secret key, region and
//     instruction come from the rerank-only extra fields). Documented models
//     are doubao-seed-rerank and base-multilingual-rerank
//     (https://www.volcengine.com/docs/84313/1254474). datas takes at most
//     200 items, and the default instruction is the console's, verbatim, as
//     the page asks for results that match the console;
//   - Ark additionally serves an Anthropic Messages surface at
//     https://ark.cn-beijing.volces.com/api/compatible/v1 with x-api-key
//     auth. This package configures OpenAI Chat Completions, which is the
//     surface every model page documents first.
//
// unverified: whether the retired text endpoint still answers for accounts
// that used it. Rows naming a text model reach it either way; before the
// catalog they were sent to the multimodal path.
//
// unverified: the model list spells lengths as "256k" / "1024k" without
// saying whether k is 1000 or 1024, so the context windows in models.json are
// left at their present values.
//
// unverified: prices in models.json come from the Ark pricing console, which
// the public docs do not render.
package providers

import (
	_ "embed"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/models/api"
	"github.com/jrmatherly/ai-kb/internal/types"
)

//go:embed assets/volcengine.svg
var volcengineIcon []byte

// VolcengineID is the provider identifier stored on model rows.
const VolcengineID = "volcengine"

// VolcengineBaseURL is the OpenAI-compatible Ark chat endpoint.
const VolcengineBaseURL = "https://ark.cn-beijing.volces.com/api/v3"

// VolcengineEmbeddingBaseURL is the Ark multimodal embedding endpoint.
const VolcengineEmbeddingBaseURL = "https://ark.cn-beijing.volces.com/api/v3/embeddings/multimodal"

const (
	volcengineMultimodalEmbeddingPath = "/api/v3/embeddings/multimodal"
	volcengineTextEmbeddingPath       = "/api/v3/embeddings"
)

// VolcengineRerankBaseURL is the Knowledge Base managed rerank endpoint.
const VolcengineRerankBaseURL = "https://api-knowledgebase.mlp.cn-beijing.volces.com"

// VolcengineAnthropicBaseURL is the documented Anthropic Messages surface. It is not
// the default for this vendor; operators who want it configure it explicitly.
const VolcengineAnthropicBaseURL = "https://ark.cn-beijing.volces.com/api/compatible/v1"

func newVolcengineProvider() *Definition {
	rerankOnly := []types.ModelType{types.ModelTypeRerank}
	return &Definition{
		ID:    VolcengineID,
		Name:  "Volcengine Ark",
		Names: map[string]string{"zh-CN": "\u706b\u5c71\u5f15\u64ce Volcengine"},
		Description: "doubao-seed-2-1-pro-260628, deepseek-v4-pro-ga-260813, " +
			"doubao-embedding-vision-250615, doubao-seed-rerank, etc.",
		Website:      "https://console.volcengine.com/ark",
		Icon:         volcengineIcon,
		API:          api.APIOpenAICompletions,
		Order:        12,
		RequiresAuth: true,
		Auth:         AuthBearer,
		URLPatterns:  []string{"volces.com", "volcengine"},
		DefaultBaseURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: VolcengineBaseURL,
			types.ModelTypeEmbedding:   VolcengineEmbeddingBaseURL,
			types.ModelTypeRerank:      VolcengineRerankBaseURL,
			types.ModelTypeVLLM:        VolcengineBaseURL,
		},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeEmbedding,
			types.ModelTypeRerank,
			types.ModelTypeVLLM,
		},
		// Rerank is signed with an IAM access-key pair, so the first
		// credential is an Access Key ID rather than an Ark bearer token.
		CredentialLabels: []CredentialLabel{{
			Label:        "Access Key ID",
			Labels:       map[string]string{"zh-CN": "Access Key ID\uff08AK/SK \u7b7e\u540d\uff09"},
			Placeholder:  "Volcengine IAM access key id (AKLT...)",
			Placeholders: map[string]string{"zh-CN": "\u706b\u5c71\u5f15\u64ce IAM Access Key ID\uff08AKLT \u5f00\u5934\uff09"},
			Hint:         "Rerank is signed with an IAM key pair; this is not an Ark API key.",
			Hints:        map[string]string{"zh-CN": "Rerank \u4f7f\u7528 IAM \u5bc6\u94a5\u5bf9\u7b7e\u540d\uff0c\u4e0d\u662f\u65b9\u821f\u7684 API Key\u3002"},
			ModelTypes:   rerankOnly,
			Required:     true,
		}},
		ExtraFields: []ExtraField{
			{
				Key:         "secret_key",
				Label:       "Secret Key",
				Labels:      map[string]string{"zh-CN": "Secret Key\uff08AK/SK \u7b7e\u540d\uff09"},
				Type:        "password",
				Required:    true,
				Placeholder: "Volcengine IAM secret key (the API Key field holds the access key)",
				Placeholders: map[string]string{
					"zh-CN": "\u706b\u5c71\u5f15\u64ce IAM Secret Access Key\uff08API Key \u90a3\u4e00\u680f\u586b\u7684\u662f Access Key ID\uff09",
				},
				ModelTypes: rerankOnly,
				Secret:     true,
			},
			{
				Key:         "region",
				Label:       "Region",
				Labels:      map[string]string{"zh-CN": "\u5730\u57df"},
				Type:        "string",
				Default:     "cn-beijing",
				Placeholder: "cn-beijing",
				ModelTypes:  rerankOnly,
			},
			{
				Key:         "instruction",
				Label:       "Rerank Instruction",
				Labels:      map[string]string{"zh-CN": "\u91cd\u6392\u6307\u4ee4"},
				Type:        "string",
				Default:     "Whether the document answers the query or matches the content retrieval intent",
				Placeholder: "Instruction passed to the rerank model",
				Placeholders: map[string]string{
					"zh-CN": "\u4f20\u7ed9\u91cd\u6392\u6a21\u578b\u7684 instruction",
				},
				ModelTypes: rerankOnly,
			},
		},
		RerankAPI:    api.RerankVolcengineKnowledge,
		EmbeddingAPI: api.EmbeddingArk,
		// Embedding rows have stored three shapes of base URL: the full
		// multimodal URL that was the default, .../api/v3 like chat, and the
		// bare host. All of them name the same service, so the request is
		// placed from the host.
		Endpoint: func(r EndpointRequest) (string, map[string]string) {
			if r.ModelType != types.ModelTypeEmbedding {
				return "", nil
			}
			root := strings.TrimRight(r.BaseURL, "/")
			if i := strings.Index(root, "/api/"); i >= 0 {
				root = root[:i]
			}
			if r.EmbeddingAPI == api.EmbeddingOpenAI {
				// The retired doubao-embedding text models, whose archived
				// reference still names this endpoint.
				return root + volcengineTextEmbeddingPath, nil
			}
			return root + volcengineMultimodalEmbeddingPath, nil
		},
		Compat: VendorCompat{
			// https://docs.volcengine.com/docs/ark/multimodal-vectorization-api:
			// the only embedding API Ark still lists. It fuses everything in
			// one request into a single vector, so a request carries one text.
			// dimensions defaults to 2048 and the current models also serve
			// 1024; encoding_format defaults to float and the reference's own
			// curl sends it. `instructions` is documented too and deliberately
			// not declared: like Jina's task it changes the document vectors
			// (Tencent/WeKnora#1401).
			Embeddings: api.EmbeddingsCompat{
				SendEncodingFormat: api.Ptr(true),
				DimensionsField:    api.Ptr("dimensions"),
				MaxBatchSize:       api.Ptr(1),
			},
			Rerank: api.RerankCompat{
				// datas "array length at most 200"
				// (https://docs.volcengine.com/docs/vector_database_vikingdb/Rerank).
				// The pre-catalog client split at 50, a constant of its own
				// rather than a documented ceiling.
				MaxDocuments:   api.Ptr(200),
				MaxConcurrency: api.Ptr(4),
			},
			OpenAICompletions: api.OpenAICompletionsCompat{
				ThinkingFormat:          api.Ptr(api.ThinkingFormatThinkingType),
				SupportsReasoningEffort: api.Ptr(true),
				PromptCacheAccounting:   api.Ptr(true),
			},
		},
		// Ark accepts all seven effort rungs on every model that supports the
		// field and maps the ones a model does not implement itself, so each
		// level is passed through under its own name.
		ThinkingLevels: api.ThinkingLevelMap{
			api.ReasoningMinimal: api.StringPtr("minimal"),
			api.ReasoningLow:     api.StringPtr("low"),
			api.ReasoningMedium:  api.StringPtr("medium"),
			api.ReasoningHigh:    api.StringPtr("high"),
			api.ReasoningXHigh:   api.StringPtr("xhigh"),
			api.ReasoningMax:     api.StringPtr("max"),
		},
	}
}
