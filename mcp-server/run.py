#!/usr/bin/env python3
"""
AI KB MCP Server quick launch script.

A minimal launcher. For more options, use main.py.

Under the stdio transport, stdout is the JSON-RPC channel. All diagnostics and
notices must go to stderr, or they corrupt the MCP stream and the client reports
"failed to start". Every print in this script writes to stderr.
"""

import os
import sys
from pathlib import Path


def main():
    """Minimal launcher."""
    current_dir = Path(__file__).parent.absolute()
    if str(current_dir) not in sys.path:
        sys.path.insert(0, str(current_dir))

    base_url = os.getenv("AIKB_BASE_URL", "http://localhost:8080/api/v1")
    api_key = os.getenv("AIKB_API_KEY", "")

    print("AI KB MCP Server", file=sys.stderr)
    print(f"Base URL: {base_url}", file=sys.stderr)
    print(f"API Key: {'set' if api_key else 'not set'}", file=sys.stderr)
    print("-" * 40, file=sys.stderr)

    try:
        from main import sync_main

        sync_main()
    except ImportError:
        print("Error: cannot import the required modules", file=sys.stderr)
        print("Make sure you have run: pip install -r requirements.txt", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        print("\nServer stopped", file=sys.stderr)
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
