# Conversation experience

In a conversation you ask questions about knowledge bases or uploaded attachments, open the sources behind each answer, and preview the files an agent generated. You can add requirements while an answer is still running, and fork or rewind an existing conversation from any message. The question minimap and history search help you revisit a conversation; Copy as Markdown saves the full transcript.

## View answers and sources

After you ask a question, you want to see what the system is doing and where the answer comes from. The progress bar shows the stages: attachment parsing, retrieval, tool calls and answer generation. Smart Reasoning also shows a collapsible thinking section, command output and file-write progress.

Citation markers in the answer body point at the source chunks, and the Sources panel on the right lists every source used in this turn, including content returned by the Wiki tool. If the agent's citation markers are switched off, the panel still shows the sources. When the answer ends the system can offer follow-up suggestions; see [Agent engine](07-agent.md).

Clicking a knowledge base citation opens the original file in the panel, jumps to the cited position and highlights it:

| Original format | Locate and highlight |
| --- | --- |
| PDF (text) | Jumps to the page and outlines the cited paragraph |
| PDF (scanned) | Jumps to the page; outlines the paragraph when the file was parsed with MinerU or PaddleOCR-VL |
| Word | Scrolls to the cited paragraph or table |
| PPT | Scrolls to the cited slide and highlights the text |
| Excel / CSV | Highlights the cited rows |
| Markdown / TXT / EPUB | Scrolls to the cited text (EPUB locates the chapter first) |
| Audio | Plays from the start of the cited segment (the speech-to-text model must return segment timestamps, as whisper-1 does) |
| Web import | Shows the cited fragment and offers to open the original page at that fragment |

The highlight narrows to the paragraph most relevant to the sentence cited in the answer. Documents imported before this feature shipped carry no position data: the panel searches the original by text, and when nothing matches it says so and leaves the document open at the top. Reparsing the document restores exact positioning. Hand-written knowledge and FAQ entries have no original file, so clicking their citation keeps the source list. "All sources" at the top of the panel returns to the list, and "View in original" on a document card in the list opens the file too. Visitors on an embed channel cannot read original files and only see the source list.

<Screenshot
  src="/screenshots/chat-references-drawer.png"
  caption="Chat page: an answer, its citation markers and the Sources panel on the right"
  hint="One answered turn with citations, the expanded Sources panel (source titles and snippets), and the session action bar at the top." />

Once Quick Answer has finished retrieval, the wait for the model shows "Connecting to the model and generating an answer…"; a question that ran no retrieval shows "Preparing an answer…". After 60 seconds the hint changes to "The model is taking longer than usual, still waiting…". The hint only reflects the waiting state; the answer and the tool results are whatever the model actually returns.

When an answer hits the model's per-response output limit and is cut off, a hint icon appears in the answer toolbar explaining that what you see is what was generated before the cut. For a complete answer, narrow the question or raise the maximum output length in the agent configuration.

## Adjust the reasoning effort

When the selected model supports graded thinking, you can change how hard it thinks for this conversation. A "Reasoning effort" control appears next to the model selector at the bottom of the input box, with the levels the model supports among Off, Auto, Minimal, Low, Medium, High, Extra high and Max. The default follows the agent configuration. Changing it affects only the current conversation and does not modify the agent; reopening the conversation restores your last choice. Switching to a model that does not support the chosen level clears the choice automatically.

## Add requirements while an answer is running

While Smart Reasoning is generating an answer, you can keep typing in the input box:

| Action | Effect |
| --- | --- |
| Enter, or click Send | Queues the message; it is sent as the next question once the current answer finishes |
| ⌘ Enter (Mac) or Alt + Enter | Adds it to the current task immediately; the agent reads it at its next step and adjusts |

Queued messages appear above the input box. Click "Supplement current task" to switch one to immediate delivery, or "Remove" to withdraw it; a message the agent has already picked up cannot be withdrawn. With an empty input box the send button stops generation, and stopping discards the queued messages. An appended message cannot carry attachments or images, but it can use @ mentions. At most 10 messages can be queued per turn. Quick Answer does not support appending; wait for the answer to finish.

<Screenshot
  src="/screenshots/chat-steer-queue.png"
  caption="Queued messages above the input box while an answer is generating"
  hint="A Smart Reasoning answer in progress, one or two queued messages above the input box, each with 'Supplement current task' and 'Remove' buttons; text being typed in the input box, and the send button hinting 'Send after completion · Enter'." />

## Fork and rewind

To explore a different direction or undo part of a conversation, use the two actions below a user message and in the answer toolbar. Both are unavailable while an answer is generating.

- **Fork**: creates a new conversation from the current one; the original is unchanged. Forking from a user message copies the conversation up to that question and puts the question back in the input box so you can edit and resend it; forking from an answer copies the conversation up to and including that answer. The new conversation is titled "original title (fork)" and marked with a branch icon in the sidebar.
- **Rewind**: deletes everything after the selected message in the current conversation; this cannot be undone. Rewinding from a user message deletes that question too and puts it back in the input box. Files generated in the deleted turns, their follow-up suggestions and their history search index entries are cleaned up as well.

When the agent is bound to a sandbox, the system records a workspace checkpoint at the end of every turn. A fork carries the workspace files as they were at the fork point; when that is impossible (the sandbox was reclaimed or replaced, for example) the fork still succeeds and the new conversation gets a fresh sandbox. A rewind rolls the workspace back to the last kept turn; when the current sandbox has no matching checkpoint, the rewind is refused so that files and conversation cannot drift apart. Without a sandbox, or with a local directory on the desktop edition as the workspace, only the conversation is rewound and no files change.

## Ask about a temporary attachment

To ask about a file without adding it to a knowledge base, upload it in the conversation. The attachment belongs to the current conversation only; it does not appear in a knowledge base list or in a vector index. Material that needs long-term retrieval or sharing belongs in a knowledge base.

After upload, the file is parsed asynchronously and shows the uploaded, processing and ready states. When you ask a question while an attachment is still parsing, the question waits for it, by default up to 60 seconds. Scanned and image-only documents can be read with a vision model.

A temporary attachment is retained for a period counted from its upload time, 24 hours by default, after which a background task removes it. Administrators can change the retention period, the parse wait time and the OCR parameters; see [Configuration and API reference](#configuration-and-api-reference) at the end of this page.

The agent configuration decides which file types can be uploaded, whether images are understood, and which parsing engine handles each file type. See [Agent engine](07-agent.md).

## Preview and download generated files

An agent bound to a sandbox can process attachments and generate files. Once the answer is complete, open them from the answer toolbar's generated-files button or from the sandbox button at the top right of the chat page, in the "Files" tab of the "Sandbox" panel. That tab switches between this turn and all files, and searches by file name. Supported file types preview in place; the rest can be downloaded and opened locally.

The "Artifacts" page in the sidebar collects the files generated across all of your web conversations. It filters by type (documents, spreadsheets, presentations, images, web pages, data) or by file name, and each file can be previewed, downloaded or followed to its conversation. When the same path is generated several times in one conversation, only the latest version is listed, with the version count shown. The page is hidden when the deployment has no sandbox capability.

<Screenshot
  src="/screenshots/artifacts-library.png"
  caption="The Artifacts page: files generated across conversations"
  hint="'Artifacts' selected in the sidebar, the type tabs (All / Documents / Spreadsheets / Presentations / Images / Web pages / Data) and the file name search box at the top, the list grouped into Today / Yesterday / Last 7 days, at least one row showing 'N versions', and the preview, download and delete actions in the row." />

For an agent to produce a downloadable file, the artifact must be saved under `/workspace/output`; the system collects and persists that directory when the answer ends. Uploaded attachments are staged in `/workspace/input`, and scripts and intermediate files can go anywhere else under `/workspace`. Input attachments and output files are managed separately: deleting an input attachment does not delete collected output files.

Files you no longer need can be deleted in the "Files" tab or on the "Artifacts" page. Deletion frees the stored bytes and cannot be undone, so it asks for confirmation first. Only the conversation's owner can delete; read-only access through a shared agent can only download. The "Files" tab lists every generation separately, and deleting removes the version you clicked; on the "Artifacts" page one row is one file, and deleting removes every historical version of that file in the conversation. A copy of the file that was saved to a knowledge base, cited again by a later answer, or copied by a fork is unaffected. After deletion the same file in the sandbox is not collected again.

File reading, script execution and the directory conventions are in [Skills and sandbox](22-skills-sandbox.md); the list, download and delete endpoints are in [Session and chat API](../04-api/02-api-chat.md).

## Save an answer to a knowledge base

To keep a good answer as a document, click "Add to Knowledge Base" in the answer toolbar. A Markdown editor opens in a drawer on the right, prefilled with the question and the answer, with the cited documents listed under a "References" heading at the end. Choose a target knowledge base (document knowledge bases only), then save a draft or publish it; indexing starts on publish. The editor offers edit, split and preview views plus full screen, a toolbar with the common formats and inserts for tables, code blocks and images, Cmd/Ctrl + S to save a draft and Cmd/Ctrl + Enter to publish.

<Screenshot
  src="/screenshots/chat-save-answer-editor.png"
  caption="The Markdown editor for saving an answer to a knowledge base"
  hint="The drawer on the right of the chat page in split view: the title input and target knowledge base selector at the top, Markdown source on the left and preview on the right in the middle, and at the bottom 'Status: draft', the word count and the 'Save draft' and 'Publish' buttons." />

## Review and export a conversation

To find your way around a long conversation, use the question minimap on the right: it jumps straight to a question, and hovering shows the turn number, the question and a summary of the answer. A time separator is inserted between messages when the date changes or more than 5 minutes have passed; the send time appears below each user message, as a time of day for today's messages and with the date for older ones. In the sidebar, a spinning icon marks conversations still generating in this browser, and API conversations carry a tag naming the caller.

"Copy as Markdown" in the conversation action menu copies the whole conversation to the clipboard: the title, the ID, the export time and every turn, with attachments and the source list. The copy is produced in the browser and is suitable for archiving or sharing a transcript.

"Search" in the sidebar finds past messages across conversations. Once a workspace administrator enables message indexing with an embedding model under Settings → Workspace → Message management, the search also uses semantic matching; otherwise it matches keywords.

## View channel sessions

Conversations created through IM, web embedding and API calls are isolated per IM identity, visitor or API key. The regular conversation list shows only the caller's own conversations.

A workspace Admin or Owner can open the IM, embed and API groups to review the conversations of each channel; a group appears once it has conversations. This is a read-only view for checking the question-and-answer records of a channel. Write operations on API key conversations remain limited to the original owner.

## Use cross-session memory

To let the assistant remember you across conversations, a workspace administrator enables long-term memory under Settings → Workspace → Long-term memory. Members then maintain their preferences and confirm pending entries under Settings → Account → My memory, and later conversations use them. Memories are isolated per workspace and caller identity.

Turning personal memory off pauses its use; delete or clear removes the data. Automatic extraction, topic management and consolidation are described in [Long-term memory](23-memory.md).

## Configuration and API reference

The parameters below are for administrators and integration developers. Day-to-day conversation operations use the UI described above.

### Attachment configuration

| Setting | Default | Effect |
| --- | --- | --- |
| `AIKB_CHAT_ATTACHMENT_WAIT_TIMEOUT_SEC` | 60 seconds | How long a question waits for attachments that are still parsing |
| `AIKB_CHAT_ATTACHMENT_TTL_HOURS` | 24 hours | Attachment retention, counted from upload time |
| `AIKB_CHAT_ATTACHMENT_OCR_CONCURRENCY` | 8 | OCR concurrency |
| `AIKB_CHAT_ATTACHMENT_OCR_MAX_PAGES` | 8 | Maximum number of pages sent to OCR |

In the agent configuration, `supported_file_types` limits the upload types (an entry also admits its family: `xlsx` covers `.xls`, `.xlsb` and `.xlsm`, `docx` covers `.doc`, `jpg` covers `.jpeg` and `.png`), `attachment_image_understanding` toggles image understanding and `chat_parser_engine_rules` selects the attachment parsing engine.

The attachment API lives under the `/api/v1/sessions` prefix:

| Operation | Method and path |
| --- | --- |
| Upload | `POST /:session_id/attachments` |
| List | `GET /:id/attachments` |
| Detail | `GET /:id/attachments/:attachment_id` |
| Preview | `GET /:id/attachments/:attachment_id/preview` |
| Delete | `DELETE /:id/attachments/:attachment_id` |

Full parameters and permissions are in [Session and chat API](../04-api/02-api-chat.md).

### Session operation endpoints

| Operation | Method and path |
| --- | --- |
| Fork | `POST /api/v1/sessions/:session_id/fork` |
| Rewind | `POST /api/v1/sessions/:session_id/rewind` |
| Append a message to a running answer | `POST /api/v1/sessions/:session_id/steer`, plus endpoints to list, withdraw and promote to immediate delivery |
| Reasoning effort for this turn | The `reasoning_effort` field of the chat request |

Request bodies, status codes and limits are in [Session and chat API](../04-api/02-api-chat.md).

### Citations and history queries

The agent's `citation_enabled` controls the citation markers in the answer body and defaults to on when omitted; the Sources panel is not affected by it.

History search uses `POST /api/v1/messages/search` and index statistics use `GET /api/v1/messages/chat-history-stats`; both require Viewer or above, and an API key needs the `message_history` capability or full access. Loading a conversation's history uses `GET /api/v1/messages/:session_id/load`, which requires Viewer or above; an API key needs the `chat` capability and is subject to the session ownership check.

A session list whose `source` is empty or `web` returns the caller's own conversations; explicitly querying `api`, `im` or `embed` is a workspace-wide view that requires Admin or above and returns 403 otherwise.

## Implementation reference

- `frontend/src/views/chat/components/`: answer, progress and tool activity display.
- `frontend/src/utils/sessionMarkdown.ts`: Copy as Markdown.
- `internal/application/service/session.go`: session listing and channel access rules.
- `internal/application/service/session_fork.go`, `session_rewind.go`, `workspace_checkpointer.go`: fork, rewind and workspace checkpoints.
- `internal/handler/session/steer.go`: appending messages to a running answer.
- `internal/application/service/temporary_document.go`: temporary attachment parsing, expiry and cleanup.
- Temporary attachments are stored in the `temporary_documents` table; the schema is in [Database and migrations](../06-development/02-database-schema.md).
