# AI KB MCP Server

> **⚠️ Deprecated**
>
> AI KB now ships a built-in MCP server: create endpoints under "Settings → Publish & Integrations → MCP Server", each with its own token, knowledge-base scope and tool list, and connect clients to `/mcp/<endpoint_id>` over Streamable HTTP. This Python package is kept only for existing deployments and will be removed in a future release.

A Model Context Protocol (MCP) server that gives access to the AI KB knowledge management API.

## Quick start

> We recommend following the [MCP configuration guide](./MCP_CONFIG.md) instead of the steps below.

### 1. Install dependencies

```bash
pip install -r requirements.txt
```

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

### 3. Run the server

**Recommended: the main entry point:**

```bash
python main.py
```

**Other ways to run it:**

```bash
# Original launch script
python run_server.py

# Quick launch script
python run.py

# Run the server module directly
python aikb_mcp_server.py

# Run as a Python module
python -m aikb_mcp_server
```

### 4. Command-line options

```bash
python main.py --help                 # show help
python main.py --check-only           # check the environment configuration only
python main.py --verbose              # enable verbose logging
python main.py --version              # show the version
```

## Install as a Python package

### From the checkout

```bash
pip install ./mcp-server
# Or run it with uvx without installing
uvx --from ./mcp-server ai-kb-mcp-server
```

> The distribution is named **`ai-kb-mcp`** (`setup.py`, `pyproject.toml`) and is not published on PyPI, so install it from this repository.
> The MCP packages on PyPI under the upstream project's name belong to that project and read its own variable names, not `AIKB_*`.
> The command-line entry points are `ai-kb-mcp-server` and `ai-kb-server`.

### Development install

```bash
pip install -e .
```

After installation you can use the command-line tools:

```bash
ai-kb-mcp-server
# or
ai-kb-server
```

### Production install

```bash
pip install .
```

### Build distribution packages

```bash
# With setuptools
python setup.py sdist bdist_wheel

# With modern build tooling
pip install build
python -m build
```

## Test the module

Run the test script to check that the module works:

```bash
python test_module.py
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

### Chunk management

- `list_chunks` - list knowledge chunks
- `delete_chunk` - delete a knowledge chunk

## Troubleshooting

If you hit import errors, make sure that:

1. All required dependencies are installed
2. Your Python version is compatible (3.10+ recommended)
3. No file name conflicts (do not name a file `mcp.py`)

## Example output

<img width="950" height="2063" alt="118d078426f42f3d4983c13386085d7f" src="https://github.com/user-attachments/assets/09111ec8-0489-415c-969d-aa3835778e14" />

### Local upload directory boundary

All transports, including stdio, restrict local file uploads to the working
directory by default. Set `MCP_ALLOWED_UPLOAD_DIRS` to a comma-separated list of
trusted absolute directories when additional roots are needed. Starting in a
filesystem root requires an explicit directory configuration. Paths and symbolic
links resolving outside the selected roots are rejected.
