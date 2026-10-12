# Upgrading an existing ParadeDB database

The production and development Compose files use `paradedb/paradedb:v0.25.11-pg17`. This page covers the upgrade from **0.22.x to 0.25.11 with PostgreSQL staying on major version 17**. It does not cover a PostgreSQL major version upgrade, and it does not mean that other older versions used in Helm can follow the same steps.

Compared with `v0.22.6-pg17`, this release line brings PostgreSQL 17.9 → 17.11, pgvector 0.8.1 → 0.8.4 (fixes a buffer overflow in parallel HNSW builds and index corruption during HNSW vacuum) and WAL-based crash recovery for pg_search (since 0.24.0, see paradedb#4901).

> [!WARNING]
> After replacing the image you **must** upgrade the extension catalog to the version that ships with the image. While the catalog stays on the old version, some extension SQL functions (for example `paradedb.version_info()`) segfault because their signatures no longer match the new binary, which crashes the whole PostgreSQL instance and sends it into recovery. Also do **not** run `ALTER EXTENSION pg_search UPDATE TO '0.22.6'` on the new image; it causes the same mismatch.

## Upgrade steps

Run every command from the repository root. In the development environment, database commands use `docker compose -f docker-compose.dev.yml`, and the development backend is stopped by hand on the host; the development Compose file has no `app` service. Keep the original data volume. **Do not run `down -v`, delete the volume or switch to a PG18 image.**

1. Stop app and every other database writer. If Langfuse is enabled, stop its web and worker services too, and stop a locally running development backend as well.

   ```bash
   # Standard deployment; in the development environment stop the backend process on the host
   docker compose stop app
   # Only when Langfuse is enabled
   docker compose stop langfuse-web langfuse-worker
   ```

2. Back up all databases and roles, including the Langfuse database, and confirm that the backup restores. Store the backup outside the database data volume.

   ```bash
   umask 077
   docker compose exec -T postgres sh -c 'pg_dumpall -U "$POSTGRES_USER"' > paradedb-before-upgrade.sql
   ```

   If the old image cannot start on the current CPU, first save a snapshot of the stopped data volume, then make a logical backup on a compatible machine or verify that the snapshot restores, and only then touch the single copy of the data.

3. With the updated Compose file, replace only the database container:

   ```bash
   docker compose pull postgres
   docker compose up -d --no-deps --wait postgres
   ```

4. Complete the SQL upgrade of the extensions. Replacing the image alone does not update the `pg_extension` version inside an existing database. The new image ships the full upgrade script chain from 0.22.x, so any 0.22.x upgrades in one step to the image's default version:

   ```bash
   docker compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1' <<'SQL'
   ALTER EXTENSION pg_search UPDATE;
   ALTER EXTENSION vector UPDATE;
   SELECT extname, extversion FROM pg_extension WHERE extname IN ('pg_search', 'vector');
   SELECT * FROM paradedb.version_info();
   SQL
   ```

   Run the same statements in **every other database that has `pg_search` or `vector` installed**, including the `postgres` database, template databases or the Langfuse database if the extension was ever installed there. Do not install the extension in a database that never needed it just to upgrade it. The `pg_search` catalog and `paradedb.version_info()` should both report 0.25.11, and `vector` should report 0.8.4.

5. Bring the writer services back, confirm the database migration status, and run representative keyword and vector searches. Existing BM25 and HNSW indexes keep working for reads and writes under the new version; there is no need to rebuild every index or re-import documents. If something looks wrong, `REINDEX`. Keep the backup until verification is complete.

## What the automatic migrations cover

The migrations that run when AI KB starts upgrade the extensions only inside the AI KB database. They honor `app.skip_embedding`, do not install missing extensions, and never downgrade or touch other release lines:

- `000115`: when the installed `pg_search` is 0.22.0 through 0.25.10 and the server offers 0.25.11, upgrade to 0.25.11; when the installed `vector` is 0.8.0 through 0.8.3 and the server offers 0.8.4, upgrade to 0.8.4.
- `000099`: a historical migration. It upgrades 0.22.2 through 0.22.5 to 0.22.6 only when the server offers the 0.22.6 extension package (that is, when the `v0.22.6-pg17` image is still in use). On the new image it is skipped and leaves the work to `000115`.

If these migrations already ran **before** the database image was replaced, they do not run again after the new image is installed; run the SQL above by hand. Older application builds without the matching migration also need the manual upgrade. Extensions in other databases cannot be delegated to the AI KB migrations.

## Rollback and reproducible verification

The only rollback is restoring the pre-upgrade backup or snapshot into a separate data volume and running it with the old image. Switching the image tag back does not undo the extension SQL changes, and from pg_search 0.24 on, the extension writes custom resource manager records into the WAL that the old image cannot replay. The migrations' down files do not attempt to downgrade the extensions either.

The repository ships an isolated verification script. By default it verifies 0.22.2 → 0.25.11; `--to 0.22.6` verifies the historical patch upgrade:

```bash
docker pull paradedb/paradedb:v0.22.2-pg17
docker pull paradedb/paradedb:v0.25.11-pg17
python3 scripts/test_paradedb_upgrade.py
```

The script uses temporary containers and volumes and maps no ports. It verifies the upgrade on the same data volume, table contents, keyword and vector retrieval, migration idempotence and the results after a restart, and it writes out a backup and logs. The test runs against fixed fixture data; a real deployment still needs to check its own data and retrieval workload.
