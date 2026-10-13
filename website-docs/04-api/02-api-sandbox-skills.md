# API reference: sandbox, skills and personal variables

Manage sandbox configs, the skill catalog, install jobs, the session terminal and personal environment variables. Every path is prefixed with `/api/v1`; in the examples `$BASE` is the server address and `$TOKEN` is the current user's Bearer token. UI steps are on [Skill catalog and sandbox](../03-features/22-skills-sandbox.md).

## Permissions

| Resource | Read | Write / probe |
| --- | --- | --- |
| Sandbox config list and detail | Viewer+; an API key must be full-access | Admin+; an API key must be full-access |
| Sandbox inventory, installed skills and their files | Admin+ | Admin+; an API key must be full-access |
| Skill catalog list | Viewer+, JWT | Catalog create, delete, install and bundle file browsing need Admin+; an API key on the write group must be full-access |
| `/skills` usable skills | Viewer+, JWT | Read-only; query parameter `sandbox_config_id`; a shared agent also passes `agent_id` and `agent_source_tenant_id` |
| Session terminal | The session owner, login JWT only | Take a ticket, then open the WebSocket |
| `/me/env-vars*` | The signed-in caller | Modifies only the caller's own values; the server derives the identity; no extra admin gate, Bearer JWT |

An ID from another workspace grants no access. The personal-variable endpoints do not replace the workspace skill management endpoints.

## Sandbox configs

| Method | Path | Request / response |
| --- | --- | --- |
| GET | `/sandbox-configs` | 200 `{success,data:[ConfigResponse],workspace_scripts_disabled}` |
| POST | `/sandbox-configs` | `{name,description?,config}`; 201 `{success,data:ConfigResponse}` |
| GET | `/sandbox-configs/:id` | 200 `{success,data:ConfigResponse}` |
| PUT | `/sandbox-configs/:id` | `{name,description?,config}`, `name` required; 200, same as detail |
| DELETE | `/sandbox-configs/:id` | Optional `force=true`; 200 `{success:true}` |
| GET | `/sandbox-configs/:id/sandboxes` | 200 `{success,data:SandboxInventory}` with `sandbox_count`, `session_ids`, `agent_names` and `unverifiable` |
| PUT | `/sandbox-configs/workspace-policy` | `{"scripts_disabled":true}`; returns `success` and `workspace_scripts_disabled` |
| POST | `/sandbox-configs/templates/query` | `{config,config_id?,ensure_standard?,replace_standard?,ensure_desktop?,replace_desktop?}`; queries the remote templates and can create or replace the standard or desktop template; `replace_*` requires `config_id` |

ConfigResponse is `{id,name,description,sandbox_type,config,created_at,updated_at}` with credentials redacted. An edit is applied on top of the saved config: a redacted placeholder keeps the old credential, and `skill_image` is maintained by the install service, so a client cannot replace it.

### config fields

| Field | Type | Description |
| --- | --- | --- |
| `sandbox_type` | string | `docker`, `cube` or `e2b`; `local` was removed; the Lite desktop `host` backend cannot be saved as a config (400) |
| `default_timeout_sec` | int | Execution timeout; 0 uses the built-in default |
| `terminal_idle_disconnect_sec` | int | How long a terminal or desktop may sit idle before disconnecting; 0 means the default 900 seconds; effective range 60 seconds to 24 hours |
| `desktop_enabled` | bool | Declares that the selected template is a Cube/E2B desktop template; cannot be switched once skills are installed |
| `allow_private_endpoints` | bool | Allows private-network cluster addresses; link-local and cloud-metadata addresses stay blocked |
| `env_vars` | map[string]string | Environment for this sandbox config; values are stored encrypted |
| `skill_rollout` | string | `next_turn` (default) or `new_session` |
| `network` | object | Cube/E2B network policy, see below |
| `cube` / `e2b` / `docker` | object | Connection settings for the matching `sandbox_type` |
| `volume_mount` | object | Optional volume settings; the backend's capability decides whether it takes effect, so the field's presence alone does not mean support |
| `skill_image` | object | Snapshot information maintained by the install service, read-only |

`volume_mount` holds `enabled`, `mount_path`, `provider`, `volume_id` and `volume_name`; `volume_owner_fingerprint` is server-managed. Mounting happens only when the volume settings and the backend's capability both allow it; when editing, keep the ownership information the server returned.

Backend settings:

| Backend | Fields |
| --- | --- |
| cube | `api_url`, `proxy_url`, `sandbox_domain`, `template_id`; `api_key` as the cluster requires; `http_timeout_sec`, `cube_sandbox_ttl_seconds`, `dns_servers` |
| e2b | `api_key` and `template_id` required; `api_url`, `sandbox_domain`, `proxy_url` for self-hosted; `http_timeout_sec`, `e2b_sandbox_ttl_seconds` |
| docker | `image` required; `host` (empty means the local socket), `tls_cert_path` (required over TCP), `cpu_limit`, `memory_limit_mb`, `pids_limit`, `network_mode` (`bridge` or `none`), `runtime`, `idle_ttl_seconds`, `http_timeout_sec` |

```bash
curl -X POST "$BASE/api/v1/sandbox-configs" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"E2B workspace","config":{"sandbox_type":"e2b","e2b":{"api_key":"<e2b-key>","template_id":"<template-id>"}}}'

curl "$BASE/api/v1/sandbox-configs/cfg-1/sandboxes" \
  -H "Authorization: Bearer $TOKEN"
```

Changing the backend identity and deleting a config both check the remote instances. A 409 carries `error.details.code` such as `sandboxes_still_live`, `sandbox_inventory_unverifiable` or `skill_snapshot_release_failed`; 423 means another request is modifying the config. `force=true` only allows deletion when the inventory cannot be verified; it never skips confirmed live or paused instances. The occupancy data in `error.details.data` points to the related sessions and agents.

### Network policy

`network`:

| Field | Description |
| --- | --- |
| `deny_egress_by_default` | `false` allows egress by default; `true` denies it |
| `allow_out` | IPv4 addresses, CIDRs, domain names or single-label wildcard domains; domains apply in deny-by-default mode |
| `deny_out` | IPv4 and CIDR deny list |
| `cube_rules` | Cube rules with `name`, `scheme`, `sni`, `host`, `methods`, `path`, `deny`, `audit` and `inject`; order matters |
| `e2b_host_rules` | E2B rules with `host` and `headers`; the host must also be in `allow_out` |
| `allow_public_inbound` | Accepted for compatibility and cleared on save; inbound always requires credentials |

`cube_rules[].inject` is `[{header,secret,format}]` and `e2b_host_rules[].headers` maps header to secret; secret fields are stored encrypted and returned redacted. Docker uses `docker.network_mode` and does not support these L7 rules.

```json
{
  "deny_egress_by_default": true,
  "allow_out": ["pypi.org", "files.pythonhosted.org"],
  "deny_out": []
}
```

### POST /system/sandbox-check

Request `{config,config_id?,deep?}`; `config_id` restores the saved, redacted credentials. `deep=false` runs connection checks only; `true` also runs a temporary script, and a remote backend really creates and destroys a sandbox, which may incur backend usage. Admin+.

A completed probe returns `{success:true,data:{ok,provider,checks,capabilities}}`. A failed probe can still be HTTP 200, so check `data.ok`; a parameter error returns 400 with `code` and `msg`. Each check carries `name`, `ok`, `message`, `reason` and `latency_ms`; `ok=null` means skipped, with `reason` as a stable code (`needs_deep_check`, `control_plane_unreachable`, `sandbox_not_created`, `sandbox_exec_failed`, `egress_restricted_by_policy`). For example, with deny-by-default egress the external probe is skipped by policy (`egress_restricted_by_policy`), which is not the same as verified external access.

```bash
curl -X POST "$BASE/api/v1/system/sandbox-check" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"config_id":"cfg-1","deep":true}'
```

## Skill catalog

| Method | Path | Request / response |
| --- | --- | --- |
| GET | `/skills/catalog` | 200 `{success,data:[CatalogItem]}`; CatalogItem carries `installations:[{skill_id,sandbox_config_id,sandbox_config_name,sandbox_type,status,enabled,error?,version,bundle_sha256,served?,updated_at}]` |
| POST | `/skills/catalog` | multipart `file` or JSON `{"source":"@owner/slug"}`; 201 `{success,data:{id,name,version,description}}` |
| POST | `/skills/catalog/:id/install` | `{"sandbox_config_ids":["cfg-1","cfg-2"]}`; 202 `{success,data:{installs,errors?}}` |
| GET | `/skills/catalog/:id/files` | 200 `{success,data:[FileEntry]}` |
| GET | `/skills/catalog/:id/files/content?path=SKILL.md` | 200 `{success,data:FileContent}` |
| DELETE | `/skills/catalog/:id` | Deletes when no sandbox still has it installed; 200 `{success:true}`, otherwise 409 |

Installing onto several sandboxes can be partially accepted: check `data.installs` and `data.errors`; HTTP 202 does not mean every install completed, and `success` is `false` when `errors` is present. Deleting from the catalog does not implicitly uninstall from any sandbox.

```bash
curl -X POST "$BASE/api/v1/skills/catalog" \
  -H "Authorization: Bearer $TOKEN" -F 'file=@skill.zip'
curl -X POST "$BASE/api/v1/skills/catalog/catalog-1/install" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"sandbox_config_ids":["cfg-1"]}'
```

Source formats, anonymous download requirements and ZIP limits are in [Skill sources](../03-features/22-skills-sandbox.md#supported-sources).

## Skills inside a sandbox

The paths below are prefixed with `/sandbox-configs/:id/skills`; every operation, reads included, needs Admin+.

| Method | Suffix | Behavior |
| --- | --- | --- |
| GET | none | `{success,data:[SkillResponse]}` |
| POST | none | Install from a ZIP `file` or a `source` JSON body; 202 `{success,data:{skill_id}}` |
| GET | `/:skillId` | `{success,data:SkillResponse}` |
| POST | `/:skillId/reinstall` | Reinstalls from the stored bundle; optional `{"instructions":"..."}` (at most 10000 characters) as install guidance; 202 `{success,data:{skill_id}}` |
| POST | `/:skillId/stop` | Stops an install; 200 with the skill state; a skill already `failed` can be called again, any other non-`installing` status returns 400 |
| GET | `/:skillId/guidance` | Guidance for the current install run: `{success,data:{accepting,messages:[{id,content,status}]}}` |
| POST | `/:skillId/guidance` | Appends guidance while the install runs: `{expected_message_id,steer_id,content}`; `expected_message_id` is the SkillResponse `install_message_id`, `steer_id` is a client-generated UUID (a retry does not append twice), `content` is 1 to 10000 characters; 202 `{success:true}`, or 409 when the install run has changed |
| PATCH | `/:skillId` | `enabled?`, `envs?`; an omitted field is left unchanged |
| DELETE | `/:skillId` | Uninstalls from this sandbox and keeps the catalog bundle |
| GET | `/:skillId/files` | File listing |
| GET | `/:skillId/files/content?path=SKILL.md` | Reads a file by relative path; traversal is rejected |
| GET | `/:skillId/install-events` | SSE install progress |
| GET | `/:skillId/transcript` | SSE install process events |

SkillResponse has `id`, `name`, `version`, `description`, `enabled`, `status`, `error`, `bundle_sha256`, `installed_snapshot_id`, `install_session_id`, `install_message_id`, `created_at`, `updated_at` and `envs:[{name,description,required,is_set}]`. While an upgrade or reinstall is in flight, or after it failed, `served:{version}` names the previous ready version the sandbox still serves; that is the version agents can use. A file response can be UTF-8 text, base64 for a small image, or binary metadata; do not assume every file has text content.

PATCH `envs` updates declared variables only; an undeclared name is ignored, and an empty string clears the value and keeps the declaration.

```bash
curl -X PATCH "$BASE/api/v1/sandbox-configs/cfg-1/skills/skill-1" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"enabled":true,"envs":{"API_TOKEN":"<workspace-token>"}}'

curl -N "$BASE/api/v1/sandbox-configs/cfg-1/skills/skill-1/install-events" \
  -H "Authorization: Bearer $TOKEN"
```

The progress payload is `{percent,stage,log?,status?,done}`. Disconnecting does not cancel the install; `done` may only mean the stream can no longer follow progress, so the skill `status` is the final word. Without Redis the stream tells you to poll the status instead. A 204 from `transcript` means the install has just started and the event cursor is not ready yet; a 404 can mean there is no log or the log has expired, so it does not prove the install failed.

`GET /skills?sandbox_config_id=cfg-1` returns the skill metadata usable by agents, `{success,data:[{name,description}],skills_available}`, which differs from the full install records an administrator sees.

When `agent_id` and `agent_source_tenant_id` (the source workspace ID of a shared agent) are both passed, it lists the skills that shared agent can @-mention: the sandbox config comes from the agent itself and `sandbox_config_id` in the query is ignored; with the agent's skill scope set to `selected` only the chosen skills are returned, and when skills are disabled the list is empty with `skills_available:false`. A caller without access to that agent gets 403.

## Session terminal

The terminal in the conversation sidebar connects over WebSocket to this session's sandbox; the Docker, Cube and E2B backends support it, the Lite host sandbox does not (`TERMINAL_UNSUPPORTED`). The routes are registered in `internal/router/routes_chat.go` and are not yet in Swagger. The desktop endpoints are on [Chat API](02-api-chat.md#sandbox-desktop); proxy requirements are on [Sandbox deployment](../06-development/04-sandbox-deployment.md#interactive-terminal-and-graphical-desktop).

### POST /api/v1/sessions/:session_id/sandbox/terminal-ticket

Issues a handshake ticket valid for two minutes. Requires the session owner's login Bearer access token; an API key cannot call it.

```bash
curl -X POST "$BASE/api/v1/sessions/$SESSION_ID/sandbox/terminal-ticket" \
  -H "Authorization: Bearer $TOKEN"
```

Response: 200 `{"success":true,"data":{"ticket":"<ticket>","expires_in":120}}`.

### GET /api/v1/sessions/:id/sandbox/terminal

WebSocket handshake, outside the normal auth middleware:

| Query parameter | Required | Description |
| --- | --- | --- |
| `ticket` | yes | The ticket from the previous step, bound to the user, workspace, session and the access token that issued it |
| `provision` | no | `1` allows creating or waking the sandbox (may incur cost); when omitted, only connects to a running sandbox |
| `agent_id` / `agent_source_tenant_id` | no | Used with `provision=1` to pick which agent's sandbox config (including a shared agent's) to create from |
| `cols` / `rows` | no | Initial terminal size |
| `pty_id` | no | Reattaches to the shell process returned in an earlier `ready` frame |

Once connected, binary frames carry terminal input and output in both directions; text frames are JSON control messages. The server sends `ready` (with `pty_id`, `backend` and `reattachable`), `exited` (with `exit_code`) and `error`; the client may send `{"type":"resize","cols":..,"rows":..}`. When `reattachable` is `false` (Docker) there is no `pty_id`, a disconnect ends the shell, and the client should not reconnect automatically. Error codes are `SANDBOX_NOT_BOUND`, `SANDBOX_PAUSED`, `SANDBOX_STOPPED` (on the Docker backend, which never pauses: the container has stopped or was reclaimed as idle), `TERMINAL_UNSUPPORTED`, `IDLE_DISCONNECTED`, `AUTH_REVOKED` and `INTERNAL`. Each session allows at most 5 concurrent terminals; beyond that the handshake returns 429.

## Lite host project directory

Available only on the AI KB Lite desktop; other deployments return 404. It opens the system directory picker on the machine running Lite, and the chosen directory is added to the approved list. Pass the returned `dir` as `project_dir` when creating a session so that session can use the directory in the [host sandbox](../03-features/22-skills-sandbox.md#lite-host). The `project_dir` field is on [Chat API](02-api-chat.md).

| Method | Path | Request / response |
| --- | --- | --- |
| POST | `/system/host-project-dir` | No body; 200 `{"code":0,"msg":"success","data":{"dir":"/Users/me/project"}}`; `dir` is empty when the picker is cancelled. Login JWT only, Viewer+; an API key is rejected |

## Personal environment variables

The caller's identity comes from the current auth context; no `user_id` is accepted. The personal endpoints return variable names, their source and update time, never plaintext values.

| Method | Path | Request |
| --- | --- | --- |
| GET | `/me/env-vars` | Returns config-wide variables and skill variables grouped by `sandbox_config_id` |
| PUT | `/me/env-vars/skill` | `{skill_id,name,value}`; the name must be one the skill declared |
| DELETE | `/me/env-vars/skill` | `{skill_id,name}` |
| PUT | `/me/env-vars/sandbox` | `{sandbox_config_id,name,value}` |
| DELETE | `/me/env-vars/sandbox` | `{sandbox_config_id,name}` |

The read response is `{success,data:[{sandbox_config_id,sandbox_config_name,description,vars,skills}]}`; `source` is `user`, `workspace` or `unset`. A successful set or delete returns `success`; deleting an unset entry returns 404. A name must be UPPER_SNAKE_CASE of at most 128 characters, and at most 50 variables are kept per scope. Personal sandbox variables reject the `AIKB_SKILL_` prefix, the names the sandbox injects, and the reserved names `PATH`, `HOME`, `USER`, `SHELL`, `LD_PRELOAD`, `LD_LIBRARY_PATH`, `PYTHONPATH`, `PYTHONHOME` and `NODE_OPTIONS`; a skill-declared variable may use the `AIKB_*` credential names it needs (`AIKB_API_KEY`, `AIKB_BASE_URL`, `AIKB_HOST`).

```bash
curl -X PUT "$BASE/api/v1/me/env-vars/skill" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"skill_id":"skill-1","name":"API_TOKEN","value":"<my-token>"}'

curl -X DELETE "$BASE/api/v1/me/env-vars/skill" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"skill_id":"skill-1","name":"API_TOKEN"}'
```

The override order is personal skill value > personal sandbox value > workspace skill value. Writing a personal value never changes the administrator's configuration. Implementation references: `internal/router/routes_infra.go`, `routes_agent.go`, `routes_auth_tenant.go`, `routes_chat.go`, and `internal/handler/sandbox_config.go`, `sandbox_check.go`, `sandbox_skill.go`, `skill_catalog.go`, `skill_handler.go`, `me_env_var.go`, `host_project_picker.go`, `session/sandbox_terminal_ws.go`.
