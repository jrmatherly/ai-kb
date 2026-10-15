# FAQ and upgrade troubleshooting

Start in Settings → System Information. It shows the application version, the database migration state and the connection status of each dependency. Then read the logs of the service involved. The Compose commands below run from the repository root. For local development the dependency services run under `docker compose -f docker-compose.dev.yml`; read the app log in the host terminal or its log file, since the development Compose file has no `app` service.

```bash
docker compose ps
docker compose logs --tail=200 app
docker compose logs --tail=200 docreader
docker compose logs --tail=200 postgres redis
```

After editing `.env`, recreate the affected container with `docker compose up -d <service>`. `docker compose restart` does not reload container environment variables.

## Locate by symptom

| Symptom | Where to look, in order |
| --- | --- |
| Upload fails, or parsing stays in processing | The failure reason and parsing trace in the document details → parsing engine connectivity → the background task queue; see [Document parsing](../03-features/03-document-parsing.md) and [Async tasks](../02-architecture/05-async-tasks.md) |
| Images display on this machine but not on other devices | Check whether the returned address contains `localhost`, a container service name or an unreachable object storage address. A third-party client that receives `resource://` resolves it as described in [File access](../03-features/21-file-access.md); it is not a usable image URL |
| A saved setting reverts to the old value | Check whether a proxy or browser cache rewrites the response, whether you are connected to a different environment, or whether you switched workspaces. For built-in models also check the YAML sync at startup; see [Model management](../03-features/06-models.md) |
| Permission denied after an upgrade | Check the current workspace role, who owns the resource and the capability scope of the API key; see [Workspaces and permissions](../03-features/01-tenant-auth.md). A platform administrator and a workspace Owner are different concepts |
| An agent reports that its model is not ready | Confirm that the model the agent references still exists and is fully configured, then run the model connectivity test; see [Model management](../03-features/06-models.md) |
| Connections to a private-network data source, model or vector store are refused | Check the SSRF validation and port policy; allow only the targets you need, as described in [Configuration](04-configuration.md) |
| The error says only allowlisted egress is permitted | The deployment has `SSRF_DNS_WHITELIST_ONLY` enabled, so a host outside the allowlist is rejected before DNS resolution. Add the target host to `SSRF_WHITELIST` or `SSRF_WHITELIST_EXTRA`, then recreate the app and docreader containers; see [Configuration](04-configuration.md) |
| The login endpoint returns an HTML 404 | Check whether `SEARXNG_PORT` equals `APP_PORT` (default 8080). On Linux, once SearXNG binds `127.0.0.1:8080`, requests to `localhost:8080` reach SearXNG first. Keep SearXNG on 8888 or another free port |
| Pulling the MinIO image fails with `pull access denied` or `401 UNAUTHORIZED` | MinIO has closed anonymous pulls from Docker Hub and quay.io. The current Compose and Helm files use the community build `pgsty/minio`; a custom orchestration file needs the same image change |
| The server exits at startup with `[startup-env] refusing to start` naming `JWT_SECRET` or `SYSTEM_AES_KEY` | `JWT_SECRET` is empty, or `SYSTEM_AES_KEY` is empty or not exactly 32 bytes. Set them in `.env` (`openssl rand -hex 32` for `JWT_SECRET`, `openssl rand -hex 16` for `SYSTEM_AES_KEY`). When upgrading, keep the `SYSTEM_AES_KEY` that encrypted your stored credentials. `.env.example` ships working defaults, and the server warns about each one until you replace it |
| Embed pages or signed links do not work on a deployment that keeps the shipped defaults | The shipped default `SYSTEM_AES_KEY` is public, so it is never used to sign. Set `SYSTEM_SIGNING_KEY` (`openssl rand -hex 32`) or your own `SYSTEM_AES_KEY` |
| Embed pages or signed links stop working after an upgrade (the embed page reports `embed session signing key is not configured`, the startup log has `[startup-env] no usable signing key`) | The signing key is `SYSTEM_SIGNING_KEY`; when it is unset the server falls back to `SYSTEM_AES_KEY`. Example keys and keys shorter than 16 characters cannot sign. Generate one with `openssl rand -hex 32` and set `SYSTEM_SIGNING_KEY`. **Do not change `SYSTEM_AES_KEY` for this**, or stored credentials can no longer be decrypted. v0.8.2 changed how embed sessions are signed (the HMAC is now keyed by the signing key instead of the channel's publish token), so every embed session issued before the upgrade is invalid and visitors must re-enter. Multi-replica deployments must share one key |
| The DingTalk bot stops replying after an upgrade | From v0.8.2 DingTalk supports Stream mode only; the upgrade migration changes `webhook` channels to `websocket`. Enable Stream mode for the app in the DingTalk developer console; see [IM integration](../03-features/12-im-integration.md) |
| The background queue keeps growing | Combine the wait time of the oldest task, the active workers and the downstream quota. More workers do not add model vendor quota; see [Capacity planning](../02-architecture/05-async-tasks.md#capacity-planning) |
| A skill is in the catalog but cannot run | Adding to the catalog and installing into a sandbox are two steps. Check the installation record, the agent's sandbox choice and the skill scope; see [Skills and sandboxes](../03-features/22-skills-sandbox.md) |
| The Local sandbox is gone after an upgrade | The `local` backend was removed. Configure Docker, CubeSandbox or E2B instead; see [Sandbox deployment and troubleshooting](../06-development/04-sandbox-deployment.md) |
| The Local Browser is paired but runs no web tasks | Local Browser must be enabled in the Smart Reasoning input box, and a paused task must be resumed explicitly; see [Local Browser](../05-clients/09-local-browser.md) |
| An embed page fails to load or returns 403 | Put the real host Origin in the allowlist; check the CSP, the reverse proxy and the Origin used by the secure-mode exchange; see [Embed channel](../03-features/13-embed-channel.md) |
| The Feishu app test passes but no folders load | The connection test only verifies the app identity; folders still need authorization; see [Feishu Drive](../03-features/24-feishu-drive.md) |

## Database migration failures {#database-migrations}

The application runs migrations at startup by default. After a failure it may still start, so "the page opens" does not mean the schema upgrade succeeded. The migration version, dirty flag and error in System Information, together with the app startup log, are the entry points.

**Do not assume a failed migration was fully rolled back.** The real state depends on the transaction boundaries of the SQL and where it failed. Before recovering, keep the logs, back up the database, and compare the migration file for the failed version with the actual schema. Do not skip the error by deleting the data volume or editing the version number by hand.

PostgreSQL uses `migrations/versioned/`, SQLite uses `migrations/sqlite/`. The Make targets below call the PostgreSQL migration script. The SQLite database of Lite must be handled with the matching driver and migration directory; a PostgreSQL DSN does not apply.

```bash
make migrate-version
```

A read-only check in the target database also works:

```sql
SELECT version, dirty FROM schema_migrations;
-- The next two statements are PostgreSQL only
SELECT version();
SELECT extname, extversion FROM pg_extension;
```

### Missing extensions or insufficient privileges

A missing `gin_trgm_ops` usually means `pg_trgm`, a missing `vector` type means pgvector, and BM25 depends on ParadeDB's `pg_search`. First confirm which database the application actually connects to, then have the database administrator check the extension packages, the extensions inside the database and object ownership.

```sql
SELECT name, default_version, installed_version
FROM pg_available_extensions
WHERE name IN ('pg_trgm', 'vector', 'pg_search');
```

Extension files that are not installed and insufficient SQL privileges are different problems. `CREATE EXTENSION IF NOT EXISTS` neither installs operating system packages nor upgrades an extension that already exists. Create only the extensions this deployment needs, so that a deployment with an external retrieval engine is not accidentally turned into PostgreSQL retrieval.

### Dirty state

`AUTO_RECOVER_DIRTY` is on by default. At startup the application tries to reset the migration version and rerun. This is a retry mechanism; it cannot fix a missing extension, a full disk or a schema drift caused by manual edits.

For manual recovery, first stop application writes and check whether the failed migration left partial changes behind. Use `force` only after confirming that the database matches the last successful version and that the failed migration can safely run again. It **only changes the version marker and runs no rollback SQL**. For example, after confirming that the last successful version is 98:

```bash
make migrate-force version=98
make migrate-up
```

The 98 here is an example. Replace it with the version you confirmed; do not mechanically subtract one from the number in the error. A failed initial migration also needs a separate check of the initialization state. After recovery, restart app, confirm that System Information no longer shows an error, and verify the affected features.

### Interrupted concurrent index build

Migration `000106` builds an index on `messages` with `CREATE INDEX CONCURRENTLY`, which does not block writes while it runs. If the build is interrupted (process restart, timeout, full disk) it leaves an INVALID `idx_messages_session_created_id` behind, and `IF NOT EXISTS` will not rebuild it. Confirm and drop that index first, then recover the migration state as described above and rerun:

```sql
SELECT indexrelid::regclass, indisvalid FROM pg_index
WHERE indexrelid = 'idx_messages_session_created_id'::regclass;
DROP INDEX CONCURRENTLY IF EXISTS idx_messages_session_created_id;
```

### Full disk and schema drift

Index builds need extra temporary space. On `No space left on device`, check the capacity of the database data volume and the temporary directory, free space, then recover according to the actual migration state.

When a column type, index or constraint does not match what the migration expects, compare against the failed file and the last successful version. Do not run the failing SQL repeatedly against the production database. When you need to reproduce a write, verify it first in an isolated database restored from a backup.

## What to include in an issue

Provide the application version or commit, the time of the failure, the complete error, the database type and version, the migration version and dirty state, and any relevant non-default configuration. Redact tokens, passwords, connection strings and document contents from logs first. The migration implementation and development workflow: [Database and migrations](../06-development/02-database-schema.md).
