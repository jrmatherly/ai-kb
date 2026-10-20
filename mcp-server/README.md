# AI KB MCP Server

> **Deprecated.** AI KB now ships a [built-in MCP server](https://ai-kb.matherly.net/docs/03-features/08-mcp): create endpoints under "Settings → Publish & Integrations → MCP Server", each with its own token, knowledge-base scope and tool list, and connect clients to `/mcp/<endpoint_id>` over Streamable HTTP. Prefer the built-in endpoint; this Python package may be removed in a future release.

A Model Context Protocol (MCP) server that gives access to the AI KB knowledge management API.

## Quick start

For MCP client JSON snippets, see the [MCP configuration guide](https://github.com/jrmatherly/ai-kb/blob/main/mcp-server/MCP_CONFIG.md).

### 1. Install

```bash
pip install ai-kb-mcp
# Or run it with uvx without installing
uvx --from ai-kb-mcp ai-kb-mcp-server
```

The distribution is **`ai-kb-mcp`**; its command-line entry points are `ai-kb-mcp-server` and `ai-kb-server`. `ai-kb-server` always runs stdio and takes no options.
The MCP packages on PyPI under the upstream project's name belong to that project and read its own variable names, not `AIKB_*`.

### 2. Set environment variables

```bash
# Linux/macOS
export AIKB_BASE_URL="http://localhost:8080/api/v1"
export AIKB_API_KEY="your_api_key_here"

# Windows PowerShell
$env:AIKB_BASE_URL="http://localhost:8080/api/v1"
$env:AIKB_API_KEY="your_api_key_here"

# Windows CMD
set AIKB_BASE_URL=http://localhost:8080/api/v1
set AIKB_API_KEY=your_api_key_here
```

Create the API key under Settings → Publish & Integrations → API Integration → API Keys.

| Variable | Default | Purpose |
| --- | --- | --- |
| `AIKB_BASE_URL` | `http://localhost:8080/api/v1` | AI KB API base URL |
| `AIKB_API_KEY` | empty | API key, sent as `X-API-Key` |
| `AIKB_CHAT_TIMEOUT` | `300` | Chat stream read timeout in seconds |
| `AIKB_VERIFY_SSL` | `true` | `false` disables TLS verification (development only) |
| `MCP_TRANSPORT` | `stdio` | `stdio`, `sse` or `http` (same as `--transport`) |
| `MCP_HOST` / `MCP_PORT` | `127.0.0.1` / `8000` | Bind address for `sse` and `http` |
| `MCP_SERVER_AUTH_TOKEN` | empty | Required for `sse` and `http`; the server refuses to start without it |
| `MCP_ALLOWED_UPLOAD_DIRS` | working directory | Comma-separated upload roots that replace the working-directory default (see below) |

### 3. Run the server

```bash
ai-kb-mcp-server                      # stdio, for MCP clients that launch the server
ai-kb-mcp-server --help               # show help
ai-kb-mcp-server --check-only         # check the environment configuration only
ai-kb-mcp-server --verbose            # enable verbose logging
ai-kb-mcp-server --version            # show the version
```

### Network transports

```bash
MCP_SERVER_AUTH_TOKEN=change_me ai-kb-mcp-server --transport http --host 0.0.0.0 --port 8000
```

The Streamable HTTP endpoint is `/mcp`; the SSE transport serves `/sse` and `/sse/messages/`.
Clients send `Authorization: Bearer <token>` or `X-MCP-Auth-Token: <token>`.

## Run from a source checkout

From the `mcp-server/` directory of the [repository](https://github.com/jrmatherly/ai-kb/tree/main/mcp-server):

```bash
pip install -r requirements.txt
python main.py                        # same options as ai-kb-mcp-server

# Or install the checkout
pip install .                         # pip install -e . for development
uvx --from . ai-kb-mcp-server         # without installing
```

`python run.py` takes the same options as `main.py`. `python aikb_mcp_server.py` and `python -m aikb_mcp_server` accept only `--transport`, `--host` and `--port`. `python run_server.py` always runs stdio and ignores options.

### Build distribution packages

```bash
uv build                              # what CI runs; or: pip install build && python -m build
```

### Test the module

```bash
python test_module.py
```

### Add a tool

1. Add the API method to the `AIKBClient` class.
2. Register a tool function with the `@mcp.tool()` decorator. The parameter type annotations generate the schema and the docstring is the description; the body calls the new client method.
3. Update the docs and tests.

## Deployment

### Docker

The bundled `Dockerfile` serves Streamable HTTP on port 8000:

```bash
docker build -t ai-kb-mcp mcp-server
docker run -e AIKB_API_KEY=your_api_key -e MCP_SERVER_AUTH_TOKEN=change_me -p 8000:8000 ai-kb-mcp
```

The image defaults to `AIKB_BASE_URL=http://app:8080/api/v1`. The host name `app` resolves only on the AI KB compose network: attach the container with `--network <compose network>` (see `docker network ls`) or pass `-e AIKB_BASE_URL=...`. The compose file also defines this image as the `mcp` service (profile `full`, host port `${MCP_PORT:-8082}`). The MCP endpoint is `http://<host>:8000/mcp`.

### systemd

Create `/etc/systemd/system/ai-kb-mcp.service`, adjusting `User` to an existing account and `ExecStart` to the installed command (`command -v ai-kb-mcp-server`):

```ini
[Unit]
Description=AI KB MCP Server
After=network.target

[Service]
Type=simple
User=aikb
WorkingDirectory=/opt/ai-kb-mcp
Environment=AIKB_BASE_URL=http://localhost:8080/api/v1
Environment=AIKB_API_KEY=your_api_key
Environment=MCP_SERVER_AUTH_TOKEN=change_me
ExecStart=/usr/local/bin/ai-kb-mcp-server --transport http --host 127.0.0.1 --port 8000
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now ai-kb-mcp
```

## Features

The MCP server provides these tools:

### Workspace management

- `create_tenant` - create a workspace
- `list_tenants` - list all workspaces

### Knowledge base management

- `create_knowledge_base` - create a knowledge base
- `list_knowledge_bases` - list knowledge bases
- `get_knowledge_base` - get knowledge base details
- `list_shared_knowledge_bases` - list knowledge bases shared with you
- `delete_knowledge_base` - delete a knowledge base
- `hybrid_search` - hybrid search

### Knowledge management

- `create_knowledge_from_file` - create knowledge from a local file
- `create_knowledge_from_url` - create knowledge from a URL
- `create_knowledge_from_text` - create knowledge from text
- `update_knowledge_from_text` - update manual Markdown knowledge, re-index it or save it as a draft
- `list_knowledge` - list knowledge
- `get_knowledge` - get knowledge details
- `delete_knowledge` - delete knowledge

### Model management

- `create_model` - create a model
- `list_models` - list models
- `get_model` - get model details

### Session management

- `create_session` - create a chat session
- `get_session` - get session details
- `list_sessions` - list sessions
- `delete_session` - delete a session

### Chat

- `chat` - send a chat message
- `agent_chat` - call an agent for multi-step retrieval and tool calls

Both chat tools assemble events on SSE blank-line boundaries, so one event can span several `data:` lines.
An event still open when the connection ends (no blank line seen yet) is dropped. Each event's data buffer is capped at 8 MiB;
exceeding the cap returns an error and closes the response connection.

### Agents

- `list_agents` - list agents
- `get_agent` - get agent details

### Chunk management

- `list_chunks` - list knowledge chunks
- `delete_chunk` - delete a knowledge chunk

### Wiki

- `wiki_search` - search a knowledge base's wiki pages
- `wiki_read_page` - read one wiki page by slug
- `wiki_index_view` - show a knowledge base's wiki index

## Troubleshooting

If you hit import errors, make sure that:

1. All required dependencies are installed
2. Your Python version is compatible (3.11 or newer)
3. No file name conflicts (do not name a file `mcp.py`)

### Local upload directory boundary

All transports, including stdio, restrict local file uploads to the working
directory by default. Set `MCP_ALLOWED_UPLOAD_DIRS` to a comma-separated list of
trusted absolute directories to replace that default; the working directory is
then allowed only if it is in the list. Starting in a
filesystem root requires an explicit directory configuration. Paths and symbolic
links resolving outside the selected roots are rejected.
