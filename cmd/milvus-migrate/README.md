# Milvus multilingual BM25 migration

Older AI KB Milvus collections have a single default text analyzer, so Chinese content may get no useful BM25 keywords. `milvus-migrate` keeps the old collection and copies the existing dense vectors into a new multilingual collection; Milvus regenerates the BM25 sparse vectors from each row's `language` field.

The dense vector metric (IP / COSINE / L2) is read from the source collection's embedding index by default. Do not change it to a value different from the source collection, or the same vectors are ranked by the wrong distance.

## Usage

Make sure Milvus is running, then run from the project root:

```bash
go run ./cmd/milvus-migrate --source aikb_embeddings --target aikb_embeddings_multilingual
```

If the current shell already exports `MILVUS_ADDRESS` and `MILVUS_COLLECTION`, you can omit the matching flags. Variables written only in `.env.local` are not injected into the `go run` process; when unsure, pass the flags explicitly:

```bash
go run ./cmd/milvus-migrate \
  --address 127.0.0.1:19530 \
  --source aikb_embeddings \
  --target aikb_embeddings_multilingual
```

For a Milvus server with authentication or a non-default database, add `--username`, `--password` and `--database` (or `MILVUS_USERNAME`, `MILVUS_PASSWORD` and `MILVUS_DB_NAME`).

To double-check the metric, pass the source collection's `--metric-type` (or the `MILVUS_METRIC_TYPE` environment variable). The value must match the source index, or the migration fails.

After the migration, set AI KB's `MILVUS_COLLECTION` to the target prefix and restart the service. **Keep the existing `MILVUS_METRIC_TYPE` unchanged**:

```dotenv
MILVUS_COLLECTION=aikb_embeddings_multilingual
```

The retrieval list matches `{prefix}_{dimension}` exactly, so `aikb_embeddings` never matches `aikb_embeddings_multilingual_*` by mistake. Only after you switch the prefix do new writes and vector and keyword retrieval all use the new collection.

The migration never deletes the old collection. Once you have confirmed that both Chinese and English BM25 recall work, delete the old collection with a Milvus admin tool; back it up first.

By default the migration reads 64 rows per batch and reads only the fields needed to rebuild the target collection, never the BM25 sparse vectors generated in the old collection. If individual chunks are very long, lower the batch size explicitly, for example with `--batch-size 32`.

## Windows PowerShell notes

AI KB's `internal/utils` parses SQL with `pg_query_go`, which needs CGO. When you run `go run` directly in a session with `CGO_ENABLED=0`, you get `undefined: pg_query.Parse` or `undefined: pg_query.Deparse`.

If you have an MSYS2 GCC (for example in a local, gitignored `.local-tools\msys64\ucrt64\bin` under the project root; the repository does not provide it), you can enable CGO for the current PowerShell session and then run the migration. These settings affect only the current window and do not change the system-wide Go configuration:

```powershell
# Make sure the current directory is the project root that contains go.mod
if (!(Test-Path -LiteralPath '.\go.mod')) { throw 'Change to the AI KB project root first' }

# Point Go at the local MSYS2 GCC
$compilerBin = Join-Path (Get-Location) '.local-tools\msys64\ucrt64\bin'
$env:CGO_ENABLED = '1'
$env:CC = Join-Path $compilerBin 'gcc.exe'
$env:CXX = Join-Path $compilerBin 'g++.exe'
$env:PATH = "$compilerBin;$env:PATH"

# Run the migration; the metric follows the source collection and the old collection is kept
go run ./cmd/milvus-migrate --address 127.0.0.1:19530 --source aikb_embeddings --target aikb_embeddings_multilingual
```

If you have no such folder, install a working GCC first and point `$env:CC` and `$env:CXX` at the absolute paths of its `gcc.exe` and `g++.exe`.
