# Claw Skill

Claw Skill gives agents in the OpenClaw ecosystem access to AI KB. Through the REST API it uploads documents, imports web pages and runs retrieval across knowledge bases.

The skill is hosted on ClawHub as [`@lyingbug/weknora`](https://clawhub.ai/lyingbug/weknora) under the MIT-0 license. What it can call, and with which permissions, is decided by the AI KB API it connects to.

## Features

This section lists what an OpenClaw agent can do with the skill.

| Capability | What it does |
| --- | --- |
| Upload files | Sends PDF, Word, Excel and other documents to a knowledge base, where they are parsed and embedded automatically |
| Import web pages | Fetches the main content of a URL into a knowledge base and can poll the parsing status |
| Write Markdown | Creates or edits a knowledge item as Markdown, suited to meeting notes and structured notes |
| Hybrid retrieval | `hybrid-search` within one knowledge base and `knowledge-search` across knowledge bases, with vector and keyword recall |
| Browse knowledge bases | Lists knowledge bases and their items and shows details |

## Install and connect

This section installs the skill and points it at your instance.

Settings → Publish & Integrations → Claw Skill shows the install steps and lets you copy the current instance's API address, an environment variable example and the install command.

1. **Get API credentials.** Copy the API key and API address from Settings → Publish & Integrations → API Integration. The Open API Info button on the setup page goes straight there.
2. **Set the environment variables** in your terminal or in `~/.zshrc` / `~/.bashrc`:

   ```bash
   export WEKNORA_BASE_URL=https://your-server.example.com/api/v1
   export WEKNORA_API_KEY=sk-xxxxx
   ```

   `WEKNORA_BASE_URL` must include `/api/v1`. The example on the setup page already has the current instance's address filled in. The skill is published outside this project and reads these variable names, so they keep the `AIKB_` prefix.

3. **Install the skill.** Run the command below where the OpenClaw CLI is installed, or follow the instructions on the ClawHub page:

   ```bash
   openclaw skills install @lyingbug/weknora
   ```

4. **Verify.** Ask the agent to list the knowledge bases or run one search to confirm that the credentials work and the instance is reachable.

## Claw Skill and MCP

This section helps you choose between Claw Skill and the built-in MCP server.

External agents can call AI KB through either one. The choice depends on how the client connects and which capabilities you need:

| | Claw Skill | MCP server (built in) |
| --- | --- | --- |
| For | Agents in the OpenClaw / ClawHub ecosystem | Clients that speak MCP (Claude Desktop, Cursor, Claude Code, VS Code Copilot and others) |
| Installation | Install the skill from ClawHub | Nothing extra to deploy. Create an endpoint under Settings → Publish & Integrations → MCP Server, and the client connects to `/mcp/<endpoint_id>` |
| Authentication | Workspace API key (`WEKNORA_API_KEY`) | One token per endpoint, which can be rotated or disabled |
| Transport | Calls the REST API directly | Streamable HTTP. Clients that only support stdio bridge through `mcp-remote` |
| Capabilities | Import, retrieval and browsing (5 kinds) | Chosen per endpoint: retrieval and reading, question answering (the endpoint's default agent), Wiki, write. Can be limited to certain knowledge bases |
| Documentation | This page | [MCP integration](../03-features/08-mcp.md) |

Use the MCP server when you need question answering or the Wiki tools, or want to limit tools and knowledge bases per endpoint. Use Claw Skill to import, search and browse material from the OpenClaw ecosystem. The Python MCP service under `mcp-server/` in the repository is deprecated. Use the built-in MCP server for new integrations.

## Related pages

These pages cover narrowing credentials, the underlying API and the other ways to connect.

- Credentials and narrowing capabilities: [Workspaces, users and authentication](../03-features/01-tenant-auth.md)
- The underlying API: [API overview](../04-api/01-api-overview.md)
- Other integrations: [Chrome extension](06-chrome-extension.md), [MCP integration](../03-features/08-mcp.md)
