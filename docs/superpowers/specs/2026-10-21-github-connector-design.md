# GitHub data source connector: design

Date: 2026-10-21, revised after review the same day. Status: agreed design, not implemented. Companion plan: `../plans/2026-10-21-github-connector.md`.

## Goal

A `github` data source connector that syncs document files from the directories of selected GitHub repositories into a knowledge base, with the same user-facing behaviour as the GitLab connector (`internal/datasource/connector/gitlab/`): streaming full sync, incremental sync by commit, deletion tombstones, checkpoints, resume after a timeout.

## What exists

- `internal/types/datasource.go:31` declares `ConnectorTypeGitHub = "github"` and `internal/datasource/connector.go:295` has a metadata entry for it (`oauth2`, "repositories, wikis, and issues"). Nothing registers a connector, so creating a `github` source fails with `ErrConnectorNotFound`.
- Upstream WeKnora (`weknora/main` at `e28c68cf`) has no GitHub connector, so there is nothing to adopt.
- The GitLab connector is the model. Its data shape is `settings.projects[]` of `{project_id, ref, paths}` plus `credentials.base_url` and `credentials.access_token`; it implements `datasource.StreamingConnector`.

## GitHub API facts the design rests on

Checked against docs.github.com (API version 2022-11-28) and the live API on 2026-10-21.

| Need | Endpoint | Fact |
| --- | --- | --- |
| Credential check | `GET /user` | 401 on a bad token |
| Repository listing | `GET /user/repos?per_page=100&sort=full_name&affiliation=owner,collaborator,organization_member` | paged through the `Link` header, `rel="next"`; `per_page` max 100 |
| Repository | `GET /repos/{owner}/{repo}` | `id`, `name`, `full_name`, `html_url`, `default_branch`; the lookup ignores case and `full_name` carries the canonical spelling (`OCTOCAT/hello-world` returns `octocat/Hello-World`) |
| Head commit | `GET /repos/{owner}/{repo}/commits/{ref}` | `sha` |
| Whole tree | `GET /repos/{owner}/{repo}/git/trees/{commit}?recursive=1` | one call; accepts a commit SHA, a branch or a tag; entries carry `path`, `type` (`blob`, `tree`, `commit` for a submodule), `mode` (`100644`, `100755`, `120000` for a symlink, `160000`, `040000`), `sha`, `size`; `truncated: true` past 100,000 entries or 7 MB of tree data; the JSON runs about 238 bytes per entry (measured on `golang/go`), so an untruncated tree can be 24 MB of JSON |
| One subtree | `GET /repos/{owner}/{repo}/git/trees/{sha}` | the direct entries of that tree, with each subtree's `sha`; `{commit}:{path}` also resolves |
| File body | `GET /repos/{owner}/{repo}/git/blobs/{sha}` with `Accept: application/vnd.github.raw+json` | raw bytes, blobs up to 100 MB |
| Directory contents | `GET /repos/{owner}/{repo}/contents/{dir}` | capped at 1,000 entries with no paging. Not used. |
| Commit compare | `GET /repos/{owner}/{repo}/compare/{base}...{head}` | merge-base comparison only (the two-dot form returns 404), capped at 300 files. Not used. |
| Auth | `Authorization: Bearer <token>` | plus `X-GitHub-Api-Version: 2022-11-28` |
| Hosts | `https://api.github.com` for github.com; `https://api.<host>` for a `*.ghe.com` data-residency enterprise; `https://<host>/api/v3` for GitHub Enterprise Server | the GitHub CLI applies the same three rules (`cli/cli` `internal/ghinstance/host.go` `RESTPrefix`, `cli/go-gh` `pkg/auth` `IsTenancy`) |
| Rate limit | 5,000 requests per hour per token | primary limit: 403 or 429 with `X-RateLimit-Remaining: 0`; secondary limit: 403 or 429 with `Retry-After` while `X-RateLimit-Remaining` is not 0 |

## Decisions

1. **Snapshot diff from git trees, not compare.** GitLab's incremental sync uses `compare?straight=true` so a reset or force-push still emits reverted files (pinned by `compare_snapshot_test.go`). GitHub has no snapshot compare. The connector takes a snapshot of the repository at the previous head commit and at the new head commit (recursive tree, one call each, always by commit SHA so the content matches the recorded head) and diffs path to blob SHA locally: a path only in the old snapshot is a deletion, a path whose SHA changed or is new is re-fetched. Renames fall out as a deletion plus an addition. A snapshot keeps only regular blobs (`mode` `100644` or `100755`) that are in scope and have a supported extension; symlinks and submodules are skipped. When a tree comes back `truncated`, the snapshot is rebuilt by walking subtrees one at a time through the non-recursive tree endpoint, following each subtree's `sha`. When the previous head's tree is gone (404 or 422, a force-push whose old commit was garbage-collected), the sync logs it and emits the whole new snapshot with no deletions. Full sync is the same snapshot with no previous head.
2. **Repository key is the lowercased `owner/repo`.** GitHub lookups ignore case, so the key is normalised to lowercase and used for the cursor, the external ID and `SourceResourceID`; `full_name` from the API is used for the title and the URL only. `settings.repositories[]` holds `{repository, ref, paths}`. The numeric `id` is kept in item metadata only.
3. **File cap is 100 MB**, the API's own limit for blobs, and a larger blob is reported as a per-item failure (`github_file_too_large`) instead of failing the sync. The size is known from the tree, so the blob is never requested. The connector therefore has one reason code, declared in `SyncReasonTexts` and checked by `internal/datasource/syncreason/parity_test.go`.
4. **Shared repository-path helpers.** `normalizePath`, `collapsePaths`, `inScope`, the supported-extension set and `knowledgeRelativePath` are identical needs for both git connectors. They move to a new package `internal/datasource/connector/repofile` and GitLab imports it; the config parser stays per connector because the JSON keys differ. `readCapped` stays per connector, as it already is in five others.
5. **Base URL handling** is a pure function `normalizeBaseURL(value string) string`, tested without network: empty, `github.com`, `www.github.com` and `api.github.com` give `https://api.github.com`; a host ending in `.ghe.com` gives `https://api.<host>` (an `api.` prefix already present is kept once); any other host is a GitHub Enterprise Server, the scheme defaults to `https` and `/api/v3` is appended unless already present. A non-empty input passes `datasource.ValidateConnectorBaseURL` (SSRF policy, which resolves the host) before normalisation, so tests that build a client whitelist their host.
6. **Rate limiting is not a permission error.** A 429, or a 403 that carries `Retry-After` or `X-RateLimit-Remaining: 0`, maps to `errRateLimited` carrying the reset time, never to `datasource.ErrPermissionDenied`. A plain 403 still maps to `ErrPermissionDenied`, a 401 to `ErrInvalidCredentials`, as the GitLab client does.
7. **The cursor records the selection it was built for and progress inside a repository.** Per repository: `head` (commit SHA), `ref` (the resolved ref name) and `paths` (the normalised roots), plus `pending: {head, done}` while a repository is being emitted. The previous snapshot is built with the recorded `ref` and `paths`, so a changed ref tombstones every old-ref item and a changed directory list emits exactly the added and removed files. Emission order is deterministic: deletions sorted by path, then changed files sorted by path. The handler is checkpointed every 50 emitted items with `pending.done` set to the count, so a resumed sync whose head is unchanged recomputes the same diff and skips the first `done` entries; a sync that is interrupted by the hourly rate limit resumes where it stopped instead of restarting the repository.
8. **A full sync is a complete listing.** The metadata entry declares `full_sync_reconcile`, and a run that started from a nil cursor (the service passes nil on the first attempt of a forced full sync) returns its cursor with `CompleteListing` set, so the service deletes the data source's knowledge items the run did not emit, including items of a ref or directory that is no longer selected.
9. **Frontend reuses the GitLab form.** The step 2 "project list" form, its state and its gate become git-connector generic. The settings key and item key differ per type (`projects`/`project_id` for GitLab, `repositories`/`repository` for GitHub); the labels come from a per-type i18n group with identical member names; choosing a type resets the list and removes the other type's settings key.
10. **Deployment gate is unchanged.** `AIKB_DATASOURCE_TYPES` must list `github` (or be `all`) for the type to appear; the default stays Notion and Confluence.

## Data shapes

Credentials: `{"base_url": "", "access_token": "<token>"}`; `base_url` optional.

Settings: `{"repositories": [{"repository": "owner/repo", "ref": "main", "paths": ["docs"]}]}`. `repository` is required, unique after normalisation, `owner/repo` with exactly one slash and no empty part; it is trimmed of spaces and slashes, stripped of a trailing `.git` and lowercased; a URL is rejected. `ref` empty means the default branch. `paths` are forward-slash relative directories, normalised and collapsed as for GitLab; empty means the whole repository.

Cursor: `{"repositories": {"owner/repo": {"head": "<sha>", "ref": "main", "paths": ["docs"], "pending": {"head": "<sha>", "done": 120}}}, "raw": "<json>"}`; `pending` is absent once a repository is complete.

Item: `ExternalID` `github:<api base url>:<owner/repo>:<ref>:<path>`; `Title` `<full_name>/<path>`; `FileName` `<repo name>-<ref>/<path>`; `ContentType` `text/plain`; `SourceResourceID` `<owner/repo>`; `Metadata` `channel=github`, `source_type=github`, `github_repository` (`full_name`), `github_repository_id`, `github_ref`, `github_path`, `github_url` (`<html_url>/blob/<ref>/<path>`). `UpdatedAt` stays unset. A zero-byte blob is emitted with `# <title>` as its body, as GitLab does. A deletion is `IsDeleted: true` with the same `ExternalID`. A per-item failure has the same `ExternalID` and `Title`, no content, and `Metadata` `channel`, `source_type`, `github_path`, `error`, `error_reason_code`, `error_reason` (the shape of `seafile/sync.go:441`).

Resources: root lists repositories as `{ExternalID: <owner/repo lowercased>, Name: full_name, Type: "repository", URL: html_url, HasChildren: true}`; a parent `owner/repo[:dir]` lists its subdirectories, read from the non-recursive tree of the default branch, as `{ExternalID: "owner/repo:<path>", Type: "directory", ParentID: parent, HasChildren: true}`.

## Touchpoints

Go: new package `connector/github` (client, connector, snapshot, types, tests), new package `connector/repofile`, `connector/gitlab` (import `repofile`, including `client_test.go:316`), `internal/datasource/connector.go` (metadata), `internal/container/container.go` (registration), `internal/application/service/knowledge_create.go` (`usesSourceIdentityDuplicateCheck`), `internal/datasource/syncreason/parity_test.go`, `internal/application/repository/knowledge_duplicate_test.go`, `internal/datasource/README.md`, `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md`.

Frontend: `views/knowledge/settings/DataSourceEditorDialog.vue` and its test, `views/knowledge/settings/datasourceIcons.ts`, new `assets/img/datasource-github.svg`, the channel maps in `views/knowledge/components/DocumentListView.vue`, `DocumentCardView.vue`, `views/knowledge/KnowledgeBase.vue`, `views/chat/components/tool-results/DocumentInfo.vue`, `components/doc-content.vue`, and `i18n/locales/en-US.ts`.

Docs: `website-docs/03-features/10-datasource.md`, `01-getting-started/01-introduction.md`, `01-getting-started/04-configuration.md`, `04-api/02-api-infra.md`, `05-clients/01-frontend.md`, `06-development/02-database-schema.md`, `06-development/03-extension-points.md`, `README.md`, `CHANGELOG.md`.

## Known limitations

- A file tracked by Git LFS is served by the blob endpoint as its pointer text, not its content (not verified against the API; the GitLab connector has the same gap). The docs say so.
- Items of a ref or directory that is no longer selected are removed only by a forced full sync (decision 8) or when the selection changes while the source is still synced incrementally (decision 7); a source whose selection changed while it was paused is reconciled on its next forced full sync.

## Out of scope

Syncing issues, pull requests or wikis; OAuth app sign-in; GitHub App installation tokens; Git LFS content; widening the supported-extension list to source code; changing the home-cluster `AIKB_DATASOURCE_TYPES` value (another repository).
