# API reference: workspaces (tenants) and members

Manage workspaces, members, invitations, API keys and the audit log. Operations act on the current active workspace; cross-workspace access is checked per route.

Every `/tenants/:id/*` route carries `PathTenantMatch()` at the group level (`internal/middleware/access.go`): the `:id` in the URL must equal the current active workspace (cross-workspace superusers are exempt), so an Owner of workspace A cannot operate on workspace B by editing the URL.

The workspace `memory_config` fields and the personal memory endpoints are on [Long-term memory API](02-api-memory.md). A workspace admin who updates a configuration submits the complete object to keep.

## Workspace lifecycle

### POST /api/v1/tenants

Purpose: create a workspace (self-service; the caller becomes its Owner). Permission: any signed-in user, including one with no workspace yet; API keys only platform keys with `system_tenants_manage`. Handler: `internal/handler/tenant.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes (`binding:"required,min=1,max=128"`) | Workspace name |
| `description` | string | no (`binding:"max=512"`) | Description |

A cross-workspace superuser or a platform key may submit a full `types.Tenant` (with `storage_quota`, `status` and so on); a client-supplied `id` is ignored.

Response: 201 `{"success":true,"data":{Tenant}}` (`data.api_key` carries a plaintext full-access key when `tenant.auto_create_api_key` / `AIKB_TENANT_AUTO_CREATE_API_KEY` is on; off by default). Self-service creation disabled (`tenant.self_service_creation_enabled` / `AIKB_TENANT_SELF_SERVICE_CREATION_ENABLED`) returns 403 (code 2005). A regular user may own at most 10 workspaces by default (`AIKB_TENANT_MAX_OWNED_PER_USER`, `tenant.max_owned_per_user`); over the cap returns 429.

```bash
curl -X POST $BASE/api/v1/tenants -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"My workspace"}'
```

### GET /api/v1/tenants

Purpose: the current active workspace, as a one-element list. Permission: signed in; API key `manage_tenant_settings` or full access. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"data":{"items":[TenantResponse]}}`

```bash
curl $BASE/api/v1/tenants -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/all

Purpose: list every workspace (cross-workspace superuser). Permission: `CrossTenant()` (`CanAccessAllTenants` on the user and `EnableCrossTenantAccess` on the cluster); platform keys need `system_tenants_read` or `system_tenants_manage`. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"data":{"items":[TenantResponse]}}`

```bash
curl $BASE/api/v1/tenants/all -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/search

Purpose: search workspaces by keyword (cross-workspace superuser). Permission: as above. Handler: `internal/handler/tenant.go`

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `keyword` | string | no | Keyword |
| `tenant_id` | string | no | Exact workspace ID |
| `page` / `page_size` | int | no | Pagination (default 1/20, maximum 100) |

Response: 200 `{"success":true,"data":{"items":[...],"total","page","page_size"}}`

```bash
curl "$BASE/api/v1/tenants/search?keyword=demo&page=1" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/:id

Purpose: workspace detail. Permission: Viewer+; platform keys need `system_tenants_read` or `system_tenants_manage`. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"data":{TenantResponse}}`

```bash
curl $BASE/api/v1/tenants/1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/tenants/:id

Purpose: update the workspace. Permission: Owner; platform keys need `system_tenants_manage`. Only `name` and `description` are writable here; quota, status and configuration live behind dedicated endpoints. Handler: `internal/handler/tenant.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | *string | no (`binding:"omitempty,min=1,max=128"`) | New name |
| `description` | *string | no (`binding:"omitempty,max=512"`) | New description |

Response: 200 `{"success":true,"data":{TenantResponse}}`

```bash
curl -X PUT $BASE/api/v1/tenants/1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"New name"}'
```

### DELETE /api/v1/tenants/:id

Purpose: delete the workspace. Permission: Owner; platform keys need `system_tenants_manage`. The workspace row and all of its memberships are soft-deleted in one transaction and members lose access at once; knowledge bases, models and other data inside it are not purged immediately, and queued Wiki tasks for a deleted workspace no longer call the model. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"message":"Workspace deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1 -H "Authorization: Bearer $TOKEN"
```

## Workspace KV configuration

`:key` is a configuration key, not a workspace ID (the workspace comes from the authentication context). Keys: `web-search-config`, `prompt-templates` (GET only), `parser-engine-config`, `storage-engine-config`, `chat-history-config`, `retrieval-config`, `memory-config`. An unknown key returns 400. `web-search-config`, `parser-engine-config` and `storage-engine-config` carry integration secrets, so both GET and PUT on them need Admin+ (or a full-access key, or a scoped key with the matching capability).

### GET /api/v1/tenants/kv/:key

Purpose: read a workspace-level KV configuration. Permission: Viewer+ (Admin+ for the three secret-bearing keys above); API key `manage_tenant_settings` or full access. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"data":{...the configuration object for the key...}}`

```bash
curl $BASE/api/v1/tenants/kv/retrieval-config -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/tenants/kv/:key

Purpose: update a workspace-level KV configuration. Permission: Admin+; API key `manage_tenant_settings` or full access. Request body: the JSON configuration object for `:key`. For `memory-config`, an unknown `write_mode` (anything but `explicit_only` or `auto`) returns 400 and `extract_delay_seconds` must be 0 to 3600. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"message":"Configuration updated"}`

```bash
curl -X PUT $BASE/api/v1/tenants/kv/web-search-config -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"enabled":true}'
```

## API keys and the API principal

### GET /api/v1/tenants/:id/api-keys

Purpose: list the workspace's active API keys (revoked keys are omitted). Permission: Owner, JWT only (API keys are denied by default). Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"data":[{id,scope_type,name,api_key,full_access,knowledge_base_ids,capabilities,last_used_at,expires_at,created_at}]}`. `api_key` is masked: the first 7 characters, `...`, and the last 4 (for example `sk-Ab12...wxyz`). The full key is returned only by POST.

```bash
curl $BASE/api/v1/tenants/1/api-keys -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/api-keys

Purpose: create a workspace API key. Permission: Owner, JWT only. Handler: `internal/handler/tenant.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes | Key name |
| `full_access` | bool | no | Full workspace access (default false) |
| `knowledge_base_ids` | []string | no | Knowledge base allow-list (scoped key); an unknown ID returns 400, one from another workspace 403 |
| `capabilities` | []string | no | Capability list (see the [API overview](./01-api-overview.md)); unknown entries are dropped |
| `expires_at_unix` | *int64 | no | Expiry timestamp; must be in the future |

Response: 201 `{"success":true,"data":{...,"api_key":"<key>","token":"<key>"}}` (both fields carry the same plaintext key). This is the only response that carries the full key; store it on receipt. A lost key cannot be retrieved: delete it and create a new one.

```bash
curl -X POST $BASE/api/v1/tenants/1/api-keys -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"ingest-bot","capabilities":["ingest","retrieve"],"knowledge_base_ids":["kb-1"]}'
```

### PUT /api/v1/tenants/:id/api-keys/:key_id

Owner, JWT only. Updates an existing key's `name`, `full_access`, `knowledge_base_ids`, `capabilities` and `expires_at_unix`. The grant fields are submitted as a whole; this is not a one-field PATCH. An omitted or `null` `expires_at_unix` clears the existing expiry. A client that keeps using the same token gets the new grants on its next authenticated request. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"data":{APIKeyResponse}}` (same fields as the list, `api_key` masked). Invalid capabilities or an unknown knowledge base return 400, a knowledge base outside the workspace 403, a missing key 404.

```bash
curl -X PUT "$BASE/api/v1/tenants/1/api-keys/5" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"search-bot","full_access":false,"knowledge_base_ids":["kb-1"],"capabilities":["retrieve"]}'
```

### DELETE /api/v1/tenants/:id/api-keys/:key_id

Purpose: revoke an API key. Permission: Owner, JWT only. Path parameter: `key_id`. A missing key returns 404.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1/api-keys/5 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/tenants/:id/api-principal-config

Purpose: read how `X-API-Key` requests map to an external user principal. Permission: Owner, JWT only. Handler: `internal/handler/tenant.go`

Response: 200 `{"success":true,"data":{"mode":"tenant|direct_header|signed_token","direct_header_name":"X-External-User-ID","signed_token_header_name":"X-External-User-Token","require_direct_header","has_hmac_secret"}}`. The header names are fixed; the HMAC secret is never returned.

```bash
curl $BASE/api/v1/tenants/1/api-principal-config -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/tenants/:id/api-principal-config

Purpose: update the external user principal configuration. Permission: Owner, JWT only.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `mode` | string | no | `tenant` (default when empty) / `direct_header` / `signed_token`; any other value returns 400 |
| `require_direct_header` | bool | no | In `direct_header` mode, reject requests without the header |
| `hmac_secret` | *string | no | Signing secret for `signed_token` mode (send `***` to keep the stored value); required when `mode` is `signed_token` |

Response: 200, same shape as GET.

```bash
curl -X PUT $BASE/api/v1/tenants/1/api-principal-config -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"mode":"signed_token","hmac_secret":"topsecret"}'
```

### POST /api/v1/tenants/:id/api-principal-test-token

Purpose: issue an external user JWT for testing. Permission: Owner, JWT only. Requires `signed_token` mode with a stored secret (otherwise 400).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `external_user_id` | string | yes | External user ID (at most 128 characters, no control characters) |
| `expires_in_seconds` | int | no | 1 to 3600, default 900 |

Response: 200 `{"success":true,"data":{"token","header_name":"X-External-User-Token","expires_in_seconds","expires_at_unix","external_user_id"}}`

```bash
curl -X POST $BASE/api/v1/tenants/1/api-principal-test-token -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"external_user_id":"u-123"}'
```

## Member management (/tenants/:id/members)

Handler: `internal/handler/tenant_member.go`. API key `manage_members` or full access; an API key can never assign the `owner` role (403). The roles are `owner`, `admin`, `contributor` and `viewer`; see [RBAC roles](../03-features/01-tenant-auth.md#rbac-roles-ownership-and-the-guard-matrix).

### GET /api/v1/tenants/:id/members

Purpose: member list. Permission: Viewer+.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `q` | string | no | Filter by email or username |
| `page` / `page_size` | int | no | Pagination (default 1/20, maximum 100; out of range returns 400) |

Response: 200 `{"success":true,"data":{"members":[{user_id,email,username,avatar,role,status,invited_by,joined_at}],"total","page","page_size"}}`

```bash
curl $BASE/api/v1/tenants/1/members -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/members

Purpose: add a member directly. Permission: Owner.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `email` | string | yes (`binding:"required,email"`) | Member email (must already be registered; otherwise 404) |
| `role` | string | yes (`binding:"required"`) | `owner` / `admin` / `contributor` / `viewer` |

Response: 201 `{"success":true,"data":{member object, same shape as the list}}`. An existing member returns 409.

```bash
curl -X POST $BASE/api/v1/tenants/1/members -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"b@ex.com","role":"contributor"}'
```

### PUT /api/v1/tenants/:id/members/:user_id

Purpose: change a member's role. Permission: Owner. Request body: `{"role":"admin"}` (`binding:"required"`).

Response: 200 `{"success":true}`

```bash
curl -X PUT $BASE/api/v1/tenants/1/members/u-123 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"admin"}'
```

### DELETE /api/v1/tenants/:id/members/:user_id

Purpose: remove a member. Permission: Owner.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1/members/u-123 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/leave

Purpose: leave the workspace (any member can leave on their own; the service refuses when the workspace would be left without an Owner, 409). Permission: Viewer+, JWT only.

Response: 200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/tenants/1/leave -H "Authorization: Bearer $TOKEN"
```

## Workspace invitations (/tenants/:id/invitations and invite-links)

Handlers: `internal/handler/tenant_invitation.go`, `internal/handler/tenant_invite_link.go`. API key `manage_members` or full access; an API key cannot invite as `owner` (403). Invitations expire after 7 days by default (`AIKB_INVITATION_TTL`, a Go duration or a number of seconds). Share links are described in [Shared invitation links](../03-features/01-tenant-auth.md#shared-invitation-links-invite-link).

### GET /api/v1/tenants/:id/invitations

Purpose: the workspace's invitations. Permission: Viewer+.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `include_terminal` | bool | no | Also return accepted, declined, revoked and expired rows |
| `page` / `page_size` | int | no | Pagination |

Response: 200 `{"success":true,"data":{"invitations":[{id,tenant_id,tenant_name,invitee_user_id,invitee_email,invitee_name,invited_by,inviter_email,inviter_name,role,status,message,expires_at,responded_at,created_at,invite_url,is_share_link,accepted_count}],"total","page","page_size"}}`. `invite_url` is present only on pending share-link rows.

```bash
curl $BASE/api/v1/tenants/1/invitations -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/invitations

Purpose: invite a member (the invitee joins after confirming under `/me/invitations`; with `tenant.auto_accept_invitation` / `AIKB_TENANT_AUTO_ACCEPT_INVITATION` on, the registered invitee is added at once). Permission: Owner.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `email` | string | yes (`binding:"required,email"`) | Invitee email (must already be registered; otherwise 404) |
| `role` | string | yes (`binding:"required"`) | Role granted |
| `message` | string | no | Note to the invitee |

Response: 201 `{"success":true,"data":{TenantInvitationResponse}}`. An existing member or a pending invitation for the same user returns 409.

```bash
curl -X POST $BASE/api/v1/tenants/1/invitations -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"c@ex.com","role":"viewer"}'
```

### DELETE /api/v1/tenants/:id/invitations/:inv_id

Purpose: revoke an invitation. Permission: Owner.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/tenants/1/invitations/12 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/tenants/:id/invite-links

Purpose: create a share link (a reusable registration invitation). Permission: Owner. Handler: `internal/handler/tenant_invite_link.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `role` | string | yes (`binding:"required"`) | Role granted by the link |
| `message` | string | no | Note |

Response: 201 `{"success":true,"data":{id,tenant_id,invite_url,role,status,expires_at,created_at,is_share_link:true,accepted_count}}` (the raw token appears only inside `invite_url`). `invite_url` is `<frontend base URL>/register?token=<token>`.

```bash
curl -X POST $BASE/api/v1/tenants/1/invite-links -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"viewer"}'
```

## Audit log

Handler: `internal/handler/audit_log.go`. Cursor pagination: rows are sorted by `id` descending, `next_cursor` is the smallest `id` on the page (0 when nothing older remains) and is passed back as `after_id`.

### GET /api/v1/tenants/:id/audit-log

Purpose: the workspace audit log (denied actions included). Permission: Admin+, JWT only.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `after_id` | int | no | Cursor (the previous response's `next_cursor`); an invalid value starts from the newest row |
| `limit` | int | no | 1 to 100, default 50 (larger values are capped) |
| `action` | string | no | Exact action filter (for example `rbac.member_added`) |
| `outcome` | string | no | `success` / `denied` |
| `actor` | string | no | Filter by actor `user_id` |

Response: 200 `{"success":true,"data":[AuditLog],"next_cursor":N}`

```bash
curl "$BASE/api/v1/tenants/1/audit-log?limit=50" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/knowledge-bases/:id/activity

Purpose: the activity feed of one knowledge base (read-only audit). Permission: KB creator OR Admin+, with read access to the knowledge base, and only from the owner workspace (an organization-shared consumer gets 403); JWT only. Query parameters as above (`after_id`, `limit`, `action`, `outcome`, `actor`). Registered by `RegisterKnowledgeBaseActivityRoutes`.

Response: 200 `{"success":true,"data":[AuditLog],"next_cursor":N}`. `details` is the action payload; when the entry was triggered by an API key it includes `api_key_id` and `api_key_name` (a name snapshot, never the key).

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/activity -H "Authorization: Bearer $TOKEN"
```

## Implementation reference

Route registration: `RegisterTenantRoutes` in `internal/router/routes_auth_tenant.go`. Handlers: `internal/handler/tenant.go`, `internal/handler/tenant_member.go`, `internal/handler/tenant_invitation.go`, `internal/handler/tenant_invite_link.go`, `internal/handler/audit_log.go`.
