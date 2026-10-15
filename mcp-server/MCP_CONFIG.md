# Run the AI KB MCP server with uv

> Prefer the built-in endpoint (Settings → Publish & Integrations → MCP Server). Its connection dialog gives ready-made snippets for Cursor, VS Code, Claude Desktop and Claude Code.
>
> For this package, we recommend `uv` for running Python-based MCP servers.
>
> You can also install the package from the checkout with `pip install ./mcp-server`, or run `uvx --from ./mcp-server ai-kb-mcp-server`. The `ai-kb-mcp` distribution is not published on PyPI.

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

### Claude Desktop

Add this to the Claude Desktop settings:

```json
{
  "mcpServers": {
    "aikb": {
      "args": [
        "--directory",
        "/path/ai-kb/mcp-server",
        "run",
        "run_server.py"
      ],
      "command": "uv",
      "env": {
        "AIKB_API_KEY": "your_api_key_here",
        "AIKB_BASE_URL": "http://localhost:8080/api/v1"
      }
    }
  }
}
```

### Cursor

In Cursor, edit `~/.cursor/mcp.json` (global) or `.cursor/mcp.json` in the project:

```json
{
  "mcpServers": {
    "aikb": {
      "command": "uv",
      "args": [
        "--directory",
        "/path/ai-kb/mcp-server",
        "run",
        "run_server.py"
      ],
      "env": {
        "AIKB_API_KEY": "your_api_key_here",
        "AIKB_BASE_URL": "http://localhost:8080/api/v1"
      }
    }
  }
}
```

### KiloCode

For KiloCode or other editors that support MCP:

```json
{
  "mcpServers": {
    "aikb": {
      "command": "uv",
      "args": [
        "--directory",
        "/path/ai-kb/mcp-server",
        "run",
        "run_server.py"
      ],
      "env": {
        "AIKB_API_KEY": "your_api_key_here",
        "AIKB_BASE_URL": "http://localhost:8080/api/v1"
      }
    }
  }
}
```

### Other MCP clients

For a generic MCP client:

```json
{
  "mcpServers": {
    "aikb": {
      "command": "uv",
      "args": [
        "--directory",
        "/path/ai-kb/mcp-server",
        "run",
        "run_server.py"
      ],
      "env": {
        "AIKB_API_KEY": "your_api_key_here",
        "AIKB_BASE_URL": "http://localhost:8080/api/v1"
      }
    }
  }
}
```
