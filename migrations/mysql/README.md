# migrations/mysql: unwired MySQL schema script

⚠️ **No code or script runs the SQL in this directory. MySQL is not currently a supported primary database for AI KB.**

## Current state

In `initDatabase()` in `internal/container/container.go`, the switch on `DB_DRIVER` has only two branches:

| `DB_DRIVER` | Migration source |
| --- | --- |
| `postgres` | `migrations/versioned/` (golang-migrate incremental migrations) |
| `sqlite` | `migrations/sqlite/` (golang-migrate incremental migrations, Lite mode) |

Any other value returns `unsupported database driver`, so setting `DB_DRIVER=mysql` makes the server fail at startup.

`00-init-db.sql` in this directory is a one-shot schema script covering 10 core tables (`tenants`, `models`, `knowledge_bases`, `knowledges`, `sessions`, `messages`, `message_suggestion_sets`, `message_suggestion_events`, `chunks`, `chunk_revisions`). It gets updated in passing when someone changes these base tables, but **no Go code, Makefile target or compose file references it**.

## What real MySQL support still needs

This schema script alone is not enough. It matches only the initial schema and has no incremental migration set equivalent to `migrations/versioned/` (the Postgres side already has 100+ incremental migrations). Full wiring needs at least:

1. A `case "mysql"` in `initDatabase()`, wired to `gorm.io/driver/mysql` and a golang-migrate MySQL DSN.
2. An incremental migration sequence in `migrations/mysql/`, kept in step with the schema evolution in `versioned/`.
3. MySQL equivalents or fallbacks for Postgres-specific features (`JSONB`, array types, `ON CONFLICT`, ParadeDB BM25 indexes and so on).
4. A decision on vector retrieval. MySQL has no vector index of its own, so it depends on an external vector store (`RETRIEVE_DRIVER`).

See Tencent/WeKnora#1418 ([issue](https://github.com/Tencent/WeKnora/issues/1418)) for the discussion. Until the work above is done, do not claim `DB_DRIVER=mysql` support in docs or config comments.
