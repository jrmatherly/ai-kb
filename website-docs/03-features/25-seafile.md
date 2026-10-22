# Seafile integration

The `seafile` connector syncs the directories and files you select in one Seafile library into a knowledge base. It supports scheduled, incremental and resumable syncs and reconciles deletions at the source. The compatibility baseline is Seafile Server community edition 10.0.1 (the api2 interface). The sync flow shared by all connectors is described in [Data source import](10-datasource.md).

## Account and API token

Decide which Seafile account the connector will act as. Use an account that can sign in to Seafile and has at least read access to the target library; the sync scope is bounded by that account's permissions. The credential is the account's user API token. The connector stores and sends only the token; it does not handle the account password, SSO login or library-specific tokens.

In Seafile 10.0.1 the "Web API Auth Token" is hidden on the personal settings page by default. An administrator enables it with `ENABLE_GET_AUTH_TOKEN_BY_SESSION = True` in `seahub_settings.py` and restarts Seahub. The token can also be obtained with the account password through `POST /api2/auth-token/`; when two-factor authentication is on, that request first fails with HTTP 400 and the response header `X-Seafile-OTP: required`; send it again with the one-time code in an `X-Seafile-OTP` request header.

The connection test in AI KB calls `/api2/auth/ping/` to validate the token. Saving a data source with a selection also confirms that the selected library exists and is not encrypted. Directory access is checked when the resource tree loads and again during each sync.

## Configure it in a knowledge base

Follow these steps to connect a library and run the first sync.

1. Open the knowledge base settings, go to Data sources, create a new source and choose "Seafile".
2. Enter the Seafile URL and the API token, then run the connection test. The URL may include a deployment path prefix, such as `https://example.com/seafile`.
3. In the resource tree, select the library root, any directories or single files. The top level lists only the unencrypted libraries the account can reach; directories expand level by level. One data source covers one library; selecting a second library is rejected.
4. Choose full or incremental mode, the sync schedule, the conflict strategy and "Sync deletions". After saving, sync a small set of files by hand first and check the sync log and the parsing results.

Selecting a directory syncs it recursively, so files and subdirectories added later are picked up automatically. Selecting a single file syncs that file only. When a selected item sits under a selected parent directory, only the parent is kept. The picker lists only the extensions AI KB can import; unsupported files such as `.zip` do not appear and are never synced.

## Network and SSRF

Know which hosts the connector reaches so you can allow them before the first sync.

| Request | Target | Carries the token |
| --- | --- | --- |
| API (`/api2/…`) | The configured Seafile URL | Yes, `Authorization: Token` |
| File download | The fileserver link Seafile returns, usually `/seafhttp/…` on the same host, sometimes a separate host | No |

Both addresses pass SSRF validation. When Seafile runs on a private network or uses a separate fileserver host, add that host to `SSRF_WHITELIST` in `.env`. Otherwise saving the credentials fails with "base_url SSRF validation failed", or the sync log shows `seafile_ssrf_blocked`.

## Folders, formats and size

This section explains where synced files land and which files are accepted.

Synced files land in the knowledge base folder `<library name>/<path inside the library>`, and the source label reads "Seafile". The library name is written into the cursor on the first sync and frozen: renaming the library in Seafile later does not change the knowledge base folder and does not trigger a re-download. Every folder and file name segment is sanitized separately.

The accepted formats are the same as for knowledge base uploads (PDF, Office, Markdown, HTML, common images, audio and others); `internal/utils/import_filetypes.go` is the authoritative list. Images need a VLM configured on the knowledge base and audio needs ASR. Without them, the picker hides those files and a sync never downloads them; once the model is configured, the next sync of the data source starts from the beginning and fetches them (see the import capability paragraph in [Data source import](10-datasource.md)).

The per-file limit follows `MAX_FILE_SIZE_MB` (default 50 MiB). A file that exceeds it in the directory listing is recorded as `seafile_file_too_large` and never downloaded; a download whose actual bytes exceed the limit is rejected the same way, never truncated. When you raise the value, raise the DocReader limit `DOCREADER_GRPC_MAX_FILE_SIZE_MB` with it and restart app, docreader and frontend; otherwise the file downloads but gRPC rejects it at the parsing stage. Under Docker Compose `DOCREADER_GRPC_MAX_FILE_SIZE_MB` defaults to `MAX_FILE_SIZE_MB`, so one setting covers both.

## Sync behavior

This section describes what a sync does with each change at the source.

Each file is stored in the cursor as a fingerprint of "object ID + modification time + size". When all three match, the download is skipped. When the fingerprint changes, the file is downloaded again and replaces the old item (delete first, then create, so the document is briefly unavailable while it is parsed). A run with no changes only scans directories, one request per directory.

| Change at the source | Behavior |
| --- | --- |
| New file or subdirectory | Included in the next sync |
| Modified content | Downloaded again and updated |
| Deleted file | Deletes the knowledge item when "Sync deletions" is on; otherwise only removed from the cursor |
| Renamed, moved or moved out of scope | The new path is imported as a new file and the old path is treated as deleted; the knowledge item ID is not preserved |
| A selected directory or file is deleted or renamed | The run stops with an error and produces no deletions; select the scope again |
| Library renamed | No change; the folder keeps the name from the first sync |

Failure handling:

| Situation | Behavior |
| --- | --- |
| A single file returns 403 / 404 / 5xx, a network error, exceeds the limit, is empty or changes during download | Recorded as a failed item; the other files continue. The path stays in the cursor and is retried in the next sync |
| A directory listing returns 403 / 404 / 5xx or a malformed response | Saves the progress so far and stops; the run produces no deletions |
| 429 / 502 / 503 / 504 or a transient network error | Up to 3 attempts per request with 1s and 2s backoff. A `Retry-After` of up to 60s is honored (the longer of the backoff and the header); a longer one stops the run |
| Token invalid (401 / 403) | Credential error; the data source enters the error state |

A checkpoint is saved every 50 cursor changes, before deletions are emitted and at the end of the run. After a task timeout or interruption, the next run continues from the last checkpoint and files already saved are not downloaded again. The first sync of a large library with tens of thousands of files may take several task runs to complete. A sync in full mode (the data source's sync mode set to Full) downloads every file again while keeping the old cursor as the deletion baseline.

"Connector fetch retry" and "ingest recovery" are two different things. The first means a file whose download failed is downloaded again in the next sync. The second means a file was handed to the knowledge base but parsing or indexing failed; the knowledge base's own retry handles that, and the sync log shows it as an ingest failure.

Sync log error codes:

| Code | Meaning |
| --- | --- |
| `seafile_permission_denied` | The token's account cannot access the file or directory |
| `seafile_not_found` | The file disappeared after the scan; retried in the next sync, and if it is gone from the library it is reconciled according to the "Sync deletions" switch |
| `seafile_file_too_large` | Exceeds `MAX_FILE_SIZE_MB` |
| `seafile_empty_file` | The file is empty and was skipped |
| `seafile_source_changed` | The file changed while it was being downloaded; retried in the next sync |
| `seafile_invalid_response` | Seafile returned a response that could not be parsed, usually a version or reverse proxy problem |
| `seafile_ssrf_blocked` | The download URL was blocked by the SSRF policy |
| `seafile_fetch_failed` | Any other fetch failure; retried in the next sync |

The frontend localizes each code as `datasource.syncError.<code>`; `SyncReasonTexts` in `connector/seafile/types.go` holds the English fallbacks, and a Go test checks the two lists agree.

## Notes

Keep these limits in mind when you change a data source or clean up after one.

- A synced data source cannot be re-pointed at another library. After changing the selection, the next sync fails with "this data source already synced Seafile library …" (naming the original library) and the cursor stays unchanged; create a new data source instead. To switch libraries before the first sync, clear the selection before saving.
- A deletion that failed stays pending in the cursor and the next sync retries it. An incremental run does not catch up on other missed deletions: a run whose directory scan failed produces no deletions, and a file deleted while "Sync deletions" was off is no longer in the cursor. There is no separate "force full sync" action; set the data source's sync mode to Full, and a run with "Sync deletions" on catches up: when it enumerated every root and started with no file already re-downloaded, it deletes each knowledge item of this data source that it did not list, including files of directories removed from the selection. A full sync that resumed from a checkpoint skips this step.
- Every run's deletions, incremental ones included (a narrowed selection deletes the files outside it on the next run), go through the mass-deletion guard: a full sync that listed no file at all, or a run that would delete 500 or more items, or 10 or more items that are at least 10% of the data source's items, deletes nothing until you run "Sync and apply deletions" (see [Data sources](./10-datasource.md#check-changes-and-failures)).
- Deleting or recreating a data source does not remove the knowledge items it imported; clean up leftover content in the knowledge base yourself.
- Encrypted libraries do not appear in the picker and cannot be synced. SeaDoc online documents, custom properties and two-way sync are out of scope.

## FAQ

Match the symptom you see to its cause.

| Symptom | Fix |
| --- | --- |
| Saving credentials fails SSRF validation | The Seafile URL is on a private network or a blocked port; add it to `SSRF_WHITELIST` |
| "Seafile library … not found" | The token's account cannot see the library, or the library was deleted |
| "encrypted Seafile libraries are not supported" | The selected library is encrypted |
| Many `seafile_ssrf_blocked` entries in the sync log | The fileserver host differs from the API host and is not allowlisted |
| `seafile_file_too_large` in the sync log | Raise `MAX_FILE_SIZE_MB` and `DOCREADER_GRPC_MAX_FILE_SIZE_MB`, then restart |
| The file downloaded but the knowledge item failed to parse | An ingest problem; check the VLM / ASR configuration and the DocReader logs |
| "this data source already synced Seafile library …" | The library was changed; restore the original library or create a new data source |

Implementation references: `internal/datasource/connector/seafile/` and `internal/application/service/datasource_service.go`.
