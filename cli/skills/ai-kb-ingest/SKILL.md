---
name: ai-kb-ingest
description: Use when adding material to an AI KB knowledge base through the `aikb` CLI - uploading local files or directories, importing a web page or remote file by URL, writing a Markdown note as a knowledge entry, and waiting for parsing to finish before searching.
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
        description: Workspace API key with the ingest capability
---

# AI KB - ingest documents, URLs and notes

**REQUIRED BACKGROUND:** read the `ai-kb-shared` skill first (auth, `--kb`
resolution, the JSON envelope, exit codes). Retrieval is in `ai-kb-rag-search`.

Every ingest command returns a document id in `data.id` and starts parsing on
the server. A document is searchable only after parsing finishes, so a task
that ingests and then searches must `doc wait` in between.

## Pick the command by what you have

| You have | Command | Notes |
| --- | --- | --- |
| A local file (PDF, Word, Excel, Markdown, text, images) | `doc upload <file> --kb <kb>` | `--name` overrides the recorded file name; `--metadata key=value` is repeatable |
| A directory of files | `doc upload <dir> --recursive --glob '*.pdf' --kb <kb>` | One document per matching file; the batch envelope has one item per file with its id or error |
| A URL (web page or remote file) | `doc fetch <url> --kb <kb>` | `--file-type pdf` when the URL has no extension; `--title` sets the display title |
| Text you wrote (notes, summaries, decisions) | `doc create --text "<markdown>" --title "<title>" --kb <kb>` | Use `--text "$(cat notes.md)"` for a file you want stored as an entry, not parsed as an upload |
| Document ids that are still parsing | `doc wait <id> [<id>...]` | Exits 0 when every document is parsed, 124 on timeout (`--timeout 20m`) |

`--tag-id <id>` on `doc fetch` and `doc create` files the entry under a tag;
`--enable-multimodel` on `doc upload` and `doc fetch` turns image-in-PDF text
extraction on for that document.

## The sequence that works

```bash
# 1. pick the knowledge base once (name or UUID; see ai-kb-shared for env and link)
export AIKB_KB_ID=$(aikb search kb "Design docs" --jq '.data[0].id')

# 2. ingest, keeping the ids
ID=$(aikb doc upload ./spec.pdf --jq '.data.id')
URL_ID=$(aikb doc fetch https://example.com/rfc.html --title "RFC" --jq '.data.id')
NOTE_ID=$(aikb doc create --text $'# Decision\n\nWe chose X because Y.' --title "Decision: X" --jq '.data.id')

# 3. wait, then search or chat (ai-kb-rag-search)
aikb doc wait "$ID" "$URL_ID" "$NOTE_ID" --timeout 20m
aikb search chunks "why X"
```

## Safety / Gotchas

- **A knowledge base must be retrieval-ready** (embedding and chat models
  bound) or uploads stay unindexed; `ai-kb-shared` ("Choosing the knowledge base") explains
  `kb create --embedding-model ... --chat-model ...` and `kb config set`.
- **`doc wait` before any search** that depends on the new material. Parsing a
  large PDF can take minutes; the default timeout is 10 minutes.
- **Prefer `--dry-run`** to show the user what a bulk `--recursive` upload will
  send before sending it; it runs offline and prints the plan.
- **Do not paste secrets into notes.** `doc create` stores the text verbatim and
  it becomes retrievable by every reader of the knowledge base.
- **Uploads are not idempotent.** Running the same `doc upload` twice creates
  two documents; check `doc list --kb <kb>` first when a run may be repeated.
- The CLI's API key needs the ingest capability on that knowledge base; an
  `auth.*` error with exit code 3 means the key cannot write there, ask the user
  for another key rather than retrying.
