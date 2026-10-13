#!/usr/bin/env python3
"""
AI KB MCP Server launch script

Under the stdio transport, stdout is the JSON-RPC channel. All diagnostics and
notices must go to stderr, or they corrupt the MCP stream and the client reports
"failed to start". Every print in this script writes to stderr.
"""

import asyncio
import os
import sys


def check_environment():
    """Check the environment configuration."""
    base_url = os.getenv("AIKB_BASE_URL")
    api_key = os.getenv("AIKB_API_KEY")

    if not base_url:
        print(
            "Warning: AIKB_BASE_URL is not set, using the default: http://localhost:8080/api/v1",
            file=sys.stderr,
        )

    if not api_key:
        print("Warning: AIKB_API_KEY is not set", file=sys.stderr)

    print(f"AI KB Base URL: {base_url or 'http://localhost:8080/api/v1'}", file=sys.stderr)
    print(f"API Key: {'set' if api_key else 'not set'}", file=sys.stderr)


def main():
    """Entry point."""
    if any(arg in ("-h", "--help") for arg in sys.argv[1:]):
        print("usage: ai-kb-server\nStarts the AI KB MCP server on stdio. For options, use ai-kb-mcp-server --help.")
        return
    print("Starting AI KB MCP Server...", file=sys.stderr)
    check_environment()

    try:
        from aikb_mcp_server import run

        asyncio.run(run())
    except ImportError as e:
        print(f"Import error: {e}", file=sys.stderr)
        print("Make sure all dependencies are installed: pip install -r requirements.txt", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        print("\nServer stopped", file=sys.stderr)
    except Exception as e:
        print(f"Server error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
