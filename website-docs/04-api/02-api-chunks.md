# API reference: chunks and tags

A chunk is the smallest retrieval unit; tags classify documents inside a knowledge base. Both groups are knowledge base resources and share the permission rules of [Knowledge bases and knowledge](./02-api-knowledge.md): reads require Viewer+ and read access to the parent knowledge base (API key `retrieve`), writes require "KB creator OR Admin+" and write access (API key `ingest`), and every API key stays bound to its knowledge base allow-list.

Shared conventions (base URL, authentication, error codes, pagination) are in the [API overview](./01-api-overview.md).

## Chunks (/api/v1/chunks)

Handler: `internal/handler/chunk.go`. Read: Viewer+ and parent KB read (API key `retrieve` or full access). Write: KB creator OR Admin+ and parent KB write (API key `ingest` or full access). Routes addressed by `:knowledge_id/:id` also check that the chunk belongs to that knowledge item: an unknown chunk returns 404, a chunk owned by another knowledge item returns 403. A write the knowledge base does not allow returns 403 and an unexpected server failure returns 500 on every route below.

### GET /api/v1/chunks/:knowledge_id

Purpose: list the chunks of a knowledge item.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `page` | int | no | Default 1 |
| `page_size` | int | no | Default 10, capped at 100 |
| `chunk_type` | string | no | Repeatable. Filters by chunk type; without it only `text` chunks are returned |

Chunk types (`internal/types/chunk.go`): `text`, `parent_text`, `image_ocr`, `image_caption`, `image_vector`, `summary`, `entity`, `relationship`, `faq`, `web_search`, `table_summary`, `table_column`, `wiki_page`.

Response: 200 `{"success":true,"data":[Chunk],"total","page","page_size"}`

```bash
curl "$BASE/api/v1/chunks/k-1?page=1" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/chunks/by-id/:id

Purpose: fetch one chunk by chunk ID (no `knowledge_id` needed).

Response: 200 `{"success":true,"data":{Chunk}}`. `Chunk.source_locators` is the chunk's position in the original file; the structure is described under `source_locators` in the [API overview](01-api-overview.md).

```bash
curl $BASE/api/v1/chunks/by-id/c-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/chunks/:knowledge_id/:id

Purpose: edit a chunk's content or its enabled state (a versioned optimistic update since migration `000078`).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `content` | string | no | New content; must not be empty after trimming, at most 200000 bytes |
| `is_enabled` | bool | no | Enable or disable the chunk |
| `expected_revision` | int | no | The `content_revision` you expect, for optimistic concurrency control |

Constraints and side effects:

- Only `text` chunks can be edited; any other type returns 400 (`only text chunks can be edited`). An empty `content`, a `content` over 200000 bytes and a new image URL (below) also return 400.
- When `expected_revision` differs from the current `content_revision` the response is **409** (`Chunk was modified by another user; refresh and retry`).
- An edit may not introduce an image URL that the source content does not contain. Deleting a Markdown image disables the matching OCR, caption and image-vector child chunks (they are disabled, not deleted, so a revert can re-enable them).
- A successful edit increments `content_revision`, stores the previous version in the `chunk_revisions` table and moves `index_status` through `processing` to `ready`. When rebuilding the retrieval index fails the row is still saved with `index_status = failed`; submitting the same content again retries the index.
- Editing a child chunk writes the change back into the parent chunk by offset (the parent's `source_content` stays immutable). An edited chunk loses its `source_locators`.
- A change to the content or the enabled state queues one document summary refresh.

Response: 200 `{"success":true,"data":{Chunk},"summary_status":"pending","description":"..."}`

```bash
curl -X PUT $BASE/api/v1/chunks/k-1/c-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"content":"Corrected content","expected_revision":2}'
```

### GET /api/v1/chunks/:knowledge_id/:id/revisions

Purpose: the chunk's version history (`chunk_revisions` table, newest revision first).

Response: 200 `{"success":true,"data":[{ChunkRevision}]}`. Each item carries `revision`, `content`, `is_enabled`, `editor_id`, `edit_source`, `edited_at`.

```bash
curl $BASE/api/v1/chunks/k-1/c-1/revisions -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/chunks/:knowledge_id/:id/revert

Purpose: roll back to a historical revision. A revert is itself a new edit: `content_revision` keeps increasing and the current content is stored as a new historical revision.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `revision` | int | yes | Target revision number (non-negative) |
| `expected_revision` | int | no | Optimistic lock with the same meaning as above; a conflict returns 409 |

An unknown `revision` or a non-`text` chunk returns 400.

Response: 200 `{"success":true,"data":{Chunk},"summary_status":"...","description":"..."}`

```bash
curl -X POST $BASE/api/v1/chunks/k-1/c-1/revert -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"revision":1}'
```

### DELETE /api/v1/chunks/:knowledge_id/:id

Purpose: delete one chunk.

Response: 200 `{"success":true,"message":"Chunk deleted"}`

```bash
curl -X DELETE $BASE/api/v1/chunks/k-1/c-1 -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/chunks/:knowledge_id

Purpose: delete every chunk of a knowledge item. An unknown knowledge item returns 404.

Response: 200 `{"success":true,"message":"All chunks under knowledge deleted"}`

```bash
curl -X DELETE $BASE/api/v1/chunks/k-1 -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/chunks/by-id/:id/questions

Purpose: delete one generated question of the chunk. Request body: `{"question_id":"..."}` (`binding:"required"`). A `question_id` the chunk does not have returns 404.

Response: 200 `{"success":true,"message":"Generated question deleted"}`

```bash
curl -X DELETE $BASE/api/v1/chunks/by-id/c-1/questions -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"question_id":"q-1"}'
```

### PUT /api/v1/chunks/by-id/:id/questions

Purpose: add or edit one generated question of the chunk. Request body: `{"question":"...","question_id":"..."}`; `question` is required, an empty `question_id` adds a new question, an unknown `question_id` returns 404. The saved question records the chunk's current `content_revision`.

Response: 200 `{"success":true,"data":{GeneratedQuestion}}`

```bash
curl -X PUT $BASE/api/v1/chunks/by-id/c-1/questions -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"question_id":"q-1","question":"How do I configure the vector store in AI KB?"}'
```

### POST /api/v1/chunks/by-id/:id/questions/regenerate

Purpose: regenerate the retrieval questions from the chunk's current content. A content edit does not delete the existing questions; they are marked stale (their revision no longer matches the current body) and this endpoint refreshes them. Only `text` chunks qualify, and the knowledge base needs a summary model; otherwise 400. An edit to the chunk while the questions are being generated returns 409.

Response: 200 `{"success":true,"data":[{GeneratedQuestion}]}`

```bash
curl -X POST $BASE/api/v1/chunks/by-id/c-1/questions/regenerate -H "Authorization: Bearer $TOKEN"
```

## Tags (/api/v1/knowledge-bases/:id/tags)

Handler: `internal/handler/tag.go`. Read: Viewer+ and KB read (API key `retrieve` or full access). Write: KB creator OR Admin+ and KB write (API key `ingest` or full access).

### GET /api/v1/knowledge-bases/:id/tags

Purpose: list tags. Query parameters: `page`, `page_size` (at most 1000), `keyword` (all optional).

Response: 200 `{"success":true,"data":{"total","page","page_size","data":[KnowledgeTagWithStats]}}`. Each item is a `KnowledgeTag` plus `knowledge_count` and `chunk_count`.

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/tags -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledge-bases/:id/tags

Purpose: create a tag.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes (`binding:"required"`) | Tag name; a name already used in the knowledge base returns 409 |
| `color` | string | no | Color |
| `sort_order` | int | no | Sort order |

Response: 200 `{"success":true,"data":{KnowledgeTag}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/tags -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"After-sales"}'
```

### PUT /api/v1/knowledge-bases/:id/tags/:tag_id

Purpose: update a tag (`tag_id` accepts the UUID or the integer `seq_id`). Request body: `name`, `color`, `sort_order` (pointer fields, all optional; an empty `name` returns 400).

Response: 200 `{"success":true,"data":{KnowledgeTag}}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/tags/t-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"After-sales support"}'
```

### DELETE /api/v1/knowledge-bases/:id/tags/:tag_id

Purpose: delete a tag. Query parameters: `force` (bool, delete a tag that is still referenced), `content_only` (bool, delete only the content under the tag and keep the tag). Optional request body: `{"exclude_ids":[int64]}`, the `seq_id` values of FAQ entries to keep.

A malformed body or a non-positive ID returns 400; an unknown ID returns 404; an ID that is not an FAQ entry of this knowledge base returns 403.

Response: 200 `{"success":true}`

```bash
curl -X DELETE "$BASE/api/v1/knowledge-bases/kb-1/tags/t-1?force=true" -H "Authorization: Bearer $TOKEN"
```

## Chunk debugging

### POST /api/v1/chunker/preview

Purpose: stateless chunking preview (the debug panel of the knowledge base editor). Permission: Viewer+; API key `retrieve`, `ingest` or full access. Handler: `internal/handler/chunker_debug.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `text` | string | yes (checked in code: non-empty, at most 64k characters) | Sample text |
| `chunking_config.chunk_size` | int | no | Characters per chunk |
| `chunking_config.chunk_overlap` | int | no | Overlap |
| `chunking_config.separators` | []string | no | Separators |
| `chunking_config.strategy` | string | no | `auto`, `heading`, `heuristic`, `recursive` or `legacy` |
| `chunking_config.token_limit` | int | no | Token limit |
| `chunking_config.languages` | []string | no | Language hints |
| `chunking_config.enable_parent_child` | bool | no | Split with the parent-child strategy; the returned chunks are the children (the retrieval granularity) |
| `chunking_config.parent_chunk_size` / `child_chunk_size` | int | no | Parent and child sizes, defaults 4096 and 384 |

Inline HTML `<table>` elements in the sample are converted to Markdown tables first, as during import. The text is neither stored nor embedded nor logged.

Response: 200 `{"success":true,"data":{"selected_tier","tier_chain","rejected","profile","chunks":[{"seq","start","end","size_chars","size_tokens_approx","context_header","content"}],"stats":{count,avg_chars,min_chars,max_chars,stddev_chars,truncated_to}}}`. At most 500 chunks are returned; when the split produced more, `stats.truncated_to` holds the full count and the stats still describe the full set. Text over the limit returns 413 with the limit in `error.details.limit`, and a split that exceeds 5 seconds returns 504; both use the standard error envelope.

```bash
curl -X POST $BASE/api/v1/chunker/preview -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"text":"# Title\nBody...","chunking_config":{"chunk_size":512}}'
```

## Implementation reference

Route registration: `RegisterChunkRoutes`, `RegisterKnowledgeTagRoutes` and `RegisterChunkerDebugRoutes` in `internal/router/routes_knowledge.go`.
