---
name: fix-finding
description: Fix one CODE-BUG row from the docs verification log (private/findings/2026-10-12-website-docs.md) - reproduce with a failing test, fix at the root, update the doc page and the row.
argument-hint: "<row number>"
disable-model-invocation: true
---

# Fix a finding

The docs pass logged code that disagrees with its intended behavior as CODE-BUG rows in `private/findings/2026-10-12-website-docs.md` (untracked). Each row names the doc page, the claim, the `path:line` evidence and what the doc now says (the intended behavior plus one sentence on the actual behavior).

1. Read the row, then the code at its evidence lines. Confirm it is still a bug at HEAD; if not, mark the row and stop.
2. Reproduce with a test that calls the code the way its users do (an HTTP handler through its router, the SDK method against the route) and asserts the intended result as a literal. It must fail before the fix.
3. Fix at the root. Find every caller of the function you change; one guard in the shared function beats one per caller. Route or parameter mismatches touch every consumer (`.claude/rules/wire-contract.md`); run the `wire-contract-reviewer` agent on the diff.
4. `scripts/validate.sh` passes.
5. On the doc page, delete the sentence that describes the buggy behavior; the page should now just state the intended behavior. Run the page's gates (`translate-docs` skill, step 8).
6. In the findings row, set the verdict to `FIXED <short-sha>` after committing. Commit subject `fix(<scope>): <what now works>`.

One row per commit. If the fix turns out to be a product decision (the doc's intended behavior might be the wrong one), stop and ask.
