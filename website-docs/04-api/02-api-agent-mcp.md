# API reference: agents, MCP and skills

Manages agents, MCP services and their credentials, skills and resource favorites. An agent's tool scope and tool-call approval policy are maintained through this group of endpoints.

## Agents (/api/v1/agents)

Read: Viewer+ (API key `read_agents`, `manage_agents`, `chat` or full). Write: creator OR Admin+ (API key `manage_agents` or full). Built-in agents (`is_builtin=true`) are always Admin+.

### GET /api/v1/agents/placeholders

Purpose: prompt placeholder definitions (registered before `/:id`). Permission: Viewer+.

Response: 200 `{"success":true,"data":{"all":{...},"system_prompt":{...},"agent_system_prompt":{...},"context_template":{...},"rewrite_system_prompt":{...},"rewrite_prompt":{...},"fallback_prompt":{...}}}`

```bash
curl $BASE/api/v1/agents/placeholders -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/agents/type-presets

Purpose: agent type presets for Smart Reasoning (rag-qa / wiki-qa / hybrid / custom and so on). Permission: Viewer+.

Response: 200 `{"success":true,"data":[{id,i18n,config,kb_filter}]}`. `i18n` is narrowed to the caller's locale plus `default`; `config` is the subset of agent config fields the preset fills in (system prompt, allowed tools and the like); `kb_filter` describes knowledge base compatibility.

```bash
curl $BASE/api/v1/agents/type-presets -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/agents

Purpose: create a custom agent. Permission: Contributor+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes (`binding:"required"`) | Name |
| `description` | string | no | Description |
| `avatar` | string | no | Avatar or emoji |
| `config` | object | no | Agent configuration (`types.CustomAgentConfig`, see below) |

Main `config` fields: `agent_mode` (`quick-answer` / `smart-reasoning`), `agent_type` (`rag-qa` / `wiki-qa` / `hybrid-rag-wiki` / `data-analysis` / `custom`), `system_prompt`, `model_id`, `temperature`, `max_iterations`, `allowed_tools`, `mcp_selection_mode` / `mcp_services`, `skills_selection_mode`, `kb_selection_mode` / `knowledge_bases`, `web_search_enabled`, `question_suggestions` and others (full definition in `internal/types/custom_agent.go`). Values are normalized rather than rejected: a negative `temperature` becomes 0.7, `max_iterations` 0 becomes 10 and a negative value means unlimited. The agent error codes 2101 (missing allowed tools), 2102 (invalid max iterations) and 2103 (invalid temperature) are defined but no handler currently raises them.

Response: 201 `{"success":true,"data":{id,name,description,avatar,is_builtin,created_by,config,creator_name,...}}`

```bash
curl -X POST $BASE/api/v1/agents -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Support assistant","config":{"agent_mode":"quick-answer","kb_selection_mode":"selected","knowledge_bases":["kb-1"]}}'
```

### GET /api/v1/agents

Purpose: list agents (including built-in ones). Permission: Viewer+. Query parameter: `creator` (`mine` / `others`, optional; built-in agents are always included).

Response: 200 `{"success":true,"data":[Agent],"disabled_own_agent_ids":[...]}`

```bash
curl $BASE/api/v1/agents -H "X-API-Key: $API_KEY"
```

### GET /api/v1/agents/:id

Purpose: agent details. Permission: Viewer+.

Response: 200 `{"success":true,"data":{Agent}}`

```bash
curl $BASE/api/v1/agents/agent-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/agents/:id

Purpose: update an agent. Permission: creator OR Admin+. Request body: `name` / `description` / `avatar` / `config` (all optional). When the agent is shared to an organization, every knowledge base newly added to its scope must be one the caller could share (the knowledge base creator, or Admin+; switching to `all` needs Admin+), otherwise 403.

Response: 200 `{"success":true,"data":{Agent}}`

```bash
curl -X PUT $BASE/api/v1/agents/agent-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"description":"Updated description"}'
```

### DELETE /api/v1/agents/:id

Purpose: delete an agent. Permission: creator OR Admin+.

Response: 200 `{"success":true,"message":"Agent deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/agents/agent-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/agents/:id/copy

Purpose: copy an agent (the copy is owned by the caller and named after the source with a copy suffix). Permission: Contributor+. No request body.

Response: 201 `{"success":true,"data":{new Agent}}`

```bash
curl -X POST $BASE/api/v1/agents/agent-1/copy -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/agents/:id/suggested-questions

Purpose: starter questions for an agent (registered outside the group to avoid a conflict with `/agents/:id/shares`). Permission: Viewer+; API key `read_agents`, `manage_agents`, `chat` or full.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `knowledge_base_ids` | string | no | Comma-separated knowledge base IDs |
| `knowledge_ids` | string | no | Comma-separated knowledge item IDs |
| `tag_scopes` | string | no | Tag scopes as a JSON array |
| `limit` | int | no | Maximum 30; omitted means the agent's configured starter count (default 6) |

Response: 200 `{"success":true,"data":{"questions":[{question,source,knowledge_base_id}]}}`

```bash
curl "$BASE/api/v1/agents/agent-1/suggested-questions?limit=6" -H "X-API-Key: $API_KEY"
```

## MCP services (/api/v1/mcp-services)

Workspace-level integration of external tool servers. Read: Viewer+; write, test and approval policy: Admin+. API key: `manage_mcp_services` or full. Handler: `internal/handler/mcp_service.go`

When the stored service cannot be read or saved, `GET`, `PUT` and `DELETE /:id`, `/:id/test`, `/:id/tools`, `/:id/resources`, `/:id/metadata/refresh`, `/:id/usage-instructions/generate`, `/:id/credentials` and `/:id/oauth/authorize-url` answer 500 with a generic message; the database cause is logged, not returned. A failure reaching the MCP server itself answers 502 (code 1011) and keeps its cause in the message.

### POST /api/v1/mcp-services

Purpose: create an MCP service. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes | Name |
| `description` | string | no | Legacy description, kept for compatibility; the management UI edits `usage_instructions` instead |
| `usage_instructions` | string | no | What the service is for, when it applies and its key constraints; the model uses it to decide when to use the service. The management UI requires it in step two |
| `enabled` | bool | no | Enabled (defaults to true) |
| `transport_type` | string | yes | `sse` / `http-streamable`; `stdio` is rejected for security reasons |
| `url` | *string | no | Service URL (SSE/HTTP) |
| `headers` | map[string]string | no | HTTP headers |
| `auth_config` | object | no | `auth_type` (`api_key` / `bearer` / `oauth`), `api_key_header`, `custom_headers`, `scopes`, `auth_server_metadata_url` (secrets go through the credentials subresource) |
| `advanced_config` | object | no | `{timeout,retry_count,retry_delay}`, defaults 30 seconds / 3 retries / 1 second; a `timeout` above 60 seconds also extends the window an agent waits for one call to this service's tools |
| `stdio_config` / `env_vars` | object | no | Kept only for legacy data; stdio is disabled and they have no effect |

Response: 200 `{"success":true,"data":{MCPServiceResponse}}` (includes `credentials:{api_key:{configured},token:{configured}}`; a service with a synced tool directory also carries `catalog:{tool_count,stale,synced_at}`). 400 when `transport_type` is `stdio` or the service URL or OAuth metadata URL fails SSRF validation.

```bash
curl -X POST $BASE/api/v1/mcp-services -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"github","transport_type":"sse","url":"https://mcp.example.com/sse"}'
```

### GET /api/v1/mcp-services

Purpose: list MCP services. Permission: Viewer+. Response: 200 `{"success":true,"data":[MCPServiceResponse]}`

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `agent_id` | string | no | Together with `agent_source_tenant_id`, lists the MCP services a shared agent can @-mention |
| `agent_source_tenant_id` | int | no | Source workspace ID of the shared agent, the same parameter chat requests use |

When both parameters are given, the shared-agent path is taken: only the services the agent names in `selected` mode in its source workspace, and which are enabled, are returned (`all` and `none` modes return an empty list). Each item carries only the ID, name, description, usage instructions, transport type, enabled state and tool directory summary, not the URL, headers, auth config or other connection details. A caller without permission to use the agent gets 403. With one or neither parameter, the caller's own workspace services are listed.

```bash
curl $BASE/api/v1/mcp-services -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/mcp-services/:id

Purpose: details. Permission: Viewer+. Response: 200 `{"success":true,"data":{MCPServiceResponse}}`. 404 when the ID is unknown.

```bash
curl $BASE/api/v1/mcp-services/mcp-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/mcp-services/:id

Purpose: partial update (map semantics; `auth_config` must not carry `api_key` or `token`, they are ignored). Permission: Admin+. Fields as for create (all optional).

When `usage_instructions` is sent it must be a non-empty string after trimming, at most 16000 characters. It can be omitted when only the connection or enabled state changes; the stored value is kept.

Response: 200 `{"success":true,"data":{MCPServiceResponse}}`. 400 when the merged transport type is `stdio` or a merged outbound URL (service URL or OAuth metadata URL, including one already stored) fails SSRF validation; 403 for a built-in service; 404 when the ID is unknown.

```bash
curl -X PUT $BASE/api/v1/mcp-services/mcp-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"enabled":false}'
```

### POST /api/v1/mcp-services/:id/usage-instructions/generate

Purpose: generate short usage instructions from the synced, non-stale MCP tool directory. Permission: Admin+; API key needs `manage_mcp_services` or full.

Request: `{"language":"en-US"}`. Recognized values are `zh-CN`, `zh-TW`, `en-US`, `ja-JP`, `ko-KR` and `ru-RU`; an omitted or unrecognized value falls back to English.

The workspace's default active chat model is used when there is one, otherwise the first active chat model. The input is the service name, the server-side description and the names and descriptions of the enabled tools; an OAuth directory uses the current user's authorization. The call does not connect to the MCP server, call tools or save the result.

Response: 200 `{"success":true,"data":{"usage_instructions":"Query remote logs by module and time range; read a log by its query ID when one is known."}}`. The target is 2 to 3 short sentences of at most 500 characters; the user can edit the text and save it with PUT. 400 when the directory is not synced or is stale, has no enabled tools, or no active chat model is configured; 503 when the model is unavailable or the generated text is empty or too long.

### DELETE /api/v1/mcp-services/:id

Purpose: delete. Permission: Admin+. Response: 200 `{"success":true,"message":"MCP service deleted successfully"}`. 403 for a built-in service; 404 when the ID is unknown.

```bash
curl -X DELETE $BASE/api/v1/mcp-services/mcp-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/mcp-services/:id/test

Purpose: connection test (probes the external service). Permission: Admin+. Response: 200 `{"success":true,"data":{"success","message","oauth_required","tools":[...],"resources":[...]}}`. A failed connection still answers 200 with `success:false` in `data`; 404 when the ID is unknown; 500 when the stored service cannot be read.

```bash
curl -X POST $BASE/api/v1/mcp-services/mcp-1/test -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/mcp-services/:id/metadata

Purpose: read the persisted tool directory without connecting upstream. Permission: Viewer+; an OAuth directory is isolated per currently authorized principal.

Response: 200 `{"success":true,"data":null}` means not synced yet; once synced, `data` is the directory snapshot with the server info, instructions, tools and sync time. A snapshot taken before a connection configuration change is marked `stale:true` and cannot be used to load runtime tools.

```bash
curl $BASE/api/v1/mcp-services/mcp-1/metadata -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/mcp-services/:id/metadata/refresh

Purpose: connect upstream explicitly, fetch the full tool list and replace the directory atomically. A static-auth directory needs Admin+; an OAuth user may sync their own directory (Viewer+). An API key needs the MCP management capability.

The response is the updated directory snapshot. A failed refresh keeps the previous snapshot; 409 when the connection changed during the refresh, 400 when the directory is invalid or too large or the upstream sync failed, 503 when metadata storage is unavailable. The refresh does not overwrite hand-written usage instructions or per-tool enable and approval policies.

```bash
curl -X POST $BASE/api/v1/mcp-services/mcp-1/metadata/refresh -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/mcp-services/:id/tools

Purpose: tool list. Permission: Viewer+. Response: 200 `{"success":true,"data":[{name,description,inputSchema,require_approval}]}`. 404 when the ID is unknown.

```bash
curl $BASE/api/v1/mcp-services/mcp-1/tools -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/mcp-services/:id/resources

Purpose: resource list. Permission: Viewer+. Response: 200 `{"success":true,"data":[{uri,name,description,mimeType}]}`. 404 when the ID is unknown.

```bash
curl $BASE/api/v1/mcp-services/mcp-1/resources -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/mcp-services/:id/credentials

Purpose: set secrets (`api_key` / `token`, pointer fields; an omitted field is kept). Permission: Admin+. Handler: `internal/handler/mcp_credentials.go`

Response: 200 `{"success":true,"data":{"fields":{"api_key":{"configured"},"token":{"configured"}}}}`. 403 for a built-in service; 404 when the ID is unknown.

```bash
curl -X PUT $BASE/api/v1/mcp-services/mcp-1/credentials -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"token":"ghp_..."}'
```

### DELETE /api/v1/mcp-services/:id/credentials/:field

Purpose: remove one credential field (`api_key` or `token`). Permission: Admin+. Response: 204. 400 for any other field name; 403 for a built-in service; 404 when the ID is unknown.

```bash
curl -X DELETE $BASE/api/v1/mcp-services/mcp-1/credentials/token -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/mcp-services/:id/tool-approvals

Purpose: list per-tool enable state and human approval policies. Permission: Viewer+. Response: 200 `{"success":true,"data":[{service_id,tool_name,require_approval,enabled,...}]}`

```bash
curl $BASE/api/v1/mcp-services/mcp-1/tool-approvals -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/mcp-services/:id/tool-approvals/:tool_name

Purpose: update a tool's `enabled` state and `require_approval` (human approval) policy. Permission: Admin+. At least one of the two must be sent; an omitted field keeps its value. A tool without a record is enabled and needs no approval.

Response: 200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/mcp-services/mcp-1/tool-approvals/create_issue \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"require_approval":true}'
```

## MCP OAuth

Handler: `internal/handler/mcp_oauth.go`

### GET /api/v1/mcp-oauth/callback

Purpose: third-party OAuth authorization callback (no bearer; the single-use `state` parameter authenticates it; registered outside the `/mcp-services` group). Query parameters: `code`, `state`, `error`.

Response: 302 redirect to the frontend (`#mcp_oauth_result=success` on success, `#mcp_oauth_error=<code>` on failure).

```bash
curl -i "$BASE/api/v1/mcp-oauth/callback?code=xxx&state=yyy"
```

### POST /api/v1/mcp-services/:id/oauth/authorize-url

Purpose: generate a per-user authorization URL. Permission: Viewer+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `redirect_uri` | string | yes | Backend callback URL (absolute) |
| `frontend_redirect` | string | no | Frontend path to open after the callback (default `/`) |

Response: 200 `{"success":true,"data":{"authorization_url","authorization_attempt"}}`. 404 when the ID is unknown.

```bash
curl -X POST $BASE/api/v1/mcp-services/mcp-1/oauth/authorize-url -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"redirect_uri":"'$BASE'/api/v1/mcp-oauth/callback"}'
```

### GET /api/v1/mcp-services/:id/oauth/status

Purpose: the caller's own authorization status. Permission: Viewer+. Query parameter: `authorization_attempt` (optional; when given, only `authorized` and `state` are returned for that attempt).

Response: 200 `{"success":true,"data":{"authorized","state":"authorized|pending","refresh_available","expires_at"}}`

```bash
curl $BASE/api/v1/mcp-services/mcp-1/oauth/status -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/mcp-services/:id/oauth/token

Purpose: revoke the caller's own OAuth token. Permission: Viewer+. Response: 204.

```bash
curl -X DELETE $BASE/api/v1/mcp-services/mcp-1/oauth/token -H "Authorization: Bearer $TOKEN"
```

## MCP server endpoints (/api/v1/mcp-endpoints)

Manages the MCP endpoints the current workspace publishes; external MCP clients connect to `/mcp/:endpoint_id`. Read: Viewer+; write: Admin+. API key: `manage_channels`/full. Handler: `internal/handler/mcp_endpoint.go`. Purpose and tool descriptions are in [MCP integration](../03-features/08-mcp.md#serving-external-clients).

| Method | Path | Description |
| --- | --- | --- |
| GET | `/mcp-endpoints` | Endpoint list (without tokens) |
| GET | `/mcp-endpoints/tools` | Tool catalog: `{groups,tools:[{name,group,destructive}],default_tools}` |
| POST | `/mcp-endpoints` | Create; 201, the response carries the one-time `token` |
| GET | `/mcp-endpoints/:endpoint_id` | Details |
| PUT | `/mcp-endpoints/:endpoint_id` | Partial update, omitted fields keep their values |
| DELETE | `/mcp-endpoints/:endpoint_id` | Delete; clients using the endpoint lose access immediately |
| POST | `/mcp-endpoints/:endpoint_id/rotate-token` | Rotate the token; the response carries the new `token`, the old one stops working immediately |

Request fields (the same for create and update, all optional):

| Field | Type | Description |
| --- | --- | --- |
| `name` | string | Name; required on create |
| `description` | string | Description |
| `enabled` | bool | Default true; a disabled endpoint answers connections with 403 |
| `knowledge_base_ids` | string[] | Accessible knowledge bases; an empty array means every knowledge base in the workspace |
| `tools` | string[] | Exposed tools, at least one; omitted on create means `default_tools` (every non-destructive tool) |
| `default_agent_id` | string | Agent used by `ask`; empty means the built-in Quick Answer agent; internal built-in agents cannot be selected |
| `rate_limit_per_minute` | int | Tool calls per minute; 0 or omitted means 60, maximum 6000 |

The response `data` is `{id,tenant_id,name,description,enabled,token_hint,knowledge_base_ids,tools,default_agent_id,rate_limit_per_minute,path,last_used_at,created_at,updated_at}`, plus `token` on create and rotate. When called with a restricted API key, the endpoint's knowledge bases and the capabilities its tools need must not exceed the key's scope, otherwise 403.

```bash
curl -X POST $BASE/api/v1/mcp-endpoints -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Product docs assistant","knowledge_base_ids":["kb-1"],"tools":["search_knowledge","read_document","ask"]}'
```

## Agent runtime interaction (/api/v1/agent)

Human approval and OAuth resumption inside a conversation. Permission: Viewer+ throughout (only the person who started the session has the context); API keys are denied by default.

### POST /api/v1/agent/tool-approvals/:pending_id

Purpose: decide a pending tool call. Handler: `ResolveToolApproval` in `internal/handler/mcp_service.go`.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `decision` | string | yes (`binding:"required"`) | `approve` / `reject` |
| `modified_args` | JSON | no | Modified tool arguments |
| `reason` | string | no | Reason |

Response: 200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/agent/tool-approvals/p-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"decision":"approve"}'
```

### POST /api/v1/agent/mcp-oauth-resolutions/:pending_id

Purpose: resume an agent run paused on MCP OAuth. Handler: `internal/handler/mcp_oauth.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `service_id` | string | yes (`binding:"required"`) | MCP service ID |
| `decision` | string | no | `authorize` (default) / `cancel` |

Response: 200 `{"success":true}`; 409 when the user has not completed authorization yet.

```bash
curl -X POST $BASE/api/v1/agent/mcp-oauth-resolutions/p-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"service_id":"mcp-1"}'
```

### POST /api/v1/agent/mcp-oauth-resolutions/:pending_id/cancel

Purpose: cancel a paused OAuth flow. No request body.

Response: 200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/agent/mcp-oauth-resolutions/p-1/cancel -H "Authorization: Bearer $TOKEN"
```

## Skills, sandbox and personal variables

`GET /api/v1/skills?sandbox_config_id=...` returns the name and description of each usable skill under that configuration plus `skills_available`; with `agent_id` + `agent_source_tenant_id` it answers for the shared agent's source workspace and that agent's sandbox configuration, see the [sandbox and skills API](02-api-sandbox-skills.md#skills-inside-a-sandbox). Catalog registration, installation, templates, progress, files and personal variables are covered in full by the [sandbox and skills API](02-api-sandbox-skills.md#personal-environment-variables).

The agent config gains `sandbox_config_id`; together with `skills_selection_mode` and `selected_skills` it decides which skills are available. Shell and file tools are registered according to the backend's capabilities; the legacy `read_skill` / `execute_skill_script` tools are no longer registered.

## Long-term memory

When the agent config's `memory_enabled` is nil the workspace setting applies; `false` disables memory reads and writes for this agent. Personal management, topic and document preferences, export and immediate consolidation are in the [long-term memory API](02-api-memory.md); usage steps are in [Long-term memory across sessions](../03-features/23-memory.md).

## User favorites (/api/v1/user/favorites)

Stored per user (not per resource creator). Permission: Viewer+ throughout, JWT only (API keys are denied by default). Handler: `internal/handler/user_resource_favorite.go`

### GET /api/v1/user/favorites

Purpose: list favorites. Query parameter: `type` (required, `kb` or `agent`).

Response: 200 `{"success":true,"data":[{user_id,tenant_id,resource_type,resource_id,created_at}]}`

```bash
curl "$BASE/api/v1/user/favorites?type=kb" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/user/favorites

Purpose: add a favorite. Request body: `{"type":"kb|agent","id":"<resource ID>"}` (both required).

Response: 200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/user/favorites -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"type":"kb","id":"kb-1"}'
```

### DELETE /api/v1/user/favorites/:type/:id

Purpose: remove a favorite. Path parameters: `type`, `id`.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/user/favorites/kb/kb-1 -H "Authorization: Bearer $TOKEN"
```

## Implementation reference

Route registration: called from `internal/router/router.go`; `RegisterCustomAgentRoutes`, `RegisterSkillRoutes` and `RegisterUserFavoriteRoutes` are defined in `routes_agent.go`, `RegisterMCPServiceRoutes` (including MCP OAuth and the `/agent` runtime interaction) in `routes_infra.go`, and `RegisterMCPEndpointRoutes` with the public `/mcp/:endpoint_id` in `routes_mcp_endpoint.go`. Handlers: `internal/handler/custom_agent.go`, `internal/handler/mcp_service.go`, `internal/handler/mcp_credentials.go`, `internal/handler/mcp_oauth.go`, `internal/handler/mcp_endpoint.go`, `internal/handler/skill_handler.go`, `internal/handler/user_resource_favorite.go`.
