# `search chunks`: ranked passages, no model

Use this when the task needs the source text itself: quoting, cross-checking,
or building your own context before answering. If the user wants a written
answer, `chat` already does retrieval and synthesis in one call.

## Invocation

```text
aikb search chunks "<query>" [--kb <uuid-or-name>] [flags]
```

| Flag | Default | Effect |
| --- | --- | --- |
| `--kb` | `AIKB_KB_ID`, then the linked directory | knowledge base to search |
| `--limit`, `-L` | 8 | results to return, 1 to 1000; the CLI trims client-side after the server responds |
| `--vector-threshold` | 0 | minimum similarity for the vector channel before fusion; 0 disables the floor |
| `--keyword-threshold` | 0 | minimum score for the keyword channel before fusion; 0 disables the floor |
| `--no-vector` | false | keyword channel only |
| `--no-keyword` | false | vector channel only |

Disabling both channels is rejected. The default of 8 keeps the result inside a
model's context window; raise it when you will filter the passages yourself.

## Output

`data` is an array of chunk objects, `meta.count` is how many came back, and
`meta.has_more` is true when the server returned more than `--limit` and the CLI trimmed the list. Every empty
result carries `meta.hint`: it reminds you that zero results can mean no
match or no indexed chunks, and points at `kb status` (`chunk_count`) and
`doc list` (`parse_status`; documents in `draft` are not indexed).

Fields on each chunk, from `aikb search chunks --help`:

| Field | Use |
| --- | --- |
| `id` | chunk id, for `chunk view <id>` |
| `content` | the passage text |
| `matched_content` | the matched span when the server reports one |
| `score` | the retrieval score; enrichment chunks the server adds for context carry 0 |
| `match_type` | an integer: 0 vector, 1 keyword, 2 nearby chunk, 3 history, 4 parent chunk, 5 related chunk, 6 graph, 7 web search, 8 direct load, 9 data analysis |
| `knowledge_id`, `knowledge_title`, `knowledge_filename`, `knowledge_source`, `knowledge_channel` | the source document |
| `chunk_index`, `seq`, `start_at`, `end_at` | position inside the document |
| `chunk_type`, `image_info`, `metadata` | chunk kind and extra data |

Project what you need:

```bash
aikb search chunks "rotation schedule" --kb ops-runbooks --limit 12 \
  --jq '.data[] | {score, match_type, knowledge_title, content}'
```

## Neighbouring commands

- `chat "<q>" --kb <kb>` when an answer is wanted, not passages.
- `search docs "<q>" --kb <kb>` when you want documents whose title or file
  name contains the query; it filters server-side and does not rank by meaning.
- `chunk list --doc <id>` to read one document's chunks in stored order; it is
  a debugging view, not retrieval.
- `message search "<q>"` to find a previous question and answer rather than
  document text.
