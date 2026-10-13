# API reference: system and platform administration

Deployment-level system information and platform administration: global settings, task queues, platform API keys, cross-workspace audit and user password reset. The feature description is [Platform administration and system administrators](../03-features/20-platform-admin.md).

The whole `/system/admin/*` group carries the `SystemAdmin()` guard. Platform API keys are scoped per capability (`system_settings_read/manage`, `system_runtime_read/manage`, `system_tenants_read/manage`, `system_audit_read`).

## System information (/api/v1/system)

Handler: `internal/handler/system.go`. API key: `manage_vector_stores` or full access (except where noted). Successful responses in this group use the `{"code":0,"msg":"success","data":...}` envelope; 4xx and 5xx responses use the standard error envelope.

### GET /api/v1/system/capabilities

Viewer+; any API key can read it. Returns `{code:0,data:{edition,capabilities}}`; each capability carries `supported` and, when unsupported, a `reason`. The frontend uses the edition, the routes actually registered and the Docker switch to show or hide menu entries; a hidden menu does not replace the backend permission check.

```bash
curl "$BASE/api/v1/system/capabilities" -H "Authorization: Bearer $TOKEN"
```

`settings.sandbox.host` in `capabilities` says whether this deployment can use the local operating-system sandbox. Today it is only supported in the native macOS desktop application (other platforms report `platform_unsupported`). `settings.sandbox.docker` is read live, so a System Settings toggle shows up without a restart.

### POST /api/v1/system/host-project-dir

Purpose: open the native folder picker on the machine running AI KB so a new session can bind a local project directory (native desktop application only). Permission: Viewer+, JWT only; API keys are always denied. No request body.

Response: 200 `{"code":0,"msg":"success","data":{"dir":"/Users/me/project"}}`; `dir` is an empty string when the user cancels. A non-desktop deployment returns 404.

### GET /api/v1/system/info

Purpose: system version and engine information. Permission: Viewer+.

Response: 200 `{"code":0,"msg":"success","data":{version,edition,commit_id,build_time,go_version,keyword_index_engine,vector_store_engine,graph_database_engine,minio_enabled,db_version,db_migration_error,started_at,uptime_seconds}}`

`db_version` is the migration version; it gets the suffix `(dirty)`, after a space, when the last migration left the schema dirty and `(failed)` when the last startup migration failed, in which case `db_migration_error` carries the message. It is `unknown` when the failure happened before the version could be read.

```bash
curl $BASE/api/v1/system/info -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/parser-engines

Purpose: parser engine list and DocReader connection state. Permission: Viewer+.

Response: 200 `{"code":0,"msg":"success","data":[...],"docreader_addr","docreader_transport","connected"}`

```bash
curl $BASE/api/v1/system/parser-engines -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/parser-engines/check

Purpose: probe the parser engines with the given configuration (`types.ParserEngineConfig` request body, merged over the stored workspace configuration). Permission: Admin+.

Response: 200, same shape as above.

```bash
curl -X POST $BASE/api/v1/system/parser-engines/check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{}'
```

### POST /api/v1/system/docreader/reconnect

Purpose: reconnect DocReader. Permission: Admin+. Request body: `{"addr":"host:port"}` (`binding:"required"`); the address goes through the SSRF check (400 when rejected).

Response: 200 `{"code":0,"msg":"...","data":[...],"docreader_addr","docreader_transport","connected":true}`. A failed connection also returns 200, with `code:1` and the error in `msg`.

```bash
curl -X POST $BASE/api/v1/system/docreader/reconnect -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"addr":"docreader:50051"}'
```

### GET /api/v1/system/storage-engine-status

Purpose: object storage engine availability. Permission: Viewer+. An engine is `available` when the workspace's storage settings, an active storage backend, or the process-wide backend that `STORAGE_TYPE` and its environment variables select configure it.

Response: 200 `{"code":0,"msg":"success","data":{"engines":[{name,allowed,available,description}],"allowed_providers":[...],"minio_env_available":bool}}`

```bash
curl $BASE/api/v1/system/storage-engine-status -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/storage-engine-check

Purpose: validate a storage configuration (probe after the SSRF check). Permission: Admin+. Request body: `provider` (required, one of `minio/cos/tos/s3/oss/ks3/obs`) plus the matching `minio|cos|tos|s3|oss|ks3|obs` configuration object. A malformed body returns 400; a provider disabled by the allow-list returns 403.

Response: 200 `{"code":0,"data":{"ok","message","bucket_created"}}`

```bash
curl -X POST $BASE/api/v1/system/storage-engine-check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"provider":"minio","minio":{"endpoint":"minio:9000"}}'
```

### POST /api/v1/system/sandbox-check

Purpose: test a sandbox configuration without saving it. Permission: Admin+. Request body: `config` (`TenantSandboxConfig`), optional `config_id` (test a stored configuration, overriding only the fields sent in `config`) and `deep` (also run a throwaway script; on a remote backend this creates and destroys one sandbox).

Response: 200 `{"success":true,"data":{"ok","provider","checks":[{name,ok,message,reason,latency_ms}],"capabilities"}}`. A check with `ok:null` was skipped and `reason` says why. One failed check makes `ok` false.

```bash
curl -X POST $BASE/api/v1/system/sandbox-check -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"config_id":"sb-1","deep":false}'
```

## System administration (/api/v1/system/admin, SystemAdmin only)

The `SystemAdmin()` guard is mounted on the group (always enforced, independent of `EnableRBAC`). Only the routes listed with a `system_*` capability accept a platform API key with that capability; the others are user sessions only. Most read routes in this group return raw rows or arrays (no envelope). Handlers: `internal/handler/system.go`, `internal/handler/audit_log.go`.

### POST /api/v1/system/admin/promote

Purpose: grant SystemAdmin. Request body: `user_id` (UUID, takes priority) or `email` (one of the two). Idempotent: promoting an existing system administrator returns 200 without a write.

Response: 200 `UserInfo` (raw object); 404 when the user is not found.

```bash
curl -X POST $BASE/api/v1/system/admin/promote -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"admin@ex.com"}'
```

### POST /api/v1/system/admin/revoke

Purpose: revoke SystemAdmin. Request body: `{"user_id":"..."}` (`binding:"required"`). Revoking yourself or the last remaining system administrator returns 400; revoking a user who is not an administrator returns 200 unchanged.

Response: 200 `UserInfo`

```bash
curl -X POST $BASE/api/v1/system/admin/revoke -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"user_id":"u-1"}'
```

### GET /api/v1/system/admin/list

Purpose: list system administrators. Query parameters: `offset` (default 0), `limit` (default 50, capped at 200).

Response: 200 `{"total":N,"admins":[UserInfo]}`

```bash
curl $BASE/api/v1/system/admin/list -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/users

Purpose: list every user across workspaces. Query parameters: `offset` (default 0), `limit` (default 50, capped at 200). No total count; `has_more` says whether a next page exists.

Response: 200 `{"users":[UserInfo],"offset","limit","has_more"}`

```bash
curl "$BASE/api/v1/system/admin/users?limit=50" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/users/status

Purpose: enable or disable a user account. Disabling revokes the user's active sessions at once. Request body: `user_id` and `is_active` (both `binding:"required"`). Disabling yourself or the last active system administrator returns 400; an unknown user returns 404.

Response: 200 `{"message":"User status updated","user":UserInfo,"is_active":bool}`

```bash
curl -X POST $BASE/api/v1/system/admin/users/status -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"user_id":"u-1","is_active":false}'
```

### POST /api/v1/system/admin/users/create

System administrators only; this route is not open to platform API keys. Request fields: `username` (2-50 characters), `email` (valid address), `password` (optional; omitted or `null` generates one). An explicit empty string still goes through the password policy check and is not treated as auto-generation.

| HTTP status | Response and meaning |
| --- | --- |
| 201 | `{user:UserInfo,generated_password?}`, created; the password is returned only when it was generated |
| 200 | `{user:UserInfo}`, the identity already exists; the account and password are left unchanged |
| 400 | Parameter or password policy violation |
| 409 | The email and the username belong to different identities |

This is a raw response object with no success/data envelope and no idempotent field; the HTTP status tells a new account from an existing one. Workspace assignment follows `auth.default_tenant_mode`.

```bash
curl -i -X POST "$BASE/api/v1/system/admin/users/create" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@example.com"}'
```

### POST /api/v1/system/admin/users/reset-password

Purpose: reset a user's password and revoke all of their sessions. Request body: `email` (`binding:"required,email"`), `new_password` (`binding:"required"`, checked against the password policy). Resetting your own password here returns 400.

Response: 200 `{"message":"Password reset successfully"}`

```bash
curl -X POST $BASE/api/v1/system/admin/users/reset-password -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"email":"a@ex.com","new_password":"newpass1"}'
```

### GET /api/v1/system/admin/api-keys

Purpose: list platform API keys (masked).

Response: 200 `{"success":true,"data":[{id,scope_type,name,api_key,full_access,knowledge_base_ids,capabilities,last_used_at,expires_at,created_at}]}`

```bash
curl $BASE/api/v1/system/admin/api-keys -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/api-keys

Purpose: create a platform API key (the plaintext is returned once). Request body: `name` (non-empty), `capabilities` (list of `system_*` capabilities, required), `expires_at_unix` (optional, must be in the future).

Response: 201 `{"success":true,"data":{...,"api_key":"<masked>","token":"<plaintext>"}}`

```bash
curl -X POST $BASE/api/v1/system/admin/api-keys -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"ops","capabilities":["system_tenants_read"]}'
```

### DELETE /api/v1/system/admin/api-keys/:key_id

Purpose: revoke a platform API key.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/api-keys/3 -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/settings and GET /api/v1/system/admin/settings/:key

Purpose: list platform runtime settings, or read one (platform key: `system_settings_read|manage`). The keys and their effects are in [Runtime settings reference](../03-features/20-platform-admin.md#runtime-settings-reference).

Response: 200 `[SystemSetting]` / `SystemSetting` (raw, no envelope; fields `key,value,value_type,description,last_modified_by,last_modified_at`). The single read returns 400 for a key not in the registry and 404 for a known key that has no stored row yet.

```bash
curl $BASE/api/v1/system/admin/settings -H "Authorization: Bearer $TOKEN"
```

### PUT /api/v1/system/admin/settings/:key

Purpose: update a setting (platform key: `system_settings_manage`). Request body: `{"value":<any JSON, validated against the registry type>}` (required). Most keys take effect on the next read; `ssrf.whitelist`, `model.max_concurrency` and the Docker sandbox switch are pushed to their subsystems at once, and the `asynq.*_concurrency` worker pool sizes need a process restart.

Response: 200 `SystemSetting`; 400 for an unknown key or a type mismatch.

```bash
curl -X PUT $BASE/api/v1/system/admin/settings/tenant.default_storage_quota_gb -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"value":20}'
```

### DELETE /api/v1/system/admin/settings/:key

Purpose: reset a setting to its environment variable or built-in default. Idempotent.

Response: 200 `{"success":true}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/settings/tenant.default_storage_quota_gb -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/runtime/queues

Purpose: asynq queue depths and concurrency state (Lite mode returns `available:false`; platform key: `system_runtime_read|manage`). The dashboard built on it is described in [Async tasks](../02-architecture/05-async-tasks.md#task-inspection-cancellation-and-the-runtime-dashboard-taskinspector).

Response: 200 `{"available",upstream_concurrency,parse_concurrency,wiki_concurrency,pools,queues,model_limiter_available,models,timestamp}`

```bash
curl $BASE/api/v1/system/admin/runtime/queues -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/runtime/queues/:queue/tasks

Purpose: list the tasks of one queue. Query parameters: `state` (`pending/active/scheduled/retry/archived/completed`), `cursor`, `page_size` (default 20, capped at 100). An unknown queue or state returns 400; an invalid cursor returns 400 with `error.details.code` set to `runtime_task_cursor_invalid`, an expired one 409 with `runtime_task_cursor_expired`.

Response: 200 `{"available","tasks":[RuntimeTaskInfo],"page_size","has_more","next_cursor"}`

```bash
curl "$BASE/api/v1/system/admin/runtime/queues/default/tasks?state=pending" -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/runtime/queues/:queue/tasks/:task_id/actions/:action

Purpose: act on a task (`action` is one of `cancel/run_now/delete`; platform key: `system_runtime_manage`). The current task state is re-read first; an action the state does not allow, or a task that is gone, returns 409. Lite mode returns 503.

Response: 200 `{"success":true}`

```bash
curl -X POST $BASE/api/v1/system/admin/runtime/queues/default/tasks/t-1/actions/cancel \
  -H "Authorization: Bearer $TOKEN"
```

### DELETE /api/v1/system/admin/runtime/queues/:queue/archived

Purpose: purge the archived (finally failed) tasks of one queue. Live tasks are untouched.

Response: 200 `{"success":true,"deleted":N}`

```bash
curl -X DELETE $BASE/api/v1/system/admin/runtime/queues/default/archived -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/system/admin/tenants/apply-default-storage-quota

Purpose: write the current default storage quota (`tenant.default_storage_quota_gb`) to every workspace (platform key: `system_tenants_manage`). No request body. Idempotent.

Response: 200 `{"affected":N,"quota_bytes":N,"quota_gb":N}`

```bash
curl -X POST $BASE/api/v1/system/admin/tenants/apply-default-storage-quota -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/system/admin/audit-log

Purpose: platform-level audit log (`tenant_id=0` rows; platform key: `system_audit_read`). Query parameters match the workspace audit log (`after_id/limit/action/outcome/actor`). Handler: `internal/handler/audit_log.go`

Response: 200 `{"success":true,"data":[AuditLog],"next_cursor":N}`

```bash
curl $BASE/api/v1/system/admin/audit-log -H "Authorization: Bearer $TOKEN"
```

## Model catalog (/api/v1/system/admin/model-catalog)

System administrator user sessions only; API keys cannot reach this group.

| Method and path | Effect |
| --- | --- |
| `GET /system/admin/model-catalog` | Returns the current `version`, `baseline`, the administrator `overlay`, the last 20 historical versions, and the `builtin` / `deployment` / `effective` catalogs |
| `POST /system/admin/model-catalog/preview` | Validates the overlay document and returns the candidate catalog (only `effective` and the normalized `overlay`; `history` / `builtin` / `deployment` are `null`); nothing is persisted or published |
| `PUT /system/admin/model-catalog` | Validates, saves a new version and publishes it; the audit row records only the version metadata |

Preview and publish take the same request body:

```json
{
  "version": 0,
  "baseline": "the deployment baseline identifier returned by GET",
  "overlay": {
    "providers": {
      "openai": {
        "models": [{"id": "gpt-5", "context_window": 128000}]
      }
    }
  }
}
```

The response is the unwrapped catalog status object. Each vendor entry also carries `model_thinking_levels` (the levels selectable per chat model id once thinking is on, merged from the vendor mapping and the protocol capabilities) and `vendor_thinking_levels` (the vendor default levels for models with no level of their own). An invalid document returns 400; a stale version or a deployment baseline that differs from the requesting instance returns 409. Publishing persists first and then switches this instance; other instances sync within about 5 seconds. To roll back, publish a historical `overlay` again with the current `version` / `baseline`. Full rules and limits are in [Model management](../03-features/06-models.md#system-administrators-maintain-the-model-catalog).

The request body is limited to 1 MiB; a larger or malformed body returns 400.

## Implementation reference

Route registration: `RegisterSystemRoutes` and `RegisterSystemAdminRoutes` in `internal/router/routes_auth_tenant.go`. Handlers: `internal/handler/system.go`, `internal/handler/system_model_catalog.go`, `internal/handler/sandbox_check.go`, `internal/handler/deployment_capabilities.go`, `internal/handler/audit_log.go`.
