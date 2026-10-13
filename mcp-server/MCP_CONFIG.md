# Run the AI KB MCP server with uv

> We recommend `uv` for running Python-based MCP servers.
>
> You can also install from PyPI with `pip install ai-kb-mcp`, or run `uvx --from ai-kb-mcp ai-kb-mcp-server` (official package `ai-kb-mcp`, maintained by [jrmatherly/ai-kb](https://github.com/jrmatherly/ai-kb)).

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

In Cursor, edit the MCP configuration file (usually `~/.cursor/mcp-config.json`):

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
