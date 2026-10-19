# Answer commands: `chat`, `session ask`, and the event stream

Read this when you need the exact shape of what `chat` or `session ask` emits,
want to parse the raw event stream, or have to stop or resume a running answer.

## Invocation

```text
aikb chat "<question>" [--kb <uuid-or-name>] [--session <id>] [--reference] [--verbose]
aikb session ask "<question>" --agent <agent-id> [--session <id>] [--reference] [--verbose]
```

| Flag | `chat` | `session ask` |
| --- | --- | --- |
| `--kb` | knowledge base; falls back to `AIKB_KB_ID`, then the linked directory | not accepted |
| `--agent`, `-a` | not accepted | required agent id |
| `--session` | continue this session instead of creating one | same |
| `--reference` | add citation events (`kb_id`, `chunk_id` indexes) | same |
| `--verbose` | add reasoning, tool and lifecycle events | same |

## Three output modes

| `--format` | What you get |
| --- | --- |
| `json` (default) | one envelope: `{ok, data:{events:[...], session_id, query, kb_id or agent_id}}`; `chat` also sets `data.assistant_message_id` |
| `text` | the selected events rendered live for a reader |
| `ndjson` | an `init` line, then the server's events verbatim, one JSON object per line |

`data.events` holds answer fragments by default; `--reference` and `--verbose`
add their event kinds to the same array. The final answer is the concatenation
of the `content` fields of events whose `response_type` is `answer`:

```bash
aikb chat "What is the retention period?" --kb policies --jq '[.data.events[].content] | join("")'
```

## The raw stream (`--format ndjson`)

The first line is written by the CLI:

```jsonc
{"type":"init","session_id":"sess_7d2","kb_id":"3b1e...","profile":"dev"}   // session ask carries agent_id instead of kb_id
```

Every following line is a server event. Values of `response_type` you will see:

| `response_type` | Meaning |
| --- | --- |
| `agent_query` | the opening metadata frame; carries `session_id` and `assistant_message_id` |
| `thinking`, `reflection` | model reasoning, when the model produces it |
| `tool_call`, `tool_result` | an agent's tool activity (`session ask`) |
| `references` | grounding chunks in `knowledge_references` |
| `answer` | a fragment of the answer in `content`; concatenate in order |
| `error` | the run failed mid-stream |
| `complete` | the terminal event |

Only `complete` ends the stream. Other frames may carry `done:true` for
themselves (the `agent_query` frame does), so do not stop reading on `done`.
A transport or HTTP failure is reported as the usual error envelope on stderr
with a typed code, not as a stream event.

Keep two ids from the stream: `session_id` for `--session`, and
`assistant_message_id` from the `agent_query` frame for `session stop` and
`session resume`.

## Stopping and resuming

Closing the connection (Ctrl-C, a killed process) leaves the server
generating. Stop it explicitly:

```bash
aikb session stop sess_7d2 --message msg_c41
```

Re-attach to a stream that dropped, or replay a finished one for inspection:

```bash
aikb session resume sess_7d2 --message msg_c41
```

`session resume` always writes NDJSON (`--format` and `--jq` have no effect):
an `init` line with `session_id`, `message_id` and `profile`, then the stored
events. The server replays from the first event rather than from where you
left off, so a consumer that already processed part of the stream must dedupe.
The buffer lives one hour when the server uses Redis and for the server
process's lifetime when it uses memory; after that `session resume`
returns `resource.not_found` (exit 4) and the query must be re-run.

The message id is `data.assistant_message_id` from `chat`, the `agent_query`
frame in NDJSON mode, or the newest assistant message in
`aikb message list --session <id>`.
