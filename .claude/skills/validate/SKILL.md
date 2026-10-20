---
name: validate
description: Run the checks for what changed in this repo through scripts/validate.sh, the local equivalent of the per-area CI workflows (`--list` prints the areas a change touches; the English-only, em dash and version gates always run). Use before every commit, after a cherry-pick batch, after a translation or rebrand batch, and whenever asked "does it pass", "run the tests", "validate", "check my changes" or "is CI green".
argument-hint: "[--base <rev>] [--list] [--area <name>]"
allowed-tools: Bash(scripts/validate.sh *)
---

# Validate

`scripts/validate.sh` maps each changed file to its area, using the `paths:` filters of our workflows in `.github/workflows/` (ported from upstream's), and runs that area's checks from those workflows. The `pre-push` hook runs it on everything since the push base.

```bash
scripts/validate.sh                 # working tree + untracked vs HEAD (before a commit)
scripts/validate.sh --base <rev>    # a range: a batch of commits, ends with the English-only gate on <rev>..HEAD
scripts/validate.sh --list          # which areas would run
scripts/validate.sh --area helm     # only that area (repeatable), whatever changed
HOOK_SKIP_TEST=1 scripts/validate.sh   # skip the Go tests for a quick loop; everything else still runs
```

Every touched area runs even when an earlier one fails. The run ends with a summary that lists each area as passed, failed or skipped, plus every check that did not run and why. The exit status is 1 when any area failed. A whole run over a large range takes tens of minutes, so run it in the background and read the summary. The Go suite takes minutes, the UI image build several, and the dsh plugin's first end-to-end run installs the harness (about 15 minutes; later runs reuse `~/.cache/ai-kb/`). Report each area with its result, and name every skipped check. A failure in code you did not touch is still reported, with the failing test name, not skipped.

The script installs what the checks need, and does nothing when it is already current: `npm ci` when `node_modules` is missing or older than the lockfile (never through a symlinked `node_modules`), `uv sync --locked`, and Playwright's WebKit (on Linux with `--with-deps` only as root). It does not install toolchains. A missing `golangci-lint`, `cargo`/`rustc`/`cargo-audit` (rustup puts them in `~/.cargo/bin`, which the anydoc area adds to `PATH`), `uv`, `node`/`npm`, `python3`, `helm`, `docker` (or a stopped Docker daemon), `pnpm`, `npx` or `shellcheck` skips that check, and the summary lists it.

The `markdown` area runs when any `.md` file or `.markdownlint-cli2.jsonc` changed, and lints the whole tree with the markdownlint-cli2 version pinned in `scripts/git-hooks/common.sh` (`MARKDOWNLINT_CLI2`, run through `npx --yes`). Fix its findings by hand or with `npx --yes markdownlint-cli2@<pin> --fix <file>`, then read the diff: the fixer breaks a wrapped line that starts with `#` (MD018), an ordered list continued after an unindented code block (MD029; indent the block under its item), `*` in a cron string (MD037; use a code span) and code spans whose edge spaces matter (MD038; reword). The pre-commit hook lints staged `.md` files and the post-edit hook reports findings in an edited one; neither fixes.

Not run locally: image pushes (`docker-image.yml`), the GHCR chart push (`helm.yml`; the chart lint and render tests do run), the store publish jobs (`extension.yml`), release binaries (`release-lite.yml`; its tap updater is tested locally by the `tap` area, `python3 -I scripts/test_update_tap.py`, and the `cli` area cross-compiles the six release targets with cgo off), the PyPI and npm publish jobs (`mcp-server.yml`, `dsh-plugin.yml`), the advisory weekly dsh@latest drift job, and `go-lint-cache.yml`, which only warms a CI cache. The CLI e2e suite (`cli-e2e.yml`) runs only with `AIKB_E2E_HOST` and `AIKB_E2E_TOKEN` set to a live server. OS and Python matrices run on this machine's platform and Python only.
