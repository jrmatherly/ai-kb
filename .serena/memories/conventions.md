# Conventions

- Language: English only, everywhere (comments, strings, prompts, docs, UI). A hit from `scripts/check-english-only.sh` on new code is a defect.
- Naming: product "AI KB", applied to docs and code 2026-10-12 (contract `scripts/rebrand/map.tsv`, exceptions `scripts/rebrand/unchanged.md`, proof `scripts/rebrand/check-repo.sh`); "WeKnora" only where it names upstream.
- Versioning: dated CalVer `YYYY.M.D[.MICRO]`, first release 2026.10.13; bump only via `make bump-version` / `make bump-lib`, check with `make check-version`; every version string is a row in `scripts/version.py`. `CHANGELOG.md` keeps upstream's entries below ours.
- Go layering: `internal/handler/*.go` (Gin handlers, one file per resource, `_test.go` beside it) -> `internal/application/service` -> `internal/application/repository`; DTOs in `internal/application/dto`; shared types `internal/types`; cross-cutting `internal/{errors,logger,middleware,tracing,utils}`.
- Go style is enforced by golangci-lint v2: gofmt + gofumpt formatting, 120-col lines, revive. Pre-commit hook rewrites gofmt and lints changed packages; `cli/**` is excluded from the root hook's gofmt pass (own module, own CI).
- Tests live next to the code (`*_test.go`; many per-feature test files per handler). Frontend tests are `*.test.ts` under `frontend/src` run by `tsx --test`. docreader tests are `docreader/tests/test_*.py` (unittest).
- Frontend i18n: every user string goes through `t('key')`; `en-US.ts` is the only bundle; a new key goes there with English text, and `localeKeyAudit.test.ts` checks message syntax and the runtime-built key trees.
- Generated files are regenerated, not edited: swagger `docs/`, model catalog, proto stubs.
- Dotfiles: root `.gitignore` ignores `.*` and re-includes a short list (`.gitignore`, `.gitkeep`, `.env.example`, `.markdownlint-cli2.jsonc`, `.shellcheckrc`, `.npmrc`, `docreader/.python-version`, `.github/`, `.claude/`, `.upstream-review/`, `.serena/`); any other tracked dotfile needs `git add -f` or a `!` line (upstream force-added ten such files, e.g. `.golangci.yml`, `.air.toml`).
- Commit subjects: conventional commits with scope, `type(scope): imperative subject` (`fix(agent)`, `feat(gallery)`, `fix(datasource/gitlab)`, `docs`, `style`, `refactor`, `test`); upstream appends `(#PR)`. Upstream issue/PR numbers quoted here are written `Tencent/WeKnora#NNNN`.
- Prose: no em dashes; terse; comments only for a non-obvious why.
- Git: `git mv` for renames; no `git stash`; local commits on request; never push.
