# GitHub Data Source Connector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `github` data source connector that syncs document files from selected directories of GitHub repositories, with streaming full and incremental sync, deletions and resumable checkpoints, on a par with the GitLab connector.

**Architecture:** A new Go package `internal/datasource/connector/github` mirrors the GitLab connector's shape (package-private HTTP client with size caps, config parser, `StreamingConnector`), but incremental sync diffs two repository snapshots built from the git tree endpoints instead of a commit compare, and the cursor records the selection and in-repository progress. Repository-path helpers shared with GitLab move to a new `connector/repofile` package. The frontend reuses the GitLab "project list" form, made generic over the two git connectors.

**Tech Stack:** Go 1.26 (`GOTOOLCHAIN=go1.26.10` to match CI), `net/http/httptest` and `testify` for tests; Vue 3 + TypeScript, vue-i18n, node test runner via tsx (`npm --prefix frontend test`), vue-tsc (`npm --prefix frontend run type-check`).

**Spec:** `docs/superpowers/specs/completed/2026-10-21-github-connector-design.md`

## Global Constraints

- English only, no em dashes anywhere (`scripts/check-english-only.sh`, `scripts/check-emdash.sh`; the pre-commit hook enforces both).
- Commit subjects: `type(scope): imperative subject`, for example `feat(datasource/github): ...`. Every commit message ends with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.
- Before each commit: run the built-in `/simplify` and `/code-review` on the diff. golangci-lint runs over the whole root module on commit, so keep unrelated Go files clean.
- No UI text outside `frontend/src/i18n/locales/en-US.ts`; product name in copy is "AI KB".
- A doc claim cites code that matches it; after a code change, `git grep -n '<old value>' -- '*.md'` for the old value and run the `docs-drift-reviewer` agent on the diff.
- GitHub API version header `X-GitHub-Api-Version: 2022-11-28`; auth header `Authorization: Bearer <token>`; blob cap 100 MB; metadata JSON cap 16 MiB; tree JSON cap 64 MiB.
- Tests that build a client against an `httptest` server mount the fake under `/api/v3` (the server's `127.0.0.1` host is an Enterprise Server host under spec decision 5) and whitelist it with the `allowLocalGitHubServer(t)` helper; the client's `baseURL` and every external ID then end in `/api/v3`.
- Work on a branch `feat/github-datasource` cut from `main`; never push to `main` directly.

## Review Focus

1. A force-push or branch reset between syncs: files that reverted must be re-emitted and files that vanished must be tombstoned, and a previous head GitHub has garbage-collected must not wedge the source (Task 4, `TestFetchStreamForcePushEmitsSnapshotChanges`, `TestFetchStreamMissingPreviousHeadFallsBackToFullEmit`).
2. A repository whose recursive tree is truncated: the sync must still be complete, built from subtree listings (Task 4, `TestSnapshotFallsBackToSubtreeWalkWhenTruncated`).
3. A token that hits the hourly or the secondary rate limit mid-repository: the sync must fail with a rate-limit error, not flag the data source as lacking permission, and the retry must resume after the last checkpoint (Task 2, `TestRateLimitIsNotPermissionDenied`; Task 4, `TestFetchStreamResumesInsideRepositoryFromPendingCursor`).
4. A repository typed as `OWNER/Repo/`, `owner/repo.git` or a full `https://github.com/owner/repo` URL, and a selection whose ref or directories changed since the last sync (Task 3, `TestParseConfigNormalisesRepository`; Task 4, `TestFetchStreamSelectionChangeTombstonesOldRefAndEmitsNewRoots`).
5. Editing a saved GitHub source and a saved GitLab source in the same session, and switching the type while creating: each must load and write its own settings key only (Task 6, `saved GitHub repositories hydrate the shared git form`, `switching the git connector type resets the repository rows`).

---

### Task 1: Shared repository-path helpers (`repofile`)

**Files:**

- Create: `internal/datasource/connector/repofile/repofile.go`
- Create: `internal/datasource/connector/repofile/repofile_test.go`
- Modify: `internal/datasource/connector/gitlab/types.go` (delete `normalizePath`, `collapsePaths`), `internal/datasource/connector/gitlab/connector.go:366-404` (delete `isSupportedFile`, `gitLabSupportedFileExtensions`, `knowledgeRelativePath`, `inScope`)
- Modify: `internal/datasource/connector/gitlab/types_test.go` (move the `normalizePath` and `knowledgeRelativePath` tests), `internal/datasource/connector/gitlab/client_test.go:316` (calls `isSupportedFile`; point it at `repofile.IsSupported`)

**Interfaces:**

- Produces: package `repofile` with `NormalizePath(value string) (string, error)`, `CollapsePaths(paths []string) []string`, `InScope(file string, roots []string) bool`, `IsSupported(file string) bool`, `KnowledgeRelativePath(root, ref, file string) string`. Bodies are the GitLab functions of the same names, unchanged; `NormalizePath` wraps `datasource.ErrInvalidConfig`.

- [ ] **Step 1: Write the failing tests**

`repofile_test.go`: move `TestNormalizePathRejectsTraversal` and `TestKnowledgeRelativePathPreservesRepositoryTreeBelowProjectAndBranch` from `gitlab/types_test.go` verbatim, renamed to the exported functions, and add:

```go
func TestCollapsePathsDropsNestedAndDuplicateRoots(t *testing.T) {
	require.Equal(t, []string{"docs", "src"}, CollapsePaths([]string{"src", "docs/api", "docs", "src"}))
	require.Nil(t, CollapsePaths([]string{"docs", ""}), "an empty root means the whole repository")
}

func TestInScopeMatchesRootOrDescendant(t *testing.T) {
	require.True(t, InScope("docs/a.md", []string{"docs"}))
	require.False(t, InScope("docs-old/a.md", []string{"docs"}))
	require.True(t, InScope("any/file.md", nil))
}

func TestIsSupportedIsCaseInsensitiveOnExtension(t *testing.T) {
	require.True(t, IsSupported("README.MD"))
	require.False(t, IsSupported("main.go"))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/repofile/`
Expected: build failure, package does not exist.

- [ ] **Step 3: Create `repofile.go` with the five exported functions, then make GitLab call them**

Replace every call in the `gitlab` package (`parseConfig`, `walkFiles`, `streamChanges`, `FetchIncremental`, `item`, `inScope` callers, and the `client_test.go` table) with the `repofile` function and delete the GitLab copies. Keep the comment on the extension set, moved with it.

- [ ] **Step 4: Run both packages' tests**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/repofile/ ./internal/datasource/connector/gitlab/`
Expected: PASS, and `grep -n 'func normalizePath\|func inScope\|func isSupportedFile' internal/datasource/connector/gitlab/*.go` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/repofile internal/datasource/connector/gitlab
git commit -m "refactor(datasource): share repository path helpers between git connectors"
```

---

### Task 2: GitHub API client

**Files:**

- Create: `internal/datasource/connector/github/client.go`
- Create: `internal/datasource/connector/github/client_test.go`

**Interfaces:**

- Consumes: `datasource.ValidateConnectorBaseURL`, `datasource.NewConnectorHTTPClient`, `datasource.ErrInvalidCredentials`, `datasource.ErrPermissionDenied`.
- Produces (package-private):
  - `normalizeBaseURL(value string) string`, pure, per spec decision 5.
  - `newClient(baseURL, token string) (*client, error)`: rejects an empty token, runs `ValidateConnectorBaseURL` on a non-empty `baseURL`, then stores `normalizeBaseURL(baseURL)`. `client` fields `baseURL, token string`, `http *http.Client`, `jsonLimit, treeLimit, rawLimit int64` (production `16 << 20`, `64 << 20`, `100 << 20`).
  - `type apiError struct{ endpoint string; status int }` with `Is` mapping 401 to `ErrInvalidCredentials` and 403 to `ErrPermissionDenied`.
  - `var errRateLimited = errors.New("github API rate limit exhausted")`; a 429, or a 403 whose `Retry-After` header is set or whose `X-RateLimit-Remaining` is `0`, returns `fmt.Errorf("%w until %s", errRateLimited, <X-RateLimit-Reset as RFC3339 UTC, or "unknown">)`.
  - `type repository struct{ ID int64; Name, FullName, HTMLURL, DefaultBranch string }` (JSON tags `id`, `name`, `full_name`, `html_url`, `default_branch`).
  - `type treeEntry struct{ Path, Mode, Type, SHA string; Size int64 }` (JSON `path`, `mode`, `type`, `sha`, `size`); `type tree struct{ SHA string; Entries []treeEntry; Truncated bool }` (JSON `sha`, `tree`, `truncated`).
  - `(c *client) ping(ctx) error` on `GET /user`.
  - `(c *client) repositories(ctx) ([]repository, error)` on `GET /user/repos?per_page=100&sort=full_name&affiliation=owner,collaborator,organization_member`, following `Link` `rel="next"`, with the same `maxPaginationHops = 10000` guard as GitLab.
  - `(c *client) repository(ctx, key string) (*repository, error)` on `GET /repos/{owner}/{repo}`.
  - `(c *client) commitSHA(ctx, key, ref string) (string, error)` on `GET /repos/{owner}/{repo}/commits/{url.PathEscape(ref)}` reading `sha`.
  - `(c *client) tree(ctx, key, sha string, recursive bool) (*tree, error)` on `GET /repos/{owner}/{repo}/git/trees/{sha}` plus `?recursive=1` when `recursive`, read with `treeLimit`.
  - `(c *client) blob(ctx, key, sha string) ([]byte, error)` on `GET /repos/{owner}/{repo}/git/blobs/{sha}` with `Accept: application/vnd.github.raw+json`, read through `readCapped(body, c.rawLimit)`.
  - Every request sets `Authorization: Bearer <token>`, `Accept: application/vnd.github+json` (except `blob`) and `X-GitHub-Api-Version: 2022-11-28`.

- [ ] **Step 1: Write the failing tests**

Test helper `allowLocalGitHubServer(t)` copied from `gitlab/client_test.go:20` (sets `SSRF_WHITELIST` and resets the cache), and `newTestClient(t, server)` that calls `newClient(server.URL, "per-source-token")` and lowers the caps when a test needs to. Tests:

```go
func TestNormalizeBaseURL(t *testing.T) {
	for in, want := range map[string]string{
		"": "https://api.github.com", "github.com": "https://api.github.com",
		"https://www.github.com/": "https://api.github.com", "https://api.github.com": "https://api.github.com",
		"octocorp.ghe.com": "https://api.octocorp.ghe.com", "https://api.octocorp.ghe.com/": "https://api.octocorp.ghe.com",
		"ghe.example.com": "https://ghe.example.com/api/v3", "https://ghe.example.com/api/v3/": "https://ghe.example.com/api/v3",
		"http://127.0.0.1:8080": "http://127.0.0.1:8080/api/v3",
	} {
		require.Equal(t, want, normalizeBaseURL(in), in)
	}
}

func TestNewClientRejectsEmptyTokenAndBlockedHost(t *testing.T) {
	_, err := newClient("", "")
	require.Error(t, err, "an empty token is rejected")
	_, err = newClient("http://127.0.0.1:1", "token") // without the whitelist helper
	require.Error(t, err, "the SSRF policy runs on a non-empty base_url")
}

func TestRequestsCarryGitHubHeaders(t *testing.T) // GET /api/v3/user; asserts Authorization "Bearer per-source-token", Accept "application/vnd.github+json", X-GitHub-Api-Version "2022-11-28"

func TestAPIErrorMapsToSentinels(t *testing.T) // 401 -> errors.Is(err, datasource.ErrInvalidCredentials); 403 -> ErrPermissionDenied; 404 -> neither

func TestRateLimitIsNotPermissionDenied(t *testing.T) {
	for name, hdr := range map[string]http.Header{
		"primary 403":   {"X-RateLimit-Remaining": {"0"}, "X-RateLimit-Reset": {"1800000000"}},
		"secondary 403": {"Retry-After": {"60"}, "X-RateLimit-Remaining": {"4000"}},
	} {
		t.Run(name, func(t *testing.T) {
			// server answers 403 with hdr
			err := c.ping(context.Background())
			require.ErrorIs(t, err, errRateLimited)
			require.NotErrorIs(t, err, datasource.ErrPermissionDenied)
		})
	}
	// a 429 with no headers is also errRateLimited; the primary case's message contains "2027-01-15T08:00:00Z"
}

func TestRepositoriesFollowLinkHeader(t *testing.T) // three pages via Link rel="next"; asserts 201 repositories and pages requested in order ["1","2","3"]

func TestRepositoriesStopAtPaginationGuard(t *testing.T) // Link header always advertises a next page; expects an error after maxPaginationHops, as gitlab/pagination_guard_test.go does

func TestTreeReportsTruncationAndUsesTreeCap(t *testing.T) // response {"sha":"t1","tree":[...],"truncated":true}; asserts Truncated, the path "/api/v3/repos/o/r/git/trees/abc" with query recursive=1 when recursive, no query otherwise; with c.treeLimit = 16 a longer body errors with "exceeds maximum size"

func TestBlobUsesRawMediaTypeAndCap(t *testing.T) // asserts Accept "application/vnd.github.raw+json"; with c.rawLimit = 8 a 9-byte body returns an error containing "exceeds maximum size"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/github/`
Expected: build failure, undefined `newClient`.

- [ ] **Step 3: Implement `client.go`**

Follow `gitlab/client.go` for `readCapped`, `getCapped`, the hop guard and the error type. `Link` parsing: take the URL in the `<...>; rel="next"` segment and request it as is (it is absolute).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/github/ -run 'Normalize|NewClient|Request|APIError|RateLimit|Repositories|Tree|Blob'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/github/client.go internal/datasource/connector/github/client_test.go
git commit -m "feat(datasource/github): add the GitHub REST client"
```

---

### Task 3: Settings parser

**Files:**

- Create: `internal/datasource/connector/github/types.go`
- Create: `internal/datasource/connector/github/types_test.go`

**Interfaces:**

- Consumes: `repofile.NormalizePath`, `repofile.CollapsePaths`.
- Produces: `type repositorySelection struct{ Repository, Ref string; Paths []string }` (JSON `repository`, `ref`, `paths`); `type config struct{ Repositories []repositorySelection }` (JSON `repositories`); `parseConfig(ds *types.DataSourceConfig) (*config, error)` reading `ds.Settings["repositories"]`; `normalizeRepository(value string) (string, error)`.
- `normalizeRepository`: trim spaces and slashes, strip a trailing `.git`, lowercase, require exactly one `/` with non-empty parts, reject anything containing `://` with `fmt.Errorf("%w: repository must be owner/repo, not a URL", datasource.ErrInvalidConfig)`. Uniqueness is checked on the normalised value.
- Error texts mirror GitLab's with `repository` in place of `project_id`: `settings.repositories is required`, `settings.repositories must be an array`, `repository must be unique and non-empty`, `at least one repository is required`.

- [ ] **Step 1: Write the failing tests**

```go
func TestParseConfigNormalisesRepository(t *testing.T) {
	for in, want := range map[string]string{"owner/repo": "owner/repo", "/OWNER/Repo/": "owner/repo", "owner/repo.git": "owner/repo"} {
		cfg, err := parseConfig(settings(in))
		require.NoError(t, err, in)
		require.Equal(t, want, cfg.Repositories[0].Repository)
	}
	for _, in := range []string{"https://github.com/owner/repo", "owner", "owner/repo/extra", ""} {
		_, err := parseConfig(settings(in))
		require.ErrorIs(t, err, datasource.ErrInvalidConfig, in)
	}
}

func TestParseConfigCollapsesDirectories(t *testing.T) // paths ["/docs/", "docs/api", "src"] -> ["docs", "src"]; copy of the GitLab test with the new keys

func TestParseConfigRejectsDuplicateRepositoryIgnoringCase(t *testing.T) // entries "owner/repo" and "Owner/Repo" -> ErrInvalidConfig

func TestParseConfigRequiresRepositories(t *testing.T) // Settings without the key, and with an empty array -> ErrInvalidConfig
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/github/ -run ParseConfig`
Expected: build failure, undefined `parseConfig`.

- [ ] **Step 3: Implement `types.go`**

Port `gitlab/types.go` `parseConfig` with the new keys and `normalizeRepository` in place of the ID check.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/github/ -run ParseConfig`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/github/types.go internal/datasource/connector/github/types_test.go
git commit -m "feat(datasource/github): parse repository selections"
```

---

### Task 4: Connector with snapshot sync

**Files:**

- Create: `internal/datasource/connector/github/connector.go`
- Create: `internal/datasource/connector/github/snapshot.go`
- Create: `internal/datasource/connector/github/connector_test.go`
- Create: `internal/datasource/connector/github/snapshot_test.go`

**Interfaces:**

- Consumes: Task 2 client, Task 3 `parseConfig`, `repofile.InScope`, `repofile.IsSupported`, `repofile.KnowledgeRelativePath`, `datasource.StreamHandler`, `types.SyncCursor.CompleteListing`.
- Produces:
  - `NewConnector() *Connector`; `(c *Connector) Type() string` returns `types.ConnectorTypeGitHub`; `var _ datasource.StreamingConnector = (*Connector)(nil)`.
  - `Validate`, `ListResources`, `ResolveResourceAncestors` (returns `[]string{}`), `FetchAll`, `FetchIncremental`, `FetchStream` with the signatures from `internal/datasource/connector.go`.
  - `type blob struct{ SHA string; Size int64 }`; `type snapshot map[string]blob` keyed by repository path, holding only entries with `Type == "blob"`, `Mode` `100644` or `100755`, in scope and supported.
  - `(c *Connector) snapshot(ctx, key, commit string, roots []string) (snapshot, error)`: recursive tree of `commit`; when `Truncated`, rebuilt by `(c *Connector) walkSubtrees(ctx, key, rootTreeSHA string, roots []string) (snapshot, error)`, which descends from the commit's root tree one non-recursive `client.tree` call per subtree, following each `tree` entry's `SHA`, and only into subtrees that are on the way to or inside a root. `snapshot` returns `errMissingCommit` (wrapping the `apiError`) when the tree request answers 404 or 422.
  - `type change struct{ Path string; Deleted bool }`; `diff(prev, next snapshot, sameRef bool) []change`: when `sameRef`, deletions for paths in `prev` not in `next`, then changes for paths in `next` whose SHA differs from `prev` or are absent from it; when not `sameRef`, deletions for every path in `prev`, then changes for every path in `next`. Each group sorted by path.
  - `type repositoryCursor struct{ Head, Ref string; Paths []string; Pending *pendingCursor }` (JSON `head`, `ref`, `paths`, `pending,omitempty`); `type pendingCursor struct{ Head string; Done int }` (JSON `head`, `done`); `type cursor struct{ Repositories map[string]repositoryCursor }` (JSON `repositories`); `gitHubCursor(cursor, complete bool) *types.SyncCursor` writes `{"repositories": ..., "raw": <json>}` with `LastSyncTime` now and `CompleteListing: complete`. Reading an old cursor whose repository value is not an object is treated as no previous head.
  - `const checkpointEvery = 50`.
  - `const reasonFileTooLarge = "github_file_too_large"`; `var SyncReasonTexts = map[string]string{reasonFileTooLarge: "The GitHub file exceeds the 100 MB API limit"}`; `(c *Connector) errorItem(repo *repository, key, ref, file, code string) types.FetchedItem` with the item's `ExternalID` and `Title`, no content, and `Metadata` `channel`, `source_type`, `github_path`, `error`, `error_reason_code`, `error_reason` (shape of `seafile/sync.go:441`).
  - Item, deletion, external ID, title, file name and metadata exactly as the spec's "Data shapes" section; the zero-byte body rule and its comment from `gitlab/connector.go:item`.

Algorithm for `FetchStream` (the one block the signatures do not determine). `complete := old == nil`. For each selection: resolve the repository (`client.repository`), the ref (empty means `DefaultBranch`) and the head SHA. Load `rc := prev.Repositories[key]`. Build `next := snapshot(key, head, roots)`. Then:

- if `rc.Head == ""`: `changes = diff(nil, next, true)`;
- else if `rc.Head == head && rc.Ref == ref && slices.Equal(rc.Paths, roots)`: nothing to emit;
- else build `prevSnap := snapshot(key, rc.Head, rc.Paths)`; on `errMissingCommit` log at Info and use `diff(nil, next, true)`; otherwise `changes = diff(prevSnap, next, rc.Ref == ref)`, where deletions take their external ID from `rc.Ref` and changes from `ref`.

If `rc.Pending != nil && rc.Pending.Head == head`, skip the first `rc.Pending.Done` entries of `changes`. Emit the rest in order: a deletion as the tombstone; a change whose blob `Size > c.client.rawLimit` as `errorItem(..., reasonFileTooLarge)` with no blob request; any other change as the item. After every `checkpointEvery` emitted entries, set `rc.Pending = &pendingCursor{Head: head, Done: n}`, store `rc` and checkpoint. When the repository is done, set `rc = repositoryCursor{Head: head, Ref: ref, Paths: roots}` (no `Pending`), store and checkpoint. Return `gitHubCursor(next, complete)`. On any error return `nil, err`.

`FetchAll` and `FetchIncremental` collect what `FetchStream` would emit, like the GitLab compatibility methods. `ListResources` per the spec's Resources shape: root through `client.repositories` (external ID lowercased); a parent `key[:dir]` through the non-recursive tree of the default branch, descending the `dir` segments by subtree SHA and keeping `tree` entries.

- [ ] **Step 1: Write the failing tests**

A test server helper `fakeGitHub(t, repo fakeRepo)` mounted under `/api/v3`, serving `/user`, `/user/repos`, `/repos/o/r`, `/repos/o/r/commits/{ref}`, `/repos/o/r/git/trees/{sha}` (recursive and not), `/repos/o/r/git/blobs/{sha}` from an in-memory map of `commit sha -> map[path]content` plus per-commit `mode` overrides (blob SHA is the hex SHA-1 of `"blob <len>\x00" + content`, so renames and reverts share SHAs as on GitHub; subtree SHAs are the SHA-1 of the sorted entry list), recording request paths; `gitHubStreamRecorder` copied from `gitlab/client_test.go:236`, with an optional `failAt int` that returns an error from the n-th `Emit`. `base := server.URL + "/api/v3"`.

```go
func TestValidateUsesDataSourceCredentials(t *testing.T) // Validate succeeds against the fake with a Bearer token; wrong token -> ErrInvalidCredentials

func TestListResourcesListsRepositoriesThenDirectories(t *testing.T) {
	// root -> [{ExternalID:"o/r", Name:"o/R", Type:"repository", URL:"https://github.com/o/R", HasChildren:true}]
	// parent "o/r" -> directories only, ExternalID "o/r:docs", ParentID "o/r"; parent "o/r:docs" -> "o/r:docs/sub"
}

func TestFetchStreamFullSyncEmitsSupportedRegularBlobsInScope(t *testing.T) {
	// head tree: docs/a.md, docs/b.go, docs/link.md (mode 120000), src/c.md, README.md; roots ["docs"]
	// expects exactly one item: ExternalID "github:"+base+":o/r:main:docs/a.md", Title "o/R/docs/a.md",
	// FileName "R-main/docs/a.md", SourceResourceID "o/r", Metadata github_url "https://github.com/o/R/blob/main/docs/a.md",
	// returned cursor CompleteListing true, connector_cursor repositories {"o/r": {head, ref "main", paths ["docs"]}} with no pending; one checkpoint
}

func TestFetchStreamIncrementalEmitsOnlyChangedAndDeleted(t *testing.T) {
	// prev head: docs/a.md "v1", docs/b.md "x"; new head: docs/a.md "v2", docs/c.md "x" (b renamed to c)
	// expects emits in order: tombstone docs/b.md, item docs/a.md, item docs/c.md; no blob request for docs/c.md's unchanged content is NOT required (the SHA is new to the path); CompleteListing false
}

func TestFetchStreamForcePushEmitsSnapshotChanges(t *testing.T) {
	// cursor head is a commit that is NOT an ancestor of the new head (fake serves both trees)
	// expects the file present only at the old head tombstoned and the file reverted at the new head emitted
}

func TestFetchStreamMissingPreviousHeadFallsBackToFullEmit(t *testing.T) {
	// fake answers 404 for trees/{old head}; expects every in-scope file of the new head emitted, no tombstones, cursor advanced to the new head
}

func TestFetchStreamUnchangedHeadEmitsNothing(t *testing.T) // same head, ref and paths as the cursor -> no Emit, one Checkpoint, cursor unchanged

func TestFetchStreamSelectionChangeTombstonesOldRefAndEmitsNewRoots(t *testing.T) {
	// cursor {head H, ref "main", paths ["docs"]}; same head, selection now ref "release", paths ["docs","guides"]
	// expects tombstones with ":main:" IDs for every docs file, then items with ":release:" IDs for docs and guides files
	// second case: same ref, paths ["docs"] -> ["docs","guides"], head unchanged: expects only the guides files emitted, no tombstones
}

func TestFetchStreamResumesInsideRepositoryFromPendingCursor(t *testing.T) {
	// 120 changed files; recorder failAt 101 -> FetchStream returns the error and nil cursor; the last Checkpoint holds pending {head, done 100}
	// resuming with that cursor emits files 101..120 only and the final cursor has no pending
}

func TestSnapshotFallsBackToSubtreeWalkWhenTruncated(t *testing.T) {
	// recursive trees endpoint answers truncated:true with an empty list; non-recursive trees serve root, docs/ and docs/sub/
	// expects the snapshot to contain docs/a.md and docs/sub/b.md, and the recorded requests to include the root, docs and docs/sub tree SHAs and NOT src's
}

func TestOversizedBlobBecomesErrorItem(t *testing.T) {
	// tree entry size 101<<20; expects one item with empty Content, the normal ExternalID and Title, Metadata error_reason_code "github_file_too_large", and no blob request
}

func TestEmptyBlobSyncsItsPath(t *testing.T) // zero-byte blob -> Content "# o/R/docs/empty.md\n"

func TestDiffOrdersDeletionsThenChanges(t *testing.T) {
	got := diff(snapshot{"a": {SHA: "1"}, "b": {SHA: "2"}}, snapshot{"b": {SHA: "3"}, "c": {SHA: "4"}}, true)
	require.Equal(t, []change{{"a", true}, {"b", false}, {"c", false}}, got)
	got = diff(snapshot{"a": {SHA: "1"}}, snapshot{"a": {SHA: "1"}}, false)
	require.Equal(t, []change{{"a", true}, {"a", false}}, got, "a ref change re-identifies every file")
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/github/`
Expected: build failure, undefined `NewConnector`.

- [ ] **Step 3: Implement `snapshot.go` (`blob`, `snapshot`, `walkSubtrees`, `change`, `diff`, `errMissingCommit`) and `connector.go` (everything else)**

Keep the GitLab file layout and comments where the reasoning is the same (empty blob, unset `UpdatedAt`, streaming memory bound); write the snapshot rationale from spec decision 1 and the cursor rationale from decision 7 as the package comments on `snapshot` and `repositoryCursor`.

- [ ] **Step 4: Run the package tests with the race detector**

Run: `GOTOOLCHAIN=go1.26.10 go test -race ./internal/datasource/connector/github/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/github
git commit -m "feat(datasource/github): sync repository files from tree snapshots"
```

---

### Task 5: Register the type and wire the service

**Files:**

- Modify: `internal/datasource/connector.go:295-302` (metadata entry)
- Modify: `internal/container/container.go:66,1898` (import and registration)
- Modify: `internal/application/service/knowledge_create.go:1157`
- Modify: `internal/application/service/knowledge_create_gitlab_test.go`
- Modify: `internal/application/repository/knowledge_duplicate_test.go:79-91`
- Modify: `internal/datasource/syncreason/parity_test.go:25`
- Modify: `frontend/src/i18n/locales/en-US.ts:7007` (`datasource.syncError`)
- Modify: `internal/datasource/README.md:7`, `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md:14`

**Interfaces:**

- Consumes: `github.NewConnector`, `github.SyncReasonTexts`.
- Produces: metadata entry `{Type: github, Name: "GitHub", Description: "Sync files from GitHub repositories", Priority: 4, AuthType: "token", Capabilities: {"incremental", "hierarchical", "deletion_sync", CapabilityFullSyncReconcile}}`; i18n key `datasource.syncError.github_file_too_large: 'The GitHub file exceeds the 100 MB API limit; GitHub does not serve larger blobs.'`.

- [ ] **Step 1: Write the failing tests**

A GitHub sibling of `TestCreateKnowledgeFromFileGitLabPreservesSourcePaths` in `knowledge_create_gitlab_test.go`:

```go
func TestCreateKnowledgeFromFileGitHubUsesSourceIdentity(t *testing.T) // same shape as the GitLab test with types.ConnectorTypeGitHub and external_id "github:https://api.github.com:o/r:main:" + file; asserts two sources sharing bytes are two rows
```

Add a documentation row to the table in `TestCheckKnowledgeExists_FileSourceIdentity` (the repository layer is already generic, so this row passes before the change and only pins the ID shape):

```go
{"github item of another source", "ds-3", "github:https://api.github.com:o/r:main:README.md", "same-hash", "md", false},
```

Add `github.SyncReasonTexts` to the list in `parity_test.go:25` (this fails until the i18n key exists).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/application/service/ ./internal/datasource/syncreason/ -run 'GitHub|Parity'`
Expected: FAIL (the service test's two sources collapse into one row; parity reports the missing key).

- [ ] **Step 3: Make the edits**

Metadata entry per Interfaces; `registry.Register(githubConnector.NewConnector())` after the GitLab line with the matching `errors.Join` message; add `types.ConnectorTypeGitHub` to the `case` in `usesSourceIdentityDuplicateCheck`; the i18n key; README and guide: "ten connector packages plus the shared `repofile` helper package", "thirteen types" with `github` in the list, and drop `GitHub` from the "metadata entries only" sentence.

- [ ] **Step 4: Run the tests to verify they pass, then build the server**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/application/service/ ./internal/application/repository/ ./internal/datasource/... && GOTOOLCHAIN=go1.26.10 go build ./cmd/server`
Expected: PASS, build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector.go internal/container/container.go internal/application/service internal/application/repository/knowledge_duplicate_test.go internal/datasource/syncreason/parity_test.go frontend/src/i18n/locales/en-US.ts internal/datasource/README.md internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md
git commit -m "feat(datasource): register the GitHub connector"
```

---

### Task 6: Frontend form, labels and icon

**Files:**

- Modify: `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue:269,283-295,737-742,785,813-819,894,1130-1135,1154,1173,1746-1763`
- Modify: `frontend/src/views/knowledge/settings/DataSourceEditorDialog.test.ts`
- Modify: `frontend/src/i18n/localeKeyAudit.test.ts`
- Modify: `frontend/src/views/knowledge/settings/datasourceIcons.ts`
- Create: `frontend/src/assets/img/datasource-github.svg`
- Modify: `frontend/src/views/knowledge/components/DocumentListView.vue:105`, `DocumentCardView.vue:156`, `frontend/src/views/knowledge/KnowledgeBase.vue:692`, `frontend/src/views/chat/components/tool-results/DocumentInfo.vue:111`, `frontend/src/components/doc-content.vue:988`
- Modify: `frontend/src/i18n/locales/en-US.ts:802,6763,6856,6870`

**Interfaces:**

- Produces in the dialog (names the test reaches through `vm`): `isGitConnector(type: string): boolean` (`gitlab` or `github`) replacing `isGitLabConnector`; `gitProjects: Ref<GitProjectInput[]>` with `GitProjectInput { project_id: string; ref: string; pathsText: string }` replacing `gitlabProjects`; `syncGitProjectsToSettings()`, `addGitProject()`, `removeGitProject(index)`; `gitSettingsKey` computed, `'projects'` for GitLab and `'repositories'` for GitHub; `gitItemKey` computed, `'project_id'` or `'repository'`; `gitLabelsKey` computed, `'datasource.gitlab'` or `'datasource.github'`, and the template and the step gate warning use `t(\`${gitLabelsKey}.projectId\`)`, `t(\`${gitLabelsKey}.projectRequired\`)` and the other members.
- Loading a saved source reads `settings[gitSettingsKey]` and maps `item[gitItemKey]` into `project_id`; saving writes the same keys and never the other type's key. `selectType` for a git connector sets `gitProjects` to one empty row and deletes both `settings.projects` and `settings.repositories` before the row is added.
- New `connectorDefs` entry: `{ type: 'github', available: true, docUrl: 'https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens', permissionDocUrl: '', permissionPageUrl: '', requiredPermissions: [], fields: [ { key: 'base_url', labelKey: 'datasource.github.baseUrl', placeholder: 'https://api.github.com', optional: true, hintKey: 'datasource.github.baseUrlHint' }, { key: 'access_token', labelKey: 'datasource.github.accessToken', placeholder: '', secret: true } ] }`.
- i18n: `knowledgeBase.channelGitHub: 'GitHub'`; `datasource.connector.github: 'GitHub'`; `datasource.connectorDesc.github: 'Sync files from GitHub repositories'`; group `datasource.github` with the same member names as `datasource.gitlab`: `baseUrl: 'GitHub API URL'`, `baseUrlHint: 'Leave empty for github.com; for GitHub Enterprise Server or a ghe.com enterprise enter the host, for example https://ghe.example.com'`, `accessToken: 'Personal access token'`, `projects: 'GitHub repositories'`, `projectsHint: 'Enter a repository as owner/repo, with optional branch and directories.'`, `project: 'Repository'`, `projectId: 'Repository'`, `projectIdPlaceholder: 'For example: owner/repo'`, `ref: 'Branch'`, `refPlaceholder: 'Leave empty to use the default branch'`, `paths: 'Directories'`, `pathsPlaceholder: 'One directory per line; leave empty to sync the whole repository'`, `addProject: 'Add repository'`, `projectRequired: 'Add at least one GitHub repository'`.
- Icon: the GitHub mark from Simple Icons (CC0), `https://raw.githubusercontent.com/simple-icons/simple-icons/develop/icons/github.svg`, saved unmodified; `datasourceIconMap.github = githubIcon`.
- Channel maps: add `github` next to `gitlab` in each of the five files, label `knowledgeBase.channelGitHub`.

- [ ] **Step 1: Write the failing tests**

In `DataSourceEditorDialog.test.ts`, next to the GitLab tests:

```ts
test('saved GitHub repositories hydrate the shared git form', async () => {
  const f = await fixture({ type: 'github', settings: { repositories: [{ repository: 'o/r', ref: 'main', paths: ['docs'] }] } })
  try {
    assert.deepEqual(f.vm.gitProjects, [{ project_id: 'o/r', ref: 'main', pathsText: 'docs' }])
    f.vm.syncGitProjectsToSettings()
    assert.deepEqual(f.vm.form.config.settings, { repositories: [{ repository: 'o/r', ref: 'main', paths: ['docs'] }] })
  } finally { f.close() }
})

test('switching the git connector type resets the repository rows', async () => {
  const f = await fixture({ create: true })
  try {
    f.vm.selectType(f.vm.connectorDefs.find((def: any) => def.type === 'gitlab'))
    f.vm.gitProjects[0].project_id = '123'
    f.vm.syncGitProjectsToSettings()
    f.vm.selectType(f.vm.connectorDefs.find((def: any) => def.type === 'github'))
    assert.deepEqual(f.vm.gitProjects, [{ project_id: '', ref: '', pathsText: '' }])
    assert.equal(f.vm.form.config.settings.projects, undefined)
    await f.replace()
    await f.vm.testConnection()
    assert.deepEqual(f.calls.map(call => call.method), ['validateCredentials'])
  } finally { f.close() }
})
```

Update the existing GitLab tests for the renamed members (`gitProjects`, `syncGitProjectsToSettings`) and add a GitLab assertion that saving still writes `settings.projects[0].project_id`. In `localeKeyAudit.test.ts` add:

```ts
test('the GitHub and GitLab form label groups have the same members', () => {
  const en = LOCALE_BUNDLES["en-US"] as any
  assert.deepEqual(Object.keys(en.datasource.github).sort(), [...Object.keys(en.datasource.gitlab), "baseUrlHint"].sort())
})
```

(`LOCALE_BUNDLES` is already imported there; the template-built `${gitLabelsKey}.*` keys are out of the audit's scope, so this is what proves they resolve.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `npm --prefix frontend test -- src/views/knowledge/settings/DataSourceEditorDialog.test.ts src/i18n/localeKeyAudit.test.ts`
Expected: FAIL, `gitProjects` undefined and `en.datasource.github` undefined.

- [ ] **Step 3: Make the dialog generic, add the definition, i18n keys, icon and channel labels**

The step 2 template block at line 1746 keeps its structure; only the predicate, the state names and the label keys change.

- [ ] **Step 4: Run the frontend checks**

Run: `npm --prefix frontend test && npm --prefix frontend run type-check`
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(ui): add the GitHub data source form"
```

---

### Task 7: Documentation, changelog and end-to-end check

**Files:**

- Modify: `website-docs/03-features/10-datasource.md:40,99-110,134,279,303,418,435,439`
- Modify: `website-docs/01-getting-started/01-introduction.md:50,110`, `website-docs/01-getting-started/04-configuration.md:144`, `website-docs/04-api/02-api-infra.md:309`, `website-docs/05-clients/01-frontend.md:194`, `website-docs/06-development/02-database-schema.md:250`, `website-docs/06-development/03-extension-points.md:501-503`, `README.md:51,233`, `CHANGELOG.md` (Unreleased)

**Interfaces:**

- Consumes: the final JSON shapes from the spec; the registered-type count is now 13.

- [ ] **Step 1: Write the docs**

A level-four "GitHub (connector/github/)" heading, matching the GitLab one, after the GitLab section in `10-datasource.md`, with four paragraphs adapted from it: credentials (`base_url` optional: github.com, a ghe.com enterprise or an Enterprise Server host; a personal access token with read access to the repositories' contents), the `settings.repositories` shape with the lowercase `owner/repo` rule, sync behaviour (incremental sync diffs repository snapshots built from git trees, so a force-push and a changed branch or directory selection are handled; progress is checkpointed every 50 files so a sync stopped by the hourly rate limit resumes where it stopped; a file over the 100 MB API limit is reported per item; a Git LFS file is imported as its pointer text; a forced full sync removes items the run did not list; deletions follow `sync_deletions`), and the example `{"credentials":{"base_url":"","access_token":"<token>"},"settings":{"repositories":[{"repository":"owner/repo","ref":"main","paths":["docs"]}]}}`. Add a GitHub row to the connector table (type identifier github, "Directories under a chosen branch or tag of a repository"), `github` to every type list and count named above, GitHub to the connectors that reconcile a full sync (line 279) and to the streaming list (lines 303 and 418), the registration line (line 435), and remove GitHub from the two "not implemented yet" sentences. CHANGELOG under `## [Unreleased]`: `### Changes` then an entry starting `- **NEW**: **GitHub data source**:` that says: sync document files from the directories of GitHub, ghe.com or GitHub Enterprise Server repositories; incremental sync diffs repository snapshots, so a force-push is handled; progress resumes after a rate limit; files over the 100 MB API limit are reported per item; enable it with the env var `AIKB_DATASOURCE_TYPES`.

- [ ] **Step 2: Check for stale claims**

Run: `git grep -n 'GitHub, Google Drive, OneDrive' -- '*.md'; git grep -n -i 'twelve types\|12 connector types' -- '*.md' internal`
Expected: no output. Then run the `docs-drift-reviewer` agent on `git diff main...HEAD`.

- [ ] **Step 3: Run the repository gates**

Run: `scripts/validate.sh --base main`
Expected: every area it lists passes, including the English-only, em dash and version gates.

- [ ] **Step 4: Prove it against the real API**

Start the stack (`make dev-start && make dev-app && make dev-frontend`) with `AIKB_DATASOURCE_TYPES=all`, create a GitHub data source for a repository you own with a `docs` directory, using a fine-grained token with read-only contents permission, and run a sync. Then push a commit that renames one file and empties another and run an incremental sync. Then change the selected branch and run again. Expected: the first sync imports the `docs` files; the second removes the old name, imports the new one, and the emptied file's knowledge item now holds its path; the third tombstones the old branch's items and imports the new branch's. Record the three sync logs in the PR description.

- [ ] **Step 5: Commit and open the PR**

```bash
git add website-docs README.md CHANGELOG.md
git commit -m "docs(datasource): document the GitHub connector"
git push -u origin feat/github-datasource
```

Open the PR against `main` with the spec link, the sync-log evidence from Step 4, and the attribution line `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
