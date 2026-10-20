---
name: ai-kb-shared
description: Use whenever an agent runs the `aikb` CLI against an AI KB server - connecting with an API key, picking a knowledge base, reading the JSON envelope and exit codes, handling the exit-10 confirmation gate, previewing writes with --dry-run, or managing knowledge bases, documents, sessions, agents and models. Load this first; the other ai-kb-* skills assume it.
metadata:
  tested_against: v2026.10.18.1
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
        description: Workspace API key (or AIKB_TOKEN with a session token)
---

# AI KB CLI - the contract every command follows

`aikb` was built for agents. Each command writes one JSON object to stdout and
ends with an exit code that names the class of outcome, so a script decides
what to do next from two values and never parses prose. This skill is that
contract. `ai-kb-rag-search` covers retrieval and chat; `ai-kb-ingest` covers
adding material.

## Read the exit code, then the envelope

Check `$?` first. It separates success, user error, auth, and transient failure
without opening the output. Verified with `aikb exit-codes`, which prints this
table as JSON.

| Exit | Error types | What to do |
| --- | --- | --- |
| 0 | none | continue (also returned by `--dry-run`) |
| 1 | `local.*`, `operation.failed`, `operation.cancelled`, `server.session_create_failed`, `internal.error` | read `error.message`, decide whether a retry can help |
| 2 | `input.invalid_argument` from flag parsing (unknown flag, wrong arg count) | fix the invocation; `aikb <command> --help` |
| 3 | `auth.*` | re-authenticate, then retry |
| 4 | `resource.not_found` | the server id is wrong; look it up again |
| 5 | `input.*` other than confirmation (a value rejected after parsing, such as a bad `--jq` program) | change the value, retry |
| 6 | `server.rate_limited` | wait, then retry |
| 7 | other `server.*`, `network.*` | transient; retry with backoff |
| 10 | `input.confirmation_required` | stop and ask the user (next section) |
| 124 | `operation.timeout` | raise `--timeout` or inspect the job |
| 130 | cancelled by SIGINT or SIGTERM | stop; do not retry |

`input.invalid_argument` appears under both 2 and 5. The envelope carries
`error.exit_code` for exactly this reason: one JSON read tells them apart.

Then read the envelope. A success looks like `{ok:true, data, meta?, profile}`;
a failure looks like `{ok:false, error}`.

- `data` holds the payload: an object for one resource, an array for a list.
  Every create command puts the new id in `data.id`.
- `meta` carries `count`, `total_count` and `has_more` on lists, `hint` when a
  success deserves a note (an empty search against an unindexed KB, for
  example), and `dry_run` plus `plan` on previews.
- `error` carries `type`, `message`, `exit_code`, and optionally `hint`,
  `retry_argv` (an argv array you can exec as-is), `retry_after_seconds`,
  `retryable`, and `risk`.
- `--jq '<program>'` projects the envelope and prints bare strings, so
  `ID=$(aikb kb create "Handbook" --jq .data.id)` needs no quote stripping.
- `--format text` renders for a human; `--format ndjson` streams raw events for
  `chat` and `session ask` (`doc wait` writes its single result on one line). `AIKB_FORMAT` sets the default.
- `aikb auth token` is the one command that prints no envelope: it writes the
  bare credential for `TOKEN=$(aikb auth token)`. `--format json` restores the
  envelope with `{token, mode, profile}`.

### Commands that report several outcomes at once

Two command shapes carry per-item results, and for both the exit code is the
summary while `data` is the detail.

`doc delete`, `chunk delete` and `session delete` with several ids, and
`doc upload --recursive`, emit a batch envelope: top-level `status` is
`success`, `partial` or `error`; `ok` is true only when nothing failed; `data`
is `[{id, ok, result|error}]`; `meta.successes` and `meta.failures` give the
counts. One failed item means exit 1, so iterate `data[]` for the ids to retry.

`doc wait` instead keeps `ok:true` whenever the wait reached a terminal state
and partitions ids into `data.completed`, `data.failed` and `data.timeout`.
Exit 1 means at least one document failed to parse, 124 means the timeout
fired. Do not treat `ok:true` as "every document parsed".

## Exit 10: the user decides, not you

Any command whose `--help` opens with a `Risk:` line (and `aikb schema <cmd>`
reports under `risk`) refuses to run without `-y`, writing exit 10 and
`error.type = input.confirmation_required`. `error.risk.level` is `destructive`
for deletes, `kb config set`, `auth logout`, `profile remove` and
`session tool-approval resolve`, and `write` for `kb update`, `agent update`,
`doc update` and `model update`. `aikb api` applies the same gate to
`-X DELETE` (destructive) and `-X PUT` / `-X PATCH` (write). Both levels block
the same way; the label only tells you whether the change is reversible.

```jsonc
{"ok":false,"error":{"type":"input.confirmation_required",
  "message":"delete knowledge base kb_9f3 requires explicit confirmation: re-run with -y/--yes",
  "exit_code":10,
  "hint":"high-risk write - re-run with -y/--yes after the user explicitly approves",
  "retry_argv":["aikb","kb","delete","kb_9f3","-y"],
  "retryable":false,
  "risk":{"level":"destructive","action":"kb.delete"}}}
```

Show the user what the command would change and wait for a yes. Only then run
`retry_argv`. Adding `-y` because the gate is "in the way" defeats the one
safeguard between an agent and an irreversible delete.

## Preview with --dry-run before any write

Every create, update, delete, upload, reparse, link and config command accepts
`--dry-run`. The commands that have no flag are `auth login`, `chat`,
`session ask` (both create a session and a message) and `doc download`
(writes a file). A dry run resolves arguments, prints
`meta.dry_run:true` and `meta.plan:{action, args}`, performs no network call or
file write, and exits 0. Use it to show the user a bulk upload or a delete
before it happens, and to catch a malformed invocation while it is still free.

## Connecting

Set two variables and every command authenticates on its own, with nothing
written to disk:

```bash
export AIKB_HOST=https://kb.example.com
export AIKB_API_KEY=...        # or AIKB_TOKEN with a bearer token; AIKB_TOKEN wins if both are set
aikb auth status               # who the server thinks you are, and which tenant
aikb doctor                    # base URL, credential, server version, credential storage
```

This is the normal agent path. Profiles exist for people who switch between
servers; use them only when the user asks for a stored connection:

```bash
aikb profile add staging --host https://staging.example.com --use
printf '%s' "$AIKB_API_KEY" | aikb auth login --with-token   # key on stdin, never on argv
```

`auth login` has no host, user or name flag; it authenticates whichever profile
is active, so `profile add ... --use` must come first. Without `--with-token`
it prompts for email and password, which an agent cannot answer. A password
profile holds a JWT that `auth refresh` renews; API-key profiles are rejected by
`auth refresh`. The global `--profile <name>` aims one invocation at another
profile without changing the default. `auth logout` clears credentials but
leaves the profile registered; `profile remove` deletes it.

## Choosing the knowledge base

Commands that act inside a knowledge base (`chat`, `search chunks`,
`search docs`, `doc upload`, `doc fetch`, `doc create`, `doc list`) find it in
this order:

1. `--kb <value>`
2. `AIKB_KB_ID`
3. `.aikb/project.yaml`, written by `aikb link <kb>` and found by walking up
   from the current directory

When none applies the command exits 1 with `local.kb_id_required`. The value
may be a UUID or a name. A name is resolved by listing the tenant's knowledge
bases and taking the exact, case-sensitive match; no match is exit 1 with
`local.kb_not_found`. `doc delete --all` ignores the fallbacks and demands an
explicit `--kb`, so a mass delete can never land on a linked directory's
knowledge base by accident. `search kb` and `search sessions` are tenant-wide
and take no `--kb`.

A knowledge base answers nothing until an embedding model is bound. `kb status`
and `kb check` expose this as `retrieval_ready`; `kb create` adds a
`meta.hint` naming the fix when it created an unready KB. Bind at creation
(`kb create "Handbook" --embedding-model <id-or-name> --chat-model <id-or-name>`)
or afterwards with `kb config set <kb> --embedding-model ... --chat-model ...`,
which is exit-10 gated. `aikb model list` shows the ids.

## Streams: stopping, resuming, approving

Ctrl-C on `chat` or `session ask` closes your socket; the server keeps
generating. To stop it, run
`aikb session stop <session-id> --message <assistant-message-id>`. To re-attach
after a dropped connection, run
`aikb session resume <session-id> --message <assistant-message-id>`, which
replays the stored events from the first one, so dedupe anything you already
consumed. `chat` returns the id as `data.assistant_message_id`; for
`session ask`, take the latest assistant message from
`aikb message list --session <id>`.

An agent run may pause when a tool call needs human sign-off. The stream
carries a pending id and the run blocks. Show the user the tool and its
arguments, and after their explicit go-ahead run
`aikb session tool-approval resolve <pending-id> -y`
(`--reject --reason "..."` to refuse, `--modified-args '{...}'` to approve with
changed arguments), then `session resume` to collect the answer. Approving
executes the tool with the user's authority, which is why the command sits
behind exit 10.

`aikb message search "<query>"` finds a past question and answer across all
sessions. Check it before re-running an expensive query the user may already
have asked.

## Command families

| Group | Verbs | Notes |
| --- | --- | --- |
| `kb` | list, view, create, update, delete, pin, unpin, status, check, config set | `status` is one HTTP call; `check` also counts failed documents |
| `doc` | list, view, create, upload, fetch, download, reparse, update, delete, wait | `update` edits title and description only; content changes need a re-upload |
| `chunk` | list, view, delete | stored order, for debugging; retrieval is `search chunks` |
| `search` | chunks, docs, kb, sessions | only `chunks` ranks by relevance |
| `chat` | | one question, grounded in one knowledge base |
| `session` | list, view, delete, ask, stop, resume, tool-approval resolve | `ask --agent <id>` runs a custom agent |
| `message` | list, search, delete | message history within sessions |
| `agent` | list, view, create, update, delete, status, check | configuration only; invoke through `session ask` |
| `model` | list, view, create, update, delete | `update` keeps the id, so KB and agent references survive a key rotation |
| `api` | | raw request to any server path when no verb fits |
| `link`, `unlink` | | bind or unbind the current directory |
| `profile`, `auth`, `config view`, `doctor`, `version` | | connection and health |
| `schema`, `exit-codes`, `skills` | | self-description |

## CLI vs MCP

Drive the CLI yourself whenever you can: it has `--dry-run`, the exit-10 gate,
name resolution and every verb. `aikb mcp serve` exists for a host that speaks
MCP and exposes a smaller read-and-ask surface: `kb_list`, `kb_view`,
`doc_list`, `doc_view`, `doc_download`, `search_chunks`, `chunk_list`,
`agent_list`, `chat`, `session_ask`. Those tools accept ids only; resolve a
name through `kb_list` first.

## Learn a command without guessing

- `aikb schema` lists every command with a one-line `used_for`;
  `aikb schema <command path>` returns one command's flags, required flags,
  output shape, examples and risk as a normal envelope.
- `AIKB_AGENT_HELP=1 aikb <command> --help` prints the same contract as JSON
  instead of the human help page.
- Plain `--help` lists the `.data` fields a command emits, so you can write a
  `--jq` projection before the first live call.

## Mistakes to avoid

| Symptom | Cause and fix |
| --- | --- |
| Exit 10 "cleared" by appending `-y` | The gate is the user's decision. Surface `error.message`, wait for approval, then run `retry_argv`. |
| `auth login` fails with no profile | Agents should set `AIKB_HOST` and `AIKB_API_KEY` instead; if a profile is wanted, `profile add <name> --host <url> --use` first. |
| `--host` passed to `auth login` | Host belongs to the profile or `AIKB_HOST`; `auth login` has only `--with-token`. |
| Batch delete reported `ok:false` yet some ids are gone | Expected. Read `data[].ok`; retry only the failed ids. |
| `doc wait` exit 1 with `ok:true` | A document failed to parse. Read `data.failed[]`; `doc reparse <id>` after fixing the source. |
| `search chunks` returns nothing on a fresh KB | `kb status` shows `retrieval_ready:false`. Bind models with `kb config set` (exit-10 gated). |
| Exit 1 `local.kb_not_found` for a name that exists | Names match exactly and case-sensitively; copy it from `aikb kb list`. |
