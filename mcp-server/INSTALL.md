# AI KB MCP Server installation and usage guide

> **Deprecated.** AI KB ships a [built-in MCP server](https://ai-kb.matherly.net/docs/03-features/08-mcp); prefer it. This package may be removed in a future release.

## Quick start

### 1. Install

From PyPI, which installs the `ai-kb-mcp-server` command:

```bash
pip install ai-kb-mcp
```

From a checkout of `mcp-server/`, which installs only the dependencies (then run `python main.py`):

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

Create the API key under Settings → Publish & Integrations → API Integration → API Keys. The full variable list is in [README.md](./README.md#2-set-environment-variables).

### 3. Run the server

There are several ways to run the server:

#### Option 1: the main entry point (recommended from a checkout)

```bash
python main.py
```

#### Option 2: the original launch script

```bash
python run_server.py
```

`run_server.py` always runs stdio and ignores every option.

Options 3 and 4 accept only `--transport`, `--host` and `--port`.

#### Option 3: run the server module directly

```bash
python aikb_mcp_server.py
```

#### Option 4: run as a Python module

```bash
python -m aikb_mcp_server
```

## Install as a Python package

### Development install

```bash
pip install -e .
```

After installation you can use the command-line tools:

```bash
ai-kb-mcp-server
# or, stdio only and without options:
ai-kb-server
```

### Production install

```bash
pip install .
```

### Build distribution packages

```bash
# Build the source distribution and wheel (what CI runs)
uv build

# Or without uv
pip install build
python -m build
```

## Command-line options

The main entry point `main.py` supports these options:

```bash
python main.py --help                 # show help
python main.py --check-only           # check the environment configuration only
python main.py --verbose              # enable verbose logging
python main.py --version              # show the version
python main.py --transport http --host 127.0.0.1 --port 8000   # Streamable HTTP (needs MCP_SERVER_AUTH_TOKEN)
```

## Environment check

Run this command to check the environment configuration:

```bash
python main.py --check-only
```

It shows the base URL and whether the API key is set, and exits with an error if `mcp` or `requests` is missing.

## Troubleshooting

### 1. Import errors

If you hit an `ImportError`, make sure that:

- All dependencies are installed: `pip install -r requirements.txt`
- Your Python version is compatible (3.11 or newer)
- There are no file name conflicts

### 2. Connection errors

If you cannot connect to the AI KB API:

- Check that `AIKB_BASE_URL` is correct
- Confirm the AI KB service is running
- Check the network connection

### 3. Authentication errors

If you hit authentication problems:

- Check that `AIKB_API_KEY` is set
- Confirm the API key is valid
- Check the permission settings

## Development

### Project layout

```text
ai-kb/mcp-server/
├── __init__.py              # package init
├── main.py                  # main entry point
├── run_server.py            # original launch script
├── run.py                   # quick launch script
├── aikb_mcp_server.py       # MCP server implementation
├── upload_paths.py          # local upload directory boundary
├── requirements.txt         # dependency list
├── setup.py                 # install script
├── pyproject.toml           # project metadata (package ai-kb-mcp on PyPI)
├── uv.lock                  # locked dependencies
├── MANIFEST.in              # package file manifest
├── Dockerfile               # container image (Streamable HTTP)
├── test_*.py, tests/        # tests
├── check_imports.py         # manual import check
├── LICENSE                  # license
├── README.md                # project overview
├── INSTALL.md               # installation guide
├── MCP_CONFIG.md            # MCP client configuration
├── EXAMPLES.md              # usage examples
└── CHANGELOG.md             # changelog
```

Adding a tool: see [README.md](./README.md#add-a-tool).

### Testing

```bash
# Run the basic check
python check_imports.py

# Test the environment configuration
python main.py --check-only

# Test server startup
python main.py --verbose
```

## Deployment

Docker and systemd: see [README.md](./README.md#deployment).

## Support

If you run into problems:

1. Check the log output
2. Check the environment configuration
3. See the Troubleshooting section
4. File an issue in the project repository: <https://github.com/jrmatherly/ai-kb/issues>
