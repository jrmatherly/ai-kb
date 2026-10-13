---
name: validate
description: Run the right checks for what changed in this repo (Go vet/test/lint/build, cli, client, frontend, docreader, model catalog, git hooks, license bundle, website-docs) via scripts/validate.sh. Use before every commit, after a cherry-pick batch, after a translation or rebrand batch, and whenever asked "does it pass", "run the tests", "validate", "check my changes" or "is CI green" (this copy has no CI; this script is it).
argument-hint: "[--base <rev>] [--list]"
allowed-tools: Bash(scripts/validate.sh *)
---

# Validate

`scripts/validate.sh` maps each changed file to its area and runs that area's commands, which are the archived upstream CI's (`private/OLD-.github/workflows/`). It reuses the git-hook helpers, so the Go, cli and frontend steps match `pre-push`.

```bash
scripts/validate.sh                 # working tree + untracked vs HEAD (before a commit)
scripts/validate.sh --base <rev>    # a range: a batch of commits, ends with the English-only gate on <rev>..HEAD
scripts/validate.sh --list          # which areas would run
HOOK_SKIP_TEST=1 scripts/validate.sh   # vet, lint and build without go test, for a quick loop
```

The Go area vets and tests every app package, as CI did, so it takes minutes; run it in the background and read only the `✗` lines and the last line. Report each area with its result. A failure in code you did not touch is still reported, with the failing test name, not skipped.
