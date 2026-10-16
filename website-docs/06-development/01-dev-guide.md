# Development guide

The development environment starts the dependency services from `docker-compose.dev.yml` and runs the Go backend, the web frontend and docreader on their own. After a change, build and test the module you touched, then check the integration paths that depend on it.

## Tech stack and requirements

AI KB is made of three processes that are developed independently, plus a CLI:

| Component | Directory | Language / runtime | Required version (source) |
| --- | --- | --- | --- |
| Main backend `app` | `cmd/server` + `internal/` | Go | **Go 1.26.0** (`go 1.26.0` in `go.mod`). Needs CGO (DuckDB and sqlite-vec bindings) |
| Document parsing service `docreader` | `docreader/` | Python + gRPC | **Python >= 3.14** (`requires-python` in `docreader/pyproject.toml`). Dependencies are managed with **uv** (the repository has `uv.lock`; the Docker image runs `uv sync --locked`) |
| Frontend `frontend` | `frontend/` | Node.js + Vue 3 | Node.js 24 (the `frontend/Dockerfile` builder is `node:24-bookworm-slim`; `package.json` declares no `engines`). Vite 7 + TypeScript ~6.0 + Vue 3.5 + TDesign, package version `0.8.2` |
| CLI | `cli/` (separate Go module) | Go | Go 1.26 (`go 1.26.0` in `cli/go.mod`) |

Recommended extra development tools:

```bash
# Database migration CLI (required by scripts/migrate.sh), at the golang-migrate version go.mod pins
go install -tags postgres "github.com/golang-migrate/migrate/v4/cmd/migrate@$(go list -m -f '{{.Version}}' github.com/golang-migrate/migrate/v4)"

# Linter (called by make lint)
# Install golangci-lint v2.14.0 (the version CI pins) from https://golangci-lint.run; the configuration is .golangci.yml in the repository root

# Swagger doc generation: make docs runs swag through go run at the github.com/swaggo/swag version in go.mod,
# so nothing needs installing. make install-swagger puts that same version on PATH.

# Python dependency management
pip install 'uv>=0.13.0'  # docreader installs its dependencies with uv sync; docreader/pyproject.toml [tool.uv] required-version sets the floor

# Hot reload for make dev-app (optional; without it dev-app falls back to go run)
go install github.com/air-verse/air@latest   # or: brew install go-air (the Homebrew formula named air is an unrelated R formatter)

# Docker + Docker Compose (the v2 plugin or standalone docker-compose; scripts/dev.sh detects either)
```

## Quick start: development mode (recommended)

Development mode runs the infrastructure in Docker and runs `app` and `frontend` locally. After changing application code you restart the process; the dependency images are not rebuilt. The entry point is `scripts/dev.sh`, and the Makefile `dev-*` targets wrap it.

```bash
# 1. Prepare environment variables: dev.sh loads .env (required), then overrides it with .env.local (optional)
cp .env.example .env
# .env.example ships default JWT_SECRET and SYSTEM_AES_KEY values, so the backend starts (with a warning for each).
# Replace them (openssl rand -hex 32, openssl rand -hex 16) before storing real credentials; an empty value stops startup.

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

`dev-app` points the backend at the containers on the host loopback address: it exports `DB_HOST=127.0.0.1`, `DOCREADER_ADDR=127.0.0.1:50051`, `MINIO_ENDPOINT=127.0.0.1:9000`, `REDIS_ADDR=127.0.0.1:6379`, `MILVUS_ADDRESS=127.0.0.1:19530`, `NEO4J_URI=bolt://127.0.0.1:7687` and `QDRANT_HOST=127.0.0.1`, overriding the container service names in `.env`, and maps `LANGFUSE_HOST=http://langfuse-web:3000` to `http://127.0.0.1:${LANGFUSE_WEB_PORT:-3000}`. It also appends `127.0.0.1` (or `DEV_REMOTE_HOST`) to `SSRF_WHITELIST_EXTRA`, because the SSRF guard otherwise rejects a loopback MinIO endpoint and the backend fails to start with `STORAGE_TYPE=minio`. It also replaces the container path `LOCAL_STORAGE_BASE_DIR=/data/files` with the repository-local `.local-data/files`, and adds the `anydoc` build tag when `make anydoc-lib` has built the static library for the host platform.

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
| `postgres` | `paradedb/paradedb:v0.26.1-pg18` (bundles pg_search/BM25) | `5432` | Always |
| `redis` | `redis:8.10.2-alpine` (`--requirepass`) | `6379` | Always |
| `docreader` | Built locally from `docker/Dockerfile.docreader` | `50051` (gRPC) | Always |
| `searxng` (+`searxng-init`) | `searxng/searxng:2026.10.15-b00212a83` | `127.0.0.1:8888` | `--full` (compose profile `searxng` or `full`; `dev.sh` has no `--searxng` flag) |
| `minio` | `pgsty/minio:RELEASE.2026-08-04T00-00-00Z` | `9000` / console `9001` | `--minio` / `--full` |
| `qdrant` | `qdrant/qdrant:v1.19.2` | `6333` / `6334` | `--qdrant` / `--full` |
| `opensearch` | `opensearchproject/opensearch:3.9.0` (security plugin off, plain HTTP) | `9200` | Profile `opensearch` / `full` |
| `opensearch-dashboards` | `opensearchproject/opensearch-dashboards:3.9.0` | `5601` | Profile `opensearch-ui` (start it separately when needed) |
| `milvus` | `milvusdb/milvus:v3.0.3` (standalone, embedded etcd; the one-shot `milvus-volume-init` hands the volume to the image's non-root user) | `19530` / `9091` | Profile `milvus` / `full` |
| `neo4j` | `neo4j:2026.09.0` (APOC plugin) | `7474` / `7687` | `--neo4j` / `--full` |
| `dex` | `dexidp/dex:v2.46.0` (OIDC test identity provider, config `misc/dex-config.yaml`) | `5556` | `--dex` / `--full` |
| `langfuse-web` / `langfuse-worker` / `langfuse-clickhouse` / `langfuse-minio` / `langfuse-db-init` | Self-hosted Langfuse v4 stack (`langfuse/langfuse:4.59.0`, ClickHouse `26.8.28`). Reuses the dev postgres (separate `langfuse` database) and redis (DB 1) | web `3000`, minio `9100/9101` | `--langfuse` (on by default in `dev.sh`, `--no-langfuse` turns it off) / `--full` |
| `odl-hybrid` | Built locally from `docker/Dockerfile.odl-hybrid` (Docling PDF backend) | `5002` | `--odl-hybrid` (large image, start on demand) |
| `sandbox` | `ghcr.io/jrmatherly/ai-kb-sandbox` (sandbox for skill scripts; build/pull only, not a long-running service) | - | Profile `full` |

The images moved to new major versions on 2026-10-15 (PostgreSQL 18, Qdrant 1.19, Weaviate 1.40, Milvus 3.0, Langfuse 4 with ClickHouse 26.8). Dev volumes created before that do not start or open on the new images; reset them once with `docker compose -f docker-compose.dev.yml --profile '*' down -v` (without `--profile '*'`, services started through a profile keep their volumes).

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
| `build-prod` | Production build: CGO_ENABLED=1, `-ldflags "-w -s"` injects Version (from `$VERSION`, else the `VERSION` file)/Edition/CommitID/BuildTime/GoVersion (package variables in `internal/handler`) and sets protobuf `conflictPolicy=warn` (avoids the qdrant/milvus proto conflict). `GO_BUILD_TAGS` adds build tags, for example `anydoc` |
| `fmt` | `go fmt ./...` |
| `lint` | `golangci-lint run --new-from-merge-base=$(LINT_BASE)`: only issues in code changed since `LINT_BASE` (default `main`), as `go-lint.yml` judges a pull request |
| `deps` | `go mod download` |
| `docs` | `go run github.com/swaggo/swag/cmd/swag@<go.mod version> init -g ./cmd/server/main.go -o ./docs --parseDependency --parseInternal --exclude ./internal/localsandbox` generates the Swagger docs |
| `install-swagger` | Installs the `swag` CLI at the version `go.mod` pins |

### Docker images and service management

| Target | Action |
| --- | --- |
| `docker-build-app` | Builds `ghcr.io/jrmatherly/ai-kb-app` (`docker/Dockerfile.app`, with version info from `scripts/get_version.sh`; `WITH_ANYDOC` defaults to `1`) |
| `docker-build-docreader` | Builds `ghcr.io/jrmatherly/ai-kb-docreader` (`docker/Dockerfile.docreader`) |
| `docker-build-frontend` | Multi-stage build of `ghcr.io/jrmatherly/ai-kb-ui` (`npm ci` + `npm run build` inside the builder, no prebuilt dist needed on the host; `VITE_FRONTEND_COMMIT` comes from git) |
| `docker-build-all` | All three images above |
| `docker-run` | Makes sure `.env` exists (copied from `.env.example`, or touched when that is missing), then `docker compose up` |
| `docker-stop` / `docker-restart` | `docker compose --profile '*' down` / `stop -t 60` + `up` |
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
| `download_duckdb_extensions` | `go run cmd/download/duckdb/duckdb.go` downloads the DuckDB `excel` extension (`read_xlsx`, used by the data analysis tools) |

### Model vendor catalog

| Target | Action |
| --- | --- |
| `model-catalog-generate` | `python3 scripts/model-catalog/generate.py` regenerates the vendor catalog metadata and protocol overrides |
| `model-catalog-check` | Checks that the generated output is current (`generate.py --check`), then runs `go test ./internal/models/...` |
| `model-catalog-diff` | Reports where model metadata differs from models.dev, for manual review only; writes nothing back. `VENDOR=deepseek` limits it to one vendor |

How to add a vendor: [Extension points](03-extension-points.md).

### Versions and releases

The product version is dated CalVer, `YYYY.M.D[.MICRO]` unpadded, tagged `vYYYY.M.D[.MICRO]`. `packages/dsh-ai-kb`, `mcp-server` and the Helm chart's own `version` keep their own semver. Every file that carries a version is one row in the `TOUCHPOINTS` table of `scripts/version.py`.

| Target | Action |
| --- | --- |
| `check-version` | `scripts/version.py check`: fails, naming file, line and value, when a product version string differs from `VERSION` or a library's strings disagree with each other |
| `bump-version` | Sets the product version everywhere and makes one signed `chore(release): v<ver>` commit. The default is today's date, then `.1`, `.2` while that version is current or its tag exists locally or on `origin`; `VERSION=YYYY.M.D[.MICRO]` picks one, which must be greater than the current version and untagged. Pass `VERSION` after `make`: an exported `VERSION` makes the bump fail rather than pin it |
| `bump-lib LIB=dsh-ai-kb\|mcp-server\|helm-chart PART=patch\|minor\|major` | The same for one library (`VERSION=x.y.z` instead of `PART`), committed as `chore(release): <lib> v<ver>` |

Bumps rewrite only the version token in each file (lockfile dependencies stay untouched) and refuse a dirty working tree. When a step after the edits fails (the lockfile check, the version check, or the commit, such as a failing hook or a missing signing key), every file gets its original bytes back. They never tag or push: they print the next commands. The `v<ver>` tag triggers `docker-image.yml`; `release-lite.yml` is started by hand with the tag. `dsh-ai-kb-v<ver>` publishes the dsh plugin and `mcp-server-v<ver>` the MCP server. Nothing publishes the Helm chart, so its bump prints no tag.

## Tests

### Go unit tests (main module)

```bash
make test          # go vet + go test on every package except /docreader/, like CI
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

- pre-commit checks whitespace, blocks CJK text in added lines (`scripts/check-english-only.sh --diff`), blocks em dashes in staged files (`scripts/check-emdash.sh --cached`), runs gofmt with auto-fix on staged Go files outside `cli/`, runs golangci-lint on the changed packages when it is installed, and lints staged `.md` files with markdownlint-cli2 (report only, no auto-fix).
- pre-push checks whitespace on the pushed commits, then runs `scripts/validate.sh --base <push base>`, which checks every validation area below that the push touches. It refuses to push a commit other than `HEAD`, because the checks run on the working tree.

`SKIP_HOOKS=1` skips the hooks once. `HOOK_SKIP_TEST=1` skips only the Go tests.

When you add or upgrade a third-party dependency or change a data file shipped with a release, update `THIRD_PARTY_NOTICES.md` and `licenses/`, then check them with `scripts/check-license-bundle.sh`.

### Validation and contribution workflow

CI lives in `.github/workflows/`: check workflows for the areas below, each triggered by `paths:` filters (`app.yml` covers go, govulncheck against `scripts/govulncheck-allowlist.txt`, model-catalog, git-hooks and licenses; miniprogram, markdown and shell run only locally), plus `guards.yml` (English-only, em dash, rebrand, `scripts/version.py check` with `scripts/test_version.py`, and `scripts/check-workflows.sh`, which requires SHA-pinned actions, a top-level `concurrency` and emoji-led names) and the publishing workflows (`docker-image.yml`, `release-lite.yml`, `mcp-server.yml`, `dsh-plugin.yml`). `scripts/validate.sh` runs the same checks locally. It maps each changed path to an area with the same `paths:` filters and runs that area's commands:

```bash
scripts/validate.sh                 # working tree and untracked files against HEAD
scripts/validate.sh --base <rev>    # everything since <rev>
scripts/validate.sh --list          # print the areas that would run
scripts/validate.sh --area helm     # run only the named area (repeatable), whatever changed
```

| Area | Runs when these change | Checks |
| --- | --- | --- |
| go | root-module `*.go`, `go.mod`, `go.sum`, `Makefile`, `config/`, `migrations/`, `skills/preloaded/`, `testdata/`, embedded data under `internal/`, `.golangci.yml` | gofmt, `go mod tidy -diff`, golangci-lint (new issues only), `go vet` and `go test` over every app package, `go test -tags desktop ./internal/container`, `go build ./cmd/server` |
| cli | `cli/**`, `.golangci.yml` | `go mod tidy -diff`, build, vet, `go test -race`, golangci-lint, `check-skill-wire-vocab.sh`, `check-secret-tokens.sh`; the acceptance e2e suite only with `AIKB_E2E_HOST` and `AIKB_E2E_TOKEN` set |
| client | `client/**`, `.golangci.yml` | `go mod tidy -diff`, vet, test, golangci-lint |
| frontend | `frontend/**`, `scripts/build_frontend_dist.sh`, `scripts/test_embed_nginx.py` | `scripts/verify_frontend_pr.sh` (`npm ci` when stale, test, type-check, build), then the UI image build and `scripts/test_embed_nginx.py` when Docker is running |
| docreader | `docreader/**`, `go.mod`, `go.sum`, `testdata/`, the docreader Dockerfiles | `uv sync --locked`, compileall, Playwright WebKit install, the unittest suite, then the Go client tests against a DocReader server the script starts on port 50051 |
| model-catalog | `internal/models/catalog/data/**`, `scripts/model-catalog/**` | `generate.py --check`, `go test ./internal/models/...` |
| git-hooks | `scripts/git-hooks/**`, `scripts/test-git-hooks.sh`, `scripts/check-english-only.sh`, `scripts/check-emdash.sh`, `scripts/emdash-allowlist.txt`, `.claude/hooks/post-edit.sh` | `scripts/test-git-hooks.sh` |
| shell | any `*.sh`, the hooks in `scripts/git-hooks/` | `shellcheck -S warning` on the changed scripts |
| licenses | `licenses/**`, license scripts, `LICENSE`, `THIRD_PARTY_NOTICES.md` | `check-license-bundle.sh`, `test-license-bundle.sh` |
| anydoc | `third_party/anydoc-go/**`, `scripts/build-anydoc-lib.sh`, `internal/infrastructure/docparser/**` | builds the Rust archive, `cargo audit`, vet, test and build with `-tags anydoc` |
| helm | `helm/**` | `helm lint`, `helm/tests`, image tag rendering |
| mcp-server | `mcp-server/**` | unittest and pytest suites, wheel build containing `upload_paths.py` |
| dsh-plugin | `packages/dsh-ai-kb/**` | typecheck, build, unit and contract tests (Node and Go), tarball file list, end-to-end run inside the pinned dsh release |
| miniprogram | `miniprogram/**`, `tests/miniprogram/**` | `npm --prefix miniprogram test` |
| website-docs | `website-docs/**` | link and anchor checks, Mermaid check, `build:docs` |
| markdown | any `*.md`, `.markdownlint-cli2.jsonc` | markdownlint-cli2 over the whole tree, with the root `.markdownlint-cli2.jsonc` |

Every run ends with the English-only gate on the lines added since the base and the em dash gate (`scripts/check-emdash.sh`) on every tracked file outside `scripts/emdash-allowlist.txt` (changelogs, release notes, `third_party/` and the generated Swagger files), then the version area: `scripts/version.py check` always, and `scripts/test_version.py` when `scripts/version.py`, its test or the `Makefile` changed (skipped and listed when `uv`, `ssh-keygen` or `make` is missing). Every touched area runs even after one fails, and the summary lists each area as passed, failed or skipped, with every check that did not run and why (a missing tool such as `cargo` or `helm`, or a stopped Docker daemon). Image pushes, release builds and package publishing are not run locally.

Generated outputs are committed and regenerated, never edited by hand: the Swagger files `docs/docs.go`, `docs/swagger.json` and `docs/swagger.yaml` come from `make docs` (`docs/swagger_contract_test.go` checks them), and the model catalog output under `internal/models/catalog/` comes from `make model-catalog-generate`.

Contribution workflow: branch, run `scripts/validate.sh`, then commit.

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

`dev.sh start` starts self-hosted Langfuse (`http://localhost:3000`) by default. Tracing stays off until the app has a key pair: open `http://localhost:3000`, register, create a project API key (or prefill one with the `LANGFUSE_INIT_*` variables in `.env.example` section I2), then set these in `.env` and restart `make dev-app`, which maps the container host name to `127.0.0.1`:

```bash
LANGFUSE_HOST=http://langfuse-web:3000
LANGFUSE_PUBLIC_KEY=pk-lf-...
LANGFUSE_SECRET_KEY=sk-lf-...
```

The Langfuse UI then shows a trace of the model calls for each session. Document processing spans are also stored in the `knowledge_processing_spans` table and visualized in the frontend.

### pprof

The code has **no built-in** `net/http/pprof` endpoint (no pprof references under `internal/` or `cmd/`). To profile, temporarily add `import _ "net/http/pprof"` in `cmd/server/main.go` and start a separate `http.ListenAndServe("localhost:6060", nil)`, or profile a specific package with `go test -bench . -cpuprofile`.

### Chunking strategy diagnostics

The chunker provides `SplitWithDiagnostics()` (`internal/infrastructure/chunker/strategy.go`). It returns the strategy chain choice, the reason each tier was rejected and the document profile. Together with `LOG_LEVEL=debug` (the `chunker: tier %s rejected` log line), it helps diagnose chunking results.

### Cloud image maintenance scripts {#cloud-image-scripts}

`scripts/cloud-image/` prepares a distribution image on a dedicated, disposable Linux build machine. A normal deployment follows [Installation](../01-getting-started/02-installation.md) and does not need these scripts.

- `prepare.sh` downloads the runtime files for `AIKB_REF`, pulls the images and installs the systemd services. The same ref sets the image version, so confirm that a matching image tag exists. The shipped systemd units (`scripts/cloud-image/systemd/ai-kb.service` and `ai-kb-firstboot.service`) name `/opt/ai-kb`, the default of `AIKB_DIR`; `prepare.sh` rewrites that path to `AIKB_DIR` when it installs them. Run `cleanup.sh` with the same `AIKB_DIR`.
- `cleanup.sh` wipes the build machine's data, keys, SSH authorizations, logs and Docker cache, then powers it off. Its effect is not limited to the AI KB directory; use it only on a dedicated build machine you have checked.
- `firstboot.sh` generates keys on a new instance, writes `.env` and starts Compose. Afterwards it disables the first-boot service; it does not delete the script.

Before building an image, check that the scripts match the chosen version. Before distributing it, verify startup, service health and key independence on a fresh instance. This description does not mean the current scripts have passed deployment or marketplace acceptance on any particular cloud platform.

For first-boot troubleshooting, see `journalctl -u ai-kb-firstboot`, `/var/log/ai-kb-firstboot.log` and the Compose state in the deployment directory. `.firstboot.done` is written after the keys are generated and before Compose starts, so the marker does not mean the services are healthy. If startup fails, keep the generated `.env` and the marker, fix the cause and start Compose again. Deleting the marker to regenerate keys can leave the configuration inconsistent with an already initialized database.
