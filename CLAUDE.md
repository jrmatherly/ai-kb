# CLAUDE.md

Guidance to Claude Code when working in this repository.

## What this is

**AI KB**: a copy of [Tencent/WeKnora](https://github.com/Tencent/WeKnora) (knowledge-base and agent platform), taken at upstream commit `97796c637` (2026-10-12, v0.8.2 + 268 commits). Not a fork: no shared git history. Remote `origin` is `jrmatherly/ai-kb`, single branch `main`. See "Upstream" below.

**Stack** (verified in the manifests): Go 1.26 server (`cmd/server`, `internal/`; Gin, GORM, gRPC, go-redis, MinIO), Vue 3 + TypeScript + Vite frontend (`frontend/`, Less, `vue-i18n` bundles in `frontend/src/i18n/locales/`), Python 3.10+ `docreader` gRPC service (`docreader/`, managed with `uv`), separate Go modules `cli/` and `client/`, SQL migrations per backend in `migrations/{mysql,paradedb,sqlite,versioned}`, Helm chart in `helm/`, docs site in `website-docs/`.

## Deviations from upstream

The authoritative list, with each deviation's status and its effect on upstream adoption, is the "Deviations decided" table in `.claude/skills/upstream-review/SKILL.md`. In one line each:

- **English only.** No CJK text anywhere. Docs are translated page by page with the `translate-docs` skill (`.claude/skills/translate-docs/SKILL.md`: glossary, verification contract, gates); doc-vs-code findings go to `private/findings/` (untracked). `scripts/check-english-only.sh` scans tracked files (exit 1 while any remains, `--summary` for progress); the pre-commit hook runs its `--diff` mode on added lines. New code is English from the start.
- **One UI locale** (`en-US`), non-English READMEs removed, product name "AI KB", repo and Go module `jrmatherly/ai-kb`, env prefix `AIKB_`, images `ghcr.io/jrmatherly/ai-kb-*`, binary `ai-kb`, docs host `ai-kb-docs.matherly.net`, dated CalVer versioning. Decided 2026-10-12. The executable rename contract is `scripts/rebrand/map.tsv` (`apply.sh` rewrites, `check.sh` proves). Docs led: `website-docs/` is translated and rebranded (done 2026-10-12, except `website-docs/sample-data/`); code follows the same rows. Back up every deleted upstream file to `private/removed/<path>` first.
- `.github/` is removed; the archived workflows in `private/OLD-.github/` (gitignored) define the validation commands until this copy has its own.

## Upstream

Upstream changes are reviewed and adopted only through the `upstream-review` skill (`.claude/skills/upstream-review/SKILL.md`): driver script, per-file divergence markers, tracked state in `.upstream-review/`. Never `git merge upstream/main` and never a merge-base; there is no shared history. Write upstream references as `Tencent/WeKnora#NNNN`.

## Development

```bash
make dev-start && make dev-app && make dev-frontend   # infra, backend (Air hot reload), frontend (Vite)
```

Validation commands per area (Go root module with the docreader filter, cli, client, frontend, docreader, model catalog, hooks) are the table "Validation per area" in the upstream-review skill; run the rows for what you touched. Generated and committed outputs (swagger `docs/`, model catalog, docreader proto stubs) are regenerated, never merged by hand; the same skill lists the generators.

Git hooks: `scripts/install-git-hooks.sh` installs `pre-commit` (whitespace, English-only gate on added lines, gofmt, golangci-lint on changed packages) and `pre-push` (CI-aligned checks).

## Git

- Commit subjects follow upstream's conventional-commit form, `type(scope): imperative subject` (`fix(agent): ...`, `feat(gallery): ...`, `docs: ...`), single line unless a body is needed.
- `git mv`, not `mv`, for renames. Never `git stash`.
- Local commits when asked. **Never push**; the user manages remote writes.

## Writing style

- No em dashes in prose, comments, docs or commit messages. Use a plain dash or a new sentence.
- Terse and precise. A comment states a non-obvious why; history belongs in the commit message.

## Agent tooling

- Serena is active for this project (`.serena/project.yml`, Go language server). Memories in `.serena/memories/` start at `mem:core`.
- CodeGraph index in `.codegraph/` (gitignored): `codegraph_explore` before grep for code questions.
