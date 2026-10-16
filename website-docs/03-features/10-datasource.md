# Data source import

A data source keeps content from platforms such as Feishu/Lark, Notion, Confluence and Yuque synced into a knowledge base. Once a connection exists, new and modified content is fetched on a schedule, and content deleted at the source is handled according to the sync settings.

Data sources are configured per knowledge base. Open the knowledge base's edit settings, go to the "Data sources" tab, create a connection with its credentials, then choose the sync scope and schedule. The first sync fetches everything in scope; later syncs update incrementally, as far as the connector supports it.

<Screenshot
  src="/screenshots/datasource-sync.png"
  caption="Data sources: connection list and sync status"
  hint="Shows the configured data sources (type, target knowledge base, last sync time, status) and the entry point to the sync logs." />

Connectors read external content, the scheduler triggers syncs, and the service layer compares changes and imports knowledge items.

## Connect and sync

This section walks through creating a connection and running the first sync.

1. As a workspace administrator, open the "Data sources" settings of the target knowledge base.
2. Choose a connector, enter its credentials and test the connection; confirm that the resources you need are listed.
3. Choose the sync scope and schedule, save, and start the first sync.
4. Check the sync status and logs; confirm that created, updated, skipped and failed items match expectations.

Updating credentials requires submitting the complete credential set, which the system validates online. Pausing a data source stops further scheduled syncs; resuming registers the schedule again.

`sync_schedule` is a six-field cron expression that includes seconds. `0 0 */6 * * *` fires every six hours; an empty string means manual sync only. The expression is validated when a data source is created, edited or resumed. An invalid value returns HTTP 400 and leaves the saved configuration and any existing schedule unchanged. A data source that already holds an invalid expression must be changed to a valid one, or cleared, before it can be resumed.

## Choose a connector

Pick the connector that matches where the content lives; the type identifier is what the API and the configuration use.

| Connector | Type identifier | What it syncs |
| --- | --- | --- |
| Feishu / Lark wiki | `feishu` / `lark` | Wiki spaces and nodes |
| Feishu Drive / Lark Drive | `feishu_drive` / `lark_drive` | Selected drive folders, see [Feishu Drive](24-feishu-drive.md) |
| Notion | `notion` | Pages and databases |
| Confluence | `confluence` | Pages in Server / Data Center or Cloud spaces |
| Yuque | `yuque` | Knowledge base documents |
| DingTalk Docs | `dingtalk` | Knowledge bases, folders and online documents |
| Tencent IMA | `ima` | Files and notes in knowledge bases |
| GitLab | `gitlab` | Directories under a chosen branch or tag of a repository |
| Seafile | `seafile` | Directories and files in a library |
| RSS / Atom | `rss` | Feed entries |

The formats, authentication and deletion detection of each connector are in the reference sections below.

## Check changes and failures

Use this section to understand what a sync changed and why it failed.

The first sync fetches the selected scope; later syncs update by connector cursor and modification data. Deletion detection depends on the connector and on the sync settings; entries that age out of an RSS feed do not count as source deletions. When a sync fails, read the log for credential, resource visibility or parsing errors first, then test the connection and retry.

With deletion sync on, a sync of a data source whose sync mode is Full also reconciles, for Feishu/Lark (wiki and Drive), Confluence Server / Data Center, Seafile and DingTalk. There is no separate "force full sync" action; the sync mode decides. When the connector listed every selected resource completely, knowledge items of the data source missing from that listing are deleted. That removes the items of resources taken out of the selection. Confluence Cloud never reconciles, because its listing API gives no way to tell a cut-short listing from a complete one. A deletion that fails stays pending, and the next sync, incremental or full, retries it.

A mass-deletion guard checks every deletion a sync would make, in every sync mode: the deletions the connector reports itself (a file gone from the source, a Confluence space or Seafile folder taken out of the selection), the full-sync reconcile's, and the ones an earlier sync held back. A sync collects them while it runs and decides once, after its listing ends. When it would delete 500 or more items, or 10 or more items that are also at least 10% of the items the data source owns, it deletes nothing and the run ends as Partial with the code `deletion_guard_tripped`. A full-sync listing that returned nothing while the data source owns items is held back the same way (`reconcile_listing_empty`). The sync log then shows how many items would go, how many of them come from resources removed from the selection, and up to 100 of their titles. Held deletions are stored with the data source's sync cursor, so later syncs keep counting them after the connector has stopped reporting them, and a held deletion is dropped only when the source reports the item, or the document it belongs to, again, or when deletion sync is turned off. While held deletions wait, every later sync holds its own deletions with them, even a batch under both limits. If the deletion is expected, choose "Sync and apply deletions" in the data source's menu: it runs one sync that applies the held deletions together with its own, without both checks, and every later sync applies them again. A run whose listing could not be confirmed complete records `reconcile_listing_incomplete` and stays a success. When a sync does delete, the log counts the items from resources removed from the selection and the items gone from the source. Seafile reports all of them as gone from the source, because its items record the library rather than the selected folder.

Editing a data source with deletion sync on shows a notice when the edit drops a selected resource or turns off DingTalk's uploaded files: the knowledge items it takes out of scope will be deleted, at the latest by the next full-mode sync, and a large removal waits for "Sync and apply deletions". Checking the parent of a selected resource widens the scope and shows no notice.

## Connector and API reference

This section is for operators and developers who need the exact behavior of each connector and of the REST API.

### Connector implementations

#### Connector capability comparison

| | Feishu / Lark | Notion | Yuque | RSS / Atom |
| --- | --- | --- | --- | --- |
| Source directory | `internal/datasource/connector/feishu/wiki/` (shares `feishu/core/`) | `connector/notion/` | `connector/yuque/` | `connector/rss/` |
| Type identifier | `feishu` / `lark` | `notion` | `yuque` | `rss` |
| Authentication | Self-built enterprise app `app_id` + `app_secret` (tenant_access_token) | Internal Integration Token (`api_key`) | Personal or team token (`api_token`, `X-Auth-Token` header) | None, or custom request headers (`auth_headers`) |
| Credential fields | `app_id`, `app_secret`, `base_url` (optional override) | `api_key` (`base_url` lives in Settings) | `api_token`, `base_url` (optional, self-hosted deployments) | `auth_headers` (optional, a credential); `feed_urls` lives in Settings |
| Resource model | Wiki space, then node tree (lazy-loaded, `spaceID:nodeToken` composite ID) | Full page and database tree (one response with parent relations) | Flat list of knowledge bases (book/repo) | One resource per feed URL (flat) |
| Content format | Export API produces `.docx` / `.xlsx`; drive files download as-is | Blocks to Markdown; databases become Markdown tables; attachments downloaded | Raw `body` Markdown (`.md`) | Readability full-text extraction, then HTML to Markdown |
| Incremental mechanism | Compares content `obj_edit_time` (cursor: `SpaceNodeTimes`) | Compares page or record `last_edited_time` (cursor: `PageEditTimes`) | Compares document `content_updated_at` (cursor: `BookDocTimes`) | Two-level fingerprint: feed signal fingerprint, then content SHA-256 |
| Deletion detection | Yes (in cursor but absent from the current tree gives `IsDeleted`; skipped when part of the listing failed) | Yes (distinguishes "deleted at source" from "deselected by the user"; the latter is not reported as a deletion) | Yes | No (feeds roll old entries off naturally) |
| Streaming resumable sync | Yes (`StreamingConnector`, checkpoint every 50 nodes or 30 seconds) | No | No | No |
| Rate limiting | 429 (or code 99991400) sets a cooldown shared by every caller of that API: the larger of `x-ogw-ratelimit-reset` and `Retry-After`, else `FEISHU_RATE_LIMIT_RETRY_WAIT` (10 s), capped by `FEISHU_RATE_LIMIT_MAX_WAIT` (5 min), at most 3 retries; network errors back off 2 s / 4 s / 8 s; 5xx retried once | - | 300 ms pause between `GetDocDetail` calls (personal tokens allow about 100 requests per 5 minutes) | - |
| Partial failure | A failed document becomes a placeholder entry with error metadata; the sync continues | A failed page is logged and skipped | A failed document becomes a placeholder entry | A failed feed yields `PartialFetchError`; only all feeds failing counts as a failure |

#### Feishu / Lark (`connector/feishu/wiki/`)

Drive app permissions, folder authorization, resource selection and the `FEISHU_DOCX_PARSE_MODE` trade-off are in [Feishu Drive](24-feishu-drive.md). The default export mode and the blocks mode treat images and attachments differently; with deletion sync on, knowledge items that belong to the current data source are deleted.

Feishu and Lark (the international edition at open.larksuite.com) are the same product on two isolated clouds with identical Wiki, docx and drive APIs, so they **share one connector implementation**. The `Region` struct in `feishu/core/region.go` selects the cloud (`RegionFeishu` / `RegionLark`, mapped to the types `feishu` / `lark` and the API hosts `open.feishu.cn` / `open.larksuite.com`). The `base_url` credential field can override the host explicitly, which keeps older data sources that pointed the feishu connector at larksuite working.

- **Authentication** (`core/client.go`): `POST /open-apis/auth/v3/tenant_access_token/internal` exchanges the app credentials for a tenant_access_token, cached behind a mutex and refreshed on expiry.
- **Resource listing** (`ListResources`): three lazy levels. `parentID==""` lists Wiki spaces; `parentID==spaceID` lists the space's top-level nodes; `parentID=="spaceID:nodeToken"` lists that node's children. Early versions recursed through the whole tree up front, which timed out on large wikis; recursion now happens only during sync. `ResolveResourceAncestors` walks up through `GetWikiNode`'s `parent_node_token`, O(depth), to re-display deep selections.
- **Content fetching** (`fetchNodeContent`) dispatches on `obj_type`:
  - `docx` / `doc`: asynchronous export API (`POST /drive/v1/export_tasks`) producing `.docx`;
  - `sheet` / `bitable`: export to `.xlsx`;
  - `file`: raw drive download (PDF, Word, images and so on);
  - `mindnote` / `slides`: **skipped** (no content read API). `fetchTally` logs a `discovered/fetched/failed/skipped_unsupported by_type` summary that explains "13 found, why only 3 synced".
- **Incremental logic**: the cursor is `FeishuCursor.SpaceNodeTimes` (`resourceID -> nodeToken -> editTime`). Change detection uses `obj_edit_time` (content edit time), **not** `node_edit_time` (which only reflects renames and moves). A node whose fetch failed **does not advance the cursor** (its old editTime is kept, so prev != current on the next run and it is retried), so a transient export failure never skips a document permanently.
- **FetchStream**: one code path for full and incremental sync (cursor==nil means full). The cursor is persisted every `FeishuStreamCheckpointInterval = 50` processed nodes, or whenever `FeishuStreamCheckpointMaxInterval = 30s` has passed since the last checkpoint. The time bound covers "few documents, but every export is extremely slow (rate limited)", where the 2-hour timeout would otherwise arrive before the first checkpoint.
- **Error classification** (`feishuFailure`): typed errors, never message text, map to stable i18n codes (`feishu_auth_or_permission` / `feishu_not_found` / `feishu_rate_limited` / `feishu_timeout` / `feishu_server_unavailable` / `feishu_api_error` (+code) / `feishu_api_error_generic` / `sync_failed`) that the frontend localizes; the raw status, body and log_id stay in the server log. Permission and not-found come from the Feishu codes in `core.APIError.Is` and the export `job_status`; 5xx is server unavailable, 429 or code 99991400 rate limited.

#### GitLab (`connector/gitlab/`)

Choose GitLab as the data source, enter `credentials.base_url` and `access_token`, then pick the projects, a branch or tag, and the directories. The token must be able to read the selected projects' repositories; the visibility of private projects follows the GitLab credential.

`config.settings.projects` is a non-empty array whose entries hold a string `project_id`, an optional `ref` and `paths`. An empty `ref` uses the default branch; empty `paths` selects the whole repository; directories are relative paths with forward slashes. Validate the credentials and browse the resources before saving a scheduled sync.

Streaming sync supports resume from a checkpoint; incremental sync updates files from the repository commit diff, and source deletions follow `sync_deletions`. Selected repository files still pass AI KB's file type, size and parser checks, so not every code or binary file can be imported. Each file is identified by the GitLab instance, project ID, branch or tag and repository path, so identical content under another path, branch, project or data source is a separate knowledge item. A re-sync of the same file replaces its knowledge item; a renamed or moved file is imported under its new path, and the old item is removed only when `sync_deletions` is on.

```json
{"credentials":{"base_url":"https://gitlab.example.com","access_token":"<token>"},"settings":{"projects":[{"project_id":"123","ref":"main","paths":["docs"]}]}}
```

#### Seafile (`connector/seafile/`)

1. Obtain the user API token of a Seafile account and confirm that the account can read the target library.
2. Add a "Seafile" data source with the Seafile address (a deployment path prefix is supported) and the API token. A private-network deployment, or a separate fileserver address, must be added to `SSRF_WHITELIST` first.
3. After testing the connection, select the root, directories or files of one library in the resource tree; encrypted libraries and unsupported file formats do not appear in the picker.
4. Start the first sync. Files enter the knowledge base as `<library name>/<path inside the library>` with "Seafile" as the source.

The connector supports scheduled, incremental and streaming checkpointed sync, and deletion sync. A sync scans directories level by level, compares file fingerprints as "object ID + modification time + size", and downloads only files that need updating. A download is two steps, an API call for the link and a fileserver request for the content; the fileserver request carries no token. A checkpoint is saved every 50 cursor changes, and an interrupted sync resumes from the last checkpoint.

A single failed file does not block the others; its known path is kept pending retry. A failed directory scan, or a `Retry-After` above 60 seconds, ends the run and suspends deletion reconciliation. Deleting knowledge items is governed by the "sync deletions" switch, and the per-file size limit follows `MAX_FILE_SIZE_MB`. A synced data source cannot be re-pointed at another library; create a new data source instead.

Configuration, format limits, failure recovery and deletion boundaries: [Seafile](25-seafile.md).

#### Tencent IMA (`connector/ima/`)

Enter `credentials.client_id` and `api_key`; `base_url` is optional and defaults to `https://ima.qq.com`. The resource list is the IMA knowledge bases visible to the credential (a flat list, folders are not expanded); the selection is saved as `config.resource_ids`, and a sync walks every subfolder of the selected knowledge bases recursively. If source authorization fails or resources are not visible, check the IMA credentials and the knowledge base access rights first.

Downloadable files go through document parsing, web pages are fetched by URL, and notes are read through the note OpenAPI. AI sessions and videos have no readable body and are skipped. Full and incremental sync are supported: a stable identity is built from the knowledge base, parent folder and title, so a same-name file replaced at the source (which changes its media_id) triggers an update; deletions are detected only after a complete listing succeeds.

```json
{"credentials":{"client_id":"<client-id>","api_key":"<api-key>"},"resource_ids":["<resource-id-from-tree>"]}
```

Feishu/Lark sync records take their update time from the content edit time, so body changes are not missed when only the wiki node time was considered. GitLab and IMA both support deletion detection; entries that roll off an RSS feed are not treated as deletions.

#### Notion (`connector/notion/`)

- **Authentication**: Internal Integration Token (credential field `api_key`), API version `NotionAPIVersion = "2026-03-11"`, default host `https://api.notion.com` (`Settings.base_url` can override it).
- **Resource listing**: the Search API returns all visible pages and databases at once, as a complete tree with `ParentID` (so a lazy request with `parentID != ""` returns empty, and `ResolveResourceAncestors` needs no extra work). `resolveParentID` handles the `data_source` objects of the 2025-09-03+ API: their `parent` points at the database container, and the real workspace location is in `database_parent`.
- **Fetching**: `fetchPage` processes a page recursively. `GetBlockChildrenAll` pulls the blocks, `BlocksToMarkdown` (`markdown.go`) renders Markdown; `file_upload` file blocks go through `ResolveBlock` for a temporary download URL; **attachments** (PDFs and the like, but not images, which are already inlined as `![](url)` in the Markdown) are downloaded as separate entries; `child_page` / `child_database` blocks recurse. Databases take two shapes: a whole database renders as one Markdown table (`buildDatabaseItem`, with each record's block content appended), and a record that appears on its own renders as "property list + block content" (`buildRecordItem`). Property extraction (`propertyToString`) follows the `type` key generically, so every property type is covered; property names are sorted alphabetically to keep incremental comparison deterministic.
- **Incremental logic**: the first sync (empty cursor) delegates to `FetchAll` and builds the cursor from the returned items' `UpdatedAt`; later syncs use `discoverAllResources` (Search API + BFS) to find every descendant of the selected roots and compare `last_edited_time` page by page. Databases go through `fetchDatabaseIncremental`: any changed record rebuilds the whole table.
- **Deletion versus deselection**: a page that disappeared at the source is reported as `IsDeleted`; a page that is still visible but no longer reachable because the user deselected an ancestor goes into the excluded set and is **not** reported as deleted. `computeExcludedSet` also ensures that "new pages the user has never seen" are not excluded: a selected parent still picks up new child pages automatically.

#### Confluence (`connector/confluence/`)

Confluence Server / Data Center and Confluence Cloud are both supported, both with HTTP Basic authentication.

| Field | Required | Description |
| --- | --- | --- |
| `edition` | No | `server` (default, Server / Data Center) or `cloud` |
| `base_url` | Yes | Confluence address; `https://` is added when the scheme is missing; for Cloud, `https://<site>.atlassian.net` gets `/wiki` appended automatically |
| `username` | Yes | Server/DC username; for Cloud, the Atlassian account email |
| `password` | Server/DC | Server/DC account password |
| `api_token` | Cloud | Created on the Atlassian account's API tokens page |

`edition`, `base_url` and `username` are not secrets; the UI also writes them into Settings so they can be shown again when editing. `password` / `api_token` are stored only as encrypted credentials.

<Screenshot
  src="/screenshots/datasource-confluence.png"
  caption="Confluence data source: edition selection and credentials"
  hint="The data source edit dialog with Confluence selected, showing the 'Confluence edition' dropdown (Server / Data Center, Cloud), the address, username and Cloud API token fields, and the space picker after a successful connection test." />

- **Scope**: the resource list is the spaces visible to the credential (flat); select at least one space. All published pages in a space are synced; attachments and blog posts are not imported.
- **Content**: the page's rendered HTML is converted to Markdown (one `.md` entry per page); a page with an empty body keeps only its title. Entry metadata carries `space_key`, `space_name` and `page_id`, plus `creator` when an author is known.
- **Incremental**: pages are compared by version number, and unchanged versions are skipped. Sync is streaming, with the cursor saved after each page; during a full sync the mid-run cursor also keeps the previous page baseline, so a retried task resumes and can still reconcile deletions.
- **Deletion guard**: when a selected space is inaccessible, when the page list is empty but the previous run had content, or when at least 20 pages are missing and they make up 80% or more of the previous page count, the whole sync fails and nothing is deleted. With deletion sync on, missing pages are deleted in every other case, subject to the mass-deletion guard described in [Check changes and failures](#check-changes-and-failures). Confluence Cloud does not take part in the full-sync reconcile described there, so a full sync on Cloud records no `reconcile_listing_incomplete`.
- **Errors**: a failed page becomes a placeholder entry with the error and the sync continues; the codes are `confluence_auth_or_permission`, `confluence_not_found`, `confluence_rate_limited`, `confluence_server_unavailable`, `confluence_api_error` and `confluence_sync_failed`; the raw response stays in the server log.

```json
{"credentials":{"edition":"cloud","base_url":"https://team.atlassian.net","username":"me@example.com","api_token":"<token>"},"resource_ids":["<space-id>"]}
```

#### Yuque (`connector/yuque/`)

- **Authentication**: a personal token (Yuque settings, Token) or a team token, credential field `api_token` (header `X-Auth-Token`), plus an optional `base_url` (self-hosted enterprise domain; `https://` is added when the scheme is missing).
- **Resource listing**: `GET /api/v2/user` identifies the token. `type=="Group"` means a team token, which lists the team's repos directly; otherwise personal repos plus the repos of joined groups are listed (Yuque returns 404 when the user has joined no group, which is treated as empty). The output is a flat list of `book` resources, sorted stably by ExternalID.
- **Fetching** (`walk`, shared by full and incremental sync): `ListBookDocs` lists documents, documents with `type != "Doc"` (Sheet, Thread, Board, Table) and `status != "1"` (drafts) are filtered out, a 300 ms sleep separates `GetDocDetail` calls to stay under the rate limit, and when `format` is `markdown` or `lake` the raw `body` Markdown is imported (other formats such as html are skipped defensively with a `skip_reason`).
- **Incremental logic**: the cursor is `yuqueCursor.BookDocTimes` (`bookID -> docID -> content_updated_at`); an unchanged value is skipped. Deletion detection: in the cursor but absent from the current list gives `IsDeleted`.
- **Folder structure (`settings` keys)**: by default every document is placed flat at the knowledge base root. Two keys control this:
  - **`folder_mode`** (default `none`): `none` writes a bare `<title>.md`, byte-for-byte identical to the historical behavior; `toc` builds `<book name>/<group>/.../<title>.md` from the book's table of contents. The frontend writes `toc` when a data source is **created**; existing data sources keep `none`, so an upgrade does not change existing sources.
  - **`toc_only`** (default `false`): import only documents that appear in the book's table of contents. **Effective only with `folder_mode=toc`**, because the table of contents is fetched only in that mode; under the flat layout the key does nothing. The criterion is "appears in the table of contents", not "has a path": a document attached to the table of contents but not placed in any group is still kept. **It is an admission filter, not a cleaner**: a filtered document is simply not imported; a copy already synced into the knowledge base stays as it is and is not reported as deleted. The Yuque web UI does not show documents created through the API and never attached to the table of contents, so treating them as deleted would make their content unreachable on both sides.
- **Folder ownership**: with `toc` on, the connector owns the folders. A folder moved by hand in the knowledge base is moved back to Yuque's structure the next time that document syncs.
- **Notes and limits**:
  - **Run one full sync after switching the folder mode**: after `folder_mode` changes on an existing data source, incremental sync only moves documents whose content changed; trigger one full sync to re-place every imported document under the new mode.
  - **Moving a document only within the Yuque table of contents is not reflected immediately**: incremental sync detects changes by `content_updated_at`, so a document that moved without a content change lands in its new folder at the next content change or the next full sync.
  - **`toc_only` does not remove imported documents**: once filtered, a document is no longer synced, the existing copy stays in the knowledge base, and a later deletion in Yuque is not synced either; clean it up by hand in the knowledge base when needed.

#### DingTalk Docs (`connector/dingtalk/`)

- **Authentication**: the Client ID and Client Secret of an internal enterprise app, plus the Union ID of an operator who can access the target knowledge bases; grant `Wiki.Workspace.Read`, `Wiki.Node.Read` and `Storage.File.Read`, then publish the app.
- **Scope**: select knowledge bases, folders or single `ALIDOC/adoc` online documents; they are converted to Markdown through the public Wiki / Blocks APIs. DingTalk sheets are not imported, and no asynchronous export callback is involved.
- **Uploaded files (off by default)**: with `Settings.include_uploaded_files` (default `false`) on, ordinary uploaded attachments in the selected scope (`FILE/DOCUMENT` entries of type `docx` / `pptx` / `xlsx` / `pdf`) are downloaded and handed to AI KB's own parsers. The file name keeps its original extension, which selects the parser; one file may be at most 64 MiB, above which it fails without retry (re-signing would return the same bytes). A failed download (401, 403, 404 and the like) requests a new signed URL and retries, at most 3 attempts.
- **Default and cost of the switch**: an unset or invalid `include_uploaded_files` is treated as `false`, so **the next sync of an existing data source after an upgrade, incremental syncs included, behaves as before** and does not suddenly start downloading and embedding attachments. Once on, those files' content enters the knowledge base, which uses storage and embedding cost; keep it off when not needed. When off, connection validation, the resource list and sync all skip uploaded attachments, and the sync log gives "`include_uploaded_files` not enabled" as the reason, not "unsupported type".
- **Where the switch applies**: connection testing (validation), the resource picker, full sync and incremental sync share one check, so turning it off in one place is consistent everywhere.
- **Sync**: incremental reads by document `modifiedTimestamp` (milliseconds), falling back to `modifiedTime` when missing; overlapping selections are merged. A full sync also reconciles deletions against the previous cursor, so `sync_mode=full` does not miss deletions. Deletions are postponed when the directory walk was incomplete.
- **Body**: the public Blocks API returns only the first-level blocks under the document root. Containers such as callouts keep rendering when the response carries `children`; otherwise the metadata is marked `nested_blocks_unavailable`, so a truncated body is not mistaken for a complete success.
- **Validation**: a connection test lists the knowledge bases, probes the root node list, and reads Blocks when an online document exists under the root. For uploaded files with ingestion on, it only resolves the storage location and requests a download URL, without downloading the body, so a missing `Wiki.Node.Read` / `Storage.File.Read` shows up early while validation stays cheap for multi-megabyte files.
- **Failure and recovery**: an invalid resource does not block the other scopes; failed scopes and documents whose body failed keep their previous version for retry. When any scope cannot be walked completely, deletions are postponed and the pending records are kept. An invalid single selection needs a permission check or re-selection.
- **Deletion switch**: local knowledge confirmed deleted at the source is removed only with deletion sync on; an inaccessible resource is not treated as deleted.

#### RSS / Atom (`connector/rss/`)

- **Configuration**: `feed_urls` (newline or comma separated, duplicates removed) lives in **Settings** (not secret, editable in the UI). `auth_headers` (one `Name: Value` per line, attached only to feed requests and never sent to third-party article pages) lives in **Credentials** and is encrypted. `HasConfiguredCredentials` special-cases RSS: only `auth_headers` counts as configured credentials.
- **Fetching**: `gofeed` parses RSS, Atom and JSON feeds. When an entry has a link, the article page is fetched and run through the readability extractor; on success the full text wins, on failure the feed's own content (`content:encoded` / `description`) is used. HTML is converted to Markdown with `html-to-markdown/v2`. The entry ID is the first non-empty of `GUID > Link > Title`.
- **Incremental logic**: two-level fingerprint. The feed-side signal fingerprint (`feedSignalFingerprint`) is compared first, and an unchanged entry is not even fetched; then the SHA-256 of the fetched content is compared. **Deletion sync is not supported** (feeds roll old entries off naturally).
- **Partial failure**: when one feed fails to fetch or parse, its old cursor is kept (`copyFeedCursor`) and the other feeds continue; the result is reported as `datasource.PartialFetchError` (the SyncLog records `partial`). Only when every feed fails is the whole sync an error.

### Data source lifecycle and REST API

Routes are registered in `RegisterDataSourceRoutes` in `internal/router/routes_infra.go` (list, detail and sync logs are Viewer+, everything else is Admin+; an API key needs `manage_datasources` or full access):

| Method and path | Permission | Description |
| --- | --- | --- |
| `GET /api/v1/datasource/types` | Viewer | Metadata of the available connectors (`ListAvailableConnectors`, sorted by Priority) |
| `POST /api/v1/datasource/validate-credentials` | Admin | Test connectivity with raw credentials (nothing is persisted); backs the "Test connection" button of the creation wizard |
| `POST /api/v1/datasource/list-resources` | Admin | List the selectable resources from an unsaved configuration (`type`, `credentials`, `settings`, `resource_ids`, `parent_id`; nothing is persisted); backs the resource step of the creation wizard |
| `POST /api/v1/datasource` | Admin | Create a data source (check the KB belongs to the workspace, check the connector type, online Validate, persist, register the cron entry) |
| `GET /api/v1/datasource?kb_id=` | Viewer | List the data sources of a knowledge base (with the latest SyncLog) |
| `GET /api/v1/datasource/:id` | Viewer | Detail |
| `PUT /api/v1/datasource/:id` | Admin | Update (credential fields are ignored; online validation runs only when the configuration actually changed and credentials exist; the cron entry is updated) |
| `DELETE /api/v1/datasource/:id` | Admin | Soft delete, remove the cron entry, cancel pending and running SyncLogs |
| `PUT /api/v1/datasource/:id/credentials` | Admin | Replace the credentials atomically (see [Credential encryption and storage](#credential-encryption-and-storage)) |
| `DELETE /api/v1/datasource/:id/credentials/:field` | Admin | Clear the credentials (`field` accepts only `credentials`) |
| `POST /api/v1/datasource/:id/validate` | Admin | Connection test for a stored data source; failure sets `status=error`, success clears the error state |
| `GET /api/v1/datasource/:id/resources?parent_id=` | Admin | List the selectable resources of the external system (`parent_id` supports lazy expansion) |
| `POST /api/v1/datasource/:id/resource-ancestors` | Admin | Resolve the ancestor chain of selected resources (re-displays deep selections when editing) |
| `POST /api/v1/datasource/:id/sync` | Admin | Trigger a sync by hand (creates a SyncLog and enqueues an Asynq task); the optional body `{"allow_mass_deletion": true}` lifts the mass-deletion guard for that run only |
| `POST /api/v1/datasource/:id/pause` / `resume` | Admin | Pause or resume (removes or re-registers the cron entry) |
| `GET /api/v1/datasource/:id/logs`, `GET /api/v1/datasource/logs/:log_id` | Viewer | Sync history |

The creation wizard persists nothing before its final step: "Test connection" calls `validate-credentials`, the resource tree comes from `list-resources`, and `POST /api/v1/datasource` runs once on Finish. Editing an existing source uses the `:id` routes.

Every `:id` path first goes through `getOwnedDataSource` and `getOwnedKnowledgeBase` for **workspace isolation** (the data source's KB must belong to the current workspace and pass the API key's KB authorization check).

Lifecycle state transitions:

```mermaid
flowchart LR
    A["Create<br/>POST /datasource"] --> B["Authorize<br/>PUT /:id/credentials<br/>(AES-256-GCM encrypted at rest + online Validate)"]
    B --> C["Select resources<br/>GET /:id/resources<br/>(ResourceIDs written to Config)"]
    C --> D["active<br/>(cron schedule / manual sync)"]
    D -- "sync failed" --> E["error"]
    E -- "validate passed / sync succeeded" --> D
    D -- "POST /:id/pause" --> F["paused"]
    F -- "POST /:id/resume" --> D
    F -- "manual sync still allowed" --> D
    D -- "DELETE /:id" --> G["soft deleted<br/>(cron entry removed + unfinished SyncLogs canceled)"]
```

## Sync and storage reference

This section is for operators and developers who need to know how syncs are scheduled, executed and stored.

### Sync scheduling (internal/datasource/scheduler.go)

`Scheduler` is built on `robfig/cron` with a parser that accepts a seconds field (six-field expressions). It keeps one cron entry per active data source that has a `SyncSchedule`; at service start, `Start()` loads every active data source from the database and registers them in bulk.

Because robfig/cron fires at **absolute wall-clock times** (`0 0 * * * *` always fires on the hour), every instance of a multi-instance deployment fires at the same moment. Two layers deduplicate:

1. **Database layer, no overlap**: `syncLogRepo.HasRunningSync` skips the run when the previous sync is still running (so a sync that outlasts the cron interval does not stack).
2. **Redis layer, cross-instance dedup**: a deterministic `asynq.TaskID = "dssync:<dsID>:<yyyyMMddHHmm>"` (truncated to the minute). Every instance produces the same TaskID within a minute, Redis lets only one enqueue succeed, and the others get `asynq.ErrTaskIDConflict`; their SyncLog is marked `canceled` ("deduplicated: another instance enqueued first").

Enqueue options: queue `types.QueueSync`, `MaxRetry(5)`, `Timeout(2*time.Hour)`. The task type is `types.TypeDataSourceSync` (`"datasource:sync"`), consumed by `mux.HandleFunc(types.TypeDataSourceSync, params.DataSourceService.ProcessSync)` in `internal/router/task.go`.

### Sync execution and knowledge ingest (datasource_service.go)

`ProcessSync` is the Asynq task handler; the full flow is in the sequence diagram below. Key points:

- **Defensive cancellation**: when the data source or the knowledge base has been deleted, the SyncLog is set to `canceled` and nil is returned (no retry).
- **Two fetch paths**: a connector that implements `StreamingConnector` goes through `processSyncStreaming` (streaming); otherwise `ForceFull || SyncMode==full` calls `FetchAll` (or `FetchAllFromCursor` when the connector implements `FullSyncWithCursor`), and an incremental run calls `FetchIncremental` with the cursor from `ParseSyncCursor()` (batch).
- **Cursor policy on the streaming path** (`streamStartCursor`): a user-triggered full sync discards the cursor on its **first attempt** and fetches everything; an Asynq **retry** (attempt > 0) and every incremental sync resume from the last checkpoint. Connectors that implement `FullStreamingConnector` (Confluence and Seafile) switch to `FetchFullStream` on a full sync: every item is re-fetched while the old cursor is kept as the baseline for deletion reconciliation.
- **Ingest core, `applyFetchedItem` then `ingestItem`**:
  - `IsDeleted=true` with `sync_deletions=true` looks up the knowledge item by workspace, knowledge base, data source ID and external_id and deletes it for real; with deletion sync off, existing knowledge stays. Deletion also depends on whether the connector provides reliable deletion detection. A failed deletion is counted in `deletion_failed`. The failed deletion stays pending in the sync cursor and the sync log says `N deletion(s) failed, the next sync retries them`; the next sync of either mode retries it, on every connector;
  - `Content` bytes are wrapped as a `multipart.FileHeader` and go through `KnowledgeService.CreateKnowledgeFromFile` (the full document parsing pipeline); a `URL` alone goes through `CreateKnowledgeFromURL`, and AI KB downloads and parses it;
  - **an update is delete then create**: an existing knowledge item found by the metadata `external_id` is removed with `DeleteKnowledge` and rebuilt, counted as Updated;
  - a duplicate file (`DuplicateKnowledgeError`) counts as Skipped, not as a failure;
  - every entry carries the metadata `external_id`, `source_resource_id` and `datasource_id`, plus whatever the connector adds. When the source provides times, `source_created_at` / `source_updated_at` are stored in UTC RFC3339; they are the source document's times, separate from AI KB's created_at / updated_at.
- **Full-sync reconcile** (`reconcileDeletions`): on a full sync with `sync_deletions=true`, the service collects every emitted external_id. When the returned cursor has `CompleteListing` set, it lists the external IDs this data source owns in the knowledge base (`ListDataSourceExternalIDs`) and sends each one the run did not emit through the same `IsDeleted` path, so the counters match connector tombstones. A sub-item (`<parent>#kind#token`) whose parent was emitted is left to the subtree sweep. A connector sets `CompleteListing` only when every selected resource was listed completely and every live item was emitted: Feishu/Lark from a nil cursor with no partial listing, Confluence and Seafile on a full run that did not resume from a checkpoint (and, for Confluence, with no space listing cut short), DingTalk from `FetchAllFromCursor` with every scope scanned. These connectors declare the `full_sync_reconcile` capability, and the service only collects emitted IDs for them. Notion, Yuque, GitLab, RSS and IMA never set `CompleteListing`, so they never reconcile. A listing that emitted nothing while the data source owns items is treated as lost access and skips the reconcile.
- **Auto-tagging**: `resolveAutoTagIDs` finds or creates a tag named after the data source in the target KB and attaches it to every synced entry, so the source is visible in the KB; a tagging failure does not block the sync.
- **Result status**: every entry failed gives `failed` (`allFetchedItemsFailedError`); a partial RSS feed failure (`PartialFetchError`) or failed documents on the streaming path give `partial`; everything else is `success`. Up to 100 failure samples are kept as `SyncItemError`.
- When a fetch fails but the connector returned a new cursor (RSS, for example), the cursor is still persisted, so a transient outage does not force a full re-fetch.

```mermaid
sequenceDiagram
    autonumber
    participant U as "User / Cron Scheduler"
    participant H as "DataSourceHandler"
    participant S as "DataSourceService"
    participant Q as "Asynq (QueueSync)"
    participant C as "Connector (Feishu, for example)"
    participant EXT as "External system API"
    participant K as "KnowledgeService"
    participant DB as "PostgreSQL"

    U->>H: POST /datasource/:id/sync (or cron trigger)
    H->>S: ManualSync(dsID)
    S->>DB: Create SyncLog(status=running)
    S->>Q: Enqueue(datasource:sync, MaxRetry=5, Timeout=2h)
    Q-->>S: ProcessSync(payload)
    S->>DB: Load DataSource / SyncLog, check the KB exists
    S->>S: ParseConfig() decrypts credentials
    alt "StreamingConnector (Feishu/Lark, GitLab, Confluence, Seafile)"
        S->>C: FetchStream(config, cursor, handler)
        loop "Walk the wiki nodes"
            C->>EXT: ListWikiNodesRecursive / ExportAndDownload
            EXT-->>C: Document content (.docx/.xlsx/raw file)
            C->>S: handler.Emit(item)
            S->>K: CreateKnowledgeFromFile (delete then create = update)
            C->>S: handler.Checkpoint(cursor) every 50 nodes or 30s
            S->>DB: Persist LastSyncCursor + SyncLog progress
        end
        C-->>S: Final cursor
    else "Batch connectors (Notion/Yuque/DingTalk/IMA/RSS)"
        S->>C: FetchAll or FetchIncremental(cursor)
        C->>EXT: List + fetch changed content
        EXT-->>C: Documents / Markdown
        C-->>S: []FetchedItem + nextCursor
        loop "Each item"
            S->>K: applyFetchedItem then ingestItem
        end
    end
    S->>DB: Update SyncLog(success/partial/failed) + DataSource(LastSyncAt/Cursor/Result)
    S->>DB: Record audit log (recordKBActivity)
```

### Credential encryption and storage

Credentials are handled separately on write, on read and in API responses:

**1. Encrypt on write, `DataSourceConfig.ToJSON()`** (`internal/types/datasource.go`):

```go
// When SYSTEM_AES_KEY is configured, every string value in Credentials is
// AES-256-GCM encrypted before serialization. This is the only write path
// through which credentials reach the DB (the GORM JSON type is a byte
// passthrough), so encrypting here keeps DataSource.Config encrypted at rest.
if key := utils.GetAESKey(); key != nil && len(out.Credentials) > 0 {
    ...
    if enc, err := utils.EncryptAESGCM(s, key); err == nil { encCreds[k] = enc }
}
```

**2. Decrypt on read, `DataSource.ParseConfig()`**: three cases are handled transparently. An empty string is returned as is; legacy plaintext without the `enc:v1:` prefix is returned as is (no migration needed); ciphertext is decrypted with `SYSTEM_AES_KEY`. When decryption fails (key missing or rotated), the credential field is blanked, the UI shows "credentials not configured", and the user re-enters them without losing the rest of the data source configuration.

**3. A separate credential subresource, `internal/handler/datasource_credentials.go`**: credentials do not go through the ordinary `PUT /datasource/:id` but through a dedicated `/credentials` subresource that replaces the whole map atomically, so the connector always receives a complete credential set:

- `PUT /api/v1/datasource/:id/credentials` replaces the credential map as a whole, then immediately calls the connector's `Validate` for an online check (invalid credentials return an error right away)
- `DELETE /api/v1/datasource/:id/credentials/credentials` clears the whole map
- Responses **never return ciphertext or plaintext**, only `{"credentials": {"configured": true/false}}`; the list and detail endpoints serialize through `dto.NewDataSourceResponse`, which strips `Credentials` by construction

The ordinary update endpoint `UpdateDataSource` (`datasource_service.go`) **always keeps the stored credentials**: credentials in the request body are ignored with a warning in the log. `StripNonSecretCredentials` also removes non-secret fields that were put into credentials by mistake (currently only RSS `feed_urls`, which belongs in `Settings`).

### Security limits (internal/datasource/httpclient.go and errors.go)

`httpclient.go` provides the two SSRF protection entry points that every connector shares:

```go
// ValidateConnectorBaseURL applies the SSRF policy to a connector base_url (an empty value passes; the caller applies the default)
func ValidateConnectorBaseURL(rawURL string) error {
    ...
    if err := utils.ValidateURLForSSRF(url); err != nil { ... }
}

// NewConnectorHTTPClient returns an HTTP client with SSRF protection on redirects and at dial time
func NewConnectorHTTPClient(timeout time.Duration) *http.Client {
    cfg := utils.DefaultSSRFSafeHTTPClientConfig()
    cfg.Timeout = timeout
    return utils.NewSSRFSafeHTTPClient(cfg)
}
```

Underneath, `internal/utils/security.go` rejects private, loopback and link-local targets and re-checks **on every redirect and at the actual dial** (not only on the initial URL), so a malicious feed or a custom base_url cannot steer AI KB toward internal services. Each connector's `parseXXXConfig` calls `ValidateConnectorBaseURL` on its base_url.

`errors.go` defines the module-level sentinel errors (`ErrConnectorNotFound`, `ErrDataSourceInvalid`, `ErrInvalidCredentials`, `ErrSyncFailed` and others) and `PartialFetchError` (some resources succeeded and some failed; the caller should process the items it got, persist the cursor, and present `Details` to the user with the partial status).

### Core abstraction: the Connector interface

Every connector implements the `Connector` interface in `internal/datasource/connector.go`:

```go
type Connector interface {
    // Type returns the connector type identifier (for example "feishu", "notion")
    Type() string
    // Validate checks the configuration and credentials by calling the external API
    Validate(ctx context.Context, config *types.DataSourceConfig) error
    // ListResources lists the syncable resources (documents, spaces, folders and so on).
    // parentID supports lazy loading of hierarchical resources: "" = top level; non-empty = direct children of that resource
    ListResources(ctx context.Context, config *types.DataSourceConfig, parentID string) ([]types.Resource, error)
    // ResolveResourceAncestors resolves the ancestor chain of selected resources so a lazy picker can re-display deep selections
    ResolveResourceAncestors(ctx context.Context, config *types.DataSourceConfig, resourceIDs []string) ([]string, error)
    // FetchAll performs a full sync of the given resources
    FetchAll(ctx context.Context, config *types.DataSourceConfig, resourceIDs []string) ([]types.FetchedItem, error)
    // FetchIncremental syncs incrementally from a cursor and returns the changed items and the next cursor
    FetchIncremental(ctx context.Context, config *types.DataSourceConfig, cursor *types.SyncCursor) ([]types.FetchedItem, *types.SyncCursor, error)
}
```

#### Optional extension: StreamingConnector (streaming resumable sync)

`StreamingConnector` fetches, imports and checkpoints item by item. It suits large syncs and avoids holding every item's content in memory at once:

```go
type StreamHandler interface {
    // Emit imports one fetched item; an error aborts the whole stream
    Emit(ctx context.Context, item types.FetchedItem) error
    // Checkpoint persists a cursor snapshot synchronously (a complete, resumable snapshot, not a delta)
    Checkpoint(ctx context.Context, cursor *types.SyncCursor) error
}

type StreamingConnector interface {
    Connector
    FetchStream(ctx context.Context, config *types.DataSourceConfig,
        cursor *types.SyncCursor, h StreamHandler) (*types.SyncCursor, error)
}
```

After a task timeout, processing continues from the most recent checkpoint. The Asynq sync task timeout is 2 hours, and the streaming path advances item by item instead of buffering every file body. The Feishu/Lark (wiki and drive), GitLab, Confluence and Seafile connectors implement `StreamingConnector`; Confluence and Seafile also implement `FullStreamingConnector`, so a full sync can still reconcile deletions. DingTalk currently uses the batch `FetchAll` / `FetchIncremental` / `FetchAllFromCursor` path; very large knowledge bases should use incremental mode to avoid loading all the Markdown at once.

#### ConnectorRegistry: registration and lookup

`ConnectorRegistry` is a plain `map[string]Connector`. Registration happens in `initConnectorRegistry()` in `internal/container/container.go`:

```go
registry.Register(wiki.NewConnector(core.RegionFeishu))             // feishu
registry.Register(wiki.NewConnector(core.RegionLark))               // lark (international edition, same implementation, different Region)
registry.Register(drive.NewDriveConnector(core.RegionFeishuDrive))  // feishu_drive
registry.Register(drive.NewDriveConnector(core.RegionLarkDrive))    // lark_drive
registry.Register(notionConnector.NewConnector())                   // notion
registry.Register(confluenceConnector.NewConnector())               // confluence
registry.Register(yuqueConnector.NewConnector())                    // yuque
registry.Register(dingtalkConnector.NewConnector())                 // dingtalk
registry.Register(imaConnector.NewConnector())                      // ima
registry.Register(rssConnector.NewConnector())                      // rss
registry.Register(gitlabConnector.NewConnector())                   // gitlab
registry.Register(seafileConnector.NewConnector())                  // seafile
```

> Note: `ConnectorMetadataRegistry` in `connector.go` still lists connectors that are not implemented yet (GitHub, Google Drive, OneDrive, Web Crawler, Slack, IMAP). The types actually registered and usable are `feishu`, `lark`, `feishu_drive`, `lark_drive`, `notion`, `confluence`, `yuque`, `dingtalk`, `ima`, `rss`, `gitlab` and `seafile`. Creating a data source with an unregistered type is rejected by `connectorRegistry.Get()` with `ErrConnectorNotFound`.

### Data model (internal/types/datasource.go)

| Struct | Description |
| --- | --- |
| `DataSource` | The data source configuration entity (table `data_sources`). Key fields: `Type` (connector type), `Config` (JSONB, with encrypted credentials), `SyncSchedule` (cron expression), `SyncMode` (`incremental` / `full`), `Status` (`active` / `paused` / `error` / `deleted`), `ConflictStrategy`, `SyncDeletions`, `LastSyncCursor` (incremental cursor, JSONB), `LastSyncAt`, `LastSyncResult`, `SyncLogRetentionDays` |
| `SyncLog` | One sync run (table `sync_logs`). Status: `running` / `success` / `partial` / `failed` / `canceled`; counters `ItemsTotal/Created/Updated/Deleted/Skipped/Failed`; `Result` holds the `SyncResult` JSON |
| `DataSourceConfig` | The decrypted configuration: `Type` + `Credentials map[string]interface{}` + `ResourceIDs []string` (selected resources) + `Settings map[string]interface{}` (non-secret settings) |
| `Resource` | A selectable resource of the external system: `ExternalID`, `Name`, `Type`, `URL`, `ParentID`, `HasChildren`, `ModifiedAt`, `Metadata` |
| `FetchedItem` | One fetched document: `ExternalID`, `Title`, `Content []byte`, `ContentType`, `FileName`, `URL`, `UpdatedAt`, `Metadata`, `IsDeleted`, `SourceResourceID` |
| `SyncCursor` | The incremental cursor: `LastSyncTime` + `ConnectorCursor map[string]interface{}` (connector-defined structure) |
| `SyncResult` | The sync summary + `Errors []SyncItemError` (failure samples, at most 100, see `maxSyncResultErrors`) |
| `SyncItemError` | A user-facing failure sample: a stable i18n `Code` + interpolation `Params` + fallback `Message`; the raw API status and response body stay in the server log |
| `DataSourceSyncPayload` | The Asynq task payload: `DataSourceID`, `TenantID`, `SyncLogID`, `ForceFull`, `Trigger` (`manual` / `schedule`) |

## References

These documents are maintained next to the code.

- Connector development guide: `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md`
- Module description: `internal/datasource/README.md`

## Implementation references

These are the source locations to start from when reading or changing the implementation.

- Connector framework and implementations: `internal/datasource/` (`connector.go`, `scheduler.go`, `httpclient.go`, `errors.go`, the implementations under `connector/`)
- HTTP layer: `internal/handler/datasource.go`, `internal/handler/datasource_credentials.go`
- Service layer: `internal/application/service/datasource_service.go`
- Data model: `internal/types/datasource.go`
