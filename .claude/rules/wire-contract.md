---
paths:
  - "internal/router/**"
  - "internal/handler/**"
  - "client/**"
  - "cli/**"
  - "frontend/src/api/**"
  - "mcp-server/**"
---

# Wire contract

An HTTP route has up to five consumers that must agree on its method, path, query and body names: the router (`internal/router`), its handler, the Go SDK (`client/`, its own module), the CLI (`cli/`, built on the SDK), the frontend (`frontend/src/api/`) and the MCP server (`mcp-server/`). Two drift bugs are already logged: `client/chunk.go:180` calls `/chunks/:id/delete-question`, which the router never registers, and `client/organization.go:367` sends `keyword` where the handler reads `q`.

- Changing a route or a parameter name: search every consumer (`rg -n '<path-fragment>|<param>' client cli frontend/src/api mcp-server`) and change them in the same commit.
- Adding a route: the handler's swagger annotations, `make docs`, and the page under `website-docs/04-api/` that documents it.
- The `wire-contract-reviewer` agent checks a diff for this; run it before committing a route change.
