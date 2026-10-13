# Development guide

The development environment starts the dependency services from `docker-compose.dev.yml` and runs the Go backend, the web frontend and docreader on their own. After a change, build and test the module you touched, then check the integration paths that depend on it.

## Tech stack and requirements

AI KB is made of three processes that are developed independently, plus a CLI:

| Component | Directory | Language / runtime | Required version (source) |
| --- | --- | --- | --- |
| Main backend `app` | `cmd/server` + `internal/` | Go | **Go 1.26.0** (`go 1.26.0` in `go.mod`). Needs CGO (DuckDB and sqlite-vec bindings) |
| Document parsing service `docreader` | `docreader/` | Python + gRPC | **Python >= 3.10.18** (`requires-python` in `docreader/pyproject.toml`). Dependencies are managed with **uv** (the repository has `uv.lock`; the Docker image runs `uv sync --locked`) |
| Frontend `frontend` | `frontend/` | Node.js + Vue 3 | Node 22 line (`devDependencies` include `@tsconfig/node22` and `@types/node ^22`). Vite 7 + TypeScript ~6.0 + Vue 3.5 + TDesign, package version `0.8.2` |
| CLI | `cli/` (separate Go module) | Go | Go 1.26 (`go 1.26.0` in `cli/go.mod`) |

Recommended extra development tools:

```bash
# Database migration CLI (required by scripts/migrate.sh)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Linter (called by make lint)
# Install from https://golangci-lint.run; the configuration is .golangci.yml in the repository root

# Swagger doc generation (called by make docs)
make install-swagger    # go install github.com/swaggo/swag/cmd/swag@latest

# Python dependency management
pip install uv          # docreader installs its dependencies with uv sync

# Hot reload for make dev-app (optional; without it dev-app falls back to go run)
go install github.com/air-verse/air@latest

# Docker + Docker Compose (the v2 plugin or standalone docker-compose; scripts/dev.sh detects either)
```

## Quick start: development mode (recommended)

Development mode runs the infrastructure in Docker and runs `app` and `frontend` locally. After changing application code you restart the process; the dependency images are not rebuilt. The entry point is `scripts/dev.sh`, and the Makefile `dev-*` targets wrap it.

```bash
# 1. Prepare environment variables: dev.sh loads .env (required), then overrides it with .env.local (optional)
cp .env.example .env

# 2. Start the infrastructure (ParadeDB/Postgres + Redis + docreader, plus Langfuse by default)
make dev-start                      # same as ./scripts/dev.sh start
make dev-start DEV_ARGS=--qdrant    # add an optional profile

# 3. In another terminal: run the backend locally (Air hot reload when installed, otherwise go run with version ldflags)
make dev-app                        # same as ./scripts/dev.sh app

# 4. In a third terminal: run the frontend locally (npm install if node_modules is missing, then npm run dev)
make dev-frontend                   # same as ./scripts/dev.sh frontend

# Other
make dev-status   # container status
make dev-logs     # follow logs
make dev-stop     # stop
make dev-restart  # stop, then start again (takes DEV_ARGS like dev-start)
```

`dev-app` points the backend at the containers on the host loopback address: it exports `DB_HOST=127.0.0.1`, `DOCREADER_ADDR=127.0.0.1:50051`, `MINIO_ENDPOINT=127.0.0.1:9000`, `REDIS_ADDR=127.0.0.1:6379`, `MILVUS_ADDRESS=127.0.0.1:19530`, `NEO4J_URI=bolt://127.0.0.1:7687` and `QDRANT_HOST=127.0.0.1`, overriding the container service names in `.env`. It also replaces the container path `LOCAL_STORAGE_BASE_DIR=/data/files` with the repository-local `.local-data/files`, and adds the `anydoc` build tag when `make anydoc-lib` has built the static library for the host platform.

The frontend dev server listens on `5173` (`server.port: 5173` in `frontend/vite.config.ts`) and proxies `/api`, `/files` and `/mcp/` to the backend. The proxy target is the first non-empty value of the process environment variables `VITE_DEV_PROXY_TARGET` and `FRONTEND_BACKEND_URL`; when neither is set, it is `http://localhost:8080`. `DEV_PROXY_TARGET` is an internal constant in the config file, not an environment variable.

To use a remote backend, set the target in `.env.local` in the repository root (replace the example address with the real backend):

```bash
VITE_DEV_PROXY_TARGET=https://kb.example.com
```

Then run `make dev-frontend` from the repository root. The script loads the root `.env`, overrides it with `.env.local` and exports the variables to Vite. Restart the frontend dev server after changing the proxy target.

When you run npm directly inside `frontend/`, pass the variable explicitly; that path does not go through the script that loads the root environment files:

```bash
cd frontend
VITE_DEV_PROXY_TARGET=https://kb.example.com npm run dev
```

`vite preview` (port `4173`) serves the production build and is the closest match to the release image. It uses the same proxy paths and target rules. Run `npm run build` in `frontend/`, then `VITE_DEV_PROXY_TARGET=https://kb.example.com npm run preview` to connect to a remote backend.

### docker-compose.dev.yml services

`docker-compose.dev.yml` contains only the dependency services, not `app` or `frontend`. The default services and the optional profile services:

| Service | Image | Ports (default) | Started when |
| --- | --- | --- | --- |
| `postgres` | `paradedb/paradedb:v0.25.11-pg17` (bundles pg_search/BM25) | `5432` | Always |
| `redis` | `redis:7.0-alpine` (`--requirepass`) | `6379` | Always |
| `docreader` | Built locally from `docker/Dockerfile.docreader` | `50051` (gRPC) | Always |
| `searxng` (+`searxng-init`) | `searxng/searxng:latest` | `127.0.0.1:8888` | `--full` (compose profile `searxng` or `full`; `dev.sh` has no `--searxng` flag) |
| `minio` | `pgsty/minio:latest` | `9000` / console `9001` | `--minio` / `--full` |
| `qdrant` | `qdrant/qdrant:v1.16.2` | `6333` / `6334` | `--qdrant` / `--full` |
| `opensearch` | `opensearchproject/opensearch:3.3.2` (security plugin off, plain HTTP) | `9200` | Profile `opensearch` / `full` |
| `opensearch-dashboards` | `opensearchproject/opensearch-dashboards:3.3.0` | `5601` | Profile `opensearch-ui` (start it separately when needed) |
| `milvus` | `milvusdb/milvus:v2.6.11` (standalone, embedded etcd) | `19530` / `9091` | Profile `milvus` / `full` |
| `neo4j` | `neo4j:latest` (APOC plugin) | `7474` / `7687` | `--neo4j` / `--full` |
| `dex` | `dexidp/dex:latest` (OIDC test identity provider, config `misc/dex-config.yaml`) | `5556` | `--dex` / `--full` |
| `langfuse-web` / `langfuse-worker` / `langfuse-clickhouse` / `langfuse-minio` / `langfuse-db-init` | Self-hosted Langfuse v3 stack. Reuses the dev postgres (separate `langfuse` database) and redis (DB 1) | web `3000`, minio `9100/9101` | `--langfuse` (on by default in `dev.sh`, `--no-langfuse` turns it off) / `--full` |
| `odl-hybrid` | Built locally from `docker/Dockerfile.odl-hybrid` (Docling PDF backend) | `5002` | `--odl-hybrid` (large image, start on demand) |
| `sandbox` | `ghcr.io/jrmatherly/ai-kb-sandbox` (sandbox for skill scripts; build/pull only, not a long-running service) | - | Profile `full` |

`dev.sh start` options: `--minio`, `--qdrant`, `--neo4j`, `--dex`, `--langfuse` (on by default), `--no-langfuse`, `--odl-hybrid`, `--full` (all optional services except odl-hybrid, Langfuse included). Pass them through the Makefile with `make dev-start DEV_ARGS=--odl-hybrid`, or combine them: `make dev-start DEV_ARGS="--full --odl-hybrid"`. `dev-stop` stops every service of the dev project, whichever profiles started it.

### Run docreader locally

`dev-start` runs docreader in a container by default. To debug the Python code locally:

```bash
cd docreader
uv sync                       # install dependencies from uv.lock (the container uses uv sync --locked --no-dev)
uv run -m docreader.main      # start the gRPC service (same as the Dockerfile CMD), listening on DOCREADER_GRPC_PORT (default 50051)
```

docreader takes many tuning parameters as environment variables: `DOCREADER_*` for PDF render DPI, scanned-document detection and the like, `SSRF_WHITELIST*` for the SSRF allowlist and `GRPC_TLS_*` / `GRPC_MTLS_REQUIRE_CLIENT_CERT` for gRPC TLS. The full list is the `docreader.environment` section of `docker-compose.dev.yml`.

### Lite mode (no external dependencies)

Lite mode compiles SQLite (+sqlite-vec) and an in-memory queue into a single binary, for quick trials and the desktop app:

```bash
make build-lite     # build the frontend into web/, then the Go binary with CGO (tags: sqlite_fts5); SKIP_FRONTEND=1 skips the frontend
make run-lite       # needs .env.lite (copy .env.lite.example); builds and starts ai-kb-lite
make package-lite   # tarball release package (scripts/package-lite.sh)
make package-mac-app  # macOS .app (scripts/package-mac-app.sh)
```

## Makefile targets

These targets are defined in the root `Makefile`. `make help` prints a summary of the main ones.

### Build and run

| Target | Action |
| --- | --- |
| `build` | `go build -o ai-kb ./cmd/server` |
| `anydoc-lib` | `scripts/build-anydoc-lib.sh`: builds the anydoc static library (Rust toolchain required; `TARGET=<rust-target-triple>` overrides the platform) |
| `build-anydoc` | `anydoc-lib`, then `go build -tags anydoc`, linking the in-process office document parser |
| `run` | `build`, then runs `./ai-kb` |
| `test` | `go test -v ./...` |
| `clean` | `go clean` and deletes the binary |
| `build-prod` | Production build: CGO_ENABLED=1, `-ldflags "-w -s"` injects Version/Edition/CommitID/BuildTime/GoVersion (package variables in `internal/handler`) and sets protobuf `conflictPolicy=warn` (avoids the qdrant/milvus proto conflict). `GO_BUILD_TAGS` adds build tags, for example `anydoc` |
| `fmt` | `go fmt ./...` |
| `lint` | `golangci-lint run` |
| `deps` | `go mod download` |
| `docs` | `swag init -g ./cmd/server/main.go -o ./docs --parseDependency --parseInternal --exclude ./internal/localsandbox` generates the Swagger docs |
| `install-swagger` | Installs the `swag` CLI |

### Docker images and service management

| Target | Action |
| --- | --- |
| `docker-build-app` | Builds `ghcr.io/jrmatherly/ai-kb-app` (`docker/Dockerfile.app`, with version info from `scripts/get_version.sh`; `WITH_ANYDOC` defaults to `1`) |
| `docker-build-docreader` | Builds `ghcr.io/jrmatherly/ai-kb-docreader` (`docker/Dockerfile.docreader`) |
| `docker-build-frontend` | Multi-stage build of `ghcr.io/jrmatherly/ai-kb-ui` (`npm ci` + `npm run build` inside the builder, no prebuilt dist needed on the host; `VITE_FRONTEND_COMMIT` comes from git) |
| `docker-build-all` | All three images above |
| `docker-run` | Makes sure `.env` exists (copied from `.env.example`, or touched when that is missing), then `docker-compose up` |
| `docker-stop` / `docker-restart` | `docker-compose down` / `stop -t 60` + `up` |
| `start-all` / `stop-all` | `scripts/start_all.sh` (start or stop all services) |
| `start-ollama` / `start-docker` | `start_all.sh --ollama` / `--docker` |
| `build-images` / `build-images-app` / `build-images-docreader` / `build-images-frontend` / `clean-images` | `scripts/build_images.sh` builds images from source or removes them |
| `check-env` / `list-containers` / `pull-images` | `start_all.sh --check / --list / --pull` |
| `show-platform` | Prints `uname -m` and the Docker build platform (amd64/arm64 detected automatically) |
| `clean-db` | Deletes the Docker volumes `<project>_postgres-data` and `<project>_minio_data` of the `docker-compose.yml` stack, where `<project>` is `COMPOSE_PROJECT_NAME` or the checkout directory name (**erases data**) |

### Database migrations

Details are in [Database and migrations](02-database-schema.md).

| Target | Action |
| --- | --- |
| `migrate-up` / `migrate-down` | `scripts/migrate.sh up / down` |
| `migrate-version` | Shows the current migration version |
| `migrate-create name=xxx` | Creates a new pair of migration files |
| `migrate-force version=N` | Forces the version (recovery from a dirty state) |
| `migrate-goto version=N` | Migrates to the given version |

### Development mode and Lite

| Target | Action |
| --- | --- |
| `dev-start` / `dev-stop` / `dev-restart` / `dev-logs` / `dev-status` | `scripts/dev.sh start/stop/restart/logs/status` (`dev-start` passes profile options through `DEV_ARGS`) |
| `dev-app` | Local backend: Air (`.air.toml`) when installed, otherwise `go run ./cmd/server` with version ldflags |
| `dev-frontend` | Local `npm run dev` |
| `build-lite` / `run-lite` / `package-lite` / `package-mac-app` | Lite mode build, run and packaging (see [Lite mode](#lite-mode-no-external-dependencies)) |
| `download_spatial` | `go run cmd/download/duckdb/duckdb.go` downloads the DuckDB spatial extension (used by the data analysis tools) |

### Model vendor catalog

| Target | Action |
| --- | --- |
| `model-catalog-generate` | `python3 scripts/model-catalog/generate.py` regenerates the vendor catalog metadata and protocol overrides |
| `model-catalog-check` | Checks that the generated output is current (`generate.py --check`), then runs `go test ./internal/models/...` |
| `model-catalog-diff` | Reports where model metadata differs from models.dev, for manual review only; writes nothing back. `VENDOR=deepseek` limits it to one vendor |

How to add a vendor: [Extension points](03-extension-points.md).

## Tests

### Go unit tests (main module)

```bash
make test          # go test -v ./...
# or one package at a time:
go test ./internal/infrastructure/chunker/...
go test -run TestXxx ./internal/application/service/...
```

The main module's tests use in-memory doubles such as `go-sqlmock` and `miniredis` (see `go.mod`), so most of them run without a real database. Some packages need CGO (DuckDB/sqlite-vec).

### docreader tests (Python)

The tests live in `docreader/tests/` and are written with the standard library `unittest` (each file calls `unittest.main()`). They cover parser routing, concurrency, EPUB/Excel/MHTML/PDF parsing, SSRF protection and more:

```bash
cd docreader
uv sync
uv run python -m unittest discover -s tests -v      # all
uv run python -m unittest tests.test_parser_routing  # one module
```

### CLI tests and acceptance tests

`cli/` is a separate Go module with its own `cli/Makefile`:

```bash
cd cli
make test            # go test ./...
make test-coverage   # with coverage
make lint            # go vet
```

Cross-cutting contract and integration tests live in `cli/acceptance/` (see `cli/acceptance/doc.go`):

- `cli/acceptance/contract/`: golden tests of the envelope JSON output shape, plus consistency of the error.code registry.
- `cli/acceptance/e2e/`: black-box tests against a real AI KB server (testscript style), behind the `acceptance_e2e` build tag. No workflow runs them in this repository. Run them by hand from `cli/` with `AIKB_E2E_HOST=<server> AIKB_E2E_TOKEN=<token> go test -tags=acceptance_e2e -v ./acceptance/e2e/...`.

### The tests/ directory and frontend tests

- `tests/miniprogram/miniprogram.test.js`: integration test for the mini program client (a Node test script). It is currently the only file in `tests/`.
- Frontend: `cd frontend && npm run type-check` (vue-tsc) and `npm test` (`tsx --test`, the Node test runner).

## Code style and contribution workflow

### Go code style

`.golangci.yml` in the repository root (golangci-lint v2 config format):

```yaml
version: "2"
linters:
  enable:
    - lll           # line length (120 columns)
    - govet
    - revive
  settings:
    lll:
      line-length: 120
      tab-width: 4
  exclusions:
    paths:
      # swag embeds OpenAPI as single-line JSON strings; line-length is not meaningful.
      - docs/docs\.go$
formatters:
  enable:
    - gofmt
    - gofumpt
```

Before committing, run:

```bash
make fmt && make lint && make test
```

The formatting standard golangci-lint enforces is **gofumpt** (stricter than gofmt), with a 120-column line limit. `make fmt` runs only `go fmt`.

The repository ships Git hooks. `./scripts/install-git-hooks.sh` points `core.hooksPath` at `scripts/git-hooks`:

- pre-commit checks whitespace, blocks CJK text in added lines (`scripts/check-english-only.sh --diff`), runs gofmt with auto-fix on staged Go files outside `cli/`, and runs golangci-lint on the changed packages when it is installed.
- pre-push checks whitespace and gofmt, runs golangci-lint on new issues, `go vet` / `go test` / `go build ./cmd/server` for the main module, the CLI checks when `cli/` changed, and `scripts/verify_frontend_pr.sh` when `frontend/` changed.

`SKIP_HOOKS=1` skips the hooks once. `HOOK_SKIP_TEST=1` skips only the pre-push tests.

When you add or upgrade a third-party dependency or change a data file shipped with a release, update `THIRD_PARTY_NOTICES.md` and `licenses/`, then check them with `scripts/check-license-bundle.sh`.

### Validation and contribution workflow

This repository has no `.github/` directory, so no CI workflows run on push or pull request. The validation commands come from the archived CI workflows, which are kept outside the tree. Run the rows for what you touched:

| Area touched | Validate with |
| --- | --- |
| Go (root module) | `go vet $(go list ./... \| grep -v /docreader/) && go test $(go list ./... \| grep -v /docreader/) && go test -tags desktop ./internal/container && golangci-lint run` |
| `internal/models/catalog/data/**`, `scripts/model-catalog/**` | `python3 scripts/model-catalog/generate.py --check && go test ./internal/models/...` (same as `make model-catalog-check`) |
| `cli/**` (own module) | `go -C cli vet ./... && go -C cli test -race ./... && cli/scripts/check-skill-wire-vocab.sh && cli/scripts/check-secret-tokens.sh` |
| `client/**` (own module) | `go -C client vet ./... && go -C client test ./...` |
| `frontend/**` | `npm --prefix frontend test && npm --prefix frontend run type-check` |
| `docreader/**` | `uv run --project docreader python -m unittest discover -s docreader/tests -p "test_*.py"` and `go test ./docreader/client ./docreader/proto` |
| `scripts/git-hooks/**`, license scripts | `bash scripts/test-git-hooks.sh`, `bash scripts/check-license-bundle.sh && bash scripts/test-license-bundle.sh` |

For the frontend, `scripts/verify_frontend_pr.sh` runs `npm test`, then `npm run type-check`, then `npm run build`.

Generated outputs are committed and regenerated, never edited by hand: the Swagger files `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml` come from `make docs` (`docs/swagger_contract_test.go` checks them), and the model catalog output under `internal/models/catalog/` comes from `make model-catalog-generate`.

Contribution workflow: branch, run `fmt + lint + test` and the validation rows above locally, then commit.

## Debugging

### Log level

Logging is implemented in `internal/logger/logger.go` (logrus). The environment variable `LOG_LEVEL` sets the level: `debug`, `info`, `warn` (`warning`), `error` or `fatal`. When it is unset or invalid, the default is **debug** (`getLogLevelFromEnv()`). `LOG_PATH` sets the output path. The server reads both from the process environment when it starts (`scripts/dev.sh` exports the values from `.env`; Compose passes them to the container); only the desktop build loads `.env` itself and re-applies them. docreader reads `LOG_LEVEL` too (passed through in compose).

Each request carries an `X-Request-ID` through the app and docreader logs (docreader's `init_logging_request_id`). When debugging, find the request id first.

### GIN_MODE and Swagger

- `GIN_MODE=release` disables Swagger UI (`internal/router/router.go`) and changes the security behavior of embed channels. For local source development, use `debug`. When Docker Compose does not set the variable, it defaults to `release`.
- The **app backend** serves Swagger. The default address is `http://localhost:8080/swagger/index.html`; if you changed `APP_PORT` in Compose, use that host-mapped port. The frontend port (default `80`) and the Vite dev port (default `5173`) have no `/swagger/` proxy.
- The release images already include the Swagger docs. Run `make docs` only when you change API annotations and need to regenerate the docs and rebuild the backend.

#### Docker Compose access steps

Set `GIN_MODE=debug` in the project `.env`, then recreate the app container so the variable takes effect (`docker compose restart app` alone does not update the container environment):

```bash
docker compose up -d --no-deps --force-recreate app
docker compose exec app printenv GIN_MODE
docker compose port app 8080
```

Check that the second command prints `debug`, then use the port the third command shows. For example, if it prints `0.0.0.0:8080`, open `http://localhost:8080/swagger/index.html` on the same machine; from another machine, replace `localhost` with the deployment host's address. When you are done, set `GIN_MODE` back to `release` and recreate the app container.

#### Blank page or load failure

First check that the requests reach the backend (the default backend port `8080` is used here):

```bash
curl -i http://localhost:8080/swagger/index.html
curl -i http://localhost:8080/swagger/doc.json
```

Both should return `200`: the first is HTML containing `swagger-ui`, the second is JSON with `swagger` and `paths` fields.

| Symptom | What to check |
| --- | --- |
| Status `200`, but the HTML is the regular frontend page (contains `<div id="app">`), and the page is blank or redirects to login | The request hit the frontend's SPA fallback. Use the app backend's mapped port; logging in to the frontend does not add a `/swagger/` proxy. |
| The backend returns `401` or `404` and no Swagger UI | Check the port you used and the actual `GIN_MODE` inside the container. `release` mode does not register the Swagger routes, and unmatched requests can fall into the auth middleware. |
| Swagger UI appears but says it cannot load the API definition | Check the `/swagger/doc.json` response from the same backend on its own. With your own reverse proxy, make sure the whole `/swagger/` path and its static assets are forwarded to app. |

### Database and migration debugging

- `AUTO_MIGRATE=false` turns off the automatic migration at startup. `AUTO_RECOVER_DIRTY` (on by default, `false` turns it off) controls automatic recovery from a dirty state (`internal/container/container.go`). A failed migration only logs a warning and does not stop startup; look for `Database migration failed` in the startup log.
- `make migrate-version` shows the schema version quickly.

### LLM tracing (Langfuse)

`dev.sh start` starts self-hosted Langfuse (`http://localhost:3000`) by default. An app run locally with `go run` needs these exports:

```bash
export LANGFUSE_HOST=http://localhost:3000
export LANGFUSE_PUBLIC_KEY=pk-lf-xxx
export LANGFUSE_SECRET_KEY=sk-lf-xxx
```

The Langfuse UI then shows a trace of the model calls for each session. Document processing spans are also stored in the `knowledge_processing_spans` table and visualized in the frontend.

### pprof

The code has **no built-in** `net/http/pprof` endpoint (no pprof references under `internal/` or `cmd/`). To profile, temporarily add `import _ "net/http/pprof"` in `cmd/server/main.go` and start a separate `http.ListenAndServe("localhost:6060", nil)`, or profile a specific package with `go test -bench . -cpuprofile`.

### Chunking strategy diagnostics

The chunker provides `SplitWithDiagnostics()` (`internal/infrastructure/chunker/strategy.go`). It returns the strategy chain choice, the reason each tier was rejected and the document profile. Together with `LOG_LEVEL=debug` (the `chunker: tier %s rejected` log line), it helps diagnose chunking results.

### Cloud image maintenance scripts {#cloud-image-scripts}

`scripts/cloud-image/` prepares a distribution image on a dedicated, disposable Linux build machine. A normal deployment follows [Installation](../01-getting-started/02-installation.md) and does not need these scripts.

- `prepare.sh` downloads the runtime files for `AIKB_REF`, pulls the images and installs the systemd services. The same ref sets the image version, so confirm that a matching image tag exists. The shipped systemd units hard-code the deployment directory (`WorkingDirectory=` in `scripts/cloud-image/systemd/aikb.service`, the default of `AIKB_DIR`); changing only the script's directory variable does not move the installation.
- `cleanup.sh` wipes the build machine's data, keys, SSH authorizations, logs and Docker cache, then powers it off. Its effect is not limited to the AI KB directory; use it only on a dedicated build machine you have checked.
- `firstboot.sh` generates keys on a new instance, writes `.env` and starts Compose. Afterwards it disables the first-boot service; it does not delete the script.

Before building an image, check that the scripts match the chosen version. Before distributing it, verify startup, service health and key independence on a fresh instance. This description does not mean the current scripts have passed deployment or marketplace acceptance on any particular cloud platform.

For first-boot troubleshooting, see `journalctl -u ai-kb-firstboot`, `/var/log/ai-kb-firstboot.log` and the Compose state in the deployment directory. `.firstboot.done` is written after the keys are generated and before Compose starts, so the marker does not mean the services are healthy. If startup fails, keep the generated `.env` and the marker, fix the cause and start Compose again. Deleting the marker to regenerate keys can leave the configuration inconsistent with an already initialized database.
