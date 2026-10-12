# Cross-session long-term memory

Long-term memory keeps a caller's profile, phrasing preferences, stable facts and standing interests for later conversations. Data is isolated per workspace and caller identity: different workspaces, IM users and embed visitors each keep their own memories.

## Enable long-term memory

To make the assistant remember a person across conversations, a workspace administrator turns the feature on and each member decides whether to use it.

1. A workspace Admin or Owner turns on the workspace switch under Settings → Workspace → Long-term memory. It is off by default.
2. Pick "explicit only" or "automatic extraction". Automatic mode can name an extraction model; leave it blank to reuse the model the conversation used.
3. Members can turn memory off for themselves under Settings → Account → My memory. A workspace switch that is on never forces personal memory on.
4. In a conversation, state the preference to remember, or add it by hand under My memory, then check in another conversation that it applies.
5. Review pending entries regularly: confirm, edit or reject inferences, and delete items that no longer hold.

| Mode | Behavior |
| --- | --- |
| `explicit_only` | Records only what the user explicitly asks to remember; no background LLM distillation runs |
| `auto` | Additionally extracts memories from conversations in a background task, batched by a delay and a minimum interval |

An agent with `memory_enabled=false` disables memory reads and writes for itself; omitting the field inherits the workspace setting. IM and embed channels follow the memory preference of the agent they are bound to. An API key must be full-access to call the personal memory management endpoints. The feature is gated by the workspace switch, the personal switch and the current request together.

## Manage memories

Under My memory, a member curates what the assistant keeps about them.

| Action | Purpose |
| --- | --- |
| Add / Edit | Maintain profile, preference, fact, task and interest entries directly; a manually edited entry is no longer overwritten by background extraction |
| Confirm a pending entry | An inferred entry moves from `pending` into use; unconfirmed entries are never injected into the prompt. If the inference replaces an existing memory, the old entry stays in effect until you confirm, and is superseded at that moment; confirming an inference that has since gone stale fails, so refresh the list |
| Reject / Delete | Withdraw a wrong or unwanted memory; a rejection is remembered so the same inference is not proposed again |
| Promote a topic | Turn a recurring topic into a standing interest right away, or stop tracking a topic |
| Document preferences | See the documents cited repeatedly and remove personalized retrieval preferences you no longer want |
| Consolidate now | Merge near-duplicate entries and archive expired tasks without waiting for the background pass |
| Export / Clear | Download your memories as JSON, or clear your memory data |

Memory entries are of kind profile, preference, fact, task or interest. A new fact on the same topic can supersede the old one. Turning the personal switch off pauses use; delete or clear removes the data. The category and status values used by the API are in the [Memory API](../04-api/02-api-memory.md).

## Effect on answers and retrieval

Memory changes how a question is understood and which material is chosen, so it helps to know what is injected and when.

The system keeps profile, preference and interest entries as a resident context block under a length budget, and recalls facts and tasks that match the current question. With an embedding model configured, semantic recall ranks all memories of that identity by relevance to the question, independent of entry importance; on PostgreSQL with pgvector the ranking runs in the database, elsewhere it is computed in the service. Smart Reasoning can also search memory and past conversations on its own. Memory only affects question understanding and material selection; it never widens knowledge base access.

With retrieval conditioning on, the system uses topics to interpret the question and uses repeatedly cited documents as a ranking signal. The conversation timeline shows memory-related steps. The background task records an extraction cursor and a pending state per session, so rapid consecutive questions, batch truncation and service restarts do not lose messages. When the model output for one message segment stays unparseable, the segment is retried up to three times, then skipped so later messages keep flowing; a skipped segment is not re-extracted automatically.

## Troubleshoot memory that is not applied

When a memory does not show up in an answer, work through the switches and states below before looking elsewhere.

| Symptom | Check |
| --- | --- |
| No memory is used | The workspace switch, the personal switch and the agent switch; confirm you did not change workspace or identity |
| Automatic extraction does not appear | `write_mode`, the extraction delay and minimum interval, the model connection and the background task |
| An inference does not affect answers | Whether it is still `pending`; it is used only after confirmation |
| A document should no longer be cited | Remove it under document preferences; knowledge base permissions are controlled separately |
| Consolidate now merged nothing | Read the returned `skipped` reason; there may be too few entries or no candidates |

## Workspace configuration

Workspace administrators maintain the memory switch, extraction mode and models on the settings page. The configuration is stored in `memory_config` and read and updated through `GET/PUT /tenants/kv/memory-config`; the full field list is in the [Memory API](../04-api/02-api-memory.md).

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Workspace switch |
| `write_mode` | string | `explicit_only` | `explicit_only` / `auto`; `PUT` rejects any other value, and a stored unknown value is read as `explicit_only` |
| `extract_model_id` | string | empty | Extraction model; empty reuses the conversation model |
| `extract_delay_seconds` | int | `90` | Delay after an answer completes before extraction runs, so several messages in a row share one call; range 5 to 3600, values below 5 are raised to 5 |
| `extract_min_interval_seconds` | int | `300` | Minimum interval between two extractions for the same person, to bound model calls; messages inside the interval wait for the next run; at most 86400 |
| `extract_instructions` | string | empty | The workspace's own extraction rules, at most 1000 characters |
| `max_items` | int | `200` | Cap on active memories per identity, at most 2000; 0 uses the default |
| `interest_threshold` | int | `3` | Number of conversations a topic must appear in before it becomes an interest, at most 20 |
| `embedding_model_id` | string | empty | Embedding model used for memory recall; empty means lexical matching only |
| `vector_recall` | bool | on when omitted | Whether semantic recall is used when an embedding model is configured |
| `retrieval_conditioning` | bool | on when omitted | Allow memory to shape question understanding and document ranking |

The extraction model does the background distillation; the embedding model matches memories against the current question. Memory adds the corresponding model calls. Deleting a model that the memory configuration references shows the dependency details.

## Implementation reference

- `internal/application/service/memory/`: scope, extraction, recall, topics, document preferences, consolidation
- `internal/handler/memory.go`, `internal/router/routes_memory.go`: personal API
- `internal/types/memory.go`: models, budgets, statuses and configuration
- `frontend/src/views/settings/MemorySettings.vue`, `MemoryWorkspaceSettings.vue`
