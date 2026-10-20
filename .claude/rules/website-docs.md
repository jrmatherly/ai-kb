---
paths:
  - "website-docs/**/*.md"
---

# Docs site

Every page under `website-docs/` was translated from upstream and verified against the code (2026-10-12), and pages added or changed since keep that contract. Beyond the documentation rule:

- Editing or porting a page: follow the `translate-docs` skill, including its gates (English-only, rebrand check, links, anchors, Mermaid, `build:docs`).
- `website-docs/sample-data/` is English (translated and renamed 2026-10-12) and pairs with the demo corpus in `examples/mcp-demo/server.py`; change them together.
