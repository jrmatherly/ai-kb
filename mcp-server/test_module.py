#!/usr/bin/env python3
"""
AI KB MCP Server 模组测试脚本

测试模组的各种启动方式和功能。unittest discover 会收集本文件中的 TestCase；
也可直接运行: python test_module.py
"""

import os
import subprocess
import sys
import unittest
from pathlib import Path

MCP_SERVER_DIR = Path(__file__).resolve().parent

REQUIRED_FILES = [
    "__init__.py",
    "main.py",
    "run_server.py",
    "aikb_mcp_server.py",
    "requirements.txt",
    "setup.py",
    "pyproject.toml",
    "README.md",
    "INSTALL.md",
    "LICENSE",
    "MANIFEST.in",
]


class ModuleIntegrationTest(unittest.TestCase):
    def test_imports(self):
        import mcp  # noqa: F401
        import requests  # noqa: F401
        import aikb_mcp_server  # noqa: F401
        from aikb_mcp_server import AIKBClient, run  # noqa: F401
        import main  # noqa: F401

    def test_environment_optional_vars(self):
        os.getenv("AIKB_BASE_URL")
        os.getenv("AIKB_API_KEY")

    def test_client_creation(self):
        from aikb_mcp_server import AIKBClient

        base_url = os.getenv("AIKB_BASE_URL", "http://localhost:8080/api/v1")
        api_key = os.getenv("AIKB_API_KEY", "test_key")
        client = AIKBClient(base_url, api_key)
        self.assertEqual(client.base_url, base_url)
        self.assertEqual(client.api_key, api_key)

    def test_required_files_exist(self):
        missing = [name for name in REQUIRED_FILES if not (MCP_SERVER_DIR / name).exists()]
        self.assertEqual(missing, [], f"Missing files: {missing}")

    def test_main_help(self):
        result = subprocess.run(
            [sys.executable, "main.py", "--help"],
            capture_output=True,
            text=True,
            timeout=10,
            cwd=MCP_SERVER_DIR,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_main_check_only(self):
        result = subprocess.run(
            [sys.executable, "main.py", "--check-only"],
            capture_output=True,
            text=True,
            timeout=10,
            cwd=MCP_SERVER_DIR,
        )
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_wiki_tools(self):
        import aikb_mcp_server

        client = aikb_mcp_server.AIKBClient("http://localhost:8080/api/v1", "test")
        for method in ["wiki_search", "wiki_read_page", "wiki_index_view"]:
            self.assertTrue(hasattr(client, method), f"AIKBClient missing: {method}")
            self.assertTrue(callable(getattr(client, method)), f"{method} not callable")

    def test_pyproject_metadata(self):
        text = (MCP_SERVER_DIR / "pyproject.toml").read_text(encoding="utf-8")
        self.assertIn("ai-kb-mcp", text)
        self.assertIn("ai-kb-mcp-server", text)


if __name__ == "__main__":
    unittest.main(verbosity=2)
