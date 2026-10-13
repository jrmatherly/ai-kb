# API reference: FAQ and Wiki

Manage FAQ entries and Wiki pages inside a knowledge base: import, search, edit and restore versions.

Both groups are knowledge base content sub-resources. Reads require Viewer+ and read access to the knowledge base (API key `retrieve` or full access). Writes require "KB creator OR Admin+" and write access (API key `ingest` or full access), and every key stays bound to its knowledge base allow-list. The cross-knowledge-base Wiki search `POST /wiki-search` has no knowledge base in its path: it requires Viewer+ and API key `retrieve` or full access, and the handler checks the allow-list and shared-knowledge-base Viewer access for every target knowledge base.

The feature pages [FAQ](../03-features/17-faq.md) and [Wiki](../03-features/14-wiki.md) describe the data model, the import pipeline and version history.

## FAQ (/api/v1/knowledge-bases/:id/faq)

Routes: `RegisterFAQRoutes` in `internal/router/routes_knowledge.go`. Handler: `internal/handler/faq.go`.

### GET /api/v1/knowledge-bases/:id/faq/entries

Purpose: list FAQ entries.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `page` / `page_size` | int | no | Pagination |
| `tag_id` | int | no | Legacy single tag seq_id |
| `tag_ids` | string | no | Comma-separated tag UUIDs (OR semantics) |
| `keyword` | string | no | Keyword |
| `search_field` | string | no | `standard_question` / `similar_questions` / `answers` (all fields by default) |
| `sort_order` | string | no | `asc` (default: newest updated first) |
| `is_enabled` | bool | no | Filter by enabled state: `true` enabled only, `false` disabled only, omitted returns all; any other value returns 400 |

Response: 200 `{"success":true,"data":{paginated FAQEntry list}}`

A `FAQEntry` carries `id` (seq_id), `chunk_id`, `knowledge_id`, `knowledge_base_id`, `tag_id`, `tag_name`, `is_enabled`, `is_recommended`, `standard_question`, `similar_questions`, `negative_questions`, `answers`, `answer_strategy`, `index_mode`, `chunk_type`, `created_at` and `updated_at`. Search results add `score`, `match_type` and `matched_question`.

```bash
curl "$BASE/api/v1/knowledge-bases/kb-1/faq/entries?page=1&is_enabled=false" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/faq/entries/export

Purpose: export the FAQ. Query parameter: `format` (`csv`, the default, or `json`). The JSON export is an array compatible with `FAQEntryPayload`, so an export can be edited and re-imported.

Response: 200 file download (`text/csv` with a UTF-8 BOM, or `application/json`).

```bash
curl -OJ "$BASE/api/v1/knowledge-bases/kb-1/faq/entries/export?format=csv" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/faq/entries/:entry_id

Purpose: one FAQ entry (`entry_id` is the integer seq_id).

Response: 200 `{"success":true,"data":{FAQEntry}}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/faq/entries/12 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledge-bases/:id/faq/entries

Purpose: batch upsert / import (asynchronous task). Handler method `UpsertEntries`.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `entries` | []FAQEntryPayload | yes (`binding:"required"`) | Entries to import |
| `mode` | string | yes (`binding:"oneof=append replace"`) | Append or replace |
| `knowledge_id` | string | no | FAQ knowledge entity ID |
| `task_id` | string | no | Custom task ID: letters, digits, `_` and `-` only, at most 128 characters, otherwise 400 |
| `dry_run` | bool | no | Validate only, write nothing |

The task processes entries in batches of 50. In `replace` mode, existing entries whose content hash matches an incoming entry are kept as they are; the rest are deleted and re-created, and tags left unused are removed afterwards. A `dry_run` runs the same validation asynchronously, returns a `task_id`, and records no knowledge base activity. Only one import may run per knowledge base at a time; a second request returns 400 with the running task ID.

Response: 200 `{"success":true,"data":{"task_id"}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/faq/entries -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"mode":"append","entries":[{"standard_question":"How do I get a refund?","answers":["Contact support"]}]}'
```

### POST /api/v1/knowledge-bases/:id/faq/entry

Purpose: create one FAQ entry. Request body (`types.FAQEntryPayload`):

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `standard_question` | string | yes (`binding:"required"`) | Standard question |
| `similar_questions` | []string | no | Similar questions |
| `negative_questions` | []string | no | Negative example questions |
| `answers` | []string | no | Answer list |
| `answer_strategy` | string | no | `all` / `random` |
| `tag_id` | int64 | no | Tag seq_id |
| `tag_name` | string | no | Tag name |
| `is_enabled` / `is_recommended` | *bool | no | Enabled / recommended |

Response: 200 `{"success":true,"data":{FAQEntry}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/faq/entry -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"standard_question":"How do I get a refund?","answers":["Refunds are accepted within 7 days"]}'
```

### PUT /api/v1/knowledge-bases/:id/faq/entries/:entry_id

Purpose: update one FAQ entry (same body as create).

In `separate` index mode the per-question index records are keyed by a `SourceID`. A full re-index keys them by position (`<chunk_id>-<position>`), while an incremental update through this route keys them by question hash (`<chunk_id>-<md5 prefix>`), so a position-keyed record is not matched by a later hash-keyed update.

Response: 200 `{"success":true,"data":{FAQEntry}}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/faq/entries/12 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"standard_question":"How do I get a refund?","answers":["Refunds are accepted within 30 days"]}'
```

### POST /api/v1/knowledge-bases/:id/faq/entries/:entry_id/similar-questions

Purpose: append similar questions. Request body: `{"similar_questions":["..."]}` (`binding:"required,min=1"`).

Response: 200 `{"success":true,"data":{FAQEntry}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/faq/entries/12/similar-questions \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"similar_questions":["How does a refund work?"]}'
```

### PUT /api/v1/knowledge-bases/:id/faq/entries/fields

Purpose: batch update entry fields (`is_enabled` / `is_recommended` / `tag_id`).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `by_id` | map[int64]object | no | Update by entry seq_id |
| `by_tag` | map[int64]object | no | Update every entry under a tag seq_id |
| `exclude_ids` | []int64 | no | Entries excluded from a `by_tag` update |

Response: 200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/faq/entries/fields -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"by_id":{"12":{"is_enabled":false}}}'
```

### PUT /api/v1/knowledge-bases/:id/faq/entries/tags

Purpose: batch change entry tags. Request body: `{"updates":{"<entry_id>":<tag_id|null>}}` (`binding:"required,min=1"`; `null` removes the tag).

Response: 200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/faq/entries/tags -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"updates":{"12":3}}'
```

### DELETE /api/v1/knowledge-bases/:id/faq/entries

Purpose: batch delete entries. Request body: `{"ids":[int64]}` (`binding:"required,min=1"`).

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/knowledge-bases/kb-1/faq/entries -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"ids":[12,13]}'
```

### POST /api/v1/knowledge-bases/:id/faq/search

Purpose: FAQ retrieval (read semantics; a scoped key with `retrieve` may call it).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query_text` | string | yes (`binding:"required"`) | Query |
| `vector_threshold` | float64 | no | Vector threshold, default 0.7 |
| `match_count` | int | no | Default 10, capped at 200 |
| `first_priority_tag_ids` / `second_priority_tag_ids` | []int64 | no | Two-level tag priority filter |
| `only_recommended` | bool | no | Recommended entries only |

Response: 200 `{"success":true,"data":[FAQEntry with match_type/score]}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/faq/search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"query_text":"refund"}'
```

### PUT /api/v1/knowledge-bases/:id/faq/import/last-result/display

Purpose: set the display state of the last import result panel. Request body: `{"display_status":"open|close"}` (`binding:"required,oneof=open close"`).

Response: 200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/faq/import/last-result/display \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"display_status":"close"}'
```

### GET /api/v1/faq/import/progress/:task_id

Purpose: progress of an FAQ import or dry run (tasks are isolated per workspace). Permission: Viewer+; API key `retrieve`, `ingest` or full access.

Response: 200 `{"success":true,"data":{FAQImportProgress}}` with `status`, `progress` (0-100), `total`, `processed`, `success_count`, `failed_count`, `partial_failed_count`, `skipped_count`, `failed_entries` (or `failed_entries_url` when there are many) and `success_entries`.

```bash
curl $BASE/api/v1/faq/import/progress/task-1 -H "X-API-Key: $API_KEY"
```

## Wiki (/api/v1/knowledgebase/:kb_id/wiki)

This group's prefix is `/knowledgebase/:kb_id/wiki` (singular, no hyphen). Routes: `RegisterWikiPageRoutes` in `internal/router/routes_knowledge.go`. Handler: `internal/handler/wiki_page.go`. Most responses in this group are **raw objects** (no `success` envelope); errors are `{"error":"..."}`.

Manual page changes made through this API are projected into the knowledge base activity stream with the actions `manual_create`, `manual_edit`, `revert` and `manual_delete`.

### GET /api/v1/knowledgebase/:kb_id/wiki/pages

Purpose: list Wiki pages.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `page_type` | string | no | Comma-separated types |
| `status` | string | no | Page status |
| `query` | string | no | Full-text search |
| `category_path` | string | no | `/`-separated path filter |
| `folder_id` | string | no | Exact folder filter (empty string = root) |
| `category_depth` | int | no | Folder depth |
| `page` / `page_size` | int | no | Pagination (default 1 / 20) |
| `sort_by` / `sort_order` | string | no | Sort field: `title`, `created_at`, `updated_at`, `page_type`, `wiki_path`, `sort_order`, `depth`; any other value sorts by `updated_at`; default `updated_at` desc |

Response: 200 `WikiPageListResponse` (`pages`, `total`, `page`, `page_size`, `total_pages`)

```bash
curl "$BASE/api/v1/knowledgebase/kb-1/wiki/pages?page=1" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledgebase/:kb_id/wiki/pages

Purpose: create a page. Request body (`types.WikiPage`): `slug` (required; the slug is unique per knowledge base, index `idx_wiki_pages_kb_slug`), `title`, `content`, `folder_id`, `page_type`, `status` and the other page fields. An unknown `page_type` or `status` returns 400, a missing slug returns 400 (`wiki page slug is required`), and a `folder_id` that names no folder returns 404.

Response: 201 `WikiPage`

```bash
curl -X POST $BASE/api/v1/knowledgebase/kb-1/wiki/pages -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"slug":"overview","title":"Architecture overview","content":"# Overview"}'
```

### PUT /api/v1/knowledgebase/:kb_id/wiki/move-page

Purpose: move a page into a folder. Request body: `{"slug":"<page slug>","folder_id":"<folder ID, empty = root>"}` (`slug` is required and travels in the body because hierarchical slugs would collide with the catch-all path parameter).

Response: 200 `WikiPage`. Unknown page or folder: 404; conflict: 409.

```bash
curl -X PUT $BASE/api/v1/knowledgebase/kb-1/wiki/move-page -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"slug":"overview","folder_id":"f-1"}'
```

### GET /api/v1/knowledgebase/:kb_id/wiki/pages/*slug

Purpose: fetch a page (`*slug` is a catch-all path).

Response: 200 `WikiPageDetail`. It keeps every `WikiPage` field and adds `in_link_titles`, a map from the slugs in `in_links` that exist to their display titles, for example `{"concept/example":"Example title"}`. With no resolvable backlinks the map is empty; targets that do not exist are left out. If the title lookup fails the route returns 500 rather than an incomplete success response.

```bash
curl $BASE/api/v1/knowledgebase/kb-1/wiki/pages/overview -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledgebase/:kb_id/wiki/pages/*slug

Purpose: update a page (same body as create; absent fields keep their stored value). The previous version is first snapshotted in full into `wiki_page_revisions`, `version` increments, and `last_edit_source` is recorded as `user` (`agent` when an agent tool writes). A concurrent edit returns 409.

Response: 200 `WikiPage`

```bash
curl -X PUT $BASE/api/v1/knowledgebase/kb-1/wiki/pages/overview -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"content":"# Updated overview"}'
```

### GET /api/v1/knowledgebase/:kb_id/wiki/revisions/*slug

Purpose: page version history (migration `000075`). Permission: Viewer+ and KBAccessRead.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `version` | int | no | When given, returns the **full text of that version** (for diffing); invalid or < 1 returns 400, not found returns 404 (versions written before the upgrade and snapshots pruned by the retention policy have no full text, which is expected) |
| `limit` | int | no | Default 50, capped at 200; list mode only |
| `offset` | int | no | Pagination offset |

Without `version` the response is the history list (newest version first, **without bodies**) plus `total` and the page's `current_version`; each item carries `edit_source` (`pipeline` / `agent` / `user` / `revert`), `editor_id` and `edited_at`.

Retention has two caps: the soft cap of 50 versions prunes only `pipeline` and empty-source snapshots, the hard cap of 200 versions applies to every source, so manual edits are not flushed out by the pipeline.

```bash
# History list
curl $BASE/api/v1/knowledgebase/kb-1/wiki/revisions/entity/acme-corp -H "Authorization: Bearer $TOKEN"
# Full text of version 3
curl "$BASE/api/v1/knowledgebase/kb-1/wiki/revisions/entity/acme-corp?version=3" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledgebase/:kb_id/wiki/revert

Purpose: roll a page back to a historical version. Permission: KB owner or Admin+ and KBAccessWrite.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `slug` | string | yes | Target page |
| `version` | int | yes | Target version number (>= 1) |

A rollback **does not move the version number backwards**: the target version's content is written as a new version with `last_edit_source` set to `revert`, so a rollback can itself be rolled back. Reverting to the current version returns 400 (usually a stale history list in the frontend). Unknown page or revision: 404; concurrent edit: 409.

Response: 200 `WikiPage`

```bash
curl -X POST $BASE/api/v1/knowledgebase/kb-1/wiki/revert -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"slug":"entity/acme-corp","version":3}'
```

### DELETE /api/v1/knowledgebase/:kb_id/wiki/pages/*slug

Purpose: delete a page.

Response: 204 No Content

```bash
curl -X DELETE $BASE/api/v1/knowledgebase/kb-1/wiki/pages/overview -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledgebase/:kb_id/wiki/folders

Purpose: list folders. Query parameters: `parent_id` (empty = root), `page_types` (comma-separated).

Response: 200 `WikiFolderListResponse` (`parent_id`, `folders`)

```bash
curl $BASE/api/v1/knowledgebase/kb-1/wiki/folders -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledgebase/:kb_id/wiki/folders

Purpose: create a folder.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes | Folder name; a blank name or one containing a path separator (`/`) returns 400 |
| `parent_id` | string | no | Parent folder |

Response: 201 `WikiFolder`

```bash
curl -X POST $BASE/api/v1/knowledgebase/kb-1/wiki/folders -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Design documents"}'
```

### PUT /api/v1/knowledgebase/:kb_id/wiki/folders/:folder_id

Purpose: rename or move a folder. Request body: `name`, `parent_id`, `move_parent` (bool), all optional. `parent_id` is applied only when `move_parent` is true, so a plain rename cannot accidentally move the folder. Moving a folder into itself or its own descendant returns 400.

Response: 200 `WikiFolder`

```bash
curl -X PUT $BASE/api/v1/knowledgebase/kb-1/wiki/folders/f-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Architecture design"}'
```

### DELETE /api/v1/knowledgebase/:kb_id/wiki/folders/:folder_id

Purpose: delete a folder.

Response: 204 No Content

```bash
curl -X DELETE $BASE/api/v1/knowledgebase/kb-1/wiki/folders/f-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledgebase/:kb_id/wiki/index

Purpose: the Wiki index page (a window per page type). Query parameters: `types` (comma-separated), `limit` (1-200, default 50, per group), `cursor` (opaque offset cursor, applied to every group; a malformed cursor returns 400).

Response: 200 `WikiIndexResponse` (`intro`, `version`, `groups`)

```bash
curl $BASE/api/v1/knowledgebase/kb-1/wiki/index -H "Authorization: Bearer $TOKEN"
```

::: warning Removed
`GET /api/v1/knowledgebase/:kb_id/wiki/log` (the Wiki change log) was retired together with migration `000077_remove_wiki_log`, which dropped the `wiki_log_entries` table. Wiki changes are now projected into the knowledge base activity stream; use `GET /api/v1/knowledge-bases/:id/activity` instead.
:::

### GET /api/v1/knowledgebase/:kb_id/wiki/graph

Purpose: the page relationship graph.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `mode` | string | no | `overview` (default, the top-N most connected pages) / `ego` (the neighborhood of a center page) |
| `center` | string | no | Center slug for ego mode (required when `mode=ego`, otherwise 400; a slug with no page returns 404) |
| `depth` | int | no | 1-3, default 1 (ego mode) |
| `types` | string | no | `page_type` filter |
| `limit` | int | no | Default 500, capped at 2000 |

Response: 200 `WikiGraphData` (`nodes`, `edges`, `meta` with `mode`, `total`, `returned`, `truncated`, `center`)

```bash
curl "$BASE/api/v1/knowledgebase/kb-1/wiki/graph?mode=ego&center=entity/acme-corp" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledgebase/:kb_id/wiki/stats

Purpose: Wiki statistics.

Response: 200 `WikiStats` (`total_pages`, `pages_by_type`, `total_links`, `orphan_count`, `recent_updates`, `pending_tasks`, `pending_issues`, `is_active`). `is_active` is true while at least one ingest batch for the knowledge base holds a live slot in the Redis in-flight set `wiki:inflight:<kb_id>`; Lite mode has no Redis and always reports false.

```bash
curl $BASE/api/v1/knowledgebase/kb-1/wiki/stats -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledgebase/:kb_id/wiki/search

Purpose: page search. Query parameters: `q` (required), `limit` (default 10, capped at 50).

Matching depends on the storage dialect:

- Postgres: `q` is a POSIX regular expression (`~*`, case-insensitive).
- SQLite / Lite: a literal `LIKE` substring match; `%` and `_` are escaped and do not act as wildcards.

Hits are ranked by where the query matched: title, then slug, then summary, then body, with `updated_at` as the tiebreaker.

Response: 200 `{"pages":[WikiPage]}`

```bash
curl "$BASE/api/v1/knowledgebase/kb-1/wiki/search?q=deployment" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/wiki-search

Purpose: Wiki page search across knowledge bases (no session, not hybrid retrieval). Handler: `SearchPagesAcross` in `internal/handler/wiki_page.go`. Same algorithm as the single-knowledge-base `GET .../wiki/search` (title > slug > summary > body), run as one query over several knowledge bases and cut to a global top-N by `match_rank`. API key: `retrieve` or full access.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query` | string | yes (`binding:"required"`) | Same as the single-knowledge-base `q`: a case-insensitive POSIX regex on Postgres, a literal `LIKE` substring on SQLite / Lite |
| `knowledge_base_ids` | []string | conditional | Several knowledge bases; at least one of this and `knowledge_base_id` is required, at most 32 in total |
| `knowledge_base_id` | string | no | Single-knowledge-base compatibility field, merged into the IDs |
| `limit` | int | no | Global top-N, default 10, maximum 50 |

A target knowledge base without the Wiki enabled returns 400. A knowledge base that does not exist or is not readable returns 404. A knowledge base outside the API key allow-list returns 403. An invalid regex (Postgres) returns 400.

Response: 200 `{"success":true,"data":[WikiSearchHit]}`. A hit carries navigation fields plus `match_snippet`, not the body; fetch the full text with `GET /knowledgebase/:kb_id/wiki/pages/{slug}`.

| Response field | Type | Description |
| --- | --- | --- |
| `id` | string | Page ID |
| `knowledge_base_id` | string | Source knowledge base, used to build the page URL across knowledge bases |
| `slug` | string | Page slug |
| `title` | string | Title |
| `page_type` | string | Page type |
| `aliases` | []string | Aliases |
| `summary` | string | Summary |
| `match_snippet` | string | About 60 characters before and after the first body hit; omitted when the body has no hit. The snippet always compiles `query` as a Go regex, so on SQLite a `query` with regex metacharacters can match in the database and still produce an empty snippet |

`content`, the tree, link and metadata fields and the timestamps are not part of search results. The single-knowledge-base `GET .../wiki/search` still returns full `WikiPage` objects.

```bash
curl -X POST $BASE/api/v1/wiki-search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"query":"deploy|release","knowledge_base_ids":["kb-1","kb-2"],"limit":10}'
```

### POST /api/v1/knowledgebase/:kb_id/wiki/rebuild-links

Purpose: rebuild the links between pages. Write permission. No request body.

Response: 200 `{"message":"Links rebuilt successfully"}`

```bash
curl -X POST $BASE/api/v1/knowledgebase/kb-1/wiki/rebuild-links -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledgebase/:kb_id/wiki/lint

Purpose: the Wiki consistency check report.

Response: 200 `WikiLintReport`

```bash
curl $BASE/api/v1/knowledgebase/kb-1/wiki/lint -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/knowledgebase/:kb_id/wiki/auto-fix

Purpose: fix lint issues automatically. Write permission. No request body.

Response: 200 `{"fixed":N,"message":"Auto-fixed N issues"}`

```bash
curl -X POST $BASE/api/v1/knowledgebase/kb-1/wiki/auto-fix -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledgebase/:kb_id/wiki/issues

Purpose: list issues. Query parameters: `slug` (filter by page), `status` (`pending` / `ignored` / `resolved`).

Response: 200 `[WikiPageIssue]`

```bash
curl $BASE/api/v1/knowledgebase/kb-1/wiki/issues -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledgebase/:kb_id/wiki/issues/:issue_id/status

Purpose: update an issue's status. Write permission. Request body: `{"status":"pending|ignored|resolved"}` (`binding:"required"`; any other value returns 400). The `issue_id` must belong to the knowledge base in the path, otherwise 404.

Response: 200 `{"message":"Issue status updated successfully"}`

```bash
curl -X PUT $BASE/api/v1/knowledgebase/kb-1/wiki/issues/i-1/status -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"status":"resolved"}'
```

## Implementation reference

Route registration: `RegisterFAQRoutes` and `RegisterWikiPageRoutes` in `internal/router/routes_knowledge.go`. Handlers: `internal/handler/faq.go`, `internal/handler/wiki_page.go`. FAQ import: `internal/application/service/knowledge_faq_import.go`; Wiki revisions and search: `internal/application/service/wiki_page.go`, `internal/application/repository/wiki_page.go`.
