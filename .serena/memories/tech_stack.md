# Tech stack

- Go 1.26 (`go.mod`), module `github.com/jrmatherly/ai-kb` (renamed 2026-10-12). Gin 1.12, GORM 1.31, gRPC 1.84, go-redis v9, minio-go v7, OpenTelemetry. Optional build tags: `anydoc` (links the Rust `anydoc` static lib, `make anydoc-lib` first), `desktop` (`cmd/desktop`, `internal/container` tests).
- Lint: golangci-lint v2 config `.golangci.yml`: `lll` 120 cols (tab width 4), `govet`, `revive`; formatters `gofmt` + `gofumpt`. CI lints modules `.`, `cli`, `client` when the config changes.
- Hot reload: Air (`.air.toml`) builds `./cmd/server` with ldflags from `scripts/get_version.sh ldflags`.
- Frontend: Vue 3, TypeScript, Vite, Less; TDesign components are registered per template by `unplugin-vue-components` (`TDesignResolver`, `dts: 'src/components.d.ts'` committed) in both entries, with no shared TDesign chunk; `vue-tsc --build` for types, tests via Node's `node:test` run by `tsx --test` (no eslint/vitest/jest in devDependencies). npm with `package-lock.json`, Node >= 24. Dockerfile at `frontend/Dockerfile`, nginx confs beside it.
- Dependency cooldown: npm `min-release-age=7` (`frontend/.npmrc`; override a security fix with `--min-release-age=0`) and uv `exclude-newer = "7 days"` (`docreader/pyproject.toml`, `dataset/pyproject.toml`, `examples/mcp-demo/pyproject.toml`).
- docreader: Python >= 3.14, `uv` >= 0.13.0 (`uv.lock`), tests with stdlib `unittest discover`; Playwright (webkit) needed for some parsers.
- dataset (`dataset/`): uv project (`pyproject.toml`, `uv.lock`), 12 offline unittest cases, CI `dataset.yml`, local `scripts/validate.sh --area dataset`.
- CLI (`cli/`): own Go module, CI runs `go test -race`, `go vet`, `cli/scripts/check-skill-wire-vocab.sh`, `cli/scripts/check-secret-tokens.sh`.
- Model catalog: pinned JSON in `internal/models/catalog/data/`, generator `scripts/model-catalog/generate.py` (`--check` in CI), diff tool `scripts/model_catalog_diff.py` (models.dev, dev aid only).
- API docs: swag (`make docs`, `make install-swagger`), output `docs/`, contract test `docs/swagger_contract_test.go`.
- Databases via migrations dir: MySQL, ParadeDB (Postgres), SQLite; `versioned/` for golang-migrate (`scripts/migrate.sh`).
- Website: `website-docs/` Node 24 project (`npm run setup && npm run build && npm run preview`); image `ghcr.io/jrmatherly/ai-kb-website` from `website-docs.yml`; in production path-mounted at `ai-kb.matherly.net/docs/` beside the app (the app owns `/`), so the docs logo links to `/docs/`.
