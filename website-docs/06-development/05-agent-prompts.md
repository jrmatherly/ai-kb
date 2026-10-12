# Conversation prompt assembly and editable scope

This page is for developers who maintain prompts, the agent editor and the model message chain. Agent configuration and the execution flow are covered in [Agent engine](../03-features/07-agent.md).

## Smart Reasoning system sections

The single assembly path is `BuildSystemPromptSections` in `internal/agent/prompts.go`. The engine and the string-returning compatibility function `BuildSystemPromptWithOptions` both use it. The sections below are listed in assembly order. Empty sections are left out of the output.

| Section | Source and role |
| --- | --- |
| `base` | An explicit body wins. Otherwise the `pure` default template is used when no knowledge base is bound, and the `rag` default template when one is |
| `steering` | How to handle messages the user adds while an answer is running (additions, changes or cancellation), keeping unfinished work. Custom bodies get it too |
| `runtime_contract` | The data and instruction boundary, the current document scope and the completion condition. When the user language is known, a default answer language is appended |
| `sources` | Source rules chosen from the set of tools actually registered. Skill installation mode uses dedicated installation verification text instead |
| `tools` | General execution, file and sandbox conventions. Specific parameters stay in the tool definitions |
| `output` | Shared output format, when to include images, and the completion check |
| `skills` | Available skill metadata. Appended only outside skill installation mode, when `read_file` is registered and metadata exists |
| `memory` | Memories recalled for this turn |
| `protocol` | Source handles and the citation protocol for output |

A custom body replaces only `base`. It does not remove the other runtime sections and does not grant tool permissions. The sections are not separate permission levels in the model API; which tools can be called is still controlled by backend registration and the execution path. The browser operation contract stays in the `local_browser` tool description. Pairing and usage are covered in [Local Browser](../05-clients/09-local-browser.md).

`renderPromptPlaceholdersWithStatus` expands the placeholders in `base`:

| Placeholder | Current behavior |
| --- | --- |
| <span v-pre>`{{knowledge_bases}}`</span> | Points to the knowledge base directory in the user message's `runtime_context` instead of expanding the full list. With no bound knowledge base it becomes a note saying none is bound |
| <span v-pre>`{{web_search_status}}`</span> | `Enabled` or `Disabled`, depending on whether the `web_search` tool is actually registered |
| <span v-pre>`{{current_time}}`</span> | The date as `YYYY-MM-DD` |
| <span v-pre>`{{language}}`</span> | The user language name |
| <span v-pre>`{{skills}}`</span> | Removed. Skill metadata comes from its own section |

## Per-turn context and message roles

`internal/agent/observe.go` adds `runtime_context` to the current user message: the date, the session ID, a summary of the bound knowledge bases, pinned documents, the question origin and other per-turn information. It is not persisted as a standing instruction in history. Knowledge base names and descriptions are length-capped and escaped. FAQ answers and full document text have to be read through the retrieval tools.

`@MCP` and `@Skill` mentions produce a `must_use` hint for the current turn. An MCP mention only sets a priority hint for an authorized service and does not remove other configured services. Services not yet exposed are found through `discover_mcp_tools`. A skill mention hints that the matching `SKILL.md` is read first. These selections cannot authorize unrelated actions and remain bound by the user's explicit source restrictions and backend permissions.

When the user adds a message during an answer and chooses to send it right away, it is appended to the end of the message list as a user message before the next iteration. The content sent to the model is wrapped in `<steer_message>` with a `<continue_task>` note (`types.SteerMessageContent`). The session history and the UI keep the user's original text. The `steering` section tells the model how to treat these messages.

Normal tool results keep the `tool` role and the call ID. During message repair, a result that cannot be paired is kept as an escaped `untrusted_tool_result` data block and is never promoted to a system instruction. The agent, ordinary Q&A and model fallback share the source data boundary rules, but the three do not use exactly the same prompt assembly.

When the iteration limit is reached or a run ends in an error, `internal/agent/finalize.go` reuses the current message list, keeping history, images and tool call pairing, then appends a closing request. The last call offers no tools and sets `tool_choice=none`. It does not request thinking, and any thinking output is dropped from the answer stream.

## Editing and saving

The frontend editor shows the effective body. On save, `frontend/src/utils/agentPromptTemplates.ts` tells a template reference apart from custom content:

- Body identical to a known template: save `system_prompt_id` or `context_template_id` and leave the body empty.
- Body actually changed: save the body and clear the old template ID.
- Rewrite and fallback fields that match the current default template are saved empty and inherit the default. Intent prompts save only the overrides that differ from the default.

For example, an unmodified Wiki template is saved as:

```json
{"agent_mode":"smart-reasoning","system_prompt_id":"wiki_researcher","system_prompt":""}
```

`ResolveCustomAgentPrompts` in `internal/config/agent_prompts.go` resolves references at request time. An explicit body always wins, and a reference is looked up only in the template set for its own field and mode. The resolved result is not written back to the saved object. An unknown reference returns an empty body, and the caller takes the default path.

Updating a default template does not overwrite existing custom bodies and does not migrate old copies of earlier defaults in bulk. To make an old configuration follow the template again, restore the default in the editor and save.

## What to check during maintenance

Decide where a new rule belongs first: role and domain method go in `base`, source selection in `sources`, calling conventions in the tool definition or `tools`, output shape in `output`, citation encoding in `protocol`. Avoid maintaining the same rule in several places.

The `[Agent][Prompt] section=... bytes=...` debug log helps locate the size of each section. Checking behavior also means inspecting the final model messages, the actual tool registry and the execution permissions, and running tasks that exercise source selection, failure recovery and output format.
