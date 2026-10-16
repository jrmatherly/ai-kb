# Platform administration and system administrators

A system administrator owns the global settings, task queues, platform API keys and cross-workspace audit of the whole AI KB deployment. A workspace Owner owns one workspace. The two identities are granted separately. Workspace roles are described in [Workspaces, users and authentication](01-tenant-auth.md).

The two identities manage different scopes:

| | Workspace Owner | System administrator |
| --- | --- | --- |
| Scope | One workspace | The whole deployment |
| How it is granted | Obtained when creating a workspace, or transferred by the previous Owner | Granted by an existing system administrator; the first one is bootstrapped through an environment variable |
| What it manages | Workspace members, models, knowledge bases, integrations, workspace audit | Global system settings, task queues, platform API keys, cross-workspace audit, user password resets |
| Implied by the other | - | **No**: being an Owner in a workspace does not make you a system administrator, and the reverse is also true |

Cross-workspace data access is controlled separately by `CanAccessAllTenants`. System administrator status does not by itself grant access to the knowledge base content of other workspaces.

<Screenshot
  src="/screenshots/settings-system-admin.png"
  caption="Platform console: System Settings, Model catalog, Task Queues, Platform API Keys and the system audit log"
  hint="Settings opened as a system administrator, showing the five sections at the bottom of the sidebar that only system administrators see, with the System Settings page in the main area." />

## Set up the first system administrator

A new deployment has no system administrator until you bootstrap one:

1. Register an account through the normal flow.
2. Set `AIKB_BOOTSTRAP_SYSTEM_ADMIN_EMAIL=<that account's email>` on the app service and restart it.
3. At startup, and only while the deployment has no system administrator, the user with that email is made a system administrator.

The bootstrap only promotes an existing account. If the email is not registered yet, a warning is logged and the check runs again on the next restart; a misconfigured value never blocks startup. Once the deployment has a system administrator the variable grants nothing more.

Later, system administrators are added or revoked in the UI. You cannot revoke yourself, and you cannot revoke the last system administrator. The endpoints are `POST /system/admin/promote` and `/revoke`; revoking a user who is not an administrator returns 200, and the audit row carries `changed=false` to show that no permission changed.

## Use the platform console

As a system administrator you see these sections in the Settings sidebar:

| Section | Purpose | Endpoints |
| --- | --- | --- |
| System Settings | Global runtime switches (registration mode, workspace policy, concurrency, SSRF allowlist and so on), applied according to each setting's effect rule; see the settings table below | `GET/PUT/DELETE /system/admin/settings[/:key]` |
| Model catalog | View the catalog models and their sources, edit or add models, or change them in bulk as JSON (saved changes take effect at once), and restore from the version history; see [Managing the model catalog](06-models.md#system-administrators-maintain-the-model-catalog) | `/system/admin/model-catalog*` |
| Task Queues | Live backlog of every asynq queue, per-task cancel, run now and delete, and bulk purge of archived tasks; Lite mode returns `available=false` | `/system/admin/runtime/queues*` |
| Platform API Keys | Platform-scoped keys for control-plane automation, with the capabilities `system_tenants_read/manage`, `system_settings_read/manage`, `system_runtime_read/manage` and `system_audit_read` | `/system/admin/api-keys` |
| Audit log | Platform-level events with `tenant_id = 0` (setting changes, administrator promotions and revocations, queue actions and so on). The workspace audit endpoint filters by tenant and never shows these rows | `GET /system/admin/audit-log` |

System administrators can also:

- **Reset a user's password** (`POST /system/admin/users/reset-password`): replaces the target user's local password and revokes all of their sessions. **You cannot reset your own password here**; the self-service change still requires the old password.
- **Apply the default storage quota in bulk** (`POST /system/admin/tenants/apply-default-storage-quota`): writes the current default quota onto every existing workspace. This updates the quota stored on those workspaces.

### Create a user

A system administrator can open a local account from Settings → System Settings → Registration & access with "Create user": enter a username (2-50 characters) and an email, and either keep "Auto-generate a random password" on or turn it off and type a password that meets the password policy. This covers deployments that disable public registration and let administrators open accounts. Workspace assignment follows `auth.default_tenant_mode`: `create_personal` creates a personal workspace, `tenantless` waits for the user to join one.

<Screenshot
  src="/screenshots/system-admin-create-user.png"
  caption="A system administrator creates a user: account details and the one-time password"
  hint="The 'Create user' dialog under Settings → System Settings → Registration & access, showing the username, email and 'Auto-generate a random password' fields; optionally the result view after creation with the one-time password and the 'Copy account details' button." />

A generated password is shown only in this result view, so copy the full account details right away; the dialog cannot be closed until you acknowledge it, and the plaintext cannot be retrieved afterwards. Every creation writes a system audit entry (`system.user_created`). If the identity already exists the existing user is returned and its password is unchanged; if the email and the username point to two different users the request is rejected. Endpoints are in the [System API](../04-api/02-api-system.md).

Creating a user and bootstrapping the first administrator are different operations: the bootstrap still only promotes an existing user and never creates an account.

## Runtime settings reference

The supported runtime settings can be changed in the console. A value in the database takes precedence over the environment variable. Most apply at once; settings that only affect new resources or need a restart are marked in the table.

| Key | Type | Default | Takes effect |
| --- | --- | --- | --- |
| `auth.registration_mode` | `self_serve` / `invite_only` | `self_serve` | At once |
| `auth.default_tenant_mode` | `create_personal` / `tenantless` | `create_personal` | Only for users who register afterwards |
| `tenant.self_service_creation_enabled` | bool | `true` | At once |
| `tenant.max_owned_per_user` | int | `10` (0 = built-in default, negative = no limit) | Read on every workspace creation |
| `tenant.default_storage_quota_gb` | int | `10` | **Read only when a workspace is created**, not written back to existing workspaces |
| `tenant.auto_create_api_key` | bool | `false` | Read on every workspace creation |
| `ssrf.whitelist` | string list | empty | At once (`SSRF_WHITELIST_EXTRA` is still maintained by the deployer and is not overridden here) |
| `asynq.core/postprocess/enrichment/maintenance/shared/wiki_concurrency` | int | see [Async task system](../02-architecture/05-async-tasks.md) | After a restart; the UI shows a "Restart required" badge |
| `model.max_concurrency` | int | `32` | At once |

These switches follow the same database-first rule:

| Key | Default | Purpose |
| --- | --- | --- |
| `auth.complex_password_enabled` | false | New registrations and new passwords must contain upper and lower case letters, digits and special characters |
| `tenant.auto_accept_invitation` | false | Inviting an already registered user by email writes the membership directly |
| `sandbox.docker_enabled` | false | Allows Docker sandbox configuration and instance creation |

`tenant.auto_create_api_key` is off by default. An integration that depends on the old registration response can enable this compatibility switch so that a new workspace automatically gets a full-access key whose plaintext is returned.

::: tip Resetting a runtime setting
After a setting is saved in the console, changing the matching environment variable no longer overrides the database value. When a configuration change seems to have no effect, check the runtime settings first; a reset (`DELETE /system/admin/settings/:key`) removes the database override and the environment variable or built-in default applies again.
:::

## Related pages

- The four workspace roles and API keys: [Workspaces, users and authentication](01-tenant-auth.md)
- Queue topology and worker pools: [Async task system](../02-architecture/05-async-tasks.md)
- Audit log and tracing: [Observability and audit](16-observability.md)
- Endpoint list: [System and platform administration API](../04-api/02-api-system.md)

## Implementation reference

- `cmd/server/bootstrap.go`: first system administrator bootstrap.
- `frontend/src/config/settingsAccess.ts`: platform settings sections.
- `internal/application/service/system_setting.go`: runtime settings registry.
