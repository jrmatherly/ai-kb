# API reference: long-term memory

Manage the caller's long-term memories, tracked topics and document preferences, plus the workspace memory configuration. Paths use the `/api/v1` prefix.

Every personal route requires Viewer+ and an API key must be full-access. The memory scope is derived from the credential; no route takes a `subject_id`. In the examples `$BASE` is the server address and `$TOKEN` the current user's Bearer token. Handler: `internal/handler/memory.go`; router: `internal/router/routes_memory.go`.

## Workspace configuration and request switches

The workspace configuration is read and written through `GET/PUT /tenants/kv/memory-config`, not through the tenant name/description update route. GET requires Viewer+, PUT requires Admin+; an API key needs `manage_tenant_settings` or full access. The response is `{success,data:MemoryConfig}`; PUT takes the configuration object directly and replaces the stored one, so submit the complete object. The personal `PUT /memory/settings` does not replace the workspace switch.

```bash
curl -X PUT "$BASE/api/v1/tenants/kv/memory-config" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"enabled":true,"write_mode":"explicit_only","max_items":200}'
```

`memory_config` fields: `enabled`, `write_mode` (`explicit_only` / `auto`), `extract_model_id`, `max_items`, `extract_delay_seconds`, `extract_min_interval_seconds`, `extract_instructions`, `interest_threshold`, `embedding_model_id`, `vector_recall`, `retrieval_conditioning`. Semantics are in [Long-term memory](../03-features/23-memory.md#workspace-configuration).

PUT validation (each failure is 400): `write_mode` must be `explicit_only` or `auto`; `max_items` 0 to 2000; `extract_delay_seconds` 0 to 3600; `extract_min_interval_seconds` 0 to 86400; `interest_threshold` 0 to 20; `embedding_model_id` at most 64 bytes; `extract_instructions` at most 1000 characters. After validation the object is normalized, on GET as well: 0 means the default (`max_items` 200, `extract_delay_seconds` 90, `extract_min_interval_seconds` 300, `interest_threshold` 3), and an `extract_delay_seconds` below 5 is raised to 5.

`CustomAgentConfig.memory_enabled` inherits the workspace setting when omitted; `false` disables memory for that agent. IM and embed channels use the configuration of the bound agent; the channel structures have no `memory_enabled` field of their own.

## Personal settings

| Method | Path | Request / response |
| --- | --- | --- |
| GET | `/memory/settings` | `{success,data:{workspace_enabled,user_enabled,effective,write_mode,item_count,max_items}}` |
| PUT | `/memory/settings` | `{"enabled":true}`, `enabled` required (400 when missing); returns the updated settings |

`effective` is `workspace_enabled && user_enabled`; a single chat is additionally bound by the agent switch. `item_count` counts active memories.

```bash
curl -X PUT "$BASE/api/v1/memory/settings" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"enabled":true}'
```

## Items

| Method | Path | Request / response |
| --- | --- | --- |
| GET | `/memory/items` | Optional `status`, `limit`, `offset`; `{success,data:[MemoryItem],total}` |
| POST | `/memory/items` | `{kind,content,importance}`; 200 `{success,data:MemoryItem}` |
| PUT | `/memory/items/:id` | `{content,importance}`; 200 `{success,data:MemoryItem}` |
| DELETE | `/memory/items/:id` | 200 `{"success":true}` |
| POST | `/memory/items/:id/confirm` | 200 `{success,data:MemoryItem}`; 409 when the inference is stale or what it replaces has changed |
| POST | `/memory/items/:id/reject` | 200 `{"success":true}` |
| DELETE | `/memory/items` | Clear the current identity; 200 `{success,removed}` |

`status` is one of `active`, `pending`, `superseded`, `archived`; omitted means no filter, any other value is 400. `limit` defaults to 50, valid range 1 to 200, out-of-range values fall back to 50; `offset` defaults to 0, negative values become 0.

`kind` is `profile`, `preference`, `fact`, `task` or `interest`; an unknown kind is stored as `fact`. Content is one short statement, collapsed to one line and cut at 300 characters. `importance` ranks items; on create a value of 0 or below becomes 3, on update it is clamped to 1 to 5. A create or update whose content is almost entirely credentials or identity numbers returns 400; other sensitive fragments are redacted before storage.

MemoryItem fields: `id`, `tenant_id`, `subject_id`, `kind`, `content`, `topic`, `normalized_key`, `importance`, `origin`, `status`, `source_session_id`, `source_message_id`, `valid_from`, `invalid_at`, `expires_at`, `replaces_id` (omitted when empty), `superseded_by`, `last_used_at`, `use_count`, `created_at`, `updated_at`. Pending items are not injected into the prompt. An edited item becomes `manual` origin so background extraction does not overwrite it, and changing its content supersedes any pending inference that was replacing it.

A pending inference that modifies an existing memory leaves the old item in effect until confirmed; confirmation activates the inference and supersedes the old item in one transaction. Confirming an item that is already active is a no-op. An inference that is no longer pending, has expired, or whose target has been modified or deleted returns 409; refresh the list. Reject deletes the item and records a tombstone so the same inference is not proposed again. Delete and clear leave the same tombstones.

```bash
curl -X POST "$BASE/api/v1/memory/items" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"kind":"preference","content":"Give the conclusion first, then the reasoning","importance":3}'

curl "$BASE/api/v1/memory/items?status=pending&limit=50&offset=0" \
  -H "Authorization: Bearer $TOKEN"

curl -X POST "$BASE/api/v1/memory/items/item-1/confirm" \
  -H "Authorization: Bearer $TOKEN"
```

## Topics and document preferences

| Method | Path | Description |
| --- | --- | --- |
| GET | `/memory/topics` | Topics being tracked and not yet promoted; `limit`/`offset` as for items; returns `data` and `total` |
| POST | `/memory/topics/:id/promote` | Promote to a standing interest now; returns `{success,data:MemoryItem}` |
| DELETE | `/memory/topics/:id` | Stop tracking the topic; returns `success` |
| GET | `/memory/documents` | Document preferences; `limit`/`offset` as for items; returns `data` and `total` |
| DELETE | `/memory/documents/:id` | Delete the preference record; returns `success`, the knowledge base document is untouched |

Topic fields: `id`, `topic`, `aliases`, `hits`, `threshold`, `last_seen_at`. Document fields: `id`, `knowledge_id`, `knowledge_base_id`, `title`, `hits`, `last_used_at`. Deletes use the preference record `id`. A deleted topic is remembered so automatic promotion does not bring the same label back.

```bash
curl "$BASE/api/v1/memory/topics" -H "Authorization: Bearer $TOKEN"
curl -X POST "$BASE/api/v1/memory/topics/topic-1/promote" \
  -H "Authorization: Bearer $TOKEN"
curl -X DELETE "$BASE/api/v1/memory/documents/affinity-1" \
  -H "Authorization: Bearer $TOKEN"
```

## Export and consolidate now

`GET /memory/export` returns `{success,total,truncated,data}` with the download filename `ai-kb-memories.json`. It walks every status in pages of 500 up to 20,000 items; check `truncated` when the ceiling is hit.

`POST /memory/consolidate` returns `{success,data:{merged,demoted,expired,reviewed,candidates,skipped?}}`. It merges near-duplicate items and archives expired tasks immediately. When nothing changed, `skipped` says why: `too_few_items`, `no_candidates`, `model_unavailable`, `model_declined`, or `too_soon` (a requested review ran less than one minute ago).

```bash
curl "$BASE/api/v1/memory/export" -H "Authorization: Bearer $TOKEN" \
  -o ai-kb-memories.json
curl -X POST "$BASE/api/v1/memory/consolidate" -H "Authorization: Bearer $TOKEN"
```

Invalid parameters, sensitive content, and writes while memory is off (workspace or personal switch) return 400; an item that does not exist in the caller's own scope returns 404 (an item owned by someone else produces the same 404); a confirm conflict returns 409; a request without a principal returns 401 and an insufficient role or key capability returns 403. No route takes a subject parameter for administrators to read other people's memories.
