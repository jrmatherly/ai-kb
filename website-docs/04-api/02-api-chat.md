# API reference: sessions, messages and chat

Create and manage sessions, read messages and temporary attachments, and receive knowledge or agent answers over SSE.

Sessions are user-private resources: the handler enforces ownership, and the route layer requires Viewer or above. API keys: session and chat routes need the `chat` capability (or full access); message search needs `message_history`; knowledge search needs `retrieve`.

## Sessions (/api/v1/sessions)

### POST /api/v1/sessions

Purpose: create a session. Handler: `internal/handler/session/handler.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `title` | string | No | Title |
| `description` | string | No | Description |
| `project_dir` | string | No | Desktop app only: bind the session to a local project directory the user has approved (an absolute path that matches the approved list exactly); otherwise 400 |

Response: 201 `{"success":true,"data":{Session}}` (`id,title,description,tenant_id,user_id,is_pinned,last_request_state,created_at,...`)

```bash
curl -X POST $BASE/api/v1/sessions -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"title":"New conversation"}'
```

### GET /api/v1/sessions

Purpose: list sessions. Handler: `internal/handler/session/handler.go`

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `page` / `page_size` | int | No | Pagination |
| `keyword` | string | No | Fuzzy match on the title |
| `source` | string | No | Filter by source (web/embed/api/feishu/wechat/slack/...) |
| `agent_id` | string | No | Filter by agent (IM sessions) |

Response: 200 `{"success":true,"data":[SessionListItem],"total","page","page_size"}`

```bash
curl "$BASE/api/v1/sessions?page=1" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/:id

Purpose: session details.

Response: 200 `{"success":true,"data":{Session}}`

```bash
curl $BASE/api/v1/sessions/s-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/sessions/:id

Purpose: update a session (title, description, pinned state). Body: `title`, `description`, `is_pinned` (all optional).

Response: 200 `{"success":true,"data":{Session}}`

```bash
curl -X PUT $BASE/api/v1/sessions/s-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"title":"Renamed"}'
```

### DELETE /api/v1/sessions/:id

Purpose: delete a session.

Response: 200 `{"success":true,"message":"Session deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/sessions/s-1 -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/sessions/batch

Purpose: delete several sessions. Body: `{"ids":["s-1"],"delete_all":false}` (one of `ids` or `delete_all:true`).

Response: 200 `{"success":true,"message":"Sessions deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/sessions/batch -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"ids":["s-1","s-2"]}'
```

### DELETE /api/v1/sessions/:id/messages

Purpose: clear the messages of a session.

Response: 200 `{"success":true,"message":"Session messages cleared successfully"}`

```bash
curl -X DELETE $BASE/api/v1/sessions/s-1/messages -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/sessions/:session_id/generate_title

Purpose: generate a session title from context messages. Handler: `internal/handler/session/title.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `messages` | []Message | Yes (`binding:"required"`) | Messages used as context |

Response: 200 `{"success":true,"data":"generated title"}`

```bash
curl -X POST $BASE/api/v1/sessions/s-1/generate_title -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"messages":[{"role":"user","content":"Introduce the product"}]}'
```

### POST /api/v1/sessions/:session_id/stop

Purpose: stop an answer that is being generated. Handler: `internal/handler/session/stream.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `message_id` | string | Yes (`binding:"required"`) | Assistant message ID |

Response: 200 `{"success":true,"message":"Generation stopped"}`

```bash
curl -X POST $BASE/api/v1/sessions/s-1/stop -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"message_id":"m-1"}'
```

Stopping also discards appended messages that have not yet reached the agent (see [Append messages to a running answer](#steer) below); no new turn starts automatically.

### POST /api/v1/sessions/:session_id/fork

Purpose: fork a new session from a historical message; the source session is unchanged. Only the session owner can fork. Handler: `internal/handler/session/fork.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `message_id` | string | Yes | Fork point. A user message: the history before it is copied and clients usually prefill that question into the input box. An assistant message: the history up to and including that answer is copied |
| `title` | string | No | Title of the new session; defaults to the source title with a "branch" suffix |

Copied messages keep their original timeline and generated files; the new session records `parent_session_id` and `forked_from_message_id`. When the source session is bound to a sandbox, the system snapshots it, and the new session starts from the workspace state of the turn at the fork point the first time it uses a sandbox. When the workspace cannot be carried over the fork still succeeds but returns `degraded: true` with a reason:

| `reason` | Meaning |
| --- | --- |
| `NO_CHECKPOINT` | No turn before the fork point has a workspace checkpoint |
| `SANDBOX_REPLACED` | The checkpoint belongs to an older sandbox the session has since replaced |
| `SANDBOX_GONE` | The source session currently has no sandbox to snapshot |
| `SNAPSHOT_UNSUPPORTED` | The sandbox backend does not support snapshots, or the snapshot failed |

Response: 200 `{"success":true,"data":{"session_id":"new session ID","degraded":false}}`. 409 (`error.details.code` is `FORK_SOURCE_BUSY`) when the source session is generating or the fork point is an unfinished answer; 404 when the session or message does not exist; 400 when the fork point is neither a user nor an assistant message.

```bash
curl -X POST $BASE/api/v1/sessions/s-1/fork -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"message_id":"m-3"}'
```

### POST /api/v1/sessions/:session_id/rewind

Purpose: rewind the current session in place to a message. Only the session owner can rewind. Handler: `internal/handler/session/rewind.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `message_id` | string | Yes | Rewind point. A user message: that message and everything after it are deleted. An assistant message: that answer is kept and everything after it is deleted |

Generated files, follow-up suggestions and chat history index entries of the deleted messages are cleaned up with them. When the session is bound to a sandbox, `/workspace` is reset to the checkpoint of the last kept turn at the same time; if the workspace reset fails the whole operation fails and no message is deleted.

Response: 200 `{"success":true,"data":{"deleted_messages":N,"workspace_reset":true,"reason":""}}`. When `workspace_reset` is false, `reason` says why only the conversation was rewound: `NO_SANDBOX` (the session has no sandbox) or `NO_CHECKPOINT` (the kept history has no checkpoint).

| Status | `error.details.code` | Meaning |
| --- | --- | --- |
| 409 | `REWIND_SOURCE_BUSY` | The session is generating, or the rewind point is an unfinished answer |
| 409 | `REWIND_NO_CHECKPOINT` | The sandbox is still there but the kept history has no usable checkpoint; refused so the files do not run ahead of the conversation |
| 409 | `REWIND_SANDBOX_REPLACED` | The checkpoint belongs to an older, replaced sandbox |
| 404 | - | The session or message does not exist |
| 500 | - | The workspace reset failed; retry |

```bash
curl -X POST $BASE/api/v1/sessions/s-1/rewind -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"message_id":"m-3"}'
```

### Append messages to a running answer (steer) {#steer}

While a Smart Reasoning answer is being generated, calling `agent-chat` again on the same session returns 409. The endpoints below queue a new message into the current turn instead. Only the session owner can call them; Quick Answer has no execution loop to inject into and does not support appending. Handler: `internal/handler/session/steer.go`

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/v1/sessions/:session_id/steer` | Append one message |
| GET | `/api/v1/sessions/:id/steer` | List the queued messages of the current turn that have not been delivered yet, used to restore the queue after a page reload; returns `assistant_message_id` and `items[]` (`steer_id`, `content`, `delivery`, `mentioned_items`); `items` is empty when no turn is running |
| DELETE | `/api/v1/sessions/:id/steer/:steer_id` | Withdraw a queued message |
| POST | `/api/v1/sessions/:session_id/steer/:steer_id/inject` | Change an `after` message to `inject` |

POST body:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query` | string | Yes | Message content, at most 10000 characters |
| `delivery` | string | No | `after` (default): sent as the next turn's question after the current turn ends; `inject`: delivered to the agent at the next iteration boundary of the current turn (including right before the final answer), and the agent adjusts and continues |
| `mentioned_items` | []object | No | @mentions, same format as the chat request |
| `steer_id` | string | No | Client-generated UUID for retry deduplication; the same ID with different content returns 409 |
| `expected_assistant_message_id` | string | No | The running assistant message ID the client sees; 409 when the run has changed, and the client should retry |
| `channel` | string | No | Source channel |

`status` in the response:

| `status` | Meaning |
| --- | --- |
| `queued` | Queued; returns `steer_id`, `delivery` and `assistant_message_id` |
| `new_run` | No turn is running; the client should send through a normal `agent-chat` call instead |
| `already_injected` | The message has already reached the agent (possible on retry, withdrawal or promotion to inject) and cannot be withdrawn |
| `deleted` / `gone` | DELETE only: withdrawn (`removed` says whether a message was actually deleted) / no turn is running |

At most 10 undelivered messages can be queued per turn; more returns 400. When the current turn ends normally, the server starts the next turn with the first undelivered message as its question and moves the rest into that turn with their original delivery mode; that turn has no `agent-chat` connection of its own, so receive it through continue-stream using the `assistant_message_id` returned by `GET /steer`. When the user stops generation, all undelivered messages are discarded. Delivered messages are written to the session history and announced to clients with the `user_message_injected` event. If the run state lookup fails the endpoint returns 503; retry.

```bash
curl -X POST $BASE/api/v1/sessions/s-1/steer -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"query":"Only look at the 2024 data","delivery":"inject"}'
```

### POST /api/v1/sessions/:session_id/pin and DELETE /api/v1/sessions/:id/pin

Purpose: pin / unpin a session. No body. Handler: `internal/handler/session/handler.go`

Response: 200 `{"success":true,"is_pinned":true|false}`

```bash
curl -X POST $BASE/api/v1/sessions/s-1/pin -H "Authorization: Bearer $TOKEN"
curl -X DELETE $BASE/api/v1/sessions/s-1/pin -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/continue-stream/:session_id

Purpose: resume an active stream after a disconnect (replays stored events, then polls for new increments every 100 ms). Handler: `internal/handler/session/stream.go`

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `message_id` | string | Yes | The assistant message ID to resume |

Response: 200 SSE (`text/event-stream`; the event format is in the overview under "Streaming protocol (SSE)"). While stored events are replayed, consecutive unfinished `answer`/`thinking`/`reflection` increments under the same event ID are coalesced into one frame; a client that accumulates content per event ID receives the same text. The live part is not coalesced.

```bash
curl -N "$BASE/api/v1/sessions/continue-stream/s-1?message_id=m-1" -H "Authorization: Bearer $TOKEN"
```

## Sandbox graphical desktop {#sandbox-desktop}

These routes are registered in `internal/router/routes_chat.go` and are not in Swagger yet. Only Cube/E2B desktop templates support them; deployment and proxy requirements are in [Sandbox deployment](../06-development/04-sandbox-deployment.md).

### POST /api/v1/sessions/:session_id/sandbox/desktop-ticket

Issues a one-shot WebSocket ticket valid for two minutes. Requires a valid login Bearer access token of the session owner; an API key alone is not enough. The JWT travels only in the Authorization header of this POST.

```bash
curl -X POST "$BASE/api/v1/sessions/$SESSION_ID/sandbox/desktop-ticket" \
  -H "Authorization: Bearer $TOKEN"
```

Response: 200 `{"success":true,"data":{"ticket":"<opaque-ticket>","expires_in":120}}`.

### GET /api/v1/sessions/:id/sandbox/desktop

The WebSocket handshake uses `?ticket=<opaque-ticket>` and bypasses the normal JWT middleware. The ticket is bound to the user, workspace, session and the original access token, and is invalidated on first use; used, expired and unknown tickets are all rejected. Proxy logs must not record the ticket query.

Only one relay per session is allowed at a time. A sandbox that is not bound, paused, without desktop support, or failing to start may complete the WebSocket upgrade first and then disconnect with a close reason such as `SANDBOX_NOT_BOUND`, `SANDBOX_PAUSED`, `DESKTOP_UNSUPPORTED` or `DESKTOP_START_FAILED`; clients should read the close reason.

### POST /api/v1/sessions/:session_id/sandbox/desktop/activity

The session owner reports keyboard and mouse activity; response 200 `{"success":true}`. It extends the lifetime only when the server-side RFB parser has degraded; while the parser is healthy it is ignored, and polling must not be used to keep a sandbox alive.

## Session attachments (temporary documents)

Handler: `internal/handler/session/temporary_document.go`

### POST /api/v1/sessions/:session_id/attachments

Purpose: upload a session-scoped temporary document (parsed asynchronously). Multipart fields: `file` (required), `agent_id` (optional; selects the parsing engine and ASR model), `parser_engine` (optional; ignored with a shared agent, whose parsing rules decide).

Response: 202 `{"success":true,"data":{TemporaryDocument}}` (`id,session_id,file_name,file_type,file_size,status(uploaded/processing/ready/failed),resource_ref,...`)

```bash
curl -X POST $BASE/api/v1/sessions/s-1/attachments -H "Authorization: Bearer $TOKEN" -F 'file=@notes.pdf'
```

### GET /api/v1/sessions/:id/attachments

Purpose: list attachments.

Response: 200 `{"success":true,"data":[TemporaryDocument]}`

```bash
curl $BASE/api/v1/sessions/s-1/attachments -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/:id/attachments/:attachment_id

Purpose: attachment details, including the parsing status.

Response: 200 `{"success":true,"data":{TemporaryDocument}}`

```bash
curl $BASE/api/v1/sessions/s-1/attachments/a-1 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/sessions/:id/attachments/:attachment_id/preview

Purpose: preview the original attachment file.

Response: 200 file stream (`Content-Disposition: inline|attachment`, `Cache-Control: private`).

```bash
curl $BASE/api/v1/sessions/s-1/attachments/a-1/preview -H "Authorization: Bearer $TOKEN" -o preview.pdf
```

### DELETE /api/v1/sessions/:id/attachments/:attachment_id

Purpose: delete an attachment.

Response: 204 No Content

```bash
curl -X DELETE $BASE/api/v1/sessions/s-1/attachments/a-1 -H "Authorization: Bearer $TOKEN"
```

## Answer suggestions

Handler: `internal/handler/message_suggestion.go`

### GET /api/v1/sessions/:id/messages/:message_id/suggestions

Purpose: read the follow-up suggestions of an assistant message.

Response: 200 `{"success":true,"data":{MessageSuggestionSet}}` (`status(generating/ready/suppressed/failed),questions:[{id,text,category,source,knowledge_base_ids}],allow_regenerate,...`)

```bash
curl $BASE/api/v1/sessions/s-1/messages/m-1/suggestions -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/sessions/:session_id/messages/:message_id/suggestions

Purpose: make sure suggestions are generated (idempotent trigger). Body: `{"regenerate":true}` (optional; forces regeneration).

Response: 200 (ready) or 202 (generating) `{"success":true,"data":{MessageSuggestionSet|null}}`

```bash
curl -X POST $BASE/api/v1/sessions/s-1/messages/m-1/suggestions -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{}'
```

### POST /api/v1/sessions/:session_id/suggestion-events

Purpose: report suggestion interaction events (analytics).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `suggestion_set_id` | string | Yes (`binding:"required"`) | Suggestion set ID |
| `question_id` | string | No | Required for click/regenerate |
| `event_type` | string | Yes (`binding:"required"`) | `impression/click/dismiss/regenerate` |

Response: 204 No Content

```bash
curl -X POST $BASE/api/v1/sessions/s-1/suggestion-events -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"suggestion_set_id":"ss-1","event_type":"impression"}'
```

## Chat and retrieval

Handler: `internal/handler/session/qa.go`. API keys: chat needs `chat` or full access; `knowledge-search` needs `retrieve` or full access.

### POST /api/v1/knowledge-chat/:session_id

Purpose: knowledge base question answering (SSE stream).

Body (shared by KnowledgeQA and AgentQA):

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query` | string | Yes | The user's question; 400 when empty or whitespace only (unless an image or attachment is included), or when it contains control characters or invalid UTF-8 |
| `knowledge_base_ids` | []string | No | Knowledge bases to retrieve from |
| `knowledge_ids` | []string | No | Restrict to specific knowledge items |
| `agent_enabled` | bool | No | Enable agent mode |
| `agent_id` | string | No | Custom agent ID |
| `agent_source_tenant_id` | uint64 | No | Source workspace of a shared agent; disambiguates shared agents with the same name from several workspaces |
| `reasoning_effort` | string | No | Reasoning effort for this turn: `off`, `auto`, `minimal`, `low`, `medium`, `high`, `xhigh`, `max` (`none`/`false` count as `off`, `true`/`on` as `auto`); omitted inherits the agent configuration. Applies to this turn only and does not modify the agent; when the model does not support the chosen level it is adjusted to the nearest one. An invalid value returns 400 |
| `web_search_enabled` | bool | No | Web search; takes effect only when the agent itself has web search enabled |
| `local_browser_enabled` | bool | No | Allow the connected Local Browser for this turn; only valid on `agent-chat` with a Smart Reasoning agent, otherwise 400. See [Local Browser](../05-clients/09-local-browser.md) |
| `summary_model_id` | string | No | Summary model; ignored with a shared agent, which always uses its configured model |
| `mcp_service_ids` | []string | No | @mentioned MCP services |
| `skill_names` | []string | No | @mentioned skills |
| `tag_ids` | []string | No | Tag filter |
| `mentioned_items` | []object | No | @mentions (type/kb_id/kb_name/service_id/skill_name) |
| `disable_title` | bool | No | Disable automatic titling |
| `images` | []object | No | Images (`data` base64 / `url` / `caption`) |
| `attachment_uploads` | []object | No | Inline attachments (`data` base64, `file_name`, `file_size`) |
| `attachment_ids` | []string | No | IDs of already uploaded session attachments |
| `channel` | string | No | Source channel |
| `suggestion_attribution` | object | No | Attribution of a clicked suggestion |
| `question_origin` | object | No | Where the suggested question the user picked came from: `knowledge_base_id` and optional `knowledge_id`. Smart Reasoning only; the agent retrieves from that source first. Ignored when the source is outside this turn's retrieval scope, which is never widened |

The chosen `reasoning_effort` is written to the session's `last_request_state.reasoning_effort`, and the frontend restores it when the session is reopened.

Response: 200 SSE stream, `event: message` + `data: StreamResponse` (see the overview), ending with a `complete` event. When the answer is cut off by the model's per-call output limit, the `answer` event carries `data.truncated: true` and the content generated before the cut is kept.

```bash
curl -N -X POST $BASE/api/v1/knowledge-chat/s-1 -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"query":"What is the refund policy?","knowledge_base_ids":["kb-1"]}'
```

### POST /api/v1/agent-chat/:session_id

Purpose: agent question answering (SSE stream, including `thinking/tool_call/tool_result/tool_approval_required/mcp_oauth_required` and other events). Same body as above.

- A failed tool call is returned as a `tool_result` event (`data.success: false`, `data.error` holds the reason) and the agent carries on; the `error` event means the whole turn failed.
- When the agent accepts an appended message mid-run it emits `user_message_injected`; while a command runs, `command_output` updates the tool card's output.
- 409 when a Smart Reasoning answer is already generating in the same session; use the [steer endpoints](#steer) to append a message instead.

```bash
curl -N -X POST $BASE/api/v1/agent-chat/s-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"query":"Analyze last quarter's data","agent_id":"agent-1"}'
```

### POST /api/v1/knowledge-search

Purpose: knowledge retrieval without a session (not streamed), the preferred endpoint for external systems that want retrieval results. It runs the same retrieval flow as in-product question answering (recall → rerank → merge → truncate), so the ranking matches the chat page. How to choose between it and `hybrid-search`: [Choosing a retrieval endpoint](./01-api-overview.md#retrieval-api). Handler: `SearchKnowledge` in `internal/handler/session/qa.go`.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query` | string | Yes (`binding:"required"`) | Query |
| `knowledge_base_id` | string | No | Single knowledge base (backward compatible) |
| `knowledge_base_ids` | []string | No | Several knowledge bases |
| `knowledge_ids` | []string | No | Specific items, unioned with `knowledge_base_id(s)` (see below) |
| `tag_ids` | []string | No | Tag filter |
| `mentioned_items` | []object | No | Tag mentions scoped to a knowledge base |
| `vector_threshold` / `keyword_threshold` | float | No | Recall thresholds; omitted uses the workspace retrieval config (defaults 0.15 / 0.3) |
| `match_count` | int | No | Number of results, at most 200; omitted uses the workspace `rerank_top_k` (default 10). The recall depth is raised to at least this value |
| `disable_keywords_match` / `disable_vector_match` | bool | No | Turn off one recall path; both `true` returns 400 |
| `rerank` | object | No | Overrides the rerank settings; `{"enabled":false}` turns rerank off. Fields: [the rerank object](./01-api-overview.md#retrieval-api). `rerank.top_k`, when given, takes precedence over `match_count` |

Omitted fields follow the workspace retrieval config (`GET /tenants/kv/retrieval-config`); a request without any of the newer fields behaves as before.

The retrieval scope is the union of `knowledge_base_id(s)` and `knowledge_ids`, with the same semantics as @mentions in the product: each knowledge base is searched as a whole, items in `knowledge_ids` that belong to those knowledge bases are already covered and add no restriction, and items from other knowledge bases are added to the search. **To search only within specific items, pass only `knowledge_ids` and not their knowledge bases**, otherwise the whole knowledge base is searched.

Response: 200 `{"success":true,"data":[SearchResult],"meta":{"rerank":{...}}}`. `SearchResult` has `id,content,knowledge_id,knowledge_title,score,chunk_type,knowledge_base_id,...`; reranked results carry `model_score` and `base_score` in `metadata`. When `data` is empty, `meta.rerank.outcome` says why; see [meta.rerank diagnostics](./01-api-overview.md#retrieval-api).

```bash
curl -X POST $BASE/api/v1/knowledge-search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"query":"deployment requirements","knowledge_base_ids":["kb-1"]}'

# Vector recall only, 5 results, rerank off
curl -X POST $BASE/api/v1/knowledge-search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"query":"deployment requirements","knowledge_base_ids":["kb-1"],"disable_keywords_match":true,"match_count":5,"rerank":{"enabled":false}}'

# Search only within specific items (no knowledge_base_ids)
curl -X POST $BASE/api/v1/knowledge-search -H "X-API-Key: $API_KEY" \
  -H 'Content-Type: application/json' -d '{"query":"deployment requirements","knowledge_ids":["k-1","k-2"]}'
```

## Messages (/api/v1/messages)

Handler: `internal/handler/message.go`

### POST /api/v1/messages/search

Purpose: search the chat history. Permission: Viewer or above; API key `message_history` or full access.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query` | string | Yes (`binding:"required"`) | Query |
| `mode` | string | No | `keyword/vector/hybrid` (default hybrid) |
| `limit` | int | No | Default 20 |
| `session_ids` | []string | No | Restrict to sessions |

Response: 200 `{"success":true,"data":{"total":N,"results":[{session_id,message_id,role,content,created_at,score}]}}`

```bash
curl -X POST $BASE/api/v1/messages/search -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"query":"quote"}'
```

### GET /api/v1/messages/chat-history-stats

Purpose: chat history index statistics. Permission: Viewer or above; API key `message_history` or full access.

Response: 200 `{"success":true,"data":{indexed_message_count,knowledge_base_size,last_indexed_at,...}}`

```bash
curl $BASE/api/v1/messages/chat-history-stats -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/messages/:session_id/load

Purpose: load session messages (paging backwards with a time cursor). Permission: Viewer or above; API key `chat` or full access.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `limit` | int | No | Default 20 |
| `before_time` | string | No | RFC3339/RFC3339Nano timestamp |

Response: 200 `{"success":true,"data":[Message]}` (`id,session_id,role,content,is_completed,images,attachments,agent_steps,...`)

```bash
curl "$BASE/api/v1/messages/s-1/load?limit=20" -H "X-API-Key: $API_KEY"
```

### DELETE /api/v1/messages/:session_id/:id

Purpose: delete one message. Permission: Viewer or above (the handler checks session ownership); API key `chat` or full access.

Response: 200 `{"success":true,"message":"Message deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/messages/s-1/m-1 -H "Authorization: Bearer $TOKEN"
```

## Session generated files

The endpoints below require Viewer or above, an API key with `chat` or full access, and pass the session ownership check. A session that does not exist or is not accessible returns 404.

| Method | Path | Response |
| --- | --- | --- |
| GET | `/api/v1/sessions/:id/artifacts` | 200 `{success:true,data:[Artifact]}`, all files of the session |
| GET | `/api/v1/sessions/:id/messages/:message_id/artifacts` | As above, files of this message only |
| GET | `/api/v1/sessions/:id/messages/:message_id/artifacts/:index/download` | 200 file stream, Content-Disposition: attachment |
| DELETE | `/api/v1/sessions/:id/messages/:message_id/artifacts/:index` | 200 `{success:true,data:{file_name,deleted}}`, deletes the file |

Artifact fields: index, handle (optional resource:// reference), file_name, file_type, file_size, source_path, mod_time, created_at. The response never returns the underlying object storage URL. The download index starts at 0 and must be the index from the message's own list; a session-wide index cannot be used to build a message download URL. An invalid index returns 400; an out-of-range index or a missing file returns 404.

```bash
curl "$BASE/api/v1/sessions/session-1/messages/message-1/artifacts" \
  -H "Authorization: Bearer $TOKEN"
curl "$BASE/api/v1/sessions/session-1/messages/message-1/artifacts/0/download" \
  -H "Authorization: Bearer $TOKEN" -o result.pdf
```

### Deleting generated files

Deleting reclaims the bytes in object storage and **cannot be undone**. Unlike download, delete is open only to the session owner: read-only access through a shared agent can download files but not delete them. A session that is not yours is always treated as 404 (no distinction between "missing" and "forbidden", consistent with the other session endpoints). A deleted file returns 404, and so does deleting it again.

The bytes are reclaimed only when nothing else holds them: the same file saved into a knowledge base, re-referenced by a later answer, or present in a forked copy of the session keeps them. A failed reclaim does not change the result (the endpoint still returns 200) and the file is gone from every list.

After deletion the file no longer appears in the list endpoints or the artifact library, but its **position in the message is kept**: `index` is the download address, and if later files moved up, existing download links would point at the wrong file. For the same reason a file with the same name in the sandbox is not collected again on the next turn; its mtime did not change because the user deleted it.

| Parameter | Description |
| --- | --- |
| `all_versions` | Also delete every earlier version with the same `source_path` in this session. Boolean, default `false` |

```bash
curl -X DELETE "$BASE/api/v1/sessions/session-1/messages/message-1/artifacts/0" \
  -H "Authorization: Bearer $TOKEN"
```

### Cross-session artifact library

`GET /api/v1/artifacts` lists every generated file in the current user's web conversations, for the Artifacts page in the home sidebar. The scope matches the session list with `source=web`: the caller's own sessions plus historical workspace-level web sessions without an owner; IM channels, the web widget (embed) and API key sessions are never included, even when an IM session has no owner in the database. Files with the same `source_path` in one session count as versions of one file; only the latest version is returned and `version_count` gives the number of versions. Files of deleted sessions or messages are not returned. Permissions are the same as in the table above.

| Parameter | Description |
| --- | --- |
| `keyword` | Filter by file name, case-insensitive |
| `file_types` | Comma-separated extensions such as `.pdf,.pptx` (the dot is optional) |
| `page` / `page_size` | Pagination; `page_size` at most 100, default 20 |

Response `{success, data:[LibraryArtifact], total, page, page_size}`, newest first. LibraryArtifact fields: session_id, session_title, message_id, index, handle (optional), file_name, file_type, file_size, source_path, created_at, version_count. To download, call the download endpoint in the table above with its session_id, message_id and index.

```bash
curl "$BASE/api/v1/artifacts?file_types=.pptx,.pdf&keyword=report&page=1&page_size=30" \
  -H "Authorization: Bearer $TOKEN"
```

`DELETE /api/v1/artifacts` deletes one file from the artifact library, addressed by the query parameters `session_id`, `message_id` and `index`, with the same semantics as the per-session delete above. The only difference is that `all_versions` defaults to `true` when omitted (an explicit value is parsed the same way by both endpoints): a library row stands for one file (`version_count` gives the number of versions) rather than one generation, so deleting only the latest version would leave the row in the list showing the previous version. Pass `all_versions=false` to delete only the current version.

```bash
curl -X DELETE "$BASE/api/v1/artifacts?session_id=session-1&message_id=message-1&index=0" \
  -H "Authorization: Bearer $TOKEN"
```

### Images and file references in answers

`GET /api/v1/sessions/:id/messages/:message_id/files?file_path=...` is a message-level authorizing proxy. `file_path` is a resource handle referenced by that message or a supported storage reference, URL-encoded by the client. The backend checks access to the message, that the resource is bound to the message, and the current access to the knowledge base or shared agent; an arbitrary file path cannot be read with just a session ID. Once authorization is revoked, references in old messages are rejected too. It serves answer images and message artifacts from shared agents and organization-shared libraries; details in [File access](../03-features/21-file-access.md).

### Per-turn usage

Messages return the persisted usage, and the agent completion event carries `turn_usage`, the aggregate of this turn's model calls by purpose. Not every tool call produces tokens; usage follows what the provider returned or what the backend has collected. Fields: [Observability](../03-features/16-observability.md).

## Implementation reference

Route registration: `RegisterSessionRoutes`, `RegisterChatRoutes` and `RegisterMessageRoutes` in `internal/router/routes_chat.go` (called from `internal/router/router.go`). Handlers: `internal/handler/session/` (handler.go, qa.go, stream.go, title.go, temporary_document.go, fork.go, rewind.go, steer.go, artifact_*.go), `internal/handler/message.go`, `internal/handler/message_suggestion.go`.
