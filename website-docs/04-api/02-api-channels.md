# API reference: IM, embed and file serving

Manage IM and web embed channels, and serve channel callbacks, visitor sessions and file access. The management side, IM platform callbacks and embed visitors each authenticate differently.

## IM callbacks (no global authentication)

### GET|POST /api/v1/im/callback/:channel_id

Purpose: event callbacks and URL verification from IM platforms (WeCom, Feishu, Slack, Telegram, Mattermost, Yunzhijia and others). Registered before the authentication middleware; each platform's own signature check applies. Unknown channel 404, disabled channel 503, platforms without HTTP callbacks (`qqbot`, `wechat`, `dingtalk`) or a channel not in `webhook` mode 403, signature failure 403, unparseable payload 400. A message is acknowledged immediately and processed asynchronously. Handler: `internal/handler/im.go`

Response: 200 `{"success":true}` (Yunzhijia additionally receives `"data":{"type":2,"content":""}`), or the ACK format the platform requires.

```bash
curl -X POST $BASE/api/v1/im/callback/ch-1 -H 'Content-Type: application/json' -d '{"event":"..."}'
```

## IM channel management (authenticated)

API key: `manage_channels` or full access. IM channels carry external bot credentials: listing is Viewer+, changes, toggles and QR code login are Admin+.

For Feishu/Lark, `credentials.api_base_url` affects both the HTTP API and the WebSocket bootstrap; Yunzhijia supports `session_mode=thread`. Configuration examples and network requirements are in [IM integration](../03-features/12-im-integration.md). The memory preference for IM and embed conversations comes from the bound agent's `config.memory_enabled`; the channel endpoints have no `memory_enabled` parameter of their own.

### POST /api/v1/agents/:id/im-channels

Purpose: create an IM channel for an agent. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `platform` | string | yes | `wecom/feishu/lark/slack/telegram/dingtalk/mattermost/wechat/qqbot/yunzhijia`; any other value 400 |
| `name` | string | no | Display name |
| `mode` | string | no | `websocket` (default; `mattermost` and `yunzhijia` default to `webhook`) / `webhook` / `longpoll` (`wechat` is forced to `longpoll`) |
| `output_mode` | string | no | `stream` (default) / `full` (`wechat` is forced to `full`) |
| `locale` | string | no | Reply language: `en-US`; empty (default) uses the deployment default language (`AIKB_LANGUAGE`, `en-US` when unset); any other value 400 |
| `session_mode` | string | no | `user` (default) / `thread` |
| `knowledge_base_id` | string | no | Knowledge base that also receives attachments; must belong to this workspace, otherwise 400 |
| `credentials` | object | no | Platform credentials |
| `enabled` | bool | no | Default true |

Response: 200 `{"data":{IMChannel}}`; 409 when a bot with the same credentials already exists.

```bash
curl -X POST $BASE/api/v1/agents/agent-1/im-channels -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"platform":"feishu","name":"Feishu support","locale":"en-US"}'
```

### GET /api/v1/agents/:id/im-channels

Purpose: the IM channels of one agent (summaries). Permission: Viewer+.

Response: 200 `{"data":[IMChannel]}`

```bash
curl $BASE/api/v1/agents/agent-1/im-channels -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/im-channels

Purpose: overview of every IM channel in the workspace (no credentials). Permission: Viewer+.

Response: 200 `{"data":[IMChannel]}`

```bash
curl $BASE/api/v1/im-channels -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/im-channels/:id

Purpose: update a channel (partial update: `name/mode/output_mode/locale/session_mode/knowledge_base_id/credentials/enabled/agent_id` are all optional; an empty `knowledge_base_id` unlinks the knowledge base, an empty `locale` restores the default language). Permission: Admin+.

Response: 200 `{"data":{IMChannel}}`; 409 on a duplicate bot.

```bash
curl -X PUT $BASE/api/v1/im-channels/ch-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"enabled":false}'
```

### DELETE /api/v1/im-channels/:id

Purpose: delete a channel. Permission: Admin+.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/im-channels/ch-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/im-channels/:id/toggle

Purpose: enable or disable. Permission: Admin+. No request body.

Response: 200 `{"data":{IMChannel}}`

```bash
curl -X POST $BASE/api/v1/im-channels/ch-1/toggle -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/wechat/qrcode

Purpose: generate a WeChat login QR code (binds a personal WeChat account to the workspace). Permission: Admin+. No request body. Handler: `internal/handler/wechat_qrcode.go`

Response: 200 `{"data":{"qrcode_url","qrcode"}}`

```bash
curl -X POST $BASE/api/v1/wechat/qrcode -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/wechat/qrcode/status

Purpose: poll the scan status; credentials are returned once confirmed. Permission: Admin+. Request body: `{"qrcode":"<identifier>"}` (required).

Response: 200 `{"data":{"status":"pending|scanned|confirmed|expired","credentials":{bot_token,ilink_bot_id,ilink_user_id,baseurl}}}` (`credentials` only when `confirmed`; `baseurl` only when the server returned one)

```bash
curl -X POST $BASE/api/v1/wechat/qrcode/status -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"qrcode":"qr-1"}'
```

## Embed channel management (authenticated)

API key: `manage_channels` or full access. Handler: `internal/handler/embed_channel.go`

### POST /api/v1/agents/:id/embed-channels

Purpose: create a web embed channel for an agent. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | no | Name |
| `enabled` | bool | no | Default true |
| `allowed_origins` | []string | yes | At least one origin (exact URL or `*.domain`; `*` is rejected in production) |
| `welcome_message` | string | no | Welcome message |
| `rate_limit_per_minute` | int | no | Per IP per minute, default 30 |
| `rate_limit_per_day` | int | no | Per channel per day, default 10000 |
| `primary_color` / `page_title` / `widget_position` | string | no | Appearance (`widget_position`: `bottom-right` default, `bottom-left`, `top-right`, `top-left`) |
| `header_title_mode` | string | no | `channel` (default) / `session` |
| `show_suggested_questions` | bool | no | Default true |
| `allow_web_search` / `allow_file_upload` | bool | no | Default false |
| `default_locale` | string | no | `en-US` or empty (follow the browser) |

`webhook_url`, `webhook_secret` and `agent_id` are accepted only by the update endpoint below; the agent comes from the path on create.

Response: 201 `{"success":true,"data":{embedChannelResponse with publish_token}}`

```bash
curl -X POST $BASE/api/v1/agents/agent-1/embed-channels -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Website support","allowed_origins":["https://example.com"]}'
```

### GET /api/v1/agents/:id/embed-channels

Purpose: the embed channels of one agent. Permission: Viewer+.

Response: 200 `{"success":true,"data":[embedChannelResponse]}`

```bash
curl $BASE/api/v1/agents/agent-1/embed-channels -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/embed-channels

Purpose: every embed channel in the workspace (no publish token). Permission: Viewer+.

Response: 200 `{"success":true,"data":[embedChannelResponse]}`

```bash
curl $BASE/api/v1/embed-channels -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/embed-channels/:channel_id

Purpose: channel detail (with the publish token, for copying the deployment snippet). Permission: Viewer+.

Response: 200 `{"success":true,"data":{embedChannelResponse}}`

```bash
curl $BASE/api/v1/embed-channels/ec-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/embed-channels/:channel_id

Purpose: update a channel (the create fields plus `webhook_url`, `webhook_secret` and `agent_id`, all optional). `allowed_origins` is validated only when present; `webhook_url` must be an `http` or `https` URL that passes the SSRF check, otherwise 400. Permission: Admin+.

Response: 200 `{"success":true,"data":{embedChannelResponse}}` (no publish token)

```bash
curl -X PUT $BASE/api/v1/embed-channels/ec-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"enabled":false}'
```

### DELETE /api/v1/embed-channels/:channel_id

Purpose: delete a channel. Permission: Admin+.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/embed-channels/ec-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/embed-channels/:channel_id/rotate-token

Purpose: rotate the publish token (the old token stops working). Permission: Admin+. No request body.

Response: 200 `{"success":true,"data":{embedChannelResponse with the new publish_token}}`

```bash
curl -X POST $BASE/api/v1/embed-channels/ec-1/rotate-token -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/embed-channels/:channel_id/preview-session

Purpose: issue a short-lived session token for the management preview (no publish token needed). Permission: Viewer+.

Response: 200 `{"success":true,"data":{"session_token","expires_in"}}`; disabled channel 403.

```bash
curl -X POST $BASE/api/v1/embed-channels/ec-1/preview-session -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/embed-channels/:channel_id/stats

Purpose: channel usage statistics. Permission: Viewer+.

Response: 200 `{"success":true,"data":{"session_count":N}}`

```bash
curl $BASE/api/v1/embed-channels/ec-1/stats -H "Authorization: Bearer $TOKEN"
```

## Embed page framing policy

### GET /api/v1/embed-frame-policy

Purpose: lets an Nginx `auth_request` subrequest fetch the channel's `Content-Security-Policy: frame-ancestors` before it serves the `/embed/:channel_id` page. No token; it returns only the policy header, never the channel configuration; response carries `Cache-Control: no-store`.

| Header | Required | Description |
| --- | --- | --- |
| `X-Embed-Page-URI` | yes | Relative URI of the embed page, such as `/embed/<channel_id>` |

Response: 204 with the CSP header; 403 when the channel does not exist or is disabled, the path is invalid, or the allowlist is empty.

```bash
curl -i $BASE/api/v1/embed-frame-policy -H "X-Embed-Page-URI: /embed/ec-1"
```

## Embed public routes (/api/v1/embed/:channel_id, EmbedAuth)

Authentication: `Authorization: Embed <publish_token|session_token>`; session-level operations also send `X-Embed-Session: <sig>`. Missing token 401, origin not allowed 403, rate limits 429 (per IP per minute, per channel per minute, per channel per day). Rate limiting and origin checks are described in the overview. Handler: `internal/handler/embed_channel.go`; middleware: `internal/middleware/embed_auth.go`

Below, `$ET` stands for the embed token header: `-H "Authorization: Embed $EMBED_TOKEN"`.

### POST /api/v1/embed/:channel_id/exchange

Purpose: exchange the publish token for a short-lived session token. A session token cannot be exchanged again (403). No request body. 503 when the session token store (Redis) is not configured.

Response: 200 `{"success":true,"data":{"session_token","expires_in"}}`

```bash
curl -X POST $BASE/api/v1/embed/ec-1/exchange -H "Authorization: Embed $PUBLISH_TOKEN"
```

### GET /api/v1/embed/:channel_id/config

Purpose: the channel's public configuration (no secrets).

Response: 200 `{"success":true,"data":{channel_id,name,display_title,knowledge_base_ids,agent_id,agent_name,agent_avatar,welcome_message,primary_color,page_title,header_title_mode,show_suggested_questions,allowed_origins,widget_position,allow_web_search,allow_file_upload,agent_web_search_enabled,agent_image_upload_enabled,default_locale}}`

```bash
curl $BASE/api/v1/embed/ec-1/config -H "Authorization: Embed $EMBED_TOKEN"
```

### GET /api/v1/embed/:channel_id/suggested-questions

Purpose: starter questions. Query parameter: `limit` (capped at 12; omitted means the agent's starter count). An empty list when the channel has suggested questions turned off.

Response: 200 `{"success":true,"data":{"questions":[...]}}`

```bash
curl "$BASE/api/v1/embed/ec-1/suggested-questions?limit=6" -H "Authorization: Embed $EMBED_TOKEN"
```

### GET /api/v1/embed/:channel_id/chunks/:chunk_id

Purpose: view a cited chunk (content is sanitized; a chunk outside the channel's knowledge bases 403, unknown chunk 404).

Response: 200 `{"success":true,"data":{chunk}}`

```bash
curl $BASE/api/v1/embed/ec-1/chunks/c-1 -H "Authorization: Embed $EMBED_TOKEN"
```

### POST /api/v1/embed/:channel_id/sessions

Purpose: create a visitor session and return its ID and signed handle. No request body. The handle is an HMAC over `embed-session:v2|publish_token|channel_id|session_id` keyed by the system signing key (`SYSTEM_SIGNING_KEY`, falling back to `SYSTEM_AES_KEY`); 503 when that key is not set. Rotating the publish token invalidates outstanding handles.

Response: 201 `{"success":true,"data":{"id":"<session_id>","sig":"<signature>"}}`

```bash
curl -X POST $BASE/api/v1/embed/ec-1/sessions -H "Authorization: Embed $EMBED_TOKEN"
```

### POST /api/v1/embed/:channel_id/knowledge-chat/:session_id

Purpose: visitor knowledge Q&A (SSE; the payload is rewritten under the channel's constraints and delegated to KnowledgeQA). Requires `X-Embed-Session` (invalid signature 403, session of another channel 403, unknown session 404). Request body as for `/knowledge-chat` (`query` required).

```bash
curl -N -X POST $BASE/api/v1/embed/ec-1/knowledge-chat/s-1 \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG" \
  -H 'Content-Type: application/json' -d '{"query":"Opening hours?"}'
```

### POST /api/v1/embed/:channel_id/agent-chat/:session_id

Purpose: visitor agent Q&A (SSE). Requires `X-Embed-Session`. Request body as above.

```bash
curl -N -X POST $BASE/api/v1/embed/ec-1/agent-chat/s-1 \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG" \
  -H 'Content-Type: application/json' -d '{"query":"Place an order for me"}'
```

### GET /api/v1/embed/:channel_id/messages/:session_id/load

Purpose: load the visitor session's messages (delegates to `LoadMessages`; query parameters `limit` (default 20) and `before_time`). Requires `X-Embed-Session`.

Response: 200 `{"success":true,"data":[Message]}`

```bash
curl "$BASE/api/v1/embed/ec-1/messages/s-1/load?limit=20" \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG"
```

### POST /api/v1/embed/:channel_id/sessions/:session_id/stop

Purpose: stop generation (delegates to StopSession; request body `{"message_id":"..."}`, required). Requires `X-Embed-Session`.

```bash
curl -X POST $BASE/api/v1/embed/ec-1/sessions/s-1/stop \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG" \
  -H 'Content-Type: application/json' -d '{"message_id":"m-1"}'
```

### GET|POST /api/v1/embed/:channel_id/sessions/:session_id/messages/:message_id/suggestions

Purpose: read (GET) or trigger (POST) message suggestions (returns `status: "suppressed"` with `suppression_reason: "channel_disabled"` when the channel has suggestions turned off). Requires `X-Embed-Session`.

Response: 200 `{"success":true,"data":{"status","questions":[...]}}`

```bash
curl $BASE/api/v1/embed/ec-1/sessions/s-1/messages/m-1/suggestions \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG"
```

### POST /api/v1/embed/:channel_id/sessions/:session_id/suggestion-events

Purpose: report a suggestion interaction event (delegates to RecordEvent, same fields as the authenticated version). Requires `X-Embed-Session`. Response: 204.

```bash
curl -X POST $BASE/api/v1/embed/ec-1/sessions/s-1/suggestion-events \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG" \
  -H 'Content-Type: application/json' -d '{"suggestion_set_id":"ss-1","event_type":"impression"}'
```

### POST /api/v1/embed/:channel_id/sessions/:session_id/events

Purpose: forward a visitor event to the channel webhook. Requires `X-Embed-Session`.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `type` | string | yes | `message_sent` / `message_received` |
| `query` / `content` | string | no | The user's question / the bot's reply |

Response: 200 `{"success":true}`; unsupported type 400.

```bash
curl -X POST $BASE/api/v1/embed/ec-1/sessions/s-1/events \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG" \
  -H 'Content-Type: application/json' -d '{"type":"message_sent","query":"Hello"}'
```

### MCP OAuth and tool approval (visitor side)

Every route below requires `X-Embed-Session` and delegates to the matching authenticated handler (`internal/handler/mcp_oauth.go`, `internal/handler/mcp_service.go`):

| Method and path | Purpose |
| --- | --- |
| `POST /api/v1/embed/:channel_id/sessions/:session_id/mcp-oauth-resolutions/:pending_id` | Resume a run paused for OAuth (body: `service_id` required, `decision` optional) |
| `POST /api/v1/embed/:channel_id/sessions/:session_id/mcp-oauth-resolutions/:pending_id/cancel` | Cancel the OAuth flow |
| `POST /api/v1/embed/:channel_id/sessions/:session_id/mcp-services/:id/oauth/authorize-url` | Generate the authorization URL (body: `redirect_uri` required) |
| `GET /api/v1/embed/:channel_id/sessions/:session_id/mcp-services/:id/oauth/status` | Query the authorization status |
| `POST /api/v1/embed/:channel_id/sessions/:session_id/tool-approvals/:pending_id` | Tool approval (body: `decision` required) |

```bash
curl -X POST $BASE/api/v1/embed/ec-1/sessions/s-1/tool-approvals/p-1 \
  -H "Authorization: Embed $EMBED_TOKEN" -H "X-Embed-Session: $SIG" \
  -H 'Content-Type: application/json' -d '{"decision":"approve"}'
```

### GET /api/v1/embed/:channel_id/files

Purpose: visitor-side image proxy (images embedded in bot replies; EmbedAuth injects the channel's workspace and the handler enforces same-workspace paths). Query parameter: `file_path` (required).

Response: 200 file stream.

```bash
curl "$BASE/api/v1/embed/ec-1/files?file_path=local://1/exports/chart.png" \
  -H "Authorization: Embed $EMBED_TOKEN" -o chart.png
```

## File serving

Implemented in `internal/router/files.go` (not in the handler package).

### GET /files

Purpose: the authenticated file proxy for every backend (local, MinIO, COS, TOS and others). Permission: any authenticated workspace member; an API key must not be knowledge-base restricted (full access, or workspace-wide `retrieve`, `middleware.AllowFileServeAPIKey()`); paths are forced to the caller's workspace (`ValidateStoragePathTenant`).

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `file_path` | string | yes | `provider://...` path (`..` rejected; another workspace 403) |

Response: 200 file stream (`X-Content-Type-Options: nosniff`; types outside the inline allowlist are served with `Content-Disposition: attachment`).

```bash
curl "$BASE/files?file_path=local://1/docs/a.png" -H "Authorization: Bearer $TOKEN" -o a.png
```

### GET|HEAD /api/v1/files/presigned

Purpose: file access through an HMAC-signed URL (images embedded in IM platforms; no authentication, signature and expiry are checked; the key is `SYSTEM_SIGNING_KEY`, falling back to `SYSTEM_AES_KEY`).

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `file_path` | string | yes | Storage path |
| `tenant_id` | uint64 | yes | Workspace ID |
| `expires` | string | yes | Unix expiry time |
| `sig` | string | yes | HMAC signature |

Response: 200 file stream (HEAD returns the headers only); invalid or expired signature 403.

```bash
curl "$BASE/api/v1/files/presigned?file_path=local://1/x.png&tenant_id=1&expires=1790000000&sig=abc" -o x.png
```

### GET /api/v1/files/presigned-preview

Purpose: diagnostic endpoint that returns the presigned HTTP URL the server would generate for a path. Permission: Admin+, API keys explicitly rejected (`DenyAPIKeyPrincipal`). Query parameter: `file_path` (required).

`file_path` must belong to the current workspace, exactly as on `/files`: a `resource://` handle must resolve to a resource of this workspace, and a storage path must carry this workspace's ID in its tenant segment; otherwise 403 and no URL is issued.

Response: 200 `{"file_path","provider","url","rewritten":bool,"hint"}`

```bash
curl "$BASE/api/v1/files/presigned-preview?file_path=local://1/x.png" -H "Authorization: Bearer $TOKEN"
```

### GET|HEAD /r/:token

Purpose: short-lived resource grant URL (for IM platforms and other clients that cannot send an authentication header). No authentication, the token is the capability; invalid or expired 404.

Response: 200 file stream (`Cache-Control: private, max-age=300`).

```bash
curl $BASE/r/abc123 -o file.png
```

## Implementation reference

Route registration: `RegisterIMRoutes`, `RegisterIMChannelRoutes`, `RegisterEmbedChannelRoutes` and `RegisterEmbedPublicRoutes` in `internal/router/routes_agent.go`; `serveFilesWithResources`, `servePresignedFiles`, `servePresignedPreview` and `serveResourceGrants` in `internal/router/files.go`. Handlers: `internal/handler/im.go`, `internal/handler/wechat_qrcode.go`, `internal/handler/embed_channel.go`.
