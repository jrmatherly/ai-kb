# AI KB MCP Server installation and usage guide

## Quick start

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

Create the API key under Settings → Publish & Integrations → API Integration → API Keys. The full variable list is in [README.md](./README.md#2-set-environment-variables).

### 3. Run the server

There are several ways to run the server:

#### Option 1: the main entry point (recommended)

```bash
python main.py
```

#### Option 2: the original launch script

```bash
python run_server.py
```

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
# or
ai-kb-server
```

### Production install

```bash
pip install .
```

### Build distribution packages

```bash
# Build the source distribution and wheel
python setup.py sdist bdist_wheel

# Or use the build tool
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
├── pyproject.toml           # project metadata (package ai-kb-mcp, not on PyPI)
├── uv.lock                  # locked dependencies
├── MANIFEST.in              # package file manifest
├── Dockerfile               # container image (Streamable HTTP)
├── test_*.py, tests/        # tests
├── LICENSE                  # license
├── README.md                # project overview
├── INSTALL.md               # installation guide
├── MCP_CONFIG.md            # MCP client configuration
├── EXAMPLES.md              # usage examples
└── CHANGELOG.md             # changelog
```

### Adding a feature

1. Add the new API method to the `AIKBClient` class
2. Register a new tool function with the `@mcp.tool()` decorator: annotate the parameters with types (the schema is generated from them), put the description in the docstring, and call the new client method from the function body
3. Update the docs and tests

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

### Docker

Build and run the bundled `Dockerfile`, which serves Streamable HTTP on port 8000:

```bash
docker build -t ai-kb-mcp mcp-server
docker run -e AIKB_API_KEY=your_api_key -e MCP_SERVER_AUTH_TOKEN=change_me -p 8000:8000 ai-kb-mcp
```

The image defaults to `AIKB_BASE_URL=http://app:8080/api/v1`; override it when AI KB runs elsewhere. The MCP endpoint is `http://<host>:8000/mcp`.

### System service

Create the systemd unit file `/etc/systemd/system/ai-kb-mcp.service`:

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

Enable the service:

```bash
sudo systemctl enable ai-kb-mcp
sudo systemctl start ai-kb-mcp
```

## Support

If you run into problems:

1. Check the log output
2. Check the environment configuration
3. See the Troubleshooting section
4. File an issue in the project repository: <https://github.com/jrmatherly/ai-kb/issues>
