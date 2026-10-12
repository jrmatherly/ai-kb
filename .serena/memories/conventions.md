# Conventions

- Language: English only, everywhere (comments, strings, prompts, docs, UI). A hit from `scripts/check-english-only.sh` on new code is a defect.
- Naming (decided 2026-10-12, not yet applied): product "AI KB"; "WeKnora" only where it names upstream. Until a surface is renamed keep upstream's spelling (clean cherry-picks).
- Versioning (decided, not yet applied): dated CalVer for our releases; `CHANGELOG.md` keeps upstream's entries as history and gains ours above them.
- Go layering: `internal/handler/*.go` (Gin handlers, one file per resource, `_test.go` beside it) -> `internal/application/service` -> `internal/application/repository`; DTOs in `internal/application/dto`; shared types `internal/types`; cross-cutting `internal/{errors,logger,middleware,tracing,utils}`.
- Go style is enforced by golangci-lint v2: gofmt + gofumpt formatting, 120-col lines, revive. Pre-commit hook rewrites gofmt and lints changed packages; `cli/**` is excluded from the root hook's gofmt pass (own module, own CI).
- Tests live next to the code (`*_test.go`; many per-feature test files per handler). Frontend tests are `*.test.ts` under `frontend/src` run by `tsx --test`. docreader tests are `docreader/tests/test_*.py` (unittest).
- Frontend i18n: every user string goes through `t('key')`; `en-US.ts` is the reference bundle and `localeKeyAudit.test.ts` fails on key drift between bundles, so adding a key means adding it to every bundle still present (`regenerate-i18n-locales` helps). Target state: `en-US` is the only bundle.
- Generated files are regenerated, not edited: swagger `docs/`, model catalog, proto stubs.
- Dotfiles: root `.gitignore` ignores `.*` and re-includes a short list (`.gitignore`, `.gitkeep`, `.env.example`, `.claude/`, `.upstream-review/`, `.serena/`); any other tracked dotfile needs `git add -f` or a `!` line (upstream force-added ten such files, e.g. `.golangci.yml`, `.air.toml`).
- Commit subjects: conventional commits with scope, `type(scope): imperative subject` (`fix(agent)`, `feat(gallery)`, `fix(datasource/gitlab)`, `docs`, `style`, `refactor`, `test`); upstream appends `(#PR)`. Upstream issue/PR numbers quoted here are written `Tencent/WeKnora#NNNN`.
- Prose: no em dashes; terse; comments only for a non-obvious why.
- Git: `git mv` for renames; no `git stash`; local commits on request; never push.
