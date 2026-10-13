---
paths:
  - "migrations/**"
  - "internal/types/**"
  - "internal/application/repository/**"
---

# Migrations

Two migration tracks, numbered independently (`website-docs/06-development/02-database-schema.md`):

- `migrations/versioned/`: PostgreSQL and ParadeDB, golang-migrate pairs `NNNNNN_name.up.sql` / `.down.sql`. `make migrate-create name=xxx` creates a pair.
- `migrations/sqlite/`: Lite mode. `000000_init` is a flattened full schema; later files follow their own numbering (`sqlite/000032_chunk_images` is `versioned/000113_chunk_images`).
- `migrations/mysql/` and `migrations/paradedb/` hold init scripts only.

A schema change that Lite also uses needs a migration in both tracks, written in each dialect, plus a `.down.sql`. Update the schema page's history table in the same commit. Database, table and index names keep upstream's `weknora_*` spelling (`scripts/rebrand/unchanged.md`).
