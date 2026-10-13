# Migration record for the old docs directory

Product, deployment, API and development documentation is now maintained in `website-docs/`. This record lets maintainers review the scope of the migration; it is not published on the documentation site. The migration deleted 87 old hand-written pages that had been migrated, duplicated or gone out of date, and no second copy of the text is kept. The old content can be traced in the upstream Git history; this copy's history starts after the deletion. `docs/` keeps only the engineering resources listed below.

## Content added to the new site

| Old page (relative to docs) | New location | Handling |
| --- | --- | --- |
| `QA.md`, `migration-troubleshooting.md` | [FAQ and upgrade troubleshooting](01-getting-started/05-troubleshooting.md) | Kept symptom diagnosis and recovery procedures; feature details link to existing sections. Corrected the claims that a failure always rolls back completely and that `force` can be applied mechanically |
| `paradedb-upgrade.md` | [Upgrading an existing ParadeDB database](01-getting-started/06-paradedb-upgrade.md) | Kept backup, image and extension SQL upgrade, rollback and verification steps. Historical test results are not presented as verification of this migration |
| `sandbox-cluster.md`, `sandbox-docker-backend.md`, `sandbox-desktop.md`, `sandbox-protocol.md` | [Sandbox deployment and troubleshooting](06-development/04-sandbox-deployment.md) | Kept templates, the daemon, the gateway, Redis, the desktop and snapshot boundaries. Explains that the old `local` backend was removed and how the desktop host differs from the server-side named configurations. Dropped the old Docker implementation comparison and unverified third-party deployment specifications |
| `browser-skill-integration.md`, `browser-skill-production.md` | [Local Browser: connection and deployment](05-clients/09-local-browser.md) | Added pairing, human-in-the-loop steps, task recovery, companion builds, proxy and multi-replica requirements. Distinguished from the knowledge management assistant extension |
| `agent-prompt-assembly.md` | [Conversation prompt assembly and editable scope](06-development/05-agent-prompts.md) | Kept the current assembly entry points, template saving and message boundaries. Left out the round-by-round review notes |
| `mcp-tool-directory.md` | [MCP (Model Context Protocol) integration: Managing the persistent directory](03-features/08-mcp.md#mcp-tool-directory), [API reference: agents, MCP and skills](04-api/02-api-agent-mcp.md) | Replaced the old register-everything description. Added the persistent directory, per-principal isolation, on-demand loading and the sync API. Fixed the old page's contradictory description of refresh behavior |
| `worker-pool-governance.md` | [Async task system: Capacity planning and scaling](02-architecture/05-async-tasks.md#capacity-planning) | Uses the new site's more recent queue topology. Added the retirement of the aggregate setting, capacity estimates and downstream quota boundaries |
| `embed-subdomain.md`, `embed-secure-mode.md` | [Web embed (embed channel): Separate subdomain](03-features/13-embed-channel.md#embed-subdomain) | Secure mode was already covered. Added the separate subdomain, runtime addresses and proxy policy. Did not migrate the pseudo-login check example that only tested whether a cookie or header was present |
| The Wiki page on connecting a Feishu Drive data source (under `wiki/`, integration and extension section) | [Feishu Drive](03-features/24-feishu-drive.md) | Added folder authorization and parsing modes. Following the current implementation, corrected the default export, the blocks fallback and its image conditions, deletion detection based on the old cursor, and the limits of failure recovery |
| `dev/opensearch-integration-test.md` | [Retrieval engines and vector stores: OpenSearch local testing](03-features/05-retrieval-engines.md#opensearch-local-testing) | Merged into the existing page. Added the development cluster, SSRF and the verification flow, and stated that a replica count of 0 currently falls back to 1. Cleanup now stops specific containers so other development data volumes are not deleted |
| `cloud-image/README.md`, `cloud-image/tencent-lighthouse.md` | [Development guide: Cloud image maintenance scripts](06-development/01-dev-guide.md#cloud-image-scripts) | Kept only the script boundaries and first-boot recovery; no separate deployment tutorial. Did not carry over old cloud platform quotas, prices, review times or fixed console paths |

## Already covered by the new site, not copied in full

Compared by topic. "Covered" means the working features and development entry points have a home on the new site; it does not mean every old example, screenshot or function-by-function walkthrough was kept.

| Old page | Maintained in |
| --- | --- |
| The development guide and the fast development mode notes | [Development guide](06-development/01-dev-guide.md) |
| `LITE.md` | [Installation](01-getting-started/02-installation.md), [Desktop client (AI KB Lite Desktop)](05-clients/05-desktop.md). The release package README still depends on it separately, see below |
| `CHUNKING.md` | [Chunking](03-features/04-chunking.md) |
| `KnowledgeGraph.md`, the page on enabling the knowledge graph | [Knowledge graph](03-features/09-knowledge-graph.md) |
| The page on using other vector databases | [Retrieval engines and vector stores](03-features/05-retrieval-engines.md), [Extension points](06-development/03-extension-points.md) |
| The page on adding a new web search engine | [Web search and page fetching](03-features/11-web-search.md), [Extension points](06-development/03-extension-points.md) |
| The data source import developer guide | [Data source import](03-features/10-datasource.md), [Extension points](06-development/03-extension-points.md). Cloud drive setup has its own new page |
| The IM integration developer guide | [IM integration](03-features/12-im-integration.md), [Extension points](06-development/03-extension-points.md). Old platform console screenshots and bulk permission lists are not reused as is |
| The OIDC authentication flow, RBAC and shared workspace pages | [Workspaces, users and authentication](03-features/01-tenant-auth.md), [API reference: authentication and users](04-api/02-api-auth.md), [API reference: organizations and sharing](04-api/02-api-org.md) |
| `BUILTIN_MODELS.md` | [Model management](03-features/06-models.md), [Platform administration and system administrators](03-features/20-platform-admin.md) |
| `BUILTIN_MCP_SERVICES.md`, the MCP usage guide, `zh/mcp-approval.md` | [MCP (Model Context Protocol) integration](03-features/08-mcp.md) |
| The Langfuse integration and logging configuration pages | [Observability and audit](03-features/16-observability.md), [Configuration reference](01-getting-started/04-configuration.md) |
| `agent-skills.md`, `agent-tools-design.md` | [Skills and sandboxes](03-features/22-skills-sandbox.md), [Agent engine](03-features/07-agent.md) and the new sandbox deployment page. Old tool names, the `local` configuration and the outdated non-root user and read-only image contracts were not migrated |
| `chat-steering.md` | [Conversation experience](03-features/18-chat-experience.md), [API reference: sessions, messages and chat](04-api/02-api-chat.md) |
| `client-integration-upgrade-notes.md` | [AI KB CLI (the aikb command-line tool)](05-clients/02-cli.md), [Go SDK](05-clients/03-go-sdk.md), [WeChat Mini Program client](05-clients/04-miniprogram.md), [External access to images and files](03-features/21-file-access.md) |
| `api/*.md` | [API overview](04-api/01-api-overview.md) and the topic pages in the same directory. Added the missing MCP metadata and sandbox desktop endpoints; there is no second hand-written API reference |
| Other pages in `wiki/` | Summary copies of the topics above, not migrated as a set. Their old navigation, backlinks and graph do not move to the new site |

## Not migrated as current product documentation

- `ROADMAP.md`: the old plan lists capabilities that already shipped and cannot stand as a current commitment. Its entry point now goes to the existing product introduction; a plan must be reconfirmed before it is written again.
- `code-slimming-audit.md`: a one-off code reduction audit. The text is deleted and survives only in Git history.
- `plans/2026-09-10-faq-enabled-filter-design.md`: a historical design record. The text is deleted; the final interface is covered by the FAQ and API sections, and the design process is in Git history.
- `poc/docker-sandbox/`: a separate Go module and an old Docker feasibility test, not user documentation. It could later move to a development experiments directory and stays out of the site.

## Engineering resources kept in docs

1. **Swagger generated package**: `internal/router/router.go` imports `github.com/jrmatherly/ai-kb/docs`, and the `docs` target in the `Makefile` writes its output to that directory. `docs.go` is compiled into the backend, and `swagger_contract_test.go` reads the JSON and YAML. Regular builds and CI do not regenerate them, so the three generated files and the test stay for now. They could later move to a separate generated package with the import, generation path, tests and lint exclusions updated. If the generated files stop being committed, a pinned-version generation step must first be wired into every build, test and release entry point.
2. **Lite release README**: `scripts/package-lite.sh` copies `docs/LITE.md` into the offline release package as its README (the upstream release workflow did the same). Moving it requires a README that still reads well offline; it cannot simply be replaced by a long page with relative site links.
3. **Images**: `docs/images/` keeps only what the repository README and the brand script use (`readme/`, `brand/`, `logo.png`); the upstream product screenshots nothing referenced were removed on 2026-10-13. The new site keeps its own images in `public/` and does not depend on the old directory.
4. **Historical experiment**: the separate Go module `poc/docker-sandbox/` and its run instructions are kept; they are not maintained product documentation. The old hand-written text (including the API pages, the Wiki copies, the roadmap, the audit and the design records) has been deleted.

Clickable documentation links in environment variable examples, frontend help and chunking examples, Helm install notes, example projects, code comments and the CHANGELOG now point to the new site. CHANGELOG text describing which files older versions added stays as written; it is not a current documentation entry point. `scripts/cloud-image/README.md` is reduced to a pointer to the new site so the duplicate tutorial is no longer maintained.

## Scope of this review

The migration was checked against the repository's current routes, handlers, sandbox and browser implementations, data source services, migration SQL and deploy scripts. The site ran its link check, Mermaid check and documentation build. This documentation pass did not run a production database upgrade, a cloud image cleanup, a real sandbox cluster or an IM platform integration.

## Accuracy and necessity review

The migration added six standalone pages: general troubleshooting, upgrading an existing ParadeDB database, Feishu Drive, Local Browser, sandbox deployment and prompt maintenance. They cover cross-chapter troubleshooting, a stateful upgrade, authorization and parsing choices, deploying a separate client, sandbox operations and a development contract. Existing pages keep only a summary and links.

- OpenSearch local testing was merged into the retrieval engines page and cloud image maintenance into the development guide, to avoid standalone tutorials with little content or no hands-on testing yet.
- The prompts page was checked against `prompts.go`, `observe.go`, `finalize.go` and `config/agent_prompts.go`. The same pass fixed the date format, the scope of MCP mentions and the role of the closing message in the old agent section, and removed a duplicate description of the browser's internal protocol.
- Feishu Drive was checked against the shared `core/engine.go`, `core/shared.go` and the data source service. The page no longer promises that a full sync fills in deletions. Third-party permission requests are left to the official API documentation, so no permission list that this pass did not verify on the platform is fixed in the page.
- OpenSearch was checked against the driver configuration and the index implementation. The invalid zero-replica example was removed and the old page's single-index dimension description was fixed.
- Database upgrade steps distinguish the production and development Compose files. The sandbox page distinguishes the desktop host from the three server-side named configurations. The cloud image section states the fixed systemd paths and when the first-boot marker is actually written.

These checks were against source code and configuration. They are not the same as a real platform integration or a production upgrade acceptance test.
