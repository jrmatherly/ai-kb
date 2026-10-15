#!/usr/bin/env python3
"""
AI KB MCP Server Package

A Model Context Protocol server that provides access to the AI KB knowledge management API.
"""

__version__ = "1.1.2"
__author__ = "AI KB Team"
__description__ = "AI KB MCP Server - Model Context Protocol server for AI KB API"

from aikb_mcp_server import AIKBClient, run

__all__ = ["AIKBClient", "run"]
