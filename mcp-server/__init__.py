#!/usr/bin/env python3
"""
AI KB MCP Server Package

A Model Context Protocol server that provides access to the AI KB knowledge management API.
"""

__version__ = "1.1.1"
__author__ = "AI KB Team"
__description__ = "AI KB MCP Server - Model Context Protocol server for AI KB API"

from weknora_mcp_server import WeKnoraClient, run

__all__ = ["WeKnoraClient", "run"]
