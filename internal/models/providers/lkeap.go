// Package providers registers Tencent Cloud LKEAP (Knowledge Engine Atomic
// Power) through its OpenAI-compatible endpoint.
//
// Chat facts (https://cloud.tencent.com/document/product/1772/115969 and
// the model list at https://cloud.tencent.com/document/product/1772/115963):
//   - the documented endpoint is
//     https://api.lkeap.cloud.tencent.com/v1/chat/completions, so the base
//     URL already carries the /v1 segment;
//   - authentication is a plain `Authorization: Bearer sk-...` console key,
//     unrelated to the Tencent Cloud SecretId / SecretKey pair;
//   - output cap is `max_tokens`; temperature (default 0.6, range [0, 2]) and
//     top_p (default 0.6, range (0, 1]) are both accepted, as are stop,
//     presence_penalty, frequency_penalty, Function Calling and
//     response_format json_object (json_schema on V3.1+);
//   - thinking is switched with `thinking: {"type": "enabled"|"disabled"}`,
//     documented as taking effect only on deepseek-v3.1-terminus and
//     deepseek-v3.2; DeepSeek R1 reasons unconditionally, so the R1 entries
//     carry "off": null and never receive a disable switch;
//   - `reasoning_effort` is not part of the LKEAP surface, so effort grading
//     stays off and only the thinking switch is sent.
//
// Rerank facts (https://cloud.tencent.com/document/product/1772/115339):
//   - it is not the OpenAI-compatible host but the Tencent Cloud API
//     lkeap.tencentcloudapi.com, action RunRerank, version 2024-05-22,
//     available in ap-beijing and ap-guangzhou only;
//   - it therefore needs the TC3-HMAC-SHA256 credential pair: the API key
//     field carries the SecretId and the rerank-only extra fields carry the
//     SecretKey and the region (default ap-guangzhou). The SecretKey is
//     genuinely required — TC3 signing cannot be done without it;
//   - the optional Model parameter defaults to lke-reranker-base, which is
//     documented as the only available rerank model.
//
// unverified: the current model list documents deepseek-v3-0324,
// deepseek-r1-0528, deepseek-v3.1-terminus and deepseek-v3.2. The plain
// deepseek-r1 / deepseek-v3 / deepseek-v3.1 ids kept in models.json are no
// longer listed, but no LKEAP retirement notice covers them, so they stay
// with their previously recorded limits.
//
// unverified: LKEAP documents neither tool_choice modes,
// stream_options.include_usage nor any prompt-cache accounting, so the
// protocol defaults are kept.
package providers

import (
	_ "embed"

	"github.com/jrmatherly/ai-kb/internal/models/api"
	"github.com/jrmatherly/ai-kb/internal/types"
)

//go:embed assets/lkeap.svg
var lkeapIcon []byte

// LkeapID is the provider identifier stored on model rows.
const LkeapID = "lkeap"

// LkeapBaseURL is the OpenAI-compatible chat endpoint.
const LkeapBaseURL = "https://api.lkeap.cloud.tencent.com/v1"

// LkeapRerankBaseURL is the TC3-signed cloud API host used for rerank.
const LkeapRerankBaseURL = "https://lkeap.tencentcloudapi.com"

func newLkeapProvider() *Definition {
	rerankOnly := []types.ModelType{types.ModelTypeRerank}
	return &Definition{
		ID:          LkeapID,
		Name:        "Tencent Cloud LKEAP",
		Names:       map[string]string{"zh-CN": "\u817e\u8baf\u4e91 LKEAP"},
		Description: "deepseek-v3.2, deepseek-v3.1-terminus, deepseek-r1-0528, lke-reranker-base",
		Descriptions: map[string]string{
			"zh-CN": "DeepSeek-V3.2\u3001DeepSeek-V3.1-Terminus\u3001DeepSeek-R1-0528\u3001lke-reranker-base",
		},
		Website:      "https://cloud.tencent.com/product/lkeap",
		Icon:         lkeapIcon,
		API:          api.APIOpenAICompletions,
		Order:        23,
		RequiresAuth: true,
		Auth:         AuthBearer,
		URLPatterns:  []string{"lkeap.cloud.tencent.com", "api.lkeap", "lkeap.tencentcloudapi.com"},
		DefaultBaseURLs: map[types.ModelType]string{
			types.ModelTypeKnowledgeQA: LkeapBaseURL,
			types.ModelTypeRerank:      LkeapRerankBaseURL,
		},
		ModelTypes: []types.ModelType{
			types.ModelTypeKnowledgeQA,
			types.ModelTypeRerank,
		},
		// The rerank API authenticates with a TC3-signed CAM identity, so the
		// first credential is a SecretId, not a bearer token. Left as the
		// generic "API Key" an operator pastes an `sk-` key that can never
		// sign a request.
		CredentialLabels: []CredentialLabel{{
			Label:        "SecretId",
			Labels:       map[string]string{"zh-CN": "SecretId\uff08TC3 \u7b7e\u540d\uff09"},
			Placeholder:  "Tencent Cloud SecretId (AKID...)",
			Placeholders: map[string]string{"zh-CN": "\u817e\u8baf\u4e91 SecretId\uff08AKID \u5f00\u5934\uff09"},
			Hint:         "Rerank is signed with a CAM key pair; this is not an sk- API key.",
			Hints:        map[string]string{"zh-CN": "Rerank \u4f7f\u7528 CAM \u5bc6\u94a5\u5bf9\u7b7e\u540d\uff0c\u4e0d\u662f sk- \u5f00\u5934\u7684 API Key\u3002"},
			ModelTypes:   rerankOnly,
			Required:     true,
		}},
		ExtraFields: []ExtraField{
			{
				Key:         "secret_key",
				Label:       "Secret Key",
				Labels:      map[string]string{"zh-CN": "SecretKey\uff08TC3 \u7b7e\u540d\uff09"},
				Type:        "password",
				Required:    true,
				Placeholder: "Tencent Cloud SecretKey (the API Key field holds the SecretId)",
				Placeholders: map[string]string{
					"zh-CN": "\u817e\u8baf\u4e91 SecretKey\uff08API Key \u90a3\u4e00\u680f\u586b\u7684\u662f SecretId\uff09",
				},
				ModelTypes: rerankOnly,
				Secret:     true,
			},
			{
				// RunRerank is only published in ap-beijing and ap-guangzhou.
				Key:         "region",
				Label:       "Region",
				Labels:      map[string]string{"zh-CN": "\u5730\u57df"},
				Type:        "select",
				Default:     "ap-guangzhou",
				Placeholder: "ap-guangzhou",
				Options: []ExtraFieldOption{
					{Label: "ap-guangzhou", Value: "ap-guangzhou"},
					{Label: "ap-beijing", Value: "ap-beijing"},
				},
				ModelTypes: rerankOnly,
			},
		},
		RerankAPI: api.RerankTencentLKEAP,
		Compat: VendorCompat{
			Rerank: api.RerankCompat{
				// RunRerank takes at most 60 documents, and Query plus Docs
				// together at most 2000 characters.
				MaxDocuments:    api.Ptr(60),
				MaxRequestChars: api.Ptr(2000),
				// Batches went out one at a time before the shared batching
				// layer existed. A 2000-character budget splits a large
				// candidate set into many requests, so the default fan-out of
				// four would be a new burst against RunRerank.
				MaxConcurrency: api.Ptr(1),
			},
			OpenAICompletions: api.OpenAICompletionsCompat{
				MaxTokensField: api.Ptr("max_tokens"),
				ThinkingFormat: api.Ptr(api.ThinkingFormatThinkingType),
			},
		},
	}
}
