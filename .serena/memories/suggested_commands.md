# Commands (run from repo root)

Dev loop

- `make dev-start` infra containers; `make dev-app` backend with Air; `make dev-frontend` Vite dev server; `make dev-stop|dev-restart|dev-logs|dev-status` (all wrap `scripts/dev.sh`).
- `make build` -> `./ai-kb` binary; `make run`; `make build-lite` -> `./ai-kb-lite` (frontend copied to `./web` and served from disk, `AIKB_WEB_DIR` overrides); `run-lite` needs `.env.lite`.

Validate

- All areas a change touches: `scripts/validate.sh [--base <rev>] [--list] [--area <name>]`; the commands below are its core. It also runs `go mod tidy -diff` and the govulncheck gate, checks gofmt on changed files (not `make fmt`), runs golangci-lint only for issues new since the base, and for the frontend runs `scripts/verify_frontend_pr.sh` (fails on a changed `src/components.d.ts`) plus a UI image build and `scripts/test_embed_nginx.py`; `check-i18n` is not run on its own (`npm test` covers `localeKeyAudit.test.ts`).
- Go: `go vet $(go list ./... | grep -v /docreader/) && go test $(go list ./... | grep -v /docreader/)`; `go test -tags desktop ./internal/container`; `go build ./cmd/server`; `golangci-lint run`; `make fmt` (go fmt).
- Model catalog: `make model-catalog-check` (= `python3 scripts/model-catalog/generate.py --check` + `go test ./internal/models/...`); regenerate with `make model-catalog-generate`.
- Frontend: `npm --prefix frontend test`, `npm --prefix frontend run type-check`, `npm --prefix frontend run build`; i18n: `npm --prefix frontend run check-i18n`.
- docreader: `uv sync --project docreader --locked`; `uv run --project docreader python -m unittest discover -s docreader/tests -p "test_*.py" -v`; Go side `go test ./docreader/client ./docreader/proto`; server `uv run --project docreader python -m docreader.main` (gRPC :50051).
- CLI/client modules: `go -C cli test -race ./... && go -C cli vet ./...`; `go -C client test ./...`. `scripts/validate.sh --area cli` also cross-compiles the six release targets with `CGO_ENABLED=0`; `--area tap` runs the tap updater tests (`python3 -I scripts/test_update_tap.py`).
- Hooks and bundles: `bash scripts/test-git-hooks.sh`; `bash scripts/check-license-bundle.sh && bash scripts/test-license-bundle.sh`; install hooks with `scripts/install-git-hooks.sh`.
- Swagger: `make docs` (regenerates `docs/`).
- Migrations: `make migrate-up|migrate-down|migrate-version|migrate-create|migrate-force|migrate-goto` (wrap `scripts/migrate.sh`).

Release

- `make bump-version [VERSION=YYYY.M.D[.MICRO]]` (also raises the Helm chart patch version, so each product release publishes a chart pinning its image tag), `make bump-lib LIB=dsh-ai-kb|extension|mcp-server|helm-chart PART=patch|minor|major`, `make check-version`.
- Cut a product release: `make bump-version`, push, `git tag -s v<ver> -m "v<ver>"` and push the tag (signed), then `gh workflow run release-lite.yml -R jrmatherly/ai-kb -f tag=v<ver>`; the run publishes the Lite tarballs, desktop apps and `aikb` archives with attestations, signs the macOS DMGs with the Developer ID certificate secrets and notarizes them once the Apple API key secrets exist (`scripts/sign-mac-app.sh`; secret names under "Signing and notarization" in `website-docs/05-clients/05-desktop.md`), and its `update-tap` job rewrites the tap `jrmatherly/homebrew-ai-kb` (no hand sync). Extension releases never take the Latest flag. Latest product release: v2026.10.20.1 (2026-10-20).

Upstream and English-only

- `.claude/skills/upstream-review/scripts/upstream-review.sh [--stat|--urls|--show <sha>|--diverged|--advance <sha>]`.
- `scripts/check-english-only.sh [-n|--summary] [path...]` exits 1 while CJK text remains; `scripts/check-emdash.sh [--cached]`; `scripts/check-workflows.sh`.
- Dataset: `scripts/validate.sh --area dataset` (12 offline unittest cases through `uv run --project dataset --locked`; deps in `dataset/pyproject.toml` + `uv.lock`, 7-day cooldown).
- Vulnerabilities: `scripts/check-govulncheck.sh [module-dir...]` (default `. cli client`; symbol-level findings against `scripts/govulncheck-allowlist.txt`, needs govulncheck and jq; `bash scripts/test-check-govulncheck.sh` tests the gate).

Darwin notes

- `sed -i ''` (BSD sed needs the empty suffix). `rg` is installed (Homebrew); prefer it over grep for the CJK scan's `--pcre2`.
- Upstream history for `git --git-dir=private/OLD-.git log ...` (archived clone, gitignored).
- Live retriever round trips (skipped unless their env var is set): Milvus `AIKB_MILVUS_TEST_ADDR=127.0.0.1:19530`, Weaviate `AIKB_WEAVIATE_TEST_HOST=127.0.0.1:18080` (plain `semitechnologies/weaviate` container), Doris `AIKB_DORIS_TEST_ADDR=doris-fe:9030` run inside a `golang` container on the compose network (`docker compose --profile doris up -d`), since the package needs cgo and the FE names `doris-be`. `go test -run TestLiveRoundTrip ./internal/application/repository/retriever/<engine>/`.
