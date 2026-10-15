# AI KB MCP Server runnable package - project summary

> **Deprecated.** AI KB ships a built-in MCP server; see [README.md](./README.md). This package is published on PyPI as `ai-kb-mcp`.

## 📁 Project layout

```text
ai-kb/mcp-server/
├── 📦 Core files
│   ├── __init__.py              # package init
│   ├── aikb_mcp_server.py   # MCP server implementation
│   ├── upload_paths.py         # local upload directory boundary
│   └── requirements.txt        # project dependencies
│
├── 🚀 Launch scripts (several options)
│   ├── main.py                 # main entry point (recommended) ⭐
│   ├── run_server.py          # original launch script
│   └── run.py                 # quick launch script
│
├── 📋 Configuration
│   ├── setup.py               # traditional install script
│   ├── pyproject.toml         # modern project configuration
│   ├── uv.lock                # locked dependencies
│   ├── MANIFEST.in            # package file manifest
│   └── Dockerfile             # container image (Streamable HTTP)
│
├── 🧪 Tests
│   ├── test_module.py         # module tests
│   ├── test_*.py, tests/      # unit tests
│   └── check_imports.py       # manual import check
│
├── 📚 Docs
│   ├── README.md              # project overview
│   ├── INSTALL.md             # detailed installation guide
│   ├── MCP_CONFIG.md          # MCP client configuration
│   ├── EXAMPLES.md            # usage examples
│   ├── CHANGELOG.md           # changelog
│   ├── PROJECT_SUMMARY.md     # project summary (this file)
│   └── LICENSE                # MIT license
```

## 🚀 Ways to launch (7)

### 1. Main entry point (recommended) ⭐

```bash
python main.py                    # basic start
python main.py --check-only       # check the environment only
python main.py --verbose          # verbose logging
python main.py --help            # show help
python main.py --transport http  # Streamable HTTP (needs MCP_SERVER_AUTH_TOKEN)
```

### 2. Original launch script

```bash
python run_server.py
```

### 3. Quick launch script

```bash
python run.py
```

### 4. Run the server directly

```bash
python aikb_mcp_server.py
```

### 5. Run as a module

```bash
python -m aikb_mcp_server
```

### 6. Command-line tools after installation

```bash
pip install -e .                  # development install
ai-kb-mcp-server               # main command
ai-kb-server                   # alias
```

### 7. Production install

```bash
pip install .                    # production install
ai-kb-mcp-server              # global command
```

## 🔧 Environment configuration

See the environment variable table in [README.md](./README.md#2-set-environment-variables).

## 🛠️ Features

See the tool list in [README.md](./README.md#features).

## 🧪 Testing

```bash
python test_module.py
```

## 🔍 Compatibility

- Python 3.11 to 3.14 (`requires-python = ">=3.11"`)
- Dependencies: see `pyproject.toml` (`mcp>=2,<3`, `requests`, `starlette`, `uvicorn`)

## 📖 Documentation

1. **README.md** - project overview, environment variables and tool list
2. **INSTALL.md** - installation, Docker and systemd deployment
3. **MCP_CONFIG.md** - MCP client configuration
4. **EXAMPLES.md** - tool call examples and workflows
5. **CHANGELOG.md** - version history
