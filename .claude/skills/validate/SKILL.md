---
name: validate
description: Run the right checks for what changed in this repo (Go vet/test/lint/build, cli, client, frontend and its UI image, docreader, model catalog, git hooks, license bundle, anydoc, helm, mcp-server, dsh plugin, miniprogram, website-docs) via scripts/validate.sh. Use before every commit, after a cherry-pick batch, after a translation or rebrand batch, and whenever asked "does it pass", "run the tests", "validate", "check my changes" or "is CI green" (this copy has no CI; this script is it).
argument-hint: "[--base <rev>] [--list] [--area <name>]"
allowed-tools: Bash(scripts/validate.sh *)
---

# Validate

`scripts/validate.sh` maps each changed file to its area, using the `paths:` filters of the archived upstream workflows (`private/OLD-.github/workflows/`), and runs that area's checks from those workflows. The `pre-push` hook runs it on everything since the push base.

```bash
scripts/validate.sh                 # working tree + untracked vs HEAD (before a commit)
scripts/validate.sh --base <rev>    # a range: a batch of commits, ends with the English-only gate on <rev>..HEAD
scripts/validate.sh --list          # which areas would run
scripts/validate.sh --area helm     # only that area (repeatable), whatever changed: what a CI job calls
HOOK_SKIP_TEST=1 scripts/validate.sh   # skip the Go tests for a quick loop; everything else still runs
```

Every touched area runs even when an earlier one fails. The run ends with a summary that lists each area as passed, failed or skipped, plus every check that did not run and why. The exit status is 1 when any area failed. A whole run over a large range takes tens of minutes, so run it in the background and read the summary. The Go suite takes minutes, the UI image build several, and the dsh plugin's first end-to-end run installs the harness (about 15 minutes; later runs reuse `~/.cache/ai-kb/`). Report each area with its result, and name every skipped check. A failure in code you did not touch is still reported, with the failing test name, not skipped.

The script installs what the checks need, and does nothing when it is already current: `npm ci` when `node_modules` is missing or older than the lockfile (never through a symlinked `node_modules`), `uv sync --locked`, and Playwright's WebKit (on Linux with `--with-deps` only as root). It does not install toolchains. A missing `golangci-lint`, `cargo`/`cargo-audit` (rustup puts them in `~/.cargo/bin`, which the anydoc area adds to `PATH`), `helm`, `docker` (or a stopped Docker daemon) or `pnpm` skips that check, and the summary lists it.

Not run locally: image pushes (`docker-image.yml`), release binaries (`release-lite.yml`), the PyPI and npm publish jobs (`mcp-server.yml`, `dsh-plugin.yml`), the advisory weekly dsh@latest drift job, and `go-lint-cache.yml`, which only warms a CI cache. The CLI e2e suite (`cli-e2e.yml`) runs only with `AIKB_E2E_HOST` and `AIKB_E2E_TOKEN` set to a live server. OS and Python matrices run on this machine's platform and Python only.
