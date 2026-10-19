---
name: ai-kb-rag-search
description: Use when answering a question from an AI KB knowledge base or pulling source passages out of one with the `aikb` CLI - grounded answers with `chat`, custom-agent answers with `session ask`, raw hybrid retrieval with `search chunks`, finding documents, knowledge bases or sessions by name, continuing a conversation, or stopping and resuming a stream. Read it whenever you are unsure which of those commands fits the task.
metadata:
  tested_against: v2026.10.13
  openclaw:
    requires:
      bins: [aikb]
    primaryEnv: AIKB_API_KEY
    envVars:
      - name: AIKB_HOST
        required: true
        description: Base URL of the AI KB server, for example https://kb.example.com
      - name: AIKB_API_KEY
        required: true
        description: Workspace API key with the retrieve capability (plus chat for `chat` and `session ask`)
---

# AI KB - answers and retrieval

**REQUIRED BACKGROUND:** `ai-kb-shared` (connection, `--kb` resolution, the
envelope, exit codes). Adding material is `ai-kb-ingest`.

Three commands look alike and are not: `chat` writes an answer from one
knowledge base, `session ask` hands the question to a custom agent, and
`search chunks` returns passages with no model in the loop. Choosing by
reflex costs a turn and often the wrong output shape, so start here.

## Decide in two questions

**Do you want prose, or evidence to reason over yourself?**

- Evidence: `search chunks "<query>" --kb <kb>`. You get ranked passages with
  their source document and no synthesis. Choose this when you will combine
  sources, quote verbatim, or check a claim.
- Prose: one of the two answer commands below.

**Plain knowledge base, or an agent the user configured?**

- `chat "<question>" --kb <kb>` retrieves from one knowledge base and answers.
- `session ask --agent <id> "<question>"` runs a custom agent with whatever
  knowledge bases, tools and web access it was given. When the user has set
  one up for the job, prefer it; `aikb agent list` shows the ids.

| Need | Command | Model involved | `data` |
| --- | --- | --- | --- |
| An answer from one knowledge base | `chat "<q>" --kb <kb>` | yes | `events`, `session_id`, `assistant_message_id`, `kb_id`, `query` |
| An answer from a configured agent | `session ask --agent <id> "<q>"` | yes, plus the agent's tools | `events`, `session_id`, `agent_id`, `query` |
| Passages to read yourself | `search chunks "<q>" --kb <kb>` | no | array of chunks, `meta.count`, `meta.has_more` |
| Documents whose title or file name contains a word | `search docs "<q>" --kb <kb>` | no | array of documents, `meta.total_count` |
| A knowledge base by name or description | `search kb "<q>"` | no | array of knowledge bases |
| An earlier session by title | `search sessions "<q>"` | no | array of sessions |
| A past question and its answer | `message search "<q>"` | no | grouped `query_content` / `answer_content` pairs |

## Continuing a conversation

`chat` and `session ask` create a session on the first call and report it as
`data.session_id`. Pass `--session <id>` on the next call to keep context; omit
it and each call starts fresh. In `--format ndjson` the id is on the leading
`init` line. `aikb message list --session <id>` reads the history back.

## Shaping the answer output

By default both answer commands buffer the stream into one envelope whose
`data.events` holds the answer fragments. Join them with
`--jq '[.data.events[].content] | join("")'`.

- `--reference` adds citation events with `kb_id` and `chunk_id` indexes.
- `--verbose` adds reasoning, tool and lifecycle events.
- `--format text` prints the answer live for a person watching.
- `--format ndjson` emits the raw server events, one per line, for your own
  parser. `references/chat.md` lists the event types.

## Gotchas

- **The knowledge base comes from `--kb`, then `AIKB_KB_ID`, then the linked
  directory.** No source means exit 1 `local.kb_id_required`; an unknown name
  means exit 1 `local.kb_not_found`. Names match exactly, case included;
  `aikb kb list` or `search kb` gives the spelling. `search kb` and
  `search sessions` span the tenant and take no `--kb`.
- **An empty result on a new knowledge base is usually indexing, not a bad
  query.** `meta.hint` says so when the CLI suspects it; `kb status` shows
  `retrieval_ready`. See `ai-kb-shared`.
- **Ctrl-C does not stop generation.** The server keeps producing and billing.
  Run `aikb session stop <session-id> --message <assistant-message-id>`.
  `chat` returns the message id as `data.assistant_message_id`; for
  `session ask` read the latest assistant message from
  `aikb message list --session <id>`.
- **A dropped stream can be replayed.**
  `aikb session resume <session-id> --message <assistant-message-id>` replays
  every stored event from the start, then tails. Details and the buffer
  lifetime are in `references/chat.md`.
- **`search chunks --limit` defaults to 8**, sized for a model context, with
  a hard range of 1 to 1000; the other `search` lists default to 30.
  `--vector-threshold`, `--keyword-threshold`, `--no-vector` and `--no-keyword`
  tune the two channels. `references/search-chunks.md` has the flag table and
  the output shape.
- **Reading a document's chunks is not search.** `chunk list --doc <id>` lists
  a document in stored order for debugging; `search chunks` ranks by relevance.

## Examples

```bash
# evidence first: pull passages, then reason over them
aikb search chunks "on-call escalation after 15 minutes" --kb ops-runbooks --limit 10 \
  --jq '.data[] | {score, knowledge_title, content}'

# a grounded answer with citations, as one JSON envelope
aikb chat "Who is paged when the primary does not acknowledge?" --kb ops-runbooks --reference

# keep the thread: session id from data.session_id of the call above
aikb chat "And on weekends?" --kb ops-runbooks --session sess_7d2

# the user's configured agent, which may call tools
aikb session ask --agent ag_triage "Draft the incident summary for INC-4821"

# recover a stream that dropped
aikb session resume sess_7d2 --message msg_c41
```
