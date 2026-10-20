# Changelog

All notable changes to this project are recorded in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Deprecated
- The whole package, in favor of the [built-in MCP server](https://ai-kb.matherly.net/docs/03-features/08-mcp) (Settings → Publish & Integrations → MCP Server, served at `/mcp/<endpoint_id>`). A future release may remove it.

### Removed
- `create_model` no longer takes `is_default`. The server's create-model request has no such field, so the flag was always ignored.

## [1.1.2] - 2026-10-15

1.1.2 changes only the docs. The 1.1.1 upload to PyPI (2026-10-15), the first under the `ai-kb-mcp` name, already shipped this section's Added entries and the AI KB rename.

### Added
- `create_knowledge_from_text` tool: creates a knowledge entry from manual Markdown text through the existing `/knowledge-bases/{id}/knowledge/manual` endpoint, completing the "text" part of Tencent/WeKnora#323. Defaults to `status="publish"`, so the entry enters parsing and indexing on creation and becomes searchable. Pass `status="draft"` to save it without indexing.
- `update_knowledge_from_text` tool: updates manual Markdown knowledge through the existing `PUT /knowledge/manual/{id}` endpoint. Re-indexes by default and can save as a draft instead, completing Tencent/WeKnora#2378.
- The README tool list now includes the existing `create_knowledge_from_file`.

### Changed
- The README links to the MCP configuration guide by absolute URL, so the link works on PyPI, and leads with the `pip install ai-kb-mcp` install. Source-checkout steps moved to their own section.
- The docs no longer say the package is unpublished.
- Renamed for AI KB: distribution `ai-kb-mcp`, module `aikb_mcp_server`, commands `ai-kb-mcp-server` / `ai-kb-server`, environment variables `AIKB_BASE_URL`, `AIKB_API_KEY`, `AIKB_CHAT_TIMEOUT` and `AIKB_VERIFY_SSL`. The SSE and HTTP transports require `MCP_SERVER_AUTH_TOKEN`.
- Entries from 1.1.1 down describe the upstream Tencent/WeKnora package and its names.

## [1.1.1] - 2026-07-30

### Changed
- The official PyPI package name is now **`tencent-weknora-mcp`** (published by the Tencent/WeKnora repository CI through Trusted Publishing).
  The former community package `weknora-mcp` is not officially maintained. Update your install commands.
- Project URLs point to the official [Tencent/WeKnora](https://github.com/Tencent/WeKnora) repository (the `mcp-server/` directory).

### Fixed
- The HTTP transport restores `stateless_http=True`, matching the pre-migration `StreamableHTTPSessionManager(stateless=True)` behavior.
- The SSE transport restores the message endpoint `/sse/messages/`, matching the pre-migration route.
- `WeKnoraClient` uses a thread-local `requests.Session`, avoiding Session races when MCP 2.x calls tools concurrently from a thread pool.
- File upload (`create_knowledge_from_file`) honors the `WEKNORA_VERIFY_SSL` setting.

### Notes
- When a tool fails, MCPServer 2.x returns `CallToolResult(isError=True)` (`ToolError`),
  instead of the old low-level API's successful response with an `"Error executing …"` prefix in a text block.
  Clients that only parse `content[0].text` usually see no difference. Integrations that rely on the `isError` flag now behave closer to the MCP spec.

## [1.1.0] - 2026-07-30

### Fixed
- Fixed the server crashing on startup under MCP Python SDK 2.x (`AttributeError: 'Server' object has no attribute 'list_tools'`).
  SDK 2.0 removed the low-level `Server` decorator API (`@app.list_tools()` / `@app.call_tool()` / `app.get_capabilities()`),
  and launching the published package through `uvx` resolves to the latest 2.x, so the connection was closed.

### Changed
- Migrated the MCP server implementation from the low-level `Server` to the high-level `MCPServer` API (mcp 2.x, FastMCP renamed).
  - The 28 tools are rewritten as `@mcp.tool()` function signatures: input parameters use type annotations (the framework generates the schema),
    descriptions live in docstrings, and the framework serializes the plain Python return values.
  - The transport layer now uses `run_stdio_async()` / `sse_app()` / `streamable_http_app()`. Authentication is still wrapped by `MCPAuthMiddleware`.
  - `WeKnoraClient` business logic (REST/SSE calls, resolve_*, wiki) is unchanged.
- The dependency bound is now `mcp>=2,<3` (the published package matches the development environment, so an unbounded range can no longer resolve to a future breaking release).

### Notes
- This release requires `mcp>=2` at runtime. If you temporarily need the old SDK, add `--with "mcp<2"` to the launch command, but upgrading to this release is recommended.

## [1.0.1] - 2026-07-28

### Fixed
- Entry scripts (`run_server.py`, `main.py`, `run.py`) now write diagnostics to stderr, so they no longer corrupt the MCP stdio stream and make the client fail to start
- Fixed the `ModuleNotFoundError` caused by the wheel missing `upload_paths.py`
- `__init__.py` now uses absolute imports, fixing package import errors when unittest/pytest collect tests

### Changed
- The PyPI distribution name is now `weknora-mcp` (the command-line entry points `weknora-mcp-server` / `weknora-server` are unchanged)
- Added a CI workflow (`.github/workflows/mcp-server.yml`). Release tags use the format `mcp-server-v*`

## [1.0.0] - 2024-01-XX

### Added
- Initial release
- WeKnora MCP Server core features
- Full WeKnora API integration
- Workspace management tools
- Knowledge base management tools
- Knowledge management tools
- Model management tools
- Session management tools
- Chat tools
- Chunk management tools
- Several launch methods
- Command-line arguments
- Environment variable configuration
- Full package install support
- Development and production modes
- Detailed docs and installation guide

### Tool list
- `create_tenant` - create a workspace
- `list_tenants` - list all workspaces
- `create_knowledge_base` - create a knowledge base
- `list_knowledge_bases` - list knowledge bases
- `get_knowledge_base` - get knowledge base details
- `delete_knowledge_base` - delete a knowledge base
- `hybrid_search` - hybrid search
- `create_knowledge_from_url` - create knowledge from a URL
- `list_knowledge` - list knowledge
- `get_knowledge` - get knowledge details
- `delete_knowledge` - delete knowledge
- `create_model` - create a model
- `list_models` - list models
- `get_model` - get model details
- `create_session` - create a chat session
- `get_session` - get session details
- `list_sessions` - list sessions
- `delete_session` - delete a session
- `chat` - send a chat message
- `list_chunks` - list knowledge chunks
- `delete_chunk` - delete a knowledge chunk

### File layout
```
WeKnora/mcp-server/
├── __init__.py              # package init
├── main.py                  # main entry point (recommended)
├── run.py                   # quick launch script
├── run_server.py           # original launch script
├── weknora_mcp_server.py   # MCP server implementation
├── test_module.py          # module test script
├── requirements.txt        # dependency list
├── setup.py               # install script (traditional)
├── pyproject.toml         # modern project configuration
├── MANIFEST.in            # package file manifest
├── LICENSE                # MIT license
├── README.md              # project overview
├── INSTALL.md             # detailed installation guide
└── CHANGELOG.md           # changelog
```

### Launch methods
1. `python main.py` - main entry point (recommended)
2. `python run_server.py` - original launch script
3. `python run.py` - quick launch script
4. `python weknora_mcp_server.py` - run directly
5. `python -m weknora_mcp_server` - run as a module
6. `weknora-mcp-server` - command-line tool after installation
7. `weknora-server` - command-line tool after installation (alias)

### Technical features
- Built on Model Context Protocol (MCP) 1.0.0+
- Async I/O
- Full error handling
- Detailed logging
- Environment variable configuration
- Command-line arguments
- Several install methods
- Development and production modes
- Full test coverage

### Dependencies
- Python 3.10+
- mcp >= 1.0.0
- requests >= 2.31.0

### Compatibility
- Windows, macOS, Linux
- Python 3.10-3.12
- Works with modern Python package managers
