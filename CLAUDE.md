# CLAUDE.md

Guidance to Claude Code when working in this repository.

## What this is

**AI KB**: a copy of [Tencent/WeKnora](https://github.com/Tencent/WeKnora) (knowledge-base and agent platform), taken at upstream commit `97796c637` (2026-10-12, v0.8.2 + 268 commits). Not a fork: no shared git history. Remote `origin` is `jrmatherly/ai-kb`, single branch `main`. See "Upstream" below.

**Stack** (verified in the manifests): Go 1.26 server (`cmd/server`, `internal/`; Gin, GORM, gRPC, go-redis, MinIO), Vue 3 + TypeScript + Vite frontend (`frontend/`, Less, `vue-i18n` bundles in `frontend/src/i18n/locales/`), Python 3.14 `docreader` gRPC service (`docreader/`, managed with `uv`), separate Go modules `cli/` and `client/`, Python `mcp-server/` (deprecated), npm `packages/dsh-ai-kb/`, `miniprogram/`, the uv project `dataset/`, SQL migrations per backend in `migrations/{mysql,paradedb,sqlite,versioned}`, Helm chart in `helm/`, docs site in `website-docs/`.

## Deviations from upstream

The authoritative list, with each deviation's status and its effect on upstream adoption, is the "Deviations decided" table in `.claude/skills/upstream-review/SKILL.md`. In one line each:

- **English only.** No CJK text anywhere. Docs are translated page by page with the `translate-docs` skill (`.claude/skills/translate-docs/SKILL.md`: glossary, verification contract, gates); doc-vs-code findings go to `private/findings/` (untracked). `scripts/check-english-only.sh` scans tracked files (exit 1 while any remains, `--summary` for progress); the pre-commit hook runs its `--diff` mode on added lines. New code is English from the start.
- **One UI locale** (`en-US`), non-English READMEs removed, product name "AI KB", repo and Go module `jrmatherly/ai-kb`, env prefix `AIKB_`, images `ghcr.io/jrmatherly/ai-kb-*`, binary `ai-kb`, docs at `ai-kb.matherly.net/docs/` (same host as the app, path-mounted), dated CalVer versioning. Decided 2026-10-12. The rename landed in the docs and the code on 2026-10-12, lowercase identifiers included (CLI `aikb`, Helm chart `ai-kb`, database `ai_kb`, headers `X-AIKB-*`): contract `scripts/rebrand/map.tsv`, proof `scripts/rebrand/check-repo.sh`, exceptions (WeKnoraCloud, BrowserSkill, history, generated swagger) in `scripts/rebrand/unchanged.md`. Back up every deleted upstream file to `private/removed/<path>` first.
- **Versions** are dated CalVer `YYYY.M.D[.MICRO]` unpadded, tags `vYYYY.M.D[.MICRO]`, first release `2026.10.13`. Bump only with `make bump-version` / `make bump-lib`, check with `make check-version`; every version string is a row in `scripts/version.py`.
- `.github/` is our own CI, ported from upstream's workflows (archived in `private/OLD-.github/`, gitignored) and rebranded: GHCR images (plus a weekly Monday rebuild), tag-only PyPI/npm publishing, actions SHA-pinned. `scripts/check-workflows.sh` (run by `guards.yml`) enforces SHA pins with `# tag` comments, top-level `concurrency` and emoji-led workflow, job and step names. A workflow command change goes into `scripts/validate.sh` too.

## Upstream

Upstream changes are reviewed and adopted only through the `upstream-review` skill (`.claude/skills/upstream-review/SKILL.md`): driver script, per-file divergence markers, tracked state in `.upstream-review/`. Never `git merge upstream/main` and never a merge-base; there is no shared history. Write upstream references as `Tencent/WeKnora#NNNN`.

## Development

```bash
make dev-start && make dev-app && make dev-frontend   # infra, backend (Air hot reload), frontend (Vite)
```

Validation commands per area (Go root module with the docreader filter, cli, client, frontend, docreader, model catalog, hooks, start-all, shell, licenses, anydoc, Helm, mcp-server, dsh plugin, extension, dataset, miniprogram, website-docs, Markdown) are `scripts/validate.sh` (`--base <rev>` for a range, `--list` to print the areas a change touches, `--area <name>` to force one; it always ends with the English-only, em dash and version gates); the `validate` skill says when to run it. Generated and committed outputs (swagger `docs/`, model catalog, docreader proto stubs) are regenerated, never merged by hand: the upstream-review skill lists the generators and a PreToolUse hook blocks hand edits to them.

Git hooks: `scripts/install-git-hooks.sh` installs `pre-commit` (whitespace, English-only gate on added lines, em dash gate on the index, gofmt, golangci-lint on changed packages, markdownlint on staged Markdown) and `pre-push` (CI-aligned checks).

## Git

- Commit subjects follow upstream's conventional-commit form, `type(scope): imperative subject` (`fix(agent): ...`, `feat(gallery): ...`, `docs: ...`), single line unless a body is needed.
- `git mv`, not `mv`, for renames. Never `git stash`.
- Local commits when asked. **Never push**; the user manages remote writes.
- A fresh clone has two remotes and `gh` picks `upstream` (Tencent/WeKnora) for `gh repo view`, `gh pr` and `gh run` until `gh repo set-default jrmatherly/ai-kb` is run once per clone (`gh variable` and `gh secret` resolve to `origin` either way). Pass `-R jrmatherly/ai-kb` when unsure.

## Writing style

- No em dashes in prose, comments, docs or commit messages. Use a plain dash or a new sentence. `scripts/check-emdash.sh` gates tracked files (validate, pre-commit); code that must match the character spells it as an escape, and history, `third_party/` and generated Swagger are in `scripts/emdash-allowlist.txt`.
- Terse and precise. A comment states a non-obvious why; history belongs in the commit message.

## Agent tooling

- Serena is active for this project (`.serena/project.yml`, Go language server). Memories in `.serena/memories/` start at `mem:core`.
- CodeGraph index in `.codegraph/` (gitignored): `codegraph_explore` before grep for code questions.
