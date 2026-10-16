# AI KB Local MCP Demo

A minimal external MCP server for testing AI KB **as an MCP client** of third-party tools.

It exposes 6 demo tools:

| Tool | Purpose |
| --- | --- |
| `echo` | Connectivity self-check |
| `add` | Adds two numbers |
| `server_time` | Returns the server's UTC time |
| `lookup_policy` | Looks up demo policies (warranty, reimbursement, POC and so on, matching `website-docs/sample-data/`) |
| `list_team_contacts` | Lists the demo project team members |
| `send_demo_alert` | Simulates an outgoing notification (good for testing tool approval) |

## 1. Start

```bash
cd examples/mcp-demo
chmod +x start.sh
./start.sh
```

`start.sh` runs the server with `uv run`, which creates the environment from `uv.lock`. It listens on `http://127.0.0.1:8010/mcp` by default (set `MCP_HOST` to change the bind address), with auth token `ai-kb-demo-token`.

To customize:

```bash
export MCP_SERVER_AUTH_TOKEN=my-secret
export MCP_PORT=9000
./start.sh
```

## 2. Self-check

In another terminal:

```bash
cd examples/mcp-demo
uv run python test_tools.py
```

It should list 6 tools and resolve 6 `lookup_policy` topics.

## 3. Connect AI KB

AI KB refuses MCP service URLs on loopback and private hosts (SSRF check), so `127.0.0.1` is rejected, and inside the Docker `app` container it would point at the container anyway. Allow the demo host first, for example with AI KB in Docker:

```bash
# AI KB .env, then restart the app container
SSRF_WHITELIST_EXTRA=host.docker.internal
# demo: listen on all interfaces
MCP_HOST=0.0.0.0 ./start.sh
```

1. Open **Toolbox → MCP Service → Add Service**.
2. Fill in:

   | Field | Value |
   | --- | --- |
   | Service Name | `Local MCP Demo` |
   | Transport Type | **HTTP Streamable** |
   | Service URL | `http://host.docker.internal:8010/mcp` |
   | Auth Method | **API Key / Token** |
   | Header Name | `Authorization` |
   | Secret / Token | `Bearer ai-kb-demo-token` (same token as `MCP_SERVER_AUTH_TOKEN`) |

3. Click **Save and continue**. The **Tools and usage** step should list 6 tools.
4. (Optional) Turn on **Require approval** for `send_demo_alert`. The agent then asks for confirmation before it calls the tool.
5. Add usage instructions (or generate them), then **Save**.
6. In the **agent** settings, select this MCP service (or all tools).

## 4. Questions to try

Ask in an agent conversation:

- "Use the MCP tool to check how long the smart home hub warranty is" → should trigger `lookup_policy`
- "Who owns the R&D POC?" → `lookup_policy` or `list_team_contacts`
- "What time is it on the MCP demo server?" → `server_time`

If you also imported the documents in `website-docs/sample-data/`, you can check that the **knowledge base retrieval answer** and the **MCP tool result** agree.

## 5. Notes

- The AI KB UI **does not support the stdio** transport. Use **HTTP Streamable** or **SSE**.
- The demo binds to `127.0.0.1` unless `MCP_HOST` says otherwise. Do not expose it to the internet.
- `send_demo_alert` sends nothing. It only returns a simulated result.

## 6. SSE mode (optional)

```bash
MCP_TRANSPORT=sse MCP_PORT=8011 ./start.sh
```

In AI KB choose the **SSE** transport and set the URL to `http://host.docker.internal:8011/sse`.
