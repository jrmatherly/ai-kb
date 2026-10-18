# API reference: models and initialization

Manage models, test connections, initialize knowledge bases and start evaluation tasks. The WeKnoraCloud endpoints connect that cloud service and are enabled per deployment (`AIKB_WEKNORACLOUD_ENABLED`).

System information and system administration (`/system`, `/system/admin`) are in [System and platform administration](./02-api-system.md).

## Models (/api/v1/models)

API key: `manage_models` or full-access.

### GET /api/v1/models/providers

Purpose: the provider catalog. The frontend renders the provider dropdown, icons, extra fields and model picker from it. Permission: Viewer+. Query parameter: `model_type` (optional: `chat/embedding/rerank/vllm/asr`; backend values such as `KnowledgeQA` are accepted too; an unknown value returns 400). When set, only providers that support that type are returned, with that type's models. The `weknoracloud` provider is omitted unless the deployment sets `AIKB_WEKNORACLOUD_ENABLED=true`. Handler: `internal/handler/model_catalog.go`. The provider list itself is described in [Model management](../03-features/06-models.md#built-in-providers).

Response: 200 `{"success":true,"data":[ModelProviderDTO]}`, each item:

| Field | Description |
| --- | --- |
| `value` / `label` / `labels` / `description` / `descriptions` / `website` | Provider id, brand name, names and descriptions per language |
| `icon` | `data:image/svg+xml;base64,...`, usable directly as `<img src>` |
| `api` / `auth` / `requiresAuth` | Default protocol (`openai-completions` and others), auth style, whether a key is required |
| `defaultUrls` / `modelTypes` | Default URL per model type and the supported types. `defaultUrls` is returned only to Admin+ (or a full-access / `manage_tenant_settings` API key); other callers get an empty value |
| `extraFields` | Provider extra configuration field definitions (`key,label,labels,type,required,default,placeholder,options,model_types,secret`); values are stored in `parameters.extra_config` |
| `credentialLabels` | Name and hint of the credential input for some model types (for Volcengine and LKEAP rerank the API Key field is really an Access Key ID / SecretId) |
| `models` | Built-in model catalog (`id,name,type,api,reasoning,input,context_window,max_output_tokens,dimension,thinking_levels,cost,source`); `source` is the provider documentation link the model's parameters are based on |
| `thinking` | Provider-level thinking encoding summary (`format`, `levels`) |
| `order` | Sort value for the list |

```bash
curl "$BASE/api/v1/models/providers?model_type=chat" -H "Authorization: Bearer $TOKEN"
```

### GET|POST /api/v1/models/catalog/resolve

Purpose: resolve the effective connection configuration (protocol, thinking levels, context) from provider, model name, `base_url` and `extra_config`, so the model editor can show it live. Permission: Viewer+.

Parameters (query parameters for GET, a JSON body for POST, same fields): `provider` (provider ID), `model`, `base_url`, `model_type` (default `chat`), `api`, `thinking_control`, `remote_model_name`, plus the non-secret extra fields the provider declares (such as `api_version` for Azure). A POST body may also carry a `spec` object (same shape as the model's `parameters.spec`) to preview a per-row catalog override. Secret fields are never accepted.

Response: 200 `{"success":true,"data":{provider,api,remote_model,cataloged,model,capabilities,base_url,url}}`, where `capabilities` is `{provider,api,cataloged,reasoning,thinking_levels,thinking_format,input,context_window,max_output_tokens,max_tokens_field}`. `base_url` and `url` (the actual request URL, returned only by providers that compute their own, such as Azure) are returned only to Admin+ (or a full-access / `manage_tenant_settings` API key). An unresolvable request returns 400. The same `capabilities` structure is returned as `ModelResponse.capabilities` for remote chat and vision models.

```bash
curl "$BASE/api/v1/models/catalog/resolve?provider=deepseek&model=deepseek-v4-pro" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/models

Purpose: create a model. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes (`binding:"required"`) | Model name |
| `display_name` | string | No | Display name |
| `type` | string | Yes (`binding:"required"`) | Model type: `KnowledgeQA` / `Embedding` / `Rerank` / `VLLM` / `ASR` (stored as given; frontend spellings such as `chat` are not accepted) |
| `source` | string | Yes (`binding:"required"`) | Source (`local` / `remote`) |
| `description` | string | No | Description |
| `parameters` | object | Yes (`binding:"required"`) | Connection parameters (`base_url`, `provider`, `extra_config`, `spec`, `context_window` and others; fields in [Model management](../03-features/06-models.md#model-configuration-fields)). `api_key` / `app_secret` can be sent on create and are changed afterwards through the credentials sub-resource |

`parameters` is validated against the model catalog (an unknown protocol, a wrong compat key or an invalid thinking level returns 400), and `base_url` goes through SSRF validation. A request whose `parameters.provider` is `weknoracloud` returns 400 while `AIKB_WEKNORACLOUD_ENABLED` is off, on POST and on PUT; a PUT that moves a stored `weknoracloud` model to another provider is accepted.

Self-hosted `generic` / `Qwen3-VL-Embedding` models submit independent inputs as a batch through a two-dimensional `messages` array by default (verified with vLLM 0.29.0); a single input keeps the one-dimensional conversation format. Text is still batched by `BATCH_EMBED_SIZE`; image batch calls default to at most 5 images, a conservative client-side batch size adjustable through `parameters.spec.compat.max_image_batch_size`. Upload parsing still schedules tasks per image and never merges images across tasks. If an older server does not support two-dimensional `messages`, set `parameters.spec.compat.batch_messages=false` to go back to one request per input; other models use conversation batching only when that capability is enabled explicitly.

Batching keeps each input's conversation content and vector index mapping, but the server is not guaranteed to compute bit-identical results across batch sizes. Where strict reproducibility matters, verify the numeric behavior of the deployment; vLLM's [Batch Invariance](https://docs.vllm.ai/en/v0.29.0/features/batch_invariance/) mode has to be enabled on the server and verified against the actual model. Set `AIKB_EMBEDDING_TEST_URL` and `AIKB_EMBEDDING_TEST_FIXTURE` and run `go test ./internal/models/embedding -run '^TestLiveQwenMessagesBatchParity$' -v -count=1`: the fixture is `{"texts":[...],"image_paths":[...]}` with at least two texts and two images; private or IP endpoints need `SSRF_WHITELIST`, and `AIKB_EMBEDDING_TEST_API_KEY` carries the credential. The test compares single, batched, reversed and repeated single requests, records the maximum absolute error and the minimum cosine similarity, fails above a strict threshold, and writes nothing to a knowledge base.

Response: 201 `{"success":true,"data":{ModelResponse}}` (`id,name,type,source,parameters,is_default,is_builtin,status,credentials,capabilities,...`; the response carries no secrets)

```bash
curl -X POST $BASE/api/v1/models -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"gpt-5.5","type":"KnowledgeQA","source":"remote","parameters":{"provider":"openai","base_url":"https://api.openai.com/v1","api_key":"sk-..."}}'
```

### GET /api/v1/models

Purpose: list models. Permission: Viewer+.

Response: 200 `{"success":true,"data":[ModelResponse]}`

```bash
curl $BASE/api/v1/models -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/models/:id

Purpose: model details. Permission: Viewer+.

Response: 200 `{"success":true,"data":{ModelResponse}}`

```bash
curl $BASE/api/v1/models/m-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/models/:id/copy

Purpose: copy a non-built-in model configuration in the current workspace. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `display_name` | string | Yes (`binding:"required"`) | Display name of the copy; must not be empty after trimming whitespace, at most 255 characters |

The server reads the full configuration by the source model ID in the path: the copy keeps the source's `name`, and `type`, `source`, `description`, `parameters` and the stored credentials are copied along; the client cannot resend them. The copy inherits neither built-in nor default status; an already active local model is not downloaded again, while other inactive local models re-enter the download flow.

Response: 201 `{"success":true,"data":{ModelResponse}}` (the response carries no secrets)

A missing source model returns 404; a built-in source model, an invalid ID or an invalid `display_name` returns 400.

```bash
curl -X POST $BASE/api/v1/models/m-1/copy -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"display_name":"GPT-4o copy"}'
```

### POST /api/v1/models/:id/debug

Purpose: debug a saved model (makes a real upstream call and incurs cost). Permission: Admin+. form-data fields: `input` (at most 64KB), `options` (JSON-encoded debug options: `system_prompt`, `temperature` (0 to 2), `top_p`, `max_tokens` (1 to 8192), `thinking`, `reasoning_effort` (`off/auto/minimal/low/medium/high/xhigh/max`, overrides `thinking` when set)), `documents` (JSON array, at most 100 items), `file` (optional).

Response: 200 `{"success":true,"data":{"ok",elapsed_ms,request,raw_response,observations,error}}`

```bash
curl -X POST $BASE/api/v1/models/m-1/debug -H "Authorization: Bearer $TOKEN" -F 'input=Hello'
```

### PUT /api/v1/models/:id

Purpose: update a model (built-in models are limited to SystemAdmin by the service layer). Permission: Admin+ or SystemAdmin (`AdminOrSystemAdmin`). Body: `name`, `display_name` (pointer), `description`, `parameters` (stored secrets are kept), `source`, `type` (all optional).

Response: 200 `{"success":true,"data":{ModelResponse}}`

```bash
curl -X PUT $BASE/api/v1/models/m-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"display_name":"GPT-4o mini"}'
```

### DELETE /api/v1/models/:id

Purpose: delete a model. Permission: Admin+.

Response: 200 `{"success":true,"message":"Model deleted"}`

While the model is still referenced by a knowledge base, an agent or long-term memory in the current workspace, the response is HTTP 400. The compatible message is kept, `error.code=2300` is set, and `error.details` names the objects and where they reference the model:

```json
{
  "success": false,
  "error": {
    "code": 2300,
    "message": "model is used by 2 knowledge base(s); reconfigure or remove those references before deleting",
    "details": {
      "knowledge_bases": [
        {"id": "kb-1", "name": "Product docs", "bindings": ["vlm_model"]},
        {"id": "kb-2", "name": "Engineering", "bindings": ["vlm_model"]}
      ],
      "agents": [],
      "long_term_memory": {"bindings": []},
      "knowledge_base_total": 2,
      "agent_total": 0
    }
  }
}
```

Knowledge base binding values: `embedding_model`, `summary_model`, `image_processing_model`, `vlm_model`, `asr_model`, `wiki_synthesis_model`, `auto_tag_model`. Agent binding values: `chat_model`, `rerank_model`, `vlm_model`, `asr_model`, `query_understand_model`, `follow_up_model`. Long-term memory binding values: `embedding_model`, `extract_model`. The details carry each object's `id`, `name` and merged `bindings`, plus `knowledge_base_total` / `agent_total`. Each list holds at most 50 entries; the delete guard goes by the totals.

```bash
curl -X DELETE $BASE/api/v1/models/m-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/models/:id/credentials

Purpose: set model secrets (secrets never travel through the main PUT). Permission: Admin+ or SystemAdmin. Handler: `internal/handler/model_credentials.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `api_key` | *string | No | New API key |
| `app_secret` | *string | No | New App Secret (with both omitted, only the status is returned) |

Response: 200 `{"success":true,"data":{"fields":{"api_key":{"configured":bool},"app_secret":{"configured":bool}}}}`

```bash
curl -X PUT $BASE/api/v1/models/m-1/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"api_key":"sk-..."}'
```

### DELETE /api/v1/models/:id/credentials/:field

Purpose: delete one secret field (`api_key` or `app_secret`). Permission: Admin+ or SystemAdmin.

Response: 204 No Content

```bash
curl -X DELETE $BASE/api/v1/models/m-1/credentials/api_key -H "Authorization: Bearer $TOKEN"
```

## WeKnoraCloud

Handler: `internal/handler/weknoracloud.go`. API key: `manage_models`/full. Both endpoints are available only when the deployment sets `AIKB_WEKNORACLOUD_ENABLED=true` (default off, read per request); while it is off they return 400.

### POST /api/v1/weknoracloud/credentials

Purpose: save the WeKnoraCloud SaaS credentials. Permission: Admin+. Body: `{"app_id":"...","app_secret":"..."}` (both `binding:"required"`).

Response: 200 `{"success":true,"message":"Credentials saved"}`. Credentials the WeKnoraCloud check rejects (HTTP 401 or 403) return 400; an unreachable or failing WeKnoraCloud returns 502 (code 1011) with the cause in the message; 400 also when the deployment gate is off.

```bash
curl -X POST $BASE/api/v1/weknoracloud/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"app_id":"app","app_secret":"secret"}'
```

### GET /api/v1/models/weknoracloud/status

Purpose: WeKnoraCloud readiness probe. Permission: Viewer+.

Response: 200, the service status object; 400 when the deployment gate is off.

```bash
curl $BASE/api/v1/models/weknoracloud/status -H "Authorization: Bearer $TOKEN"
```

## Initialization (/api/v1/initialization)

Handler: `internal/handler/initialization.go`. KB configuration endpoints: API key `manage_kbs` (write) / `retrieve` (read); model check endpoints: `manage_models` (full-access works for all).

### GET /api/v1/initialization/config/:kbId

Purpose: read the KB's current model and parsing configuration. Permission: Viewer+, KB read.

A model's `baseUrl` is returned only to Admin+ of the KB's own workspace (or a full-access / `manage_tenant_settings` API key). A workspace that reaches the KB through an organization share sees only whether credentials are configured (`credentials.*`), not the source workspace's model URLs or storage bucket details.

Response: 200 `{"success":true,"data":{"hasFiles",llm,embedding,rerank,multimodal,documentSplitting,nodeExtract,questionGeneration}}`

```bash
curl $BASE/api/v1/initialization/config/kb-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/initialization/initialize/:kbId

Purpose: initialize the KB's model and parsing configuration (the first-time setup wizard). Permission: KB creator OR Admin+, KB write.

Only the KB's own workspace may call it; a workspace with edit access through an organization share is rejected (403). When the KB already has models bound, this endpoint updates those models in place, which needs the same permission as `PUT /models/:id` (Admin+, or an API key with the `manage_models` capability), otherwise 403.

Main fields (`InitializationRequest`):

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `llm.source` / `llm.modelName` | string | Yes | LLM source and model name |
| `llm.baseUrl` / `llm.apiKey` | string | No | Connection parameters |
| `embedding.source` / `embedding.modelName` | string | Yes | Embedding model |
| `embedding.baseUrl` / `embedding.apiKey` / `embedding.dimension` | mixed | No | Connection and dimension |
| `rerank.enabled` + `rerank.modelName/baseUrl/apiKey` | mixed | No | Rerank configuration |
| `multimodal.enabled` + `multimodal.vlm.*` + `multimodal.storageType` + `multimodal.cos.*\|minio.*` | mixed | No | Multimodal and image storage |
| `documentSplitting.chunkSize` / `separators` | int / []string | Yes | Chunking configuration |
| `documentSplitting.chunkOverlap` | int | No | Overlap |
| `nodeExtract.*` | mixed | No | Graph extraction (enabled/text/tags/nodes/relations) |
| `questionGeneration.*` | mixed | No | Question generation (enabled/questionCount) |

Response: 200 `{"success":true,"message":"Knowledge base configuration updated","data":{"models":[Model],"knowledge_base":{KnowledgeBase}}}`

```bash
curl -X POST $BASE/api/v1/initialization/initialize/kb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"llm":{"source":"remote","modelName":"gpt-4o-mini"},"embedding":{"source":"remote","modelName":"text-embedding-3-small"},"documentSplitting":{"chunkSize":512,"separators":["\n\n"]}}'
```

### PUT /api/v1/initialization/config/:kbId

Purpose: update the KB's model and chunking configuration (`KBModelConfigRequest`: `llmModelId` required; `embeddingModelId`, `vlm_config`, `asr_config`, `documentSplitting.*`, `multimodal.enabled`, `storageProvider`, `storageBackendId`, `nodeExtract.*`, `questionGeneration.*` optional). Permission: KB creator OR Admin+, KB write.

Through an organization share, the effective share permission must be admin; an editor can edit content but not settings (403). The storage binding (`storageBackendId` / `storageProvider`) can be changed only by the KB's own workspace; another workspace that submits a value different from the current one gets 403.

Response: 200 `{"success":true,"message":"Configuration updated"}`

```bash
curl -X PUT $BASE/api/v1/initialization/config/kb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"llmModelId":"m-1","embeddingModelId":"m-2"}'
```

### GET /api/v1/initialization/ollama/status

Purpose: Ollama availability probe. Permission: Viewer+.

Response: 200 `{"success":true,"data":{"available","version","baseUrl","error"}}`

```bash
curl $BASE/api/v1/initialization/ollama/status -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/initialization/ollama/models

Purpose: list local Ollama models. Permission: Viewer+. This route and the model check and download routes below return 502 (code 1011) with the cause when Ollama cannot be reached or fails.

Response: 200 `{"success":true,"data":{"models":[...]}}`

```bash
curl $BASE/api/v1/initialization/ollama/models -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/initialization/ollama/models/check

Purpose: check in one call whether models already exist. Permission: Admin+. Body: `{"models":["llama3"]}` (`binding:"required"`).

Response: 200 `{"success":true,"data":{"models":{"llama3":true}}}`

```bash
curl -X POST $BASE/api/v1/initialization/ollama/models/check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"models":["llama3"]}'
```

### POST /api/v1/initialization/ollama/models/download

Purpose: pull an Ollama model (asynchronous task). Permission: Admin+. Body: `{"modelName":"llama3"}` (`binding:"required"`).

Response: 200 `{"success":true,"data":{"taskId","modelName","status","progress"}}`

```bash
curl -X POST $BASE/api/v1/initialization/ollama/models/download -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"modelName":"llama3"}'
```

### GET /api/v1/initialization/ollama/download/progress/:taskId

Purpose: download task progress. Permission: Viewer+.

Response: 200 `{"success":true,"data":{id,modelName,status,progress,message,startTime,endTime}}`

```bash
curl $BASE/api/v1/initialization/ollama/download/progress/task-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/initialization/ollama/download/tasks

Purpose: list all download tasks. Permission: Viewer+.

Response: 200 `{"success":true,"data":[DownloadTask]}`

```bash
curl $BASE/api/v1/initialization/ollama/download/tasks -H "Authorization: Bearer $TOKEN"
```

### Model connectivity checks (all POST, permission Admin+)

The body is always a `ModelTestRequest`:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `source` | string | No | Default `remote` |
| `modelName` | string | Yes | Model name |
| `baseUrl` / `apiKey` / `appSecret` | string | No | Connection parameters |
| `provider` / `interfaceType` | string | No | Provider / interface type |
| `dimension` / `supportsDimensionOverride` | int / bool | No | Embedding dimension; whether the dimension is sent in the request |
| `customHeaders` / `extraConfig` | map | No | Extensions |
| `spec` | object | No | Per-row catalog override, same shape as the model's `parameters.spec` |
| `modelId` | string | No | ID of a stored model: secrets, `extraConfig` and `spec` missing from the request are filled in from that model |

| Endpoint | Purpose | Response data |
| --- | --- | --- |
| `POST /api/v1/initialization/remote/check` | LLM remote connectivity | `{available,message}` |
| `POST /api/v1/initialization/embedding/test` | Embedding test | `{available,message,dimension}` |
| `POST /api/v1/initialization/rerank/check` | Rerank test | `{available,message}` |
| `POST /api/v1/initialization/asr/check` | ASR test | `{available,message}` |

```bash
curl -X POST $BASE/api/v1/initialization/remote/check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"modelName":"gpt-4o-mini","baseUrl":"https://api.openai.com/v1","apiKey":"sk-..."}'
```

### POST /api/v1/initialization/multimodal/test

Purpose: end-to-end multimodal test (VLM plus image storage). Permission: Admin+. multipart fields: `image` (required), `vlm_model`, `vlm_base_url` (required), `vlm_api_key`, `vlm_interface_type`, `storage_type` (`cos|minio`, required) with the matching `cos_*`/`minio_*` fields, `chunk_size`, `chunk_overlap`, `separators`.

Response: 200 `{"success":true,"data":{"success","caption","ocr","processing_time"}}`

```bash
curl -X POST $BASE/api/v1/initialization/multimodal/test -H "Authorization: Bearer $TOKEN" \
  -F 'image=@demo.png' -F 'vlm_model=qwen-vl' -F 'vlm_base_url=http://x' -F 'storage_type=minio'
```

### POST /api/v1/initialization/extract/text-relation

Purpose: test graph extraction on text. Permission: Admin+. Body: `text` (required, at most 5000 characters), `tags` (required, at least one), `model_id` (required).

Response: 200 `{"success":true,"data":{"nodes":[GraphNode],"relations":[GraphRelation]}}`

```bash
curl -X POST $BASE/api/v1/initialization/extract/text-relation -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"text":"Alice works at Acme","tags":["person","company"],"model_id":"m-1"}'
```

### POST /api/v1/initialization/extract/fabri-tag

Purpose: generate sample tags. Permission: Admin+. No body.

Response: 200 `{"success":true,"data":{"tags":[...]}}`

```bash
curl -X POST $BASE/api/v1/initialization/extract/fabri-tag -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/initialization/extract/fabri-text

Purpose: generate sample text for the given tags. Permission: Admin+. Body: `{"tags":[...],"model_id":"m-1"}` (`model_id` required).

Response: 200 `{"success":true,"data":{"text":"..."}}`

```bash
curl -X POST $BASE/api/v1/initialization/extract/fabri-text -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"model_id":"m-1","tags":["person"]}'
```

## Evaluation (/api/v1/evaluation)

Handler: `internal/handler/evaluation.go`. API key: `run_evaluations`/full.

### POST /api/v1/evaluation

Purpose: start an evaluation task (drives LLM calls and incurs cost). Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `dataset_id` | string | No | Dataset ID |
| `knowledge_base_id` | string | No | Target KB |
| `chat_id` | string | No | Chat model ID |
| `rerank_id` | string | No | Rerank model ID |

With `chat_id` or `rerank_id` omitted, the first model of that type in the workspace is used. The evaluation KB copies only the source KB's embedding and summary models. Details: [Evaluation](../03-features/15-evaluation.md).

Response: 200 `{"success":true,"data":{evaluation task}}`

```bash
curl -X POST $BASE/api/v1/evaluation -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"knowledge_base_id":"kb-1","chat_id":"m-1"}'
```

### GET /api/v1/evaluation

Purpose: read evaluation results. Permission: Viewer+. Query parameter: `task_id` (required).

Response: 200 `{"success":true,"data":{evaluation result}}`

```bash
curl "$BASE/api/v1/evaluation?task_id=task-1" -H "Authorization: Bearer $TOKEN"
```

## Implementation reference

Route registration: `internal/router/router.go` calls `RegisterModelRoutes`, `RegisterInitializationRoutes`, `RegisterEvaluationRoutes` and `RegisterWeKnoraCloudRoutes` (defined in `internal/router/routes_infra.go`). Handlers: `internal/handler/model.go`, `internal/handler/model_catalog.go`, `internal/handler/model_credentials.go`, `internal/handler/initialization.go`, `internal/handler/evaluation.go`, `internal/handler/weknoracloud.go`.
