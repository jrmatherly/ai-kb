# Commands (run from repo root)

Dev loop

- `make dev-start` infra containers; `make dev-app` backend with Air; `make dev-frontend` Vite dev server; `make dev-stop|dev-restart|dev-logs|dev-status` (all wrap `scripts/dev.sh`).
- `make build` -> `./ai-kb` binary; `make run`; `make build-lite` -> `./ai-kb-lite` (frontend copied to `./web` and served from disk, `AIKB_WEB_DIR` overrides); `run-lite` needs `.env.lite`.

Validate

- All areas a change touches: `scripts/validate.sh [--base <rev>] [--list] [--area <name>]`; the per-area commands below are what it runs.
- Go: `go vet $(go list ./... | grep -v /docreader/) && go test $(go list ./... | grep -v /docreader/)`; `go test -tags desktop ./internal/container`; `go build ./cmd/server`; `golangci-lint run`; `make fmt` (go fmt).
- Model catalog: `make model-catalog-check` (= `python3 scripts/model-catalog/generate.py --check` + `go test ./internal/models/...`); regenerate with `make model-catalog-generate`.
- Frontend: `npm --prefix frontend test`, `npm --prefix frontend run type-check`, `npm --prefix frontend run build`; i18n: `npm --prefix frontend run check-i18n`.
- docreader: `uv sync --project docreader --locked`; `uv run --project docreader python -m unittest discover -s docreader/tests -p "test_*.py" -v`; Go side `go test ./docreader/client ./docreader/proto`; server `uv run --project docreader python -m docreader.main` (gRPC :50051).
- CLI/client modules: `go -C cli test -race ./... && go -C cli vet ./...`; `go -C client test ./...`.
- Hooks and bundles: `bash scripts/test-git-hooks.sh`; `bash scripts/check-license-bundle.sh && bash scripts/test-license-bundle.sh`; install hooks with `scripts/install-git-hooks.sh`.
- Swagger: `make docs` (regenerates `docs/`).
- Migrations: `make migrate-up|migrate-down|migrate-version|migrate-create|migrate-force|migrate-goto` (wrap `scripts/migrate.sh`).

Release

- `make bump-version [VERSION=YYYY.M.D[.MICRO]]`, `make bump-lib LIB=dsh-ai-kb|mcp-server|helm-chart PART=patch|minor|major`, `make check-version`.

Upstream and English-only

- `.claude/skills/upstream-review/scripts/upstream-review.sh [--stat|--urls|--show <sha>|--diverged|--advance <sha>]`.
- `scripts/check-english-only.sh [-n|--summary] [path...]` exits 1 while CJK text remains; `scripts/check-emdash.sh [--cached]`; `scripts/check-workflows.sh`.

Darwin notes

- `sed -i ''` (BSD sed needs the empty suffix). `rg` is installed (Homebrew); prefer it over grep for the CJK scan's `--pcre2`.
- Upstream history for `git --git-dir=private/OLD-.git log ...` (archived clone, gitignored).
