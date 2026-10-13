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
```

## Environment check

Run this command to check the environment configuration:
```bash
python main.py --check-only
```

It shows:
- The AI KB API base URL
- Whether the API key is set
- Whether the dependencies are installed

## Troubleshooting

### 1. Import errors
If you hit an `ImportError`, make sure that:
- All dependencies are installed: `pip install -r requirements.txt`
- Your Python version is compatible (3.10+ recommended)
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
```
ai-kb/mcp-server/
├── __init__.py              # package init
├── main.py                  # main entry point
├── run_server.py           # original launch script
├── aikb_mcp_server.py   # MCP server implementation
├── requirements.txt        # dependency list
├── setup.py               # install script
├── pyproject.toml         # project metadata (PyPI: ai-kb-mcp)
├── MANIFEST.in            # package file manifest
├── LICENSE                # license
├── README.md              # project overview
└── INSTALL.md             # installation guide
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
Create a `Dockerfile`:
```dockerfile
FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .
RUN pip install -e .

ENV AIKB_BASE_URL=http://localhost:8080/api/v1
EXPOSE 8000

CMD ["ai-kb-mcp-server"]
```

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
ExecStart=/usr/local/bin/ai-kb-mcp-server
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
4. File an issue in the project repository: https://github.com/jrmatherly/ai-kb/issues
