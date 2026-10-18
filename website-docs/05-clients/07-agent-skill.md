# Agent skill

The Agent skill gives a coding agent retrieval over your knowledge bases through the `aikb` CLI. The skills ship inside the `aikb` binary in the open Agent Skills format (a `SKILL.md` per skill), which Claude Code, Codex, Cursor and other agents read, so one install serves every agent on the machine.

## What the skills do

This section lists what an agent can do once the skills are installed.

| Skill | What it does |
| --- | --- |
| `ai-kb-shared` | The background every other skill needs: authentication, how `--kb` resolves a knowledge base, the JSON envelope, exit codes and streaming output |
| `ai-kb-rag-search` | Picks the right command for a goal: `search chunks` for ranked chunks to reason over, `chat` for a grounded answer with references, `session ask --agent` for a custom agent, `search kb` to find a knowledge base |

The skills call the CLI, so they can do only what the CLI's API key allows. Narrow the key to the knowledge bases the agent should see, as [Workspaces, users and authentication](../03-features/01-tenant-auth.md) describes.

## Install and connect

This section installs the skills and points the CLI at your instance.

Settings → Publish & Integrations → Agent skill shows the same steps and lets you copy the install command.

1. **Install and connect the CLI.** Install `aikb` and configure a profile with the server address and an API key, as the [CLI page](02-cli.md) describes. The skills run through that command and inherit its credentials.
2. **Install the skills.** Pass the skills directory your agent reads; Claude Code uses `~/.claude/skills`:

   ```bash
   aikb skills install --dir ~/.claude/skills
   ```

   The command prints what it wrote:

   ```json
   {"ok":true,"data":{"dir":"~/.claude/skills","installed":["~/.claude/skills/ai-kb-rag-search/SKILL.md","~/.claude/skills/ai-kb-rag-search/references/chat.md","~/.claude/skills/ai-kb-rag-search/references/search-chunks.md","~/.claude/skills/ai-kb-shared/SKILL.md"],"skills":["ai-kb-rag-search","ai-kb-shared"]}}
   ```

   `aikb skills install --dry-run` lists the files without writing them; `--force` overwrites an earlier install.

3. **Verify.** Ask the agent which knowledge bases it can see, then one question with references, to confirm the credentials and that the server is reachable.

## Agent skill and MCP

This section helps you choose between the skills and the built-in MCP server.

| | Agent skill | MCP server (built in) |
| --- | --- | --- |
| For | Agents that run shell commands and read `SKILL.md` (Claude Code, Codex, Cursor) | Clients that speak MCP (Claude Desktop, Cursor, Claude Code, VS Code Copilot and others) |
| Installation | `aikb skills install` into the agent's skills directory | Nothing extra to deploy. Create an endpoint under Settings → Publish & Integrations → MCP Server, and the client connects to `/mcp/<endpoint_id>` |
| Authentication | The CLI profile's API key | One token per endpoint, which can be rotated or disabled |
| Transport | The CLI calls the REST API | Streamable HTTP. Clients that only support stdio bridge through `mcp-remote` |
| Capabilities | Retrieval, grounded answers and knowledge base lookup | Chosen per endpoint: retrieval and reading, question answering, Wiki, write. Can be limited to certain knowledge bases |
| Documentation | This page and the [CLI page](02-cli.md) | [MCP integration](../03-features/08-mcp.md) |

Use the MCP server when the client speaks MCP or you want tools limited per endpoint. Use the skills when the agent works from a terminal and should decide for itself when to search.

## Related pages

- The CLI the skills run through: [CLI](02-cli.md)
- Credentials and narrowing capabilities: [Workspaces, users and authentication](../03-features/01-tenant-auth.md)
- The other way to connect: [MCP integration](../03-features/08-mcp.md)
