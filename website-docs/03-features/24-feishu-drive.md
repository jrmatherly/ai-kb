# Feishu Drive

The `feishu_drive` and `lark_drive` connectors sync the documents and files in a chosen Drive folder into a knowledge base. Wiki spaces use the `feishu` / `lark` connectors instead. The sync flow shared by every connector (schedule, scope, logs, deletion sync) is in [Data source import](10-datasource.md); this page covers what is specific to Drive.

## App identity and folder permissions

Before creating the data source, give a self-built app the right to read the folder.

Use the App ID and App Secret of a self-built enterprise app in the matching region; Feishu and Lark credentials cannot be mixed. Grant read scopes for the APIs the connector actually calls: folder listing and file download, cloud document export, and docx content reading in `blocks` mode. The UI's prerequisite steps name `drive:drive:readonly`, `drive:export:readonly` and `docx:document:readonly`. Scope names, substitute scopes and approval requirements follow the Feishu Open Platform descriptions of the [folder list API](https://open.feishu.cn/document/server-docs/docs/drive-v1/folder/list), the [export API](https://open.feishu.cn/document/server-docs/docs/drive-v1/export_task/create) and the other endpoints used.

After configuring scopes, publish an app version and share the target folder with a scope the app can access. If you share through the group the app belongs to, check the relationship between the app, the folder owner and the group. API scopes and file access are two separate checks: obtaining a tenant access token does not mean the app can read any folder.

The connection test in AI KB verifies the app credentials only; folder access is verified when the resource tree loads. If the connection succeeds but loading the folder returns 403, check the sharing scope and whether the app version with the new scopes was published.

## Configure in a knowledge base

Follow these steps to create the data source and run the first sync.

1. Knowledge base settings → Data sources → New, then choose "Feishu Drive" or "Lark Drive".
2. Enter the App ID and App Secret and run the connection test.
3. Enter the `folder_token` of a specific folder, or its full `/drive/folder/<token>` link, load the resource tree and select the sync scope. The field cannot be left empty to mean the root of the Drive space.
4. Choose full or incremental sync, the schedule, the conflict policy and deletion sync. After saving, sync a few files by hand first and check the logs and parsing results.

An incremental sync compares the stored cursor with the current file tree. A file that disappeared is reported as deleted only when every selected resource was listed completely; when a sub-folder listing fails, deletion detection is skipped for that resource. With deletion sync on, such a report deletes **the knowledge items that belong to this data source**; with it off, the items stay. A first sync, or a user-triggered full sync, carries no previous cursor, so neither can detect past deletions. A failed deletion is not retried either: the sync log counts it, but a later full sync drops the cursor and an incremental sync no longer holds the vanished file, so remove the leftover item by hand after reading the log.

When a document itself is updated, attachment sub-items from `blocks` mode that no longer appear in the document are removed as part of the content update. That cleanup is independent of the deletion sync switch, which governs source documents.

## Files and parse modes

This section explains which Drive file types are imported and how a docx document is read.

Ordinary files are downloaded and parsed by file type. Spreadsheets and Bitables are exported as xlsx, folders are walked recursively, and shortcuts resolve to their target file. Unsupported cloud document types (mind notes, slides, boards) are skipped; the sync log records the skip reason and a per-type tally.

New-style docx documents have two paths, selected by `FEISHU_DOCX_PARSE_MODE` on the **app service**. The variable affects the Wiki and the Drive connectors alike:

| Mode | Path | Use and limits |
| --- | --- | --- |
| Empty or `export` (default) | Asynchronous docx export, then document parsing | Images are parsed with the parent document; export and parsing take longer; embedded file-block attachments are not kept by the export |
| `blocks` | Blocks API converted to Markdown; falls back to export when the API fails or the body renders empty | Parseable attachments become separate items; with multimodal sync on, images become separate items too, and retrieval does not automatically relate them back to the body |

To use `blocks` mode, set it in `.env` and recreate the app service:

```dotenv
FEISHU_DOCX_PARSE_MODE=blocks
```

```bash
docker compose up -d app
```

Changing the mode affects later fetches only; content already imported is not converted to the other format. Whether image content is retrievable also depends on object storage, OCR or multimodal processing and the index state, see [Document parsing](03-document-parsing.md).

## Troubleshooting

Use this table when a Drive sync fails or imports less than expected.

| Symptom | What to do |
| --- | --- |
| Invalid token or the credential test fails | Check the Feishu/Lark region, the App ID and App Secret, and any private proxy address |
| Loading the folder fails | Use the link of a specific folder; check the API scopes and the folder sharing |
| Cloud documents fail while ordinary files work | Check the export scope; `blocks` mode also needs the docx read scope |
| Fewer items synced than files in the folder | Check for unsupported types, sub-folder authorization, and the failed and skipped counts |
| Images are not recalled with the body text | Confirm the parse mode and the multimodal state; `blocks` mode's separate images and `export` mode's inline images have different semantics |
| A changed document is briefly not retrievable | An update may delete the old knowledge item and import it again; wait for the new version to finish parsing and indexing |

Implementation reference: `internal/datasource/connector/feishu/drive/`, `internal/datasource/connector/feishu/core/shared.go` and `internal/application/service/datasource_service.go`.
