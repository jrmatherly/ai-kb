# API reference: organizations and sharing

Manage organization members and the knowledge base and agent shares between workspaces. An organization's members are workspaces.

An organization (Organization) has workspaces (tenants) as its members. Permission levels below are the caller's role in their own workspace (Owner, Admin, Contributor, Viewer, see [RBAC](../03-features/01-tenant-auth.md#rbac-roles-ownership-and-the-guard-matrix)); where the service also checks the caller's workspace role inside the organization (`admin`, `editor`, `viewer`), that is stated per route. API keys: the organization routes accept `manage_spaces` or a full-access key; knowledge base and agent share management is only available to a full-access key. Handler: `internal/handler/organization.go`

## Organization management (/api/v1/organizations)

### POST /api/v1/organizations

Purpose: create an organization. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | yes (`binding:"required"`, 1 to 255 characters) | Organization name |
| `description` | string | no | Description (at most 1000 characters) |
| `avatar` | string | no | Avatar URL (at most 512 characters) |
| `member_limit` | int | no | Maximum number of member workspaces; `0` means unlimited; default 50 |
| `invite_code_validity_days` | int | no | Invite code validity in days: `0` (never expires), `1`, `7` or `30`; default 7. Any other value returns 400 |

`searchable` and `require_approval` are not accepted on create (both start as `false`); set them with `PUT /api/v1/organizations/:id`.

Response: 201 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl -X POST $BASE/api/v1/organizations -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"Engineering"}'
```

### GET /api/v1/organizations

Purpose: list the organizations my workspace belongs to. Permission: Viewer+.

Response: 200 `{"success":true,"data":{"organizations":[OrganizationResponse],"total":N,"resource_counts":{"knowledge_bases":{"by_organization":{}},"agents":{"by_organization":{}}}}}`. `invite_code`, `invite_code_expires_at` and `pending_join_request_count` are filled only for an organization admin or the owner workspace.

```bash
curl $BASE/api/v1/organizations -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/preview/:code

Purpose: preview an organization by invite code (without joining). Permission: Viewer+. Path parameter: `code`, the invite code. An unknown or expired code returns 404.

Response: 200 `{"success":true,"data":{id,name,description,avatar,member_count,share_count,agent_share_count,is_already_member,require_approval,created_at}}`

```bash
curl $BASE/api/v1/organizations/preview/ABC123 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/join

Purpose: join an organization with an invite code. Permission: Admin+. Request body: `{"invite_code":"..."}` (required, 8 to 32 characters). An unknown or expired code returns 404; a full organization (`member_limit` reached) returns 400.

Response: 200 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl -X POST $BASE/api/v1/organizations/join -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"invite_code":"ABC123"}'
```

### POST /api/v1/organizations/join-request

Purpose: submit a join request to an organization that requires approval. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `invite_code` | string | yes (`binding:"required"`, 8 to 32 characters) | Invite code |
| `message` | string | no | Note for the reviewer (at most 500 characters) |
| `role` | string | no | Requested role: `viewer`, `editor` or `admin`; default `viewer` |

Returns 400 when the organization does not require approval (use `join` instead), when the caller's workspace is already a member, when a pending request already exists, or when the organization is full.

Response: 200 `{"success":true,"data":{JoinRequest}}`

```bash
curl -X POST $BASE/api/v1/organizations/join-request -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"invite_code":"ABC123","message":"Please add us"}'
```

### GET /api/v1/organizations/search

Purpose: search discoverable (`searchable`) organizations. Permission: Viewer+.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `q` | string | no | Keyword (name or description) |
| `limit` | int | no | Default 20; a value outside 1 to 100 falls back to 20 |

Response: 200 `{"success":true,"data":[SearchableOrganization],"total":N}`. Each item carries `id`, `name`, `description`, `avatar`, `member_count`, `member_limit`, `share_count`, `agent_share_count`, `is_already_member`, `require_approval`.

```bash
curl "$BASE/api/v1/organizations/search?q=engineering" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/join-by-id

Purpose: join a discoverable organization by its ID (no invite code). Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `organization_id` | string | yes (`binding:"required"`) | Target organization ID |
| `message` | string | no | Note for the reviewer (at most 500 characters) |
| `role` | string | no | Requested role: `viewer`, `editor` or `admin`; default `viewer` |

When the organization requires approval, this creates a join request instead of joining and still returns the organization. A workspace that is already a member gets the organization back unchanged. 404 when the organization does not exist, 403 when it is not searchable, 400 when it is full.

Response: 200 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl -X POST $BASE/api/v1/organizations/join-by-id -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"organization_id":"org-1"}'
```

### GET /api/v1/organizations/:id

Purpose: organization detail. Permission: Viewer+.

Response: 200 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl $BASE/api/v1/organizations/org-1 -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/organizations/:id

Purpose: update an organization. Permission: Admin+; the service requires the caller's workspace to be an organization `admin` (not only the owner). Request body fields: `name`, `description`, `avatar`, `require_approval`, `searchable`, `invite_code_validity_days`, `member_limit`, all optional; only fields present are changed. `invite_code_validity_days` outside `0/1/7/30` and a `member_limit` below the current member count are rejected.

Response: 200 `{"success":true,"data":{OrganizationResponse}}`

```bash
curl -X PUT $BASE/api/v1/organizations/org-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"description":"Updated description","searchable":true}'
```

### DELETE /api/v1/organizations/:id

Purpose: delete an organization. Permission: Admin+; the service requires the organization's owner workspace. Every knowledge base and agent share into the organization is deleted with it.

Response: 200 `{"success":true,"message":"Organization deleted successfully"}`

```bash
curl -X DELETE $BASE/api/v1/organizations/org-1 -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/:id/leave

Purpose: take my workspace out of the organization. Permission: Admin+. No request body. The owner workspace cannot leave (403); it transfers ownership or deletes the organization. The knowledge bases and agents my workspace shared into the organization are revoked together.

Response: 200 `{"success":true,"message":"Left organization successfully"}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/leave -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/:id/request-upgrade

Purpose: request a higher role for my workspace inside the organization. Permission: Admin+.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `requested_role` | string | yes (`binding:"required"`) | Requested organization role (`viewer`, `editor` or `admin`) |
| `message` | string | no | Reason (at most 500 characters) |

Returns 400 when the workspace is not a member, is already an `admin`, requests the same or a lower role, or already has a pending upgrade request.

Response: 200 `{"success":true,"data":{JoinRequest}}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/request-upgrade -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"requested_role":"editor"}'
```

### POST /api/v1/organizations/:id/invite-code

Purpose: generate a new organization invite code. Permission: Admin+; the service requires the caller's workspace to be an organization `admin`. No request body. The code expires after the organization's `invite_code_validity_days`.

Response: 200 `{"success":true,"data":{"invite_code":"..."}}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/invite-code -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/search-tenants

Purpose: resolve a workspace that can be invited, by workspace ID. Permission: Admin+, and the caller's workspace must be an organization `admin` (otherwise 403). Since v0.8.2 it accepts only a full workspace ID; there is no cross-workspace search by name, and the `limit` parameter is gone.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `q` | string | yes | Full workspace ID |

Response: 200 `{"success":true,"data":[{"tenant_id","tenant_name"}]}`. When `q` is empty or not a valid ID, the workspace does not exist, or it is already a member, `data` is an empty array; otherwise it holds the single candidate.

```bash
curl "$BASE/api/v1/organizations/org-1/search-tenants?q=10002" -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/search-users

Purpose: deprecated alias with the same behavior as `search-tenants`. Permission: Admin+. Same parameters.

```bash
curl "$BASE/api/v1/organizations/org-1/search-users?q=10002" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/organizations/:id/invite

Purpose: add a workspace to the organization directly. Permission: Admin+, and the caller's workspace must be an organization `admin` (otherwise 403).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `tenant_id` | uint64 | one of the two | Target workspace ID (preferred); 404 when it does not exist |
| `user_id` | string | one of the two | Compatibility path: a user ID, resolved to that user's workspace; 404 when the user does not exist |
| `representative_user_id` | string | no | Ignored, kept for compatibility: a directly added workspace gets no representative user, so the inviter cannot choose whose details appear in the member list |
| `role` | string | yes (`binding:"required"`) | Organization role: `viewer`, `editor` or `admin` |

Returns 400 when neither `tenant_id` nor `user_id` is given, when the workspace is already a member, or when the organization is full.

Response: 200 `{"success":true,"message":"Member added successfully"}`

```bash
curl -X POST $BASE/api/v1/organizations/org-1/invite -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"tenant_id":2,"role":"viewer"}'
```

### GET /api/v1/organizations/:id/members

Purpose: list the organization's member workspaces. Permission: Viewer+; the caller's workspace must be a member (otherwise 403). `email` is returned only on the caller's own workspace row; other workspaces expose only the representative's username and avatar. Workspaces added directly have no representative user.

Response: 200 `{"success":true,"data":{"members":[{id,user_id,representative_user_id,role,tenant_id,tenant_name,username,email,avatar,joined_at}],"total":N}}`

```bash
curl $BASE/api/v1/organizations/org-1/members -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/organizations/:id/members/:tenant_id

Purpose: change a member workspace's organization role. Permission: Admin+; the service requires the caller's workspace to be an organization `admin`, and the owner workspace's role cannot be changed. Path parameter `tenant_id` is the member workspace ID (a non-numeric value returns 400). Request body: `{"role":"editor"}` (required; `viewer`, `editor` or `admin`).

Response: 200 `{"success":true,"message":"Member role updated successfully"}`

```bash
curl -X PUT $BASE/api/v1/organizations/org-1/members/2 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"role":"editor"}'
```

### DELETE /api/v1/organizations/:id/members/:tenant_id

Purpose: remove a member workspace (including my own). Permission: Admin+; removing another workspace requires the caller's workspace to be an organization `admin`, and the owner workspace cannot be removed. The knowledge bases and agents the removed workspace shared into the organization are revoked together.

Response: 200 `{"success":true,"message":"Member removed successfully"}`

```bash
curl -X DELETE $BASE/api/v1/organizations/org-1/members/2 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/join-requests

Purpose: the pending join and upgrade request queue. Permission: Admin+, and the caller's workspace must be an organization `admin` (otherwise 403). Only requests with status `pending` are returned.

Response: 200 `{"success":true,"data":{"requests":[{id,user_id,username,email,message,request_type,prev_role,requested_role,status,created_at,reviewed_at}],"total":N}}`. `request_type` is `join` or `upgrade`; `prev_role` is set only for upgrade requests.

```bash
curl $BASE/api/v1/organizations/org-1/join-requests -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/organizations/:id/join-requests/:request_id/review

Purpose: approve or reject a join or upgrade request. Permission: Admin+, and the caller's workspace must be an organization `admin` (otherwise 403).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `approved` | bool | no | `true` approves, `false` or omitted rejects |
| `message` | string | no | Review note (at most 500 characters) |
| `role` | string | no | Role granted on approval; overrides the applicant's requested role |

Returns 400 when the request was already reviewed or when approving would exceed `member_limit`.

Response: 200 `{"success":true,"message":"Review completed"}`

```bash
curl -X PUT $BASE/api/v1/organizations/org-1/join-requests/req-1/review \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"approved":true}'
```

### GET /api/v1/organizations/:id/shares

Purpose: list the knowledge bases shared into the organization. Permission: Viewer+; the caller's workspace must be a member (otherwise 403). A knowledge-base-restricted API key sees only the knowledge bases on its allow-list.

Response: 200 `{"success":true,"data":{"shares":[KnowledgeBaseShareResponse],"total":N}}`. Each share carries `permission` (what the source workspace granted), `my_role_in_org` and `my_permission` (the lower of the two), plus `knowledge_base_name`, `knowledge_base_type`, `knowledge_count`, `chunk_count`, `source_tenant_id`, `shared_by_username`.

```bash
curl $BASE/api/v1/organizations/org-1/shares -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/agent-shares

Purpose: list the agents shared into the organization. Permission: Viewer+; the caller's workspace must be a member (otherwise 403).

Response: 200 `{"success":true,"data":{"shares":[AgentShareResponse],"total":N}}`. Besides the share fields, each item summarizes the agent's scope: `scope_kb` (`all`, `selected` or `none`) with `scope_kb_count`, `scope_web_search`, `scope_mcp` with `scope_mcp_count`, and `agent_avatar`.

```bash
curl $BASE/api/v1/organizations/org-1/agent-shares -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/shared-knowledge-bases

Purpose: organization view for the list page: every knowledge base shared in the organization, including my own, merged with the knowledge bases carried by shared agents. Permission: Viewer+; the caller's workspace must be a member (otherwise 403).

Response: 200 `{"success":true,"data":[...],"total":N}`. Each row carries `is_mine` and, for knowledge bases reached through a shared agent, `source_from_agent` (`agent_id`, `agent_name`, `kb_selection_mode`). The owner's vector store metadata (`vector_store_id`, `vector_store_name`, `vector_store_engine_type`) is stripped from every row, including my own.

```bash
curl $BASE/api/v1/organizations/org-1/shared-knowledge-bases -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/organizations/:id/shared-agents

Purpose: organization view for the list page: every agent shared in the organization. Permission: Viewer+; the caller's workspace must be a member (otherwise 403).

Response: 200 `{"success":true,"data":[SharedAgentInfo],"total":N}`

```bash
curl $BASE/api/v1/organizations/org-1/shared-agents -H "Authorization: Bearer $TOKEN"
```

## Knowledge base shares (/api/v1/knowledge-bases/:id/shares)

API key: full access only. Handler: `internal/handler/organization.go`

### POST /api/v1/knowledge-bases/:id/shares

Purpose: share a knowledge base into an organization. Permission: KB creator OR Admin+. The service also requires that the caller's workspace owns the knowledge base and holds the `editor` or `admin` role in the organization (a `viewer` workspace gets 403). Sharing the same knowledge base into the same organization again updates the existing share's permission.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `organization_id` | string | yes (`binding:"required"`) | Target organization |
| `permission` | string | yes (`binding:"required"`) | Share permission, in organization role terms (`viewer` or `editor`) |

Response: 201 `{"success":true,"data":{KBShare}}`

```bash
curl -X POST $BASE/api/v1/knowledge-bases/kb-1/shares -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"organization_id":"org-1","permission":"viewer"}'
```

### GET /api/v1/knowledge-bases/:id/shares

Purpose: list the shares of a knowledge base. Permission: Viewer+ on the route; the service requires the caller's workspace to own the knowledge base (otherwise 403; 404 when it does not exist), which also keeps a full-access API key from listing another workspace's shares.

Response: 200 `{"success":true,"data":{"shares":[KnowledgeBaseShareResponse],"total":N}}`

```bash
curl $BASE/api/v1/knowledge-bases/kb-1/shares -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/knowledge-bases/:id/shares/:share_id

Purpose: change a share's permission. Permission: KB creator OR Admin+. Request body: `{"permission":"editor"}` (required).

Service rules (shared with unsharing); the caller's workspace role comes from the request context and a missing role counts as Viewer:

- the original sharer may act from the source workspace while holding Contributor+ there;
- an Admin+ of the source workspace may manage everything that workspace shared;
- an Admin+ user of a workspace whose organization role is `admin` may only **lower** the permission or remove the share, never raise it above the current value.

`share_id` must belong to the knowledge base in the path, otherwise 404.

Response: 200 `{"success":true,"message":"Share permission updated successfully"}`

```bash
curl -X PUT $BASE/api/v1/knowledge-bases/kb-1/shares/s-1 -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"permission":"editor"}'
```

### DELETE /api/v1/knowledge-bases/:id/shares/:share_id

Purpose: remove a share. Permission: KB creator OR Admin+, with the service rules above. `share_id` must belong to the knowledge base in the path, otherwise 404.

Response: 200 `{"success":true,"message":"Share removed successfully"}`

```bash
curl -X DELETE $BASE/api/v1/knowledge-bases/kb-1/shares/s-1 -H "Authorization: Bearer $TOKEN"
```

## Agent shares (/api/v1/agents/:id/shares)

API key: full access only. Handler: `internal/handler/organization.go`

### POST /api/v1/agents/:id/shares

Purpose: share an agent into an organization. Permission: agent creator OR Admin+. Request body is the same as for a knowledge base share (`organization_id` and `permission`, both required), but agent shares are read-only: the stored permission is always `viewer`, whatever the body says. The caller's workspace must own the agent and hold `editor` or `admin` in the organization (403 otherwise). Built-in agents cannot be shared (400): every workspace has its own copy under the same ID, so the receiver could not tell them apart. The agent must have a chat model, and a rerank model when its knowledge search tool is enabled (400 otherwise). The agent's knowledge base scope opens to organization members, so the caller must be allowed to share each of those knowledge bases directly (knowledge base creator or Admin+); `kb_selection_mode: all` can be shared only by Admin+ or a full-access API key, otherwise 403.

Response: 201 `{"success":true,"data":{AgentShare}}`

```bash
curl -X POST $BASE/api/v1/agents/agent-1/shares -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"organization_id":"org-1","permission":"viewer"}'
```

### GET /api/v1/agents/:id/shares

Purpose: list the shares of an agent. Permission: agent creator OR Admin+; the service also requires the caller's workspace to own the agent (otherwise 403; 404 when it does not exist).

Response: 200 `{"success":true,"data":{"shares":[AgentShareResponse],"total":N}}`

```bash
curl $BASE/api/v1/agents/agent-1/shares -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/agents/:id/shares/:share_id

Purpose: remove an agent share. Permission: agent creator OR Admin+, with the same service rules as removing a knowledge base share. `share_id` must belong to the agent in the path, otherwise 404.

Response: 200 `{"success":true,"message":"Share removed successfully"}`

```bash
curl -X DELETE $BASE/api/v1/agents/agent-1/shares/s-1 -H "Authorization: Bearer $TOKEN"
```

## Aggregate views of shared resources

### GET /api/v1/shared-knowledge-bases

Purpose: list the knowledge bases shared to my workspace through its organizations, with the owner's vector store metadata stripped. Permission: Viewer+; API key `manage_spaces` or full access. A knowledge-base-restricted API key sees only the knowledge bases on its allow-list.

Response: 200 `{"success":true,"data":[...],"total":N}`

```bash
curl $BASE/api/v1/shared-knowledge-bases -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/shared-agents

Purpose: list the agents shared to my workspace through its organizations (agents my own workspace shared are excluded). Permission: Viewer+; API key as above.

Response: 200 `{"success":true,"data":[SharedAgentInfo],"total":N}`. `SharedAgentInfo` carries `agent`, `share_id`, `organization_id`, `org_name`, `source_tenant_id`, `shared_by_username`, `permission` (the lower of the share permission and my organization role, capped by my workspace role), `disabled_by_me`, and `web_search_ready`. `web_search_ready` is a single boolean, "is web search usable in the source workspace": the source workspace's provider configuration is never returned (it would leak configuration), and the receiving workspace's provider IDs are not compared (that would report false negatives). The embedded `agent` is a receiver view with the prompts, templates and creator removed.

When another endpoint is called with a shared agent and the same agent is shared by several workspaces, `agent_source_tenant_id` selects the source workspace. The value is validated against the share relation; an invalid or unauthorized value returns 400 and never falls back silently to another source.

```bash
curl $BASE/api/v1/shared-agents -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/shared-agents/disabled

Purpose: mark a shared agent as disabled for my workspace (affects the session agent picker for the whole workspace). Permission: Admin+; API key as above. The agent must be my own or shared to my workspace (otherwise 403).

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `agent_id` | string | yes (`binding:"required"`) | Shared agent ID |
| `disabled` | bool | no | Disable or re-enable (default false) |

Response: 200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/shared-agents/disabled -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"agent_id":"agent-1","disabled":true}'
```

## Implementation reference

Route registration: `RegisterOrganizationRoutes` in `internal/router/routes_agent.go`. Handler: `internal/handler/organization.go`. Services: `internal/application/service/organization.go`, `kbshare.go`, `agent_share.go`, `agent_share_scope.go`. Types: `internal/types/organization.go`.
