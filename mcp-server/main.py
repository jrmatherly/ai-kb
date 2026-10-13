#!/usr/bin/env python3
"""
AI KB MCP Server main entry point.

One entry point for starting the AI KB MCP server.
Ways to run it:
1. python main.py
2. python -m aikb_mcp_server
3. ai-kb-mcp-server (after installation)

Under the stdio transport, stdout is the JSON-RPC channel. All diagnostics and
notices must go to stderr, or they corrupt the MCP stream and the client reports
"failed to start". Every print in this file writes to stderr.
"""

import argparse
import asyncio
import os
import sys
from pathlib import Path


def setup_environment():
    """Set up the environment and import path."""
    current_dir = Path(__file__).parent.absolute()
    if str(current_dir) not in sys.path:
        sys.path.insert(0, str(current_dir))


def check_dependencies():
    """Check that dependencies are installed."""
    try:
        import mcp
        import requests

        return True
    except ImportError as e:
        print(f"Missing dependency: {e}", file=sys.stderr)
        print("Run: pip install -r requirements.txt", file=sys.stderr)
        return False


def check_environment_variables():
    """Check the environment variables."""
    base_url = os.getenv("AIKB_BASE_URL")
    api_key = os.getenv("AIKB_API_KEY")

    print("=== AI KB MCP Server environment check ===", file=sys.stderr)
    print(f"Base URL: {base_url or 'http://localhost:8080/api/v1 (default)'}", file=sys.stderr)
    print(f"API Key: {'set' if api_key else 'not set (warning)'}", file=sys.stderr)

    if not base_url:
        print("Tip: you can set the AIKB_BASE_URL environment variable", file=sys.stderr)

    if not api_key:
        print("Warning: setting the AIKB_API_KEY environment variable is recommended", file=sys.stderr)

    print("=" * 40, file=sys.stderr)
    return True


def parse_arguments():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
        description="AI KB MCP Server - Model Context Protocol server for AI KB API",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  python main.py                    # start with the default configuration
  python main.py --check-only       # check the environment only, do not start the server
  python main.py --verbose          # enable verbose logging
  
Environment variables:
  AIKB_BASE_URL       AI KB API base URL (default: http://localhost:8080/api/v1)
  AIKB_API_KEY        AI KB API key
  MCP_SERVER_AUTH_TOKEN  Required for the SSE/HTTP transports; clients send it as Authorization: Bearer
        """,
    )

    parser.add_argument(
        "--check-only", action="store_true", help="Check the environment configuration only, do not start the server"
    )

    parser.add_argument("--verbose", "-v", action="store_true", help="Enable verbose logging")

    parser.add_argument(
        "--version", action="version", version="AI KB MCP Server 1.1.1"
    )

    parser.add_argument(
        "--transport",
        choices=["stdio", "sse", "http"],
        default=os.getenv("MCP_TRANSPORT", "stdio"),
        help="Transport type: stdio (default), sse, or http",
    )
    parser.add_argument(
        "--host",
        default=os.getenv("MCP_HOST", "127.0.0.1"),
        help="Bind host for network transports (default: 127.0.0.1)",
    )
    parser.add_argument(
        "--port",
        type=int,
        default=int(os.getenv("MCP_PORT", "8000")),
        help="Bind port for network transports (default: 8000)",
    )

    return parser.parse_args()


async def main():
    """Entry point."""
    args = parse_arguments()

    setup_environment()

    if not check_dependencies():
        sys.exit(1)

    check_environment_variables()

    if args.check_only:
        print("Environment check complete.", file=sys.stderr)
        return

    if args.verbose:
        import logging

        logging.basicConfig(level=logging.DEBUG)
        print("Verbose logging enabled", file=sys.stderr)

    try:
        print(f"Starting AI KB MCP Server (transport={args.transport})...", file=sys.stderr)

        from aikb_mcp_server import run_stdio, run_sse, run_http

        # Select transport mode based on CLI argument or MCP_TRANSPORT env var
        # - stdio: Default, used by VS Code Copilot for local integration
        # - sse: Server-Sent Events over HTTP, suitable for cloud/remote deployments
        # - http: Streamable HTTP sessions (MCP 2025-03-26 spec), compatible with REST clients
        if args.transport == "stdio":
            # Stdio mode: communication via stdin/stdout pipes (typical for CLI integrations)
            await run_stdio()
        elif args.transport == "sse":
            # SSE mode: HTTP server with Server-Sent Events for bidirectional streaming
            await run_sse(args.host, args.port)
        elif args.transport == "http":
            # HTTP mode: HTTP REST server with request/response model
            await run_http(args.host, args.port)

    except ImportError as e:
        print(f"Import error: {e}", file=sys.stderr)
        print("Make sure all files are in the right place", file=sys.stderr)
        sys.exit(1)
    except KeyboardInterrupt:
        print("\nServer stopped", file=sys.stderr)
    except Exception as e:
        print(f"Server error: {e}", file=sys.stderr)
        if args.verbose:
            import traceback

            traceback.print_exc()
        sys.exit(1)


def sync_main():
    """Synchronous entry point for entry_points."""
    asyncio.run(main())


if __name__ == "__main__":
    asyncio.run(main())
