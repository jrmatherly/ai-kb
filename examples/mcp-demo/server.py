#!/usr/bin/env python3
"""
AI KB local MCP demo server

A minimal external MCP server for testing the client connection in AI KB under
Toolbox → MCP Service. By default it serves Streamable HTTP at http://127.0.0.1:8010/mcp

Start:
  export MCP_SERVER_AUTH_TOKEN=ai-kb-demo-token
  python server.py

AI KB settings:
  Transport: HTTP Streamable
  URL: http://127.0.0.1:8010/mcp
  Auth: API Key / Token, header Authorization, value "Bearer <MCP_SERVER_AUTH_TOKEN>"
"""

from __future__ import annotations

import argparse
import asyncio
import logging
import os
import re
import secrets
import sys
from datetime import datetime, timezone
from typing import Any

from mcp.server import MCPServer

logging.basicConfig(level=logging.INFO, format="%(levelname)s %(message)s")
logger = logging.getLogger("mcp-demo")

mcp = MCPServer("ai-kb-mcp-demo", version="0.1.0")

# Demo corpus that matches website-docs/sample-data/, so agent tool answers can be compared with knowledge base answers.
DEMO_POLICIES: dict[str, str] = {
    "warranty": "Smart Home Hub Pro has a 24-month warranty on the unit and 12 months on battery accessories. Damage from disassembly or water is not covered.",
    "offline_voice": "If voice uses cloud recognition, only the app and the local touchscreen work without internet. With a local voice pack configured, basic commands keep working.",
    "device_limit": "A Personal edition account can bind up to 3 hubs. The Enterprise edition is licensed per contract, 50 by default.",
    "travel_hotel_tier1": "Business trip accommodation in tier-1 cities (Beijing, Shanghai, Guangzhou, Shenzhen) is reimbursed up to 600 CNY per night (tax included).",
    "travel_meal": "Meals on business trips are not reimbursed separately. The travel allowance in tier-1 cities is 150 CNY per day.",
    "poc_owner": "The support knowledge base POC technical lead is Zhang Ming (R&D), the product contact is Li Wei, and the QA lead is Zhao Lei.",
    "poc_deadline": "The support knowledge base POC targets an intranet demo by 2024-03-01.",
    "matter_cert": "Firmware 3.5 is planned for a staged rollout by the end of March 2024, with Matter 1.2 certification completed.",
}

DEMO_CONTACTS: list[dict[str, str]] = [
    {"name": "Chen Hao", "role": "Product Director", "department": "Product"},
    {"name": "Zhang Ming", "role": "Lead, Knowledge Base and AI Module", "department": "R&D"},
    {"name": "Li Wei", "role": "Product Operations", "department": "Product"},
    {"name": "Wang Xue", "role": "Lead, Interaction Design", "department": "Design"},
    {"name": "Zhao Lei", "role": "QA Manager", "department": "QA"},
]


def network_transport_auth_token() -> str:
    return os.getenv("MCP_SERVER_AUTH_TOKEN", "").strip()


def require_network_transport_auth(transport: str) -> str:
    token = network_transport_auth_token()
    if transport in ("sse", "http") and not token:
        logger.error(
            "MCP_SERVER_AUTH_TOKEN is required for %s transport. "
            "Example: export MCP_SERVER_AUTH_TOKEN=ai-kb-demo-token",
            transport,
        )
        sys.exit(1)
    return token


class MCPAuthMiddleware:
    """Bearer auth middleware for the SSE and HTTP transports."""

    def __init__(self, app, token: str):
        self.app = app
        self.token = token

    async def __call__(self, scope, receive, send):
        if scope.get("type") != "http":
            await self.app(scope, receive, send)
            return

        headers = {
            k.decode("latin-1").lower(): v.decode("latin-1")
            for k, v in scope.get("headers", [])
        }
        provided = ""
        auth = headers.get("authorization", "")
        if auth.lower().startswith("bearer "):
            provided = auth[7:].strip()
        elif "x-mcp-auth-token" in headers:
            provided = headers["x-mcp-auth-token"]

        if not provided or not secrets.compare_digest(provided, self.token):
            body = b'{"error":"unauthorized"}'
            await send(
                {
                    "type": "http.response.start",
                    "status": 401,
                    "headers": [[b"content-type", b"application/json"]],
                }
            )
            await send({"type": "http.response.body", "body": body})
            return

        await self.app(scope, receive, send)


@mcp.tool()
def echo(message: str) -> dict[str, Any]:
    """Echo a message back to check MCP connectivity."""
    return {"echo": message}


@mcp.tool()
def add(a: float, b: float) -> dict[str, Any]:
    """Return the sum of two numbers."""
    return {"a": a, "b": b, "sum": a + b}


@mcp.tool()
def server_time() -> dict[str, str]:
    """Return the MCP demo server's current UTC time."""
    now = datetime.now(timezone.utc)
    return {
        "iso": now.isoformat(),
        "unix": str(int(now.timestamp())),
    }


@mcp.tool()
def lookup_policy(topic: str) -> dict[str, Any]:
    """Look up demo policy or project information. topic is one of warranty/offline_voice/device_limit/travel_hotel_tier1/travel_meal/poc_owner/poc_deadline/matter_cert, or a keyword such as "warranty", "reimbursement" or "POC"."""
    text = topic.strip().lower()
    key = text.replace(" ", "_")
    aliases = {
        "warranty": "warranty",
        "guarantee": "warranty",
        "offline": "offline_voice",
        "voice": "offline_voice",
        "device": "device_limit",
        "hotel": "travel_hotel_tier1",
        "reimburse": "travel_hotel_tier1",
        "meal": "travel_meal",
        "allowance": "travel_meal",
        "owner": "poc_owner",
        "zhang ming": "poc_owner",
        "poc": "poc_owner",
        "acceptance": "poc_deadline",
        "matter": "matter_cert",
        "certification": "matter_cert",
    }
    if key not in DEMO_POLICIES:
        for alias, mapped in aliases.items():
            if alias in text:
                key = mapped
                break

    if key in DEMO_POLICIES:
        return {"topic": key, "answer": DEMO_POLICIES[key], "source": "mcp-demo/static"}

    words = [w for w in re.split(r"[^a-z0-9]+", text) if len(w) >= 3]
    matches = {k: v for k, v in DEMO_POLICIES.items() if any(w in k for w in words)}
    if len(matches) == 1:
        only_key = next(iter(matches))
        return {"topic": only_key, "answer": matches[only_key], "source": "mcp-demo/static"}

    return {
        "topic": topic,
        "available_topics": sorted(DEMO_POLICIES.keys()),
        "hint": "Pass one of the keys above as topic, or a keyword such as \"warranty\", \"reimbursement\" or \"POC\".",
    }


@mcp.tool()
def list_team_contacts(department: str = "") -> dict[str, Any]:
    """List the demo project team members, optionally filtered by department (Product / R&D / Design / QA)."""
    rows = DEMO_CONTACTS
    if department.strip():
        needle = department.strip().lower()
        rows = [c for c in rows if needle in c["department"].lower()]
    return {"count": len(rows), "contacts": rows}


@mcp.tool()
def send_demo_alert(channel: str, message: str) -> dict[str, Any]:
    """Simulate sending a notification to an external channel. Nothing is actually sent.

    Useful for testing MCP tool approval in AI KB: mark this tool as requiring approval.
    """
    return {
        "ok": True,
        "simulated": True,
        "channel": channel,
        "message": message,
        "sent_at": datetime.now(timezone.utc).isoformat(),
    }


async def run_http(host: str, port: int) -> None:
    auth_token = require_network_transport_auth("http")
    try:
        import uvicorn
    except ImportError as e:
        raise ImportError("HTTP transport requires: pip install starlette uvicorn") from e

    starlette_app = MCPAuthMiddleware(
        mcp.streamable_http_app(host=host, stateless_http=True),
        auth_token,
    )
    logger.info("Streamable HTTP MCP demo listening on http://%s:%d/mcp", host, port)
    config = uvicorn.Config(starlette_app, host=host, port=port, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()


async def run_sse(host: str, port: int) -> None:
    auth_token = require_network_transport_auth("sse")
    try:
        import uvicorn
    except ImportError as e:
        raise ImportError("SSE transport requires: pip install starlette uvicorn") from e

    starlette_app = MCPAuthMiddleware(
        mcp.sse_app(host=host, message_path="/sse/messages/"),
        auth_token,
    )
    logger.info("SSE MCP demo listening on http://%s:%d/sse", host, port)
    config = uvicorn.Config(starlette_app, host=host, port=port, log_level="info")
    server = uvicorn.Server(config)
    await server.serve()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="AI KB local MCP demo server")
    parser.add_argument(
        "--transport",
        choices=["http", "sse"],
        default=os.getenv("MCP_TRANSPORT", "http"),
        help="Network transport (default: http / Streamable HTTP)",
    )
    parser.add_argument("--host", default=os.getenv("MCP_HOST", "127.0.0.1"))
    parser.add_argument("--port", type=int, default=int(os.getenv("MCP_PORT", "8010")))
    return parser.parse_args()


async def main() -> None:
    args = parse_args()
    if args.transport == "http":
        await run_http(args.host, args.port)
    else:
        await run_sse(args.host, args.port)


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("stopped")
