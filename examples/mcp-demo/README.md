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

`start.sh` creates `.venv` and installs the dependencies. It listens on `http://127.0.0.1:8010/mcp` by default, with auth token `ai-kb-demo-token`.

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
source .venv/bin/activate
python test_tools.py
```

It should list 6 tools.

## 3. Connect AI KB

1. Open **Settings → MCP Services → New**.
2. Fill in:

| Field | Value |
| --- | --- |
| Name | `Local MCP Demo` |
| Transport | **HTTP Streamable** |
| URL | `http://127.0.0.1:8010/mcp` |
| Auth | **Bearer** |
| Token | `ai-kb-demo-token` (same as `MCP_SERVER_AUTH_TOKEN`) |

3. Save, then click **Test Connection**. It should find 6 tools.
4. In the **agent** settings, select this MCP service (or all tools).
5. (Optional) Turn on **Require human approval** for `send_demo_alert`. The agent then asks for confirmation before it calls the tool.

## 4. Questions to try

Ask in an agent conversation:

- "Use the MCP tool to check how long the smart home hub warranty is" → should trigger `lookup_policy`
- "Who owns the R&D POC?" → `lookup_policy` or `list_team_contacts`
- "What time is it on the MCP demo server?" → `server_time`

If you also imported the documents in `website-docs/sample-data/`, you can check that the **knowledge base retrieval answer** and the **MCP tool result** agree.

## 5. Notes

- The AI KB UI **does not support the stdio** transport. Use **HTTP Streamable** or **SSE**.
- The demo binds only to `127.0.0.1`. Do not expose it to the internet.
- `send_demo_alert` sends nothing. It only returns a simulated result.

## 6. SSE mode (optional)

```bash
MCP_TRANSPORT=sse MCP_PORT=8011 ./start.sh
```

In AI KB choose the **SSE** transport and set the URL to `http://127.0.0.1:8011/sse`.
