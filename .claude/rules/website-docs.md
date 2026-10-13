---
paths:
  - "website-docs/**/*.md"
---

# Docs site

Every page under `website-docs/` was translated and verified against the code on 2026-10-12; a claim a reader can act on (env var, port, path, config key, API path, default) cites code that matches it. Keep it that way:

- Editing or porting a page: follow the `translate-docs` skill, including its gates (English-only, rebrand check, links, anchors, Mermaid, `build:docs`).
- A doc and the code disagree: fix the doc and log a row in `private/findings/`; never change code to match a doc in the same edit.
- `website-docs/sample-data/` is English (translated and renamed 2026-10-12) and pairs with the demo corpus in `examples/mcp-demo/server.py`; change them together.
