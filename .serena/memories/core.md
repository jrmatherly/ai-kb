# AI KB (ai-kb, copy of Tencent/WeKnora)

- Product name "AI KB" (rebrand applied 2026-10-12, contract `scripts/rebrand/map.tsv`; "WeKnora" remains only where it names upstream). Copy of `Tencent/WeKnora@97796c637` (2026-10-12, v0.8.2+268), identical at the copy except `.github/` (removed) and `.gitignore`. No shared git history with upstream. Remote `origin` = `jrmatherly/ai-kb`, branch `main`. Provenance, markers, decided deviations and the review workflow: `mem:upstream`.
- Standing deviation: **English only**. No CJK text in code, comments, strings, prompts, docs, UI. Scanner and the adoption rules that follow from it: `mem:upstream`. The translation pass and the removal of the non-English UI bundles and READMEs landed 2026-10-12; `scripts/check-english-only.sh` keeps it that way.
- `CLAUDE.md` (= `AGENTS.md` symlink) written 2026-10-12 for this copy.
- Source map (root Go module `github.com/jrmatherly/ai-kb`):
  - `cmd/server` entry (`main.go`, `bootstrap.go`); other binaries `cmd/desktop`, `cmd/download`, `cmd/milvus-migrate`.
  - `internal/handler` HTTP handlers (Gin) -> `internal/application/service` -> `internal/application/repository`; `internal/types` shared types, `internal/infrastructure/{chunker,docparser,web_fetch,web_search}`, `internal/models` (provider/protocol/catalog layers; generated catalog under `internal/models/catalog/`), `internal/agent`, `internal/mcp`, `internal/mcpserver`, `internal/sandbox`, `internal/localsandbox`, `internal/router`, `internal/middleware`.
  - Separate Go modules: `cli/` (own AGENTS.md, CHANGELOG, Makefile), `client/` (Go SDK), `docreader/` client+proto packages (Go) beside the Python service, `third_party/anydoc-go/`.
  - `frontend/` Vue 3 + Vite + TS, `frontend/src/i18n/locales/en-US.ts`, the only message bundle (key-usage audit `localeKeyAudit.test.ts`). Details: `mem:frontend/core`.
  - `docreader/` Python gRPC document parser (port 50051), `uv`-managed. Details: `mem:docreader/core`.
  - `migrations/{mysql,paradedb,sqlite,versioned}`, `config/*.yaml` (builtin agents, agent type presets, models overlay), `helm/`, `docker/`, `docker-compose*.yml`, `website-docs/` docs site, `docs/` swagger output + poc.
- Generated and committed (regenerate, never hand-merge): `docs/docs.go`, `docs/swagger.{json,yaml}` (`make docs`), model catalog (`make model-catalog-generate`), docreader proto stubs (`make -C docreader proto`).
- Build/test/lint stack: `mem:tech_stack`. Commands: `mem:suggested_commands`. Style and layering: `mem:conventions`. Done criteria: `mem:task_completion`.
