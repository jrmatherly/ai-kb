# Run the AI KB MCP server with uv

> **Deprecated.** AI KB ships a [built-in MCP server](https://ai-kb.matherly.net/docs/03-features/08-mcp) (Settings → Publish & Integrations → MCP Server). Prefer it: its connection dialog gives ready-made snippets for Cursor, VS Code, Claude Desktop and Claude Code.

For this package, we recommend `uv` for running Python-based MCP servers.

## 1. Install uv

```bash
# macOS/Linux
curl -LsSf https://astral.sh/uv/install.sh | sh

# Or with Homebrew (macOS)
brew install uv

# Windows
powershell -ExecutionPolicy ByPass -c "irm https://astral.sh/uv/install.ps1 | iex"
```

## 2. MCP client configuration

Every client takes the same server entry under `mcpServers`. Put it in the client's config file:

- Claude Desktop: `claude_desktop_config.json`
- Cursor: `~/.cursor/mcp.json` (global) or `.cursor/mcp.json` in the project
- KiloCode and other MCP clients: their MCP settings file

### From PyPI

```json
{
  "mcpServers": {
    "aikb": {
      "command": "uvx",
      "args": ["--from", "ai-kb-mcp", "ai-kb-mcp-server"],
      "env": {
        "AIKB_API_KEY": "your_api_key_here",
        "AIKB_BASE_URL": "http://localhost:8080/api/v1"
      }
    }
  }
}
```

### From a checkout

```json
{
  "mcpServers": {
    "aikb": {
      "command": "uv",
      "args": ["--directory", "/path/to/ai-kb/mcp-server", "run", "run_server.py"],
      "env": {
        "AIKB_API_KEY": "your_api_key_here",
        "AIKB_BASE_URL": "http://localhost:8080/api/v1"
      }
    }
  }
}
```

`run_server.py` always runs stdio. Both entries launch the server over stdio; to serve it over HTTP or SSE instead, see [Network transports](./README.md#network-transports).
