# API reference: knowledge bases and knowledge

Create knowledge bases, import and manage documents, query parse progress, and copy or move content.

Permission summary: read routes require Viewer+ and read access to the knowledge base (own, organization-shared, or visible through a shared agent); write routes require "KB creator OR Admin+" and write access. API keys: reads need `retrieve`, content writes need `ingest`, knowledge base lifecycle needs `manage_kbs` (a full-access key covers all three), and every key stays bound to its knowledge base allow-list.

Chunk, tag and chunk preview endpoints (`/chunks`, `/knowledge-bases/:id/tags`, `/chunker/preview`) are on [Chunks and tags](./02-api-chunks.md).

## Knowledge bases (/api/v1/knowledge-bases)

### POST /api/v1/knowledge-bases

Purpose: create a knowledge base. Permission: Contributor+; API key `manage_kbs` or full access. Handler: `internal/handler/knowledgebase.go`

Request body (`types.KnowledgeBase`):

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes | Name |
| `description` | string | no | Description |
| `type` | string | no | `document` (default), `faq` or `wiki` |
| `embedding_model_id` | string | no | Embedding model ID |
| `chunking_config` | object | no | Chunking configuration (chunk_size, overlap, separators, strategy and so on) |
| `image_processing_config` | object | no | Image processing: `model_id`, `image_attrs_enabled`, `image_actions` (`{ ocr: { on: [...], on_unobserved: bool } }`), `image_vector_enabled` (image vector indexing and recall, default false, see [Knowledge base](../03-features/02-knowledge-base.md)) |
| `storage_provider_config` | object | no | Storage configuration |
| `vector_store_id` | string | no | Vector store binding (an invalid binding returns code 2200 or 2201) |
| `faq_config` / `wiki_config` / `extract_config` / `indexing_strategy` | object | no | Type-specific configuration |
| `summary_model_id` | string | no | Summary model; also the default model for auto-tagging and the AI description |
| `auto_tag_config` / `profile_config` | object | no | Auto-tagging and the AI knowledge base description (document type only, see below) |

Response: 201 `{"success":true,"data":{KnowledgeBase}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Product docs","type":"document"}'
```

### Auto-tagging and AI description configuration

On create, `auto_tag_config` and `profile_config` sit at the top level; on update they go under `config.auto_tag_config` and `config.profile_config`. Both are supported for document knowledge bases only and default to `enabled=false`.

`auto_tag_config` (auto-tagging):

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | false | After parsing, pick tags asynchronously from the existing tags |
| `model_id` | string | empty | Falls back to the knowledge base `summary_model_id` |
| `max_tags` | int | 3 | Maximum tags per document, capped at 10 |
| `skip_if_tagged` | bool | true | Skip documents that already carry tags; `false` lets the model add more |

Once enabled it applies to newly parsed and reparsed documents; existing documents are not rescanned. With no candidate tags or no usable model, import still completes.

`profile_config` (AI knowledge base description):

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | false | When on, adding, deleting or moving a document or updating a summary refreshes `generated_profile` |
| `model_id` | string | empty | Falls back to the knowledge base `summary_model_id` |
| `custom_instructions` | string | empty | Extra requirements appended to the generation prompt |

`generated_profile` is read-only and written by the system; it never overwrites a hand-written `description`. It can also be generated on demand with `profile/generate` below. Update example:

```bash
curl -X PUT "$BASE/api/v1/knowledge-bases/kb-1" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Product docs","config":{"auto_tag_config":{"enabled":true,"max_tags":3,"skip_if_tagged":true}}}'
```

### GET /api/v1/knowledge-bases

Purpose: list knowledge bases. Permission: Viewer+; API key `retrieve` or full access.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `agent_id` | string | no | Only the knowledge bases visible through this shared agent |
| `agent_source_tenant_id` | uint64 | no | When the same agent is shared by several workspaces, selects the source workspace; validated against the share relation, an invalid value returns 400 |
| `creator` | string | no | `mine` or `others` |

Response: 200 `{"success":true,"data":[KnowledgeBase]}` (no pagination fields)

```bash
curl $BASE/api/v1/knowledge-bases -H "X-API-Key: $API_KEY"
```

### GET /api/v1/knowledge-bases/:id

Purpose: knowledge base detail (shared knowledge bases carry `my_permission`). Permission: Viewer+, KB read. Query parameter: `agent_id` (optional).

Response: 200 `{"success":true,"data":{KnowledgeBase}}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id

Purpose: update a knowledge base. Permission: creator OR Admin+, KB write; API key `manage_kbs` or full access.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes (`binding:"required"`) | Name |
| `description` | string | no | Description |
| `config` | object | no | Partial configuration update: `chunking_config`, `image_processing_config`, `faq_config`, `wiki_config`, `auto_tag_config`, `profile_config`, `indexing_strategy`. An omitted `image_processing_config` is left unchanged; a present one replaces the whole object (fields left out, such as `image_vector_enabled`, fall back to false) |
| `vlm_config` | object | no | Multimodal configuration, replaced as a whole. Only `enabled`, `model_id` (must be a VLLM model), `description_language` and `custom_instructions` (at most 4000 characters) are read; fields left out are treated as empty, so omit the object to leave it unchanged. On a shared knowledge base this needs the owner workspace or an admin share (an editor share gets 403). The legacy inline fields `model_name`, `base_url`, `api_key` and `interface_type` are never written: enabling a managed `model_id` or disabling clears stored legacy values, except that a pure legacy knowledge base (no stored `model_id`, only `model_name` and `base_url`) echoing back its GET result (`enabled=false` with `model_name` and `base_url` matching storage) is treated as no change. For every other knowledge base, `enabled=true` without `model_id` returns 400 |

Response: 200 `{"success":true,"data":{KnowledgeBase}}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Product docs v2"}'
```

### DELETE /api/v1/knowledge-bases/:id

Purpose: delete a knowledge base (locked to the owner workspace plus Admin; a shared editor cannot delete). Permission: creator OR Admin+, KB write; API key `manage_kbs` or full access.

Response: 200 `{"success":true,"message":"Knowledge base deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/knowledge-bases/kb-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id/pin

Purpose: pin or unpin (stored per user). Permission: Viewer+, KB read. No request body.

Response: 200 `{"success":true,"data":{KnowledgeBase (is_pinned toggled)}}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/pin -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledge-bases/:id/hybrid-search (GET also accepted)

Purpose: low-level recall inside a knowledge base (vector plus keyword). No rerank by default, recall scores are returned; rerank is optional. Suited to recall evaluation, precomputed vectors and other cases that need control over raw recall. For ordinary retrieval use [`knowledge-search`](./02-api-chat.md); to choose between them see [Which retrieval API](./01-api-overview.md#retrieval-api). Permission: Viewer+, KB read; API key `retrieve` or full access. GET with a JSON body is kept only for backward compatibility; POST is recommended.

Query parameter: `resource_urls=handle|public` (`public` replaces `resource://` references in result `content` and `image_info` with loadable direct links, see [API overview](./01-api-overview.md)).

Request body (`types.SearchParams`):

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query_text` | string | conditional | Query text (required unless `query_embedding` is given with keyword matching disabled; always required when rerank is on) |
| `query_embedding` | []float32 | no | Precomputed vector |
| `vector_threshold` / `keyword_threshold` | float64 | no | Match thresholds |
| `match_count` | int | no | Maximum results (default 50) |
| `disable_keywords_match` / `disable_vector_match` | bool | no | Turn off one recall path |
| `knowledge_base_ids` | []string | no | Search several knowledge bases in one call; the path `:id` must be among them (otherwise 404) and all of them must share one embedding model (otherwise 400) |
| `knowledge_ids` | []string | no | Restrict to these knowledge items |
| `tag_ids` | []string | no | Tag filter (OR) |
| `only_recommended` | bool | no | FAQ: recommended entries only |
| `skip_context_enrichment` | bool | no | Skip parent chunk and context completion |
| `rerank` | object | no | Present means rerank is on (`{}` uses the workspace's configured model); fields in [the rerank object](./01-api-overview.md#retrieval-api) |

Response: 200 `{"success":true,"data":[SearchResult]}`; with `rerank` the response also carries `meta.rerank` (see [meta.rerank diagnostics](./01-api-overview.md#retrieval-api)).

```bash
curl -X POST "$BASE/api/v1/knowledge-bases/kb-1/hybrid-search?resource_urls=public" -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"query_text":"refund process","match_count":5}'

# Fix the recall parameters, then rerank with a chosen model
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/hybrid-search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"query_text":"refund process","vector_threshold":0.3,"match_count":5,"rerank":{"model_id":"rr-1","threshold":0.2}}'
```

### POST /api/v1/knowledge-bases/copy

Purpose: copy content across knowledge bases (asynchronous task). Permission: Contributor+; API key `manage_kbs` or full access (the source and target allow-list is checked in the handler).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `source_id` | string | yes (`binding:"required"`) | Source knowledge base |
| `target_id` | string | no | Target knowledge base (created automatically when empty) |
| `task_id` | string | no | Custom task ID |

Response: 200 `{"success":true,"data":{"task_id","source_id","target_id","message"}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/copy -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"source_id":"kb-1"}'
```

### POST /api/v1/knowledge-bases/:id/duplicate

Purpose: create a copy of a knowledge base (settings only; no content, index, or shares). Permission: Contributor+, source KB read; API key `manage_kbs` or full access. No request body.

Response: 201 `{"success":true,"data":{"source_id","target_id","message","knowledge_base":{...}}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/duplicate -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledge-bases/:id/profile/generate

Purpose: regenerate the knowledge base's AI description (`generated_profile`) now. Runs one document profile aggregation and one small model call synchronously; the hand-written `description` is untouched. Permission: same as updating a knowledge base (creator or Admin with KB write); API key `manage_kbs` or full access. No request body. Document type only; 400 when no model is configured; 502 (code 1011) with the cause when the model call fails or returns no usable description.

Response: 200 `{"success":true,"data":{"gist","topics":[...],"typical_questions":[...],"stats":{"document_count",...},"status":"ready","model_id","generated_at"}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/profile/generate -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/copy/progress/:task_id

Purpose: copy progress (tasks are isolated per workspace). Permission: Viewer+; API key `retrieve`, `manage_kbs` or full access.

Response: 200 `{"success":true,"data":{status,progress,message,...}}`

```bash
curl $BASE/api/v1/knowledge-bases/copy/progress/task-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/move-targets

Purpose: list the knowledge bases that can be move targets (same type, same embedding model, not temporary). Permission: Viewer+, KB read.

Response: 200 `{"success":true,"data":[KnowledgeBase]}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/move-targets -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/files

Purpose: knowledge-base-scoped file proxy (renders images inside shared knowledge base content; the request's tenant context is rewritten to the knowledge base owner). Permission: Viewer+, KB read; a key restricted to specific knowledge bases is refused, a workspace-wide `retrieve` or full-access key passes. Registered by `serveKBScopedFiles` (`internal/router/files.go`).

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `file_path` | string | yes | `provider://...` storage path (`..` is rejected) |

Response: 200 file stream (`Content-Type` inferred from the file name; `Cache-Control: private, no-store`).

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/files?file_path=local://1/exports/chart.png" \
  -H "Authorization: Bearer $TOKEN" -o chart.png
```

## Knowledge (KB content, /api/v1/knowledge-bases/:id/knowledge and /api/v1/knowledge)

### POST /api/v1/knowledge-bases/:id/knowledge/file

Purpose: create a knowledge item by uploading a file. Permission: KB creator OR Admin+, KB write; API key `ingest` or full access. Handler: `internal/handler/knowledge.go`

multipart/form-data fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `file` | file | yes | The uploaded file |
| `fileName` | string | no | Custom file name, or a relative path with directories such as `docs/spec/design.md`; the directory part becomes the folder, the last segment the file name |
| `metadata` | JSON string | no | Custom metadata |
| `enable_multimodel` | bool | no | Multimodal processing switch |
| `tag_ids` | string | no | Comma-separated tag IDs |
| `channel` | string | no | Ingest channel |
| `process_config` | JSON string | no | Parse configuration overrides (`KnowledgeProcessOverrides`), see the next table |

Common `process_config` fields (all optional; an omitted field uses the knowledge base configuration):

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `summary_enabled` | bool | true | Generate a summary for this import; when off, parsing, indexing and everything else still run |
| `parser_engine_rules` | []object | KB configuration | Parser engine per file type |
| `parser_engine_overrides` | map[string]string | empty | Engine parameters such as `pdf_force_scanned` |
| `chunking_config` | object | KB configuration | Chunking parameters |
| `enable_multimodel` / `vlm_config` / `asr_config` | - | KB configuration | Multimodal and speech-to-text |
| `question_generation_config` | object | KB configuration | Question generation |
| `graph_enabled` / `extract_config` | - | KB configuration | Knowledge graph extraction |

Response: 200 `{"success":true,"data":{Knowledge}}`. A duplicate file returns 409 with `error.details.code` set to `"duplicate_file"` and the existing Knowledge in `error.details.data`. Same-name items that are being deleted or whose parse failed do not count as duplicates.

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/knowledge/file \
  -H "X-API-Key: $API_KEY" -F 'file=@./manual.pdf' -F 'enable_multimodel=true'
```

**Uploading into a folder:** pass a path relative to the knowledge base root in `fileName`, with `/` between directories, keeping the file name and extension. The upload endpoint has no separate `folder_path` field.

```bash
curl -X POST "$BASE/api/v1/knowledge-bases/kb-1/knowledge/file" \
  -H "X-API-Key: $API_KEY" \
  -F 'file=@./design.md' \
  -F 'fileName=docs/spec/design.md'
```

This request files the document under `docs/spec`; the returned Knowledge has `folder_path` `docs/spec` and `file_name` `design.md`. `file=@./design.md` names the local file, `fileName` the location in the knowledge base. With `fileName` omitted or set to a bare file name, the document lands at the knowledge base root.

Folders of a document knowledge base are derived from each document's `folder_path`, so nothing is created up front: after an upload to a new path, the folder and its intermediate levels appear in the folder tree. Creating or keeping an empty folder (one with no documents and no documents in subfolders) is not supported. The Wiki folder endpoints manage Wiki folders, not document knowledge bases.

After uploading, `GET /api/v1/knowledge-bases/:id/knowledge/folders` on this page returns the folder tree. Existing documents move to a new path with `POST /api/v1/knowledge/folder` without re-uploading; that move endpoint is the one that takes a `folder_path` request field.

### POST /api/v1/knowledge-bases/:id/knowledge/url

Purpose: create a knowledge item by fetching a URL. Permission and API key as above.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `url` | string | yes (`binding:"required"`) | Address to fetch |
| `file_name` / `file_type` / `title` | string | no | Overrides |
| `enable_multimodel` | *bool | no | Multimodal switch |
| `tag_ids` | []string | no | Tags |
| `channel` | string | no | Channel |
| `process_config` | object | no | Parse overrides |

Response: 201 `{"success":true,"data":{Knowledge}}`; a duplicate URL returns 409 (`error.details.code` is `"duplicate_url"`).

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/knowledge/url -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"url":"https://example.com/doc"}'
```

### POST /api/v1/knowledge-bases/:id/knowledge/manual

Purpose: create a manual (Markdown) knowledge item. Permission and API key as above.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `title` | string | no | Title |
| `content` | string | no | Markdown content |
| `status` | string | no | `draft` or `publish` |
| `tag_ids` | []string | no | Tags |
| `channel` | string | no | Channel |
| `process_config` | object | no | Parse overrides |

Response: 200 `{"success":true,"data":{Knowledge}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/knowledge/manual -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"title":"FAQ digest","content":"# Content","status":"publish"}'
```

### GET /api/v1/knowledge-bases/:id/knowledge

Purpose: list the knowledge items in a knowledge base. Permission: Viewer+, KB read; API key `retrieve` or full access.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `page` / `page_size` | int | no | Pagination (defaults 1 and 20) |
| `tag_ids` | string | no | Comma-separated tags (OR) |
| `keyword` | string | no | Keyword |
| `file_type` | string | no | File type filter |
| `parse_status` | string | no | `pending`, `processing`, `finalizing`, `completed` or `failed` |
| `source` | string | no | A channel, or `manual` / `url` |
| `start_time` / `end_time` | string | no | RFC3339, filters on `updated_at` |
| `folder_path` | string | no | Filter by folder; an empty string means the knowledge base root, omitting the parameter disables the folder filter |
| `folder_recursive` | bool | no | With `folder_path`, `true` includes documents in subfolders |
| `sort_by` | string | no | Sort field: `updated_at`, `created_at` or `file_name`; default `created_at` |
| `sort_order` | string | no | Sort direction: `asc` or `desc`; default `desc` |

Without sort parameters the order is `created_at desc`; any other value returns 400. With `updated_at`, reparsing, editing and status changes affect the order; with `file_name`, sorting is case-insensitive on the displayed file name and falls back to the title and then the source when the file name is empty. Ties are ordered by knowledge ID so pages stay stable.

Response: 200 `{"success":true,"data":[Knowledge],"total","page","page_size"}`

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/knowledge?page=1&parse_status=completed" -H "X-API-Key: $API_KEY"
```

### POST /api/v1/knowledge-bases/:id/knowledge/batch-download

Purpose: download the original files of several documents from one knowledge base as a ZIP. Permission is the same as single-file download: Contributor+ with KB write (an organization-shared Viewer cannot download); API key `retrieve` or full access.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `ids` | []string | yes | Knowledge IDs, 1 to 200 |

Behavior:

- The original files may total at most 512 MiB; more returns 400.
- Items without an original file (web page imports, for example) are skipped; when none of the selected items has one, the response is 400.
- The ZIP keeps the knowledge base folder structure; duplicate names get a numeric suffix.
- Any ID that does not exist or belongs to another knowledge base returns 404; a read failure returns 500; an archive with missing files is never produced.
- One instance handles at most 4 batch downloads at a time; more returns 429.

Response: 200 `application/zip` stream, named like `knowledge-files-20260923-150405.zip`.

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/knowledge/batch-download \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"ids":["k-1","k-2"]}' -o knowledge-files.zip
```

### GET /api/v1/knowledge-bases/:id/knowledge/folders

Purpose: the knowledge base's folder tree. Uploading a whole directory keeps its structure (the `knowledges.folder_path` column exists since migration `000079`; paths from historical `file_name` values were backfilled into it). Permission: Viewer+ with KBAccessRead.

The tree is aggregated from each item's `folder_path`; intermediate folders with no direct documents but with documents in subfolders are returned too. Once the last document in a folder and all of its subfolders is moved away, the folder disappears from the tree.

Response: 200. Example (only the `docs/spec/design.md` uploaded above):

```json
{
  "success": true,
  "data": {
    "root_document_count": 0,
    "total_document_count": 1,
    "folders": [{
      "path": "docs",
      "name": "docs",
      "document_count": 0,
      "total_count": 1,
      "children": [{
        "path": "docs/spec",
        "name": "spec",
        "document_count": 1,
        "total_count": 1
      }]
    }]
  }
}
```

`root_document_count` is the number of documents at the knowledge base root and `total_document_count` the number in the whole knowledge base; a folder node's `document_count` counts direct documents only, `total_count` includes every subfolder.

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/knowledge/folders -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id/knowledge/folders

Purpose: rename or move a folder, re-pathing all of its subfolders with it. When the target path exists the two folders merge; moving a folder into its own subtree is not allowed. Permission: KB owner or Admin+ with KBAccessWrite.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `from` | string | yes | Current path |
| `to` | string | yes | New path |

Response: 200 `{"success":true,"data":{"moved_count":N,"folder_path":"<normalized to>"}}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/knowledge/folders -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"from":"design-docs/old","to":"archive/design-docs"}'
```

### DELETE /api/v1/knowledge-bases/:id/knowledge

Purpose: clear every item in a knowledge base (destructive). Permission: Admin+, KB write; API key full access only.

Response: 200 `{"success":true,"message":"Knowledge base contents clear task submitted","data":{"deleted_count":N}}`; an already empty knowledge base returns `message:"Knowledge base is already empty"` with `deleted_count:0`.

```bash
curl -X DELETE $BASE/api/v1/knowledge-bases/kb-1/knowledge -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/batch

Purpose: fetch knowledge items by ID (across knowledge bases; the handler checks access itself). Permission: Viewer+; API key `retrieve` or full access.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `ids` | []string | yes | Knowledge IDs (repeat the parameter) |
| `kb_id` | string | no | Restrict to one knowledge base |
| `agent_id` | string | no | Shared agent scope |
| `agent_source_tenant_id` | uint64 | no | Source workspace selector for the shared agent, validated against the share relation |

Response: 200 `{"success":true,"data":[Knowledge]}`. Items in `pending`, `processing` or `finalizing` also carry `last_activity_at` (RFC3339), the later of the row's `updated_at` and the newest write to any of its spans. An item with no progress for more than 20 minutes also carries `stall_state`: `queued` means a task is still waiting in the asynq queue or the Wiki persistent queue (backlog), `stalled` means nothing is left that could advance it (probably stuck). The verdict is the same one housekeeping uses; the queue side is one full-queue scan shared by every request and cached for 60 seconds. When the probe fails, `stall_state` is omitted and the UI shows the item as parsing normally.

```bash
curl "$BASE/api/v1/knowledge/batch?ids=k-1&ids=k-2" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id

Purpose: knowledge item detail. Permission: Viewer+, parent KB read.

Response: 200 `{"success":true,"data":{Knowledge}}`

```bash
curl $BASE/api/v1/knowledge/k-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id/stages and GET /api/v1/knowledge/:id/spans

Purpose: parse stages and trace (both paths use the same handler, `GetKnowledgeSpans`). Permission: Viewer+, parent KB read. Query parameter: `attempt` (int, 0 = latest attempt).

Response: 200 `{"success":true,"data":{"knowledge_id","attempt","latest_attempt","parse_status","current_stage","last_activity_at","stall_state","trace":{...},"last_error":{...}}}`

`last_activity_at` is returned only while parsing is in flight, as the later of the row's `updated_at` and the newest write to any span of this attempt; `stall_state` has the meaning above. `current_stage` is the stage still running; when no stage is running (for example `finalizing`, where the post-processing stage is closed but sub-tasks such as the summary are still running) it is the stage that owns the still-running child span. For an item that housekeeping judged stuck, the span at the stuck position is marked failed with `TASK_STALLED` and `last_error` points at it first.

```bash
curl $BASE/api/v1/knowledge/k-1/spans -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/knowledge/:id

Purpose: delete a knowledge item (asynchronous). Permission: KB creator OR Admin+, KB write; API key `ingest` or full access.

Response: 200 `{"success":true,"message":"Delete task submitted","data":{"task_id"}}`

```bash
curl -X DELETE $BASE/api/v1/knowledge/k-1 -H "X-API-Key: $API_KEY"
```

### PUT /api/v1/knowledge/:id

Purpose: update knowledge metadata. Permission as above. Request body (a subset of `types.Knowledge`): `title`, `description`, `custom_metadata` (all optional). An omitted `description` keeps the current summary, an explicit empty string clears it, and a non-empty value stores a hand-written summary; the UI edits it on the document content page.

`custom_metadata` is descriptive metadata entered by users (stored apart from the system's internal `metadata`, migration `000078`); the validation rules are in `internal/application/service/knowledge.go`:

| Constraint | Value |
| --- | --- |
| Field count | at most 20 |
| Key length | 1 to 64 characters, not blank |
| Value type | string, number, boolean or null |
| Value length | at most 1000 characters |

The update replaces the whole object. When the metadata changes and the document already has a summary, one summary refresh is queued (`summary_status` becomes `pending`). The metadata text takes part in summary generation and in the document-level model context (`Knowledge.CustomMetadataText()`).

Response: 200 `{"success":true,"message":"Knowledge updated successfully","data":{Knowledge}}`

```bash
curl -X PUT $BASE/api/v1/knowledge/k-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"title":"New title","custom_metadata":{"department":"R&D","classification":"internal","version":3}}'
```

### POST /api/v1/knowledge/:id/regenerate-summary

Purpose: regenerate a document's summary after its chunks or custom metadata were edited. Permission: KB owner or Admin+, with write access to the parent knowledge base.

Two behaviors: a document with no summary yet (`summary_status` empty or `none`) generates one synchronously; a document with a summary queues a refresh task instead, `summary_status` becomes `pending`, and `knowledge_summary_refresh.go` runs it asynchronously.

Response: 200 `{"success":true,"data":{Knowledge}}`. 400 when the knowledge base has no summary model, the document has too little text, or a newer edit superseded the run; 502 (code 1011) with the model's error when the synchronous summary model call fails; other failures return the generic 500.

```bash
curl -X POST $BASE/api/v1/knowledge/k-1/regenerate-summary -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge/manual/:id

Purpose: update the content of a manual knowledge item (a subset of `ManualKnowledgePayload`: `title`, `content`, `status`, ...). Permission as above.

Response: 200 `{"success":true,"data":{Knowledge}}`

```bash
curl -X PUT $BASE/api/v1/knowledge/manual/k-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"content":"# Updated content","status":"publish"}'
```

### POST /api/v1/knowledge/:id/reparse

Purpose: reparse a knowledge item. Permission as above. Request body (optional): `{"process_config":{...}}`.

Response: 200 `{"success":true,"message":"Knowledge reparse task submitted","data":{Knowledge}}`

```bash
curl -X POST $BASE/api/v1/knowledge/k-1/reparse -H "X-API-Key: $API_KEY"
```

### POST /api/v1/knowledge/:id/cancel-parse

Purpose: cancel parsing. Permission as above. No request body.

Response: 200 `{"success":true,"message":"Knowledge parse cancelled","data":{Knowledge}}`

```bash
curl -X POST $BASE/api/v1/knowledge/k-1/cancel-parse -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id/download

Purpose: download the original source file (stricter than preview: Contributor+ with KB write; an organization-shared Viewer cannot download source files). API key `retrieve` or full access.

Response: 200 binary stream (`application/octet-stream`).

```bash
curl -OJ $BASE/api/v1/knowledge/k-1/download -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/:id/preview

Purpose: preview the parsed file content. Permission: Viewer+, KB read.

Response: 200 preview stream (text or HTML).

```bash
curl $BASE/api/v1/knowledge/k-1/preview -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge/image/:id/:chunk_id

Purpose: update a chunk's image information (caption, OCR text and so on). Permission: KB creator OR Admin+, KB write. Path parameters: `id` knowledge ID, `chunk_id` chunk ID. Request body: `{"image_info": "<string>"}`, the image information serialized as a string.

Response: 200 `{"success":true,"message":"Knowledge chunk image updated successfully"}`

```bash
curl -X PUT $BASE/api/v1/knowledge/image/k-1/c-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"image_info":"{\"caption\":\"Architecture diagram\"}"}'
```

### GET /api/v1/knowledge/search

Purpose: file search across knowledge bases (the @file picker in conversations). Permission: Viewer+; API key `retrieve` or full access.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `keyword` / `query` | string | conditional | Keyword (the two are equivalent); when empty, `recent=true` is required, otherwise 400 |
| `file_types` | string | no | Comma-separated extension filter such as `csv,xlsx` |
| `offset` / `limit` | int | no | Pagination; `limit` defaults to 20, range 1 to 100 |
| `recent` | bool | no | With an empty keyword, return recent files |
| `agent_id` | string | no | Shared agent scope |
| `agent_source_tenant_id` | uint64 | no | Source workspace selector for the shared agent, validated against the share relation |

Response: 200 `{"success":true,"data":[Knowledge],"has_more":bool,"total":N}`

```bash
curl "$BASE/api/v1/knowledge/search?keyword=report&limit=20" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge/move/progress/:task_id

Purpose: progress of a move task. Permission: Viewer+; API key `retrieve` or full access.

Response: 200 `{"success":true,"data":{MoveProgress}}`

```bash
curl $BASE/api/v1/knowledge/move/progress/task-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge/tags

Purpose: update the tags of several knowledge items. Permission: Contributor+; API key `ingest` or full access (the KB allow-list is checked in the handler).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `updates` | `map[string][]string` | yes (`binding:"required,min=1"`) | knowledge_id to tag_ids |
| `kb_id` | string | no | Restrict to one knowledge base |

Response: 200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/knowledge/tags -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"updates":{"k-1":["t-1"]},"kb_id":"kb-1"}'
```

### POST /api/v1/knowledge/batch-reparse

Purpose: reparse several items. Permission: Contributor+; API key `ingest` or full access.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `kb_id` | string | yes (`binding:"required"`) | Knowledge base ID |
| `ids` | []string | yes (`binding:"required"`) | Knowledge IDs |
| `process_config` | object | no | Parse overrides |

Response: 200 `{"success":true,"message":"Batch reparse task submitted","data":{"task_id","reparse_count"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/batch-reparse -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"kb_id":"kb-1","ids":["k-1","k-2"]}'
```

### POST /api/v1/knowledge/batch-delete

Purpose: delete several items (at most 200). Permission: Contributor+; API key `ingest` or full access.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `kb_id` | string | yes (`binding:"required"`) | Knowledge base ID |
| `ids` | []string | yes (`binding:"required"`) | Knowledge IDs (at most 200) |

Response: 200 `{"success":true,"message":"Batch delete task submitted","data":{"task_id","deleted_count"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/batch-delete -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"kb_id":"kb-1","ids":["k-1"]}'
```

### POST /api/v1/knowledge/folder

Purpose: file several documents under a folder (changes the folder only; the knowledge base stays the same and nothing is reparsed). Permission: Contributor+; API key `ingest`.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `kb_id` | string | yes | Knowledge base ID |
| `knowledge_ids` | []string | yes | Documents to move |
| `folder_path` | string | no | Target folder path relative to the knowledge base root, such as `docs/spec`; a new path needs no prior creation, an empty string moves the documents back to the root |

Response: 200 `{"success":true,"data":{"moved_count":N,"folder_path":"<normalized path>"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/folder -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"kb_id":"kb-1","knowledge_ids":["k-1","k-2"],"folder_path":"design-docs"}'
```

### POST /api/v1/knowledge/move

Purpose: move knowledge items to another knowledge base (asynchronous). Permission: Contributor+; API key `ingest` or full access (both the source and the target must be in the allow-list).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `knowledge_ids` | []string | yes (`binding:"required,min=1"`) | Items to move |
| `source_kb_id` | string | yes (`binding:"required"`) | Source knowledge base |
| `target_kb_id` | string | yes (`binding:"required"`) | Target knowledge base |
| `mode` | string | yes (`binding:"required,oneof=reuse_vectors reparse"`) | Reuse the vectors or reparse |

Response: 200 `{"success":true,"data":{"task_id","source_kb_id","target_kb_id","knowledge_count","message"}}`

```bash
curl -X POST $BASE/api/v1/knowledge/move -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"knowledge_ids":["k-1"],"source_kb_id":"kb-1","target_kb_id":"kb-2","mode":"reuse_vectors"}'
```

## Implementation reference

Route registration: `RegisterKnowledgeBaseRoutes` and `RegisterKnowledgeRoutes` in `internal/router/routes_knowledge.go`. Handlers: `internal/handler/knowledgebase.go`, `internal/handler/knowledge.go`.
