---
paths:
  - "**/*.md"
---

# Documentation

A claim a reader can act on (command, flag, env var, default, port, path, version, API path, image or package name) cites code that matches it. Before changing one, open the code it describes; after changing code, search every tracked `.md` file for the old value (`git grep -n '<old value>' -- '*.md'`) and run the `docs-drift-reviewer` agent on the diff.

- A doc and the code disagree: fix the doc and log a row in `private/findings/`; never change code to match a doc in the same edit.
- `third_party/` and `testdata/` documents are upstream or fixture text; `private/` is untracked.
- English only, no em dashes, markdownlint clean (the post-edit hook reports, the pre-commit hook enforces).
