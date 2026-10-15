#!/usr/bin/env python3
"""
AI KB MCP Server install script
"""

from setuptools import setup


def read_readme():
    try:
        with open("README.md", "r", encoding="utf-8") as f:
            return f.read()
    except FileNotFoundError:
        return "AI KB MCP Server - Model Context Protocol server for AI KB API"


def read_requirements():
    try:
        with open("requirements.txt", "r", encoding="utf-8") as f:
            return [
                line.strip() for line in f if line.strip() and not line.startswith("#")
            ]
    except FileNotFoundError:
        return [
            "mcp>=2,<3",
            "requests>=2.31.0",
            "starlette>=0.27.0",
            "uvicorn>=0.24.0",
        ]


setup(
    name="ai-kb-mcp",
    version="1.1.2",
    author="AI KB Team",
    author_email="support@matherly.net",
    description="AI KB MCP Server - Model Context Protocol server for AI KB API",
    long_description=read_readme(),
    long_description_content_type="text/markdown",
    url="https://github.com/jrmatherly/ai-kb/tree/main/mcp-server",
    py_modules=["aikb_mcp_server", "upload_paths", "main", "run_server", "run", "test_module"],
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "Operating System :: OS Independent",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
        "Programming Language :: Python :: 3.13",
        "Programming Language :: Python :: 3.14",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: Internet :: WWW/HTTP :: HTTP Servers",
        "Topic :: Scientific/Engineering :: Artificial Intelligence",
    ],
    python_requires=">=3.11",
    install_requires=read_requirements(),
    entry_points={
        "console_scripts": [
            "ai-kb-mcp-server=main:sync_main",
            "ai-kb-server=run_server:main",
        ],
    },
    include_package_data=True,
    data_files=[
        ("", ["README.md", "requirements.txt", "LICENSE"]),
    ],
    keywords="mcp model-context-protocol aikb knowledge-management api-server",
)
