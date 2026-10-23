# Bitbucket Cloud Data Source Connector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `bitbucket` data source connector that syncs document files from selected directories of Bitbucket Cloud repositories, with streaming full and incremental sync, deletions and resumable checkpoints, enabled by default next to GitHub.

**Architecture:** A new Go package `internal/datasource/connector/bitbucket` copies the GitHub connector's layout (package-private HTTP client, config parser, `StreamingConnector`) but, since Bitbucket exposes no blob ids, every run lists the new head and diffs with `diffstat` only when both heads exist under the same ref; an interrupted run is drained before the current selection is reconciled, so there is no pending chain. The frontend turns the three per-type ternaries of the git form into one lookup table and adds the type to the shared surfaces.

**Tech Stack:** Go 1.26 (`GOTOOLCHAIN=go1.26.10` to match CI), `net/http/httptest` and `testify`; Vue 3 + TypeScript, vue-i18n, node test runner via tsx (`npm --prefix frontend test`), vue-tsc (`npm --prefix frontend run type-check`).

**Spec:** `docs/superpowers/specs/2026-10-22-bitbucket-connector-design.md`

## Global Constraints

- English only, no em dashes anywhere (`scripts/check-english-only.sh`, `scripts/check-emdash.sh`; the pre-commit hook enforces both).
- Commit subjects: `type(scope): imperative subject`, for example `feat(datasource/bitbucket): ...`. Every commit message ends with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.
- Before each commit: run the built-in `/simplify` and `/code-review` on the diff. golangci-lint runs over the whole root module on commit, so keep unrelated Go files clean. Lines at most 120 characters, gofumpt.
- No UI text outside `frontend/src/i18n/locales/en-US.ts`; product name in copy is "AI KB".
- A doc claim cites code that matches it; after a code change, `git grep -n '<old value>' -- '*.md'` for the old value and run the `docs-drift-reviewer` agent on the diff.
- API host is fixed: `https://api.bitbucket.org/2.0`; no `base_url` credential, no SSRF validation of user input. Auth header `Authorization: Basic base64(username:token)` when `username` is set, else `Authorization: Bearer <token>`. Raw file cap 100 MB; JSON response cap 16 MiB; `pagelen=100`; listing `max_depth=50`, per-directory fallback `max_depth=10`; checkpoint every 50 entries; rate-limit wait at most 30 minutes, once per run.
- The connector copies the capped reader, the status-to-sentinel error type, the config paths loop and the empty-file body from GitHub (spec decision 13); nothing is extracted into `repofile`.
- Tests that build a client against an `httptest` server override the fixed host: `newClient` takes the base URL as its first parameter and production callers pass the `apiBaseURL` constant; the `next` guard and the external ID use the client's `baseURL` field, so a test's IDs start with the fake server's URL. The fake is whitelisted with `allowLocalServer(t)` copied from `github/client_test.go`.
- Work on a branch `feat/bitbucket-datasource` cut from `main`; never push to `main` directly. The GitHub icon swap (Task 1) is the first commit on that branch.

## Review Focus

1. The diffstat direction: `diffstat/{new}..{old}` reports a file added at `new` as `added`; reading it the `git diff` way would tombstone every new file and re-import every deleted one (Task 4, `TestFetchStreamIncrementalUsesDiffstatDirection`).
2. A listing cut off at the depth limit: a `commit_directory` at the limit of a listing must fail the sync, because a listing missing files tombstones them (Task 4, `TestListingFailsOnDirectoryAtDepthLimit`).
3. A pending run whose head moved before the retry: the pending must be drained first, every file it emitted and the new head lacks must be tombstoned, and a file it emitted that the new head keeps must not be tombstoned (Task 4, `TestFetchStreamDrainsPendingBeforeMovedHead`).
4. A raw fetch that answers 3xx (an LFS pointer the listing did not flag): the token must not follow the redirect off host, and the item is a per-item failure (Task 2, `TestRawDoesNotFollowRedirects`; Task 4, `TestRawRedirectBecomesLFSErrorItem`).
5. Editing a saved Bitbucket source, a GitHub source and a GitLab source in one session: each loads and writes its own settings key and labels only (Task 6, `saved Bitbucket repositories hydrate the shared git form`, `the git connector table covers every git type`).

---

### Task 1: Replace the GitHub icon with the official Invertocat and record icon provenance

**Files:**

- Modify: `frontend/src/assets/img/datasource-github.svg` (today the Simple Icons tracing)
- Create: `frontend/src/assets/img/ICONS.md`

**Interfaces:**

- Produces: `ICONS.md`, a Markdown table with the columns `File`, `Source`, `Checked against`, one row per third-party mark. First row: `datasource-github.svg`, the Invertocat (`github-mark.svg`) from `GitHub_Logos.zip` at `https://github.com/logos`, checked against the GitHub Logos and Usage page (the mark may be used to link to GitHub or show a repository, unmodified).
- The dark-mode rule at `DataSourceTypeIcon.vue:96-99` stays: the Invertocat is black like the tracing it replaces.

- [ ] **Step 1: Fetch the official asset**

Download `https://github.com/logos` → `GitHub_Logos.zip` into an empty scratch directory (ask the operator once before the download; name the file, the source and its size), extract it there and copy `github-mark/github-mark.svg` over `frontend/src/assets/img/datasource-github.svg` unmodified.

- [ ] **Step 2: Write `ICONS.md`**

The table per Interfaces, headed by one sentence: every third-party mark under this directory is reproduced unmodified from the source in its row.

- [ ] **Step 3: Verify the asset renders and the audit passes**

Run: `head -c 200 frontend/src/assets/img/datasource-github.svg; npm --prefix frontend test -- src/views/knowledge/settings/DataSourceEditorDialog.test.ts && npm --prefix frontend run type-check`
Expected: the file starts with `<svg`, has no `<title>GitHub</title>` from Simple Icons, both commands PASS.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/assets/img/datasource-github.svg frontend/src/assets/img/ICONS.md
git commit -m "chore(ui): use GitHub's own Invertocat for the data source icon"
```

---

### Task 2: Bitbucket API client

**Files:**

- Create: `internal/datasource/connector/bitbucket/client.go`
- Create: `internal/datasource/connector/bitbucket/client_test.go`

**Interfaces:**

- Consumes: `datasource.NewConnectorHTTPClient`, `datasource.ErrInvalidCredentials`, `datasource.ErrPermissionDenied`, `datasource.ErrResourceNotFound`, `datasource.ErrInvalidConfig`.
- Produces (package-private):
  - `const apiBaseURL = "https://api.bitbucket.org/2.0"`, `maxJSONResponseBytes int64 = 16 << 20`, `maxRawFileBytes int64 = 100 << 20`, `maxPaginationHops = 10000`, `pageLen = 100`, `listingDepth = 50`, `fallbackListingDepth = 10`, `rateLimitWaitMax = 30 * time.Minute`, `statusTimedOut = 555`.
  - `newClient(baseURL, username, token string) (*client, error)`: rejects an empty token with `ErrInvalidConfig`; `client` fields `baseURL, auth string` (`auth` is the full header value, `Basic ...` when `username` is non-empty after trimming, else `Bearer <token>`), `http, raw *http.Client` (both `NewConnectorHTTPClient(30 * time.Second)`; `raw` gets `CheckRedirect` returning `http.ErrUseLastResponse`), `jsonLimit, rawLimit int64`, `waited bool`.
  - `type apiError struct{ endpoint string; status int }` with `Error()` and `Is` mapping 401 to `ErrInvalidCredentials`, 403 to `ErrPermissionDenied`, 404 to `ErrResourceNotFound`, 555 to `errTimedOut`.
  - `var errRateLimited = errors.New("bitbucket API rate limit exhausted")`, `var errTimedOut = errors.New("bitbucket API timed out")`, `var errLFSRedirect = errors.New("bitbucket file is served by redirect")`.
  - `type rateLimitError struct{ reset time.Duration }` whose `Error()` is `"bitbucket API rate limit exhausted, resets in <reset>"` and whose `Is(errRateLimited)` is true; built from a 429's `x-ratelimit-reset` header (seconds; absent or unparsable means `reset` 0 and the text `"resets at an unknown time"`).
  - `(c *client) fetch(ctx, rawURL string, out any) error`: GET with `Authorization: c.auth` and `Accept: application/json`, reads through `readCapped(body, c.jsonLimit)` and unmarshals. On a 429 whose reset is at most `rateLimitWaitMax` and `!c.waited`: set `waited`, log at Warn, wait for the reset (or `ctx.Done()`), retry once; any other 429 returns the `rateLimitError`.
  - `(c *client) pages(ctx, first string, each func(values json.RawMessage) error) error`: follows `next` only, with GitHub's three guards (repeated URL, more than `maxPaginationHops`, and a `next` not prefixed by `c.baseURL + "/"`), each page decoded as `struct{ Values json.RawMessage; Next string }`.
  - `type repository struct{ UUID, FullName string; MainBranch *struct{ Name string }; Links struct{ HTML struct{ Href string } } }` (JSON `uuid`, `full_name`, `mainbranch`, `links.html.href`); `(r repository) slug() string` is the part after `/`.
  - `type srcEntry struct{ Type, Path string; Size int64; Attributes []string }` (JSON `type`, `path`, `size`, `attributes`); `(e srcEntry) has(attribute string) bool`.
  - `type diffEntry struct{ Status string; Old, New *struct{ Path string } }` (JSON `status`, `old`, `new`).
  - `(c *client) ping(ctx) error` on `/user`.
  - `(c *client) workspaces(ctx) ([]string, error)` on `/user/workspaces?pagelen=100&fields=values.slug,next`, slugs in page order.
  - `(c *client) repositories(ctx, workspace string) ([]repository, error)` on `/repositories/{ws}?role=member&pagelen=100&fields=values.full_name,values.uuid,values.mainbranch.name,values.links.html.href,next`.
  - `(c *client) repository(ctx, key string) (*repository, error)` on `/repositories/{ws}/{slug}`.
  - `(c *client) headCommit(ctx, key, ref string) (string, error)`: `/refs/branches/{ref}` reading `target.hash`, on 404 `/refs/tags/{ref}`, on 404 and a 40-hex `ref` `/commit/{ref}` reading `hash`; a final 404 is returned as is (it already `Is` `ErrResourceNotFound`). `ref` is passed through `url.PathEscape` segment by segment (a branch name may contain `/`).
  - `(c *client) commitExists(ctx, key, hash string) (bool, error)` on `/commit/{hash}`: 404 is `false, nil`.
  - `(c *client) listing(ctx, key, commit, dir string, depth int) ([]srcEntry, error)` on `/src/{commit}/{dir}/?max_depth={depth}&pagelen=100&fields=values.path,values.type,values.size,values.attributes,next` (`dir` empty lists the root; every segment of `dir` escaped), all pages concatenated.
  - `(c *client) diffstat(ctx, key, newHead, oldHead string) ([]diffEntry, error)` on `/diffstat/{new}..{old}?topic=false&renames=false&pagelen=100`.
  - `(c *client) raw(ctx, key, commit, path string) ([]byte, error)` on `/src/{commit}/{path}` through `c.raw` with no `Accept` header, read through `readCapped(body, c.rawLimit)`; a 3xx returns `errLFSRedirect`; other statuses map through `apiError`.
  - `repoPath(key string) string` returns `/repositories/{ws}/{slug}` with both parts escaped; `escapePath(p string) string` escapes each `/`-separated segment.

- [ ] **Step 1: Write the failing tests**

Helpers: `allowLocalServer(t)` (copy of `github/client_test.go`'s `allowLocalGitHubServer`), `newTestClient(t, server, username string) *client` calling `newClient(server.URL, username, "per-source-token")`. Tests:

```go
func TestNewClientRejectsEmptyToken(t *testing.T) // newClient(apiBaseURL, "", "") -> ErrInvalidConfig

func TestAuthHeaderIsBasicWithUsernameAndBearerWithout(t *testing.T) {
	// username "me@example.com" -> Authorization "Basic " + base64("me@example.com:per-source-token")
	// username "" -> "Bearer per-source-token"; both requests carry Accept "application/json"
}

func TestAPIErrorMapsToSentinels(t *testing.T) // 401 -> ErrInvalidCredentials; 403 -> ErrPermissionDenied; 404 -> ErrResourceNotFound; 555 -> errTimedOut; 500 -> none of them

func TestRateLimitWaitsOnceForAShortReset(t *testing.T) {
	// first /user answers 429 with x-ratelimit-reset "1", second answers 200: ping succeeds and the server saw two requests
	// a third 429 on the same client (c.waited already true) returns errRateLimited and not ErrPermissionDenied
}

func TestRateLimitBeyondTheWaitEndsTheRun(t *testing.T) // 429 with x-ratelimit-reset "3600": one request, errors.Is(err, errRateLimited), message contains "1h0m0s"; no reset header -> message contains "unknown time"

func TestPagesFollowNextOnly(t *testing.T) // three pages, each `next` an absolute URL on the server; asserts 3 requests in order and the concatenated values

func TestPagesRejectRepeatedOffHostAndEndlessNext(t *testing.T) {
	// a `next` equal to the current URL -> error containing "repeated"
	// a `next` on "https://evil.example" -> error containing "outside the API base URL" and no request to it
	// a `next` that always advances -> error after maxPaginationHops (set the hop count low through a test-only var? no: serve "?page=N" and assert the error message names maxPaginationHops, as gitlab/pagination_guard_test.go does)
}

func TestHeadCommitResolvesBranchThenTagThenHash(t *testing.T) {
	// "release/1.0" -> GET /refs/branches/release%2F1.0 answers {"target":{"hash":"b1"}} -> "b1"
	// "v1" -> branches 404, tags 200 {"target":{"hash":"t1"}} -> "t1"
	// a 40-hex value -> branches 404, tags 404, GET /commit/<hash> {"hash": <hash>} -> <hash>
	// "nope" -> branches 404, tags 404, no commit request, errors.Is(err, datasource.ErrResourceNotFound)
}

func TestListingSendsDepthFieldsAndEscapesSegments(t *testing.T) // listing(key, "c1", "docs/a b", 50) requests "/src/c1/docs/a%20b/?max_depth=50&pagelen=100&fields=values.path,values.type,values.size,values.attributes,next"; a root listing requests "/src/c1/?..."

func TestDiffstatSendsNewThenOld(t *testing.T) // diffstat(key, "new", "old") requests "/diffstat/new..old?topic=false&renames=false&pagelen=100" and decodes status, old.path and new.path with nil old on "added"

func TestRawDoesNotFollowRedirects(t *testing.T) {
	// /src/c1/big.bin answers 301 Location "https://media.example/x": raw returns errLFSRedirect and the fake saw exactly one request
	// a 9-byte body with c.rawLimit = 8 -> error containing "exceeds maximum size"
	// the request has no Accept header and carries the auth header
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/bitbucket/`
Expected: build failure, undefined `newClient`.

- [ ] **Step 3: Implement `client.go`**

Copy `readCapped` and the `apiError` shape from `github/client.go`; the rate-limit wait replaces GitHub's `shortRetryAfter` (Bitbucket sends no `Retry-After`, only `x-ratelimit-reset` in seconds).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/bitbucket/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/bitbucket/client.go internal/datasource/connector/bitbucket/client_test.go
git commit -m "feat(datasource/bitbucket): add the Bitbucket Cloud REST client"
```

---

### Task 3: Settings parser

**Files:**

- Create: `internal/datasource/connector/bitbucket/types.go`
- Create: `internal/datasource/connector/bitbucket/types_test.go`

**Interfaces:**

- Consumes: `repofile.NormalizePath`, `repofile.CollapsePaths`.
- Produces: `type repositorySelection struct{ Repository, Ref string; Paths []string }` (JSON `repository`, `ref`, `paths`); `type config struct{ Repositories []repositorySelection }`; `parseConfig(ds *types.DataSourceConfig) (*config, error)` reading `ds.Settings["repositories"]`; `normalizeRepository(value string) (string, error)`: a copy of `github/types.go` with the error texts `repository must be workspace/slug, not a URL` and `repository must be workspace/slug`.

- [ ] **Step 1: Write the failing tests**

```go
func TestParseConfigNormalisesRepository(t *testing.T) {
	for in, want := range map[string]string{"Atlassian/AUI": "atlassian/aui", "/ws/repo/": "ws/repo", "ws/repo.git": "ws/repo"} {
		cfg, err := parseConfig(settings(in))
		require.NoError(t, err, in)
		require.Equal(t, want, cfg.Repositories[0].Repository)
	}
	for _, in := range []string{"https://bitbucket.org/ws/repo", "ws", "ws/repo/extra", "{uuid}/repo", ""} {
		_, err := parseConfig(settings(in))
		require.ErrorIs(t, err, datasource.ErrInvalidConfig, in)
	}
}

func TestParseConfigCollapsesDirectories(t *testing.T) // paths ["/docs/", "docs/api", "src"] -> ["docs", "src"]

func TestParseConfigRejectsDuplicateRepositoryIgnoringCase(t *testing.T) // "ws/repo" and "WS/Repo" -> ErrInvalidConfig

func TestParseConfigRequiresRepositories(t *testing.T) // missing key and empty array -> ErrInvalidConfig
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/bitbucket/ -run ParseConfig`
Expected: build failure, undefined `parseConfig`.

- [ ] **Step 3: Implement `types.go`**

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/bitbucket/ -run ParseConfig`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/bitbucket/types.go internal/datasource/connector/bitbucket/types_test.go
git commit -m "feat(datasource/bitbucket): parse repository selections"
```

---

### Task 4: Connector with listing-and-diffstat sync

**Files:**

- Create: `internal/datasource/connector/bitbucket/listing.go`
- Create: `internal/datasource/connector/bitbucket/connector.go`
- Create: `internal/datasource/connector/bitbucket/listing_test.go`
- Create: `internal/datasource/connector/bitbucket/connector_test.go`
- Modify: `internal/types/datasource.go:32` (add `ConnectorTypeBitbucket = "bitbucket"` after `ConnectorTypeGitHub`; the connector's `Type()` needs it, and nothing else reads it until Task 5)

**Interfaces:**

- Consumes: Task 2 client, Task 3 `parseConfig`, `repofile.InScope`, `repofile.KnowledgeRelativePath`, `ds.Importable`, `ds.Imports`, `old.RecordedImports()`, `datasource.StreamHandler`, `types.SyncCursor.CompleteListing`.
- Produces in `listing.go`:
  - `type files map[string]int64` (in-scope importable regular file path to size).
  - `var errMissingCommit = errors.New("bitbucket commit is gone")`; `var errDepthExceeded = errors.New("bitbucket listing reached its depth limit")`.
  - `(c *Connector) files(ctx, key, commit string, roots []string) (files, error)`: one `client.listing(key, commit, root, listingDepth)` per root (`roots` empty means one listing of `""`). Keeps `commit_file` entries that are `InScope(roots)`, pass `c.importable` and have none of the attributes `link`, `subrepository`, `lfs`. A `commit_directory` whose depth below the listed directory equals the listing's `max_depth` returns `errDepthExceeded`. A 404 on a non-empty root: when `client.commitExists(key, commit)` is false return `errMissingCommit`, else the root is empty; a 404 on the root listing is `errMissingCommit`. On `errTimedOut` the root is rebuilt by `(c *Connector) walk(ctx, key, commit, dir string, roots []string) (files, error)`: `client.listing(dir, fallbackListingDepth)`, then recursion into each `commit_directory` at exactly the depth limit that `leadsToRoot(path, roots)` (copied from `github/snapshot.go:104`) or is inside a root; an `errTimedOut` inside `walk` is returned (a single-directory timeout fails the sync).
  - `type change struct{ Path string; Deleted bool }`.
  - `(c *Connector) changeList(ctx, key string, base repositoryCursor, head, ref string, roots []string, next files) (list []change, baselineGone bool, err error)` per spec decision 1, in this order: tombstones sorted by path, then items sorted by path.
    - `base.Head == ""`: items are every path of `next`.
    - `base.Ref != ref`: tombstones are every path of `c.files(key, base.Head, base.Paths)`; items are every path of `next`. `errMissingCommit` on that listing logs at Info, yields no tombstones and `baselineGone` true.
    - same ref: `client.diffstat(key, head, base.Head)`. On `errTimedOut` the change set is unknown: items are every path of `next`, tombstones are the base-listing paths absent from `next` (an `errMissingCommit` there yields none and `baselineGone` true); on `ErrResourceNotFound` the same with `baselineGone` true and no base listing. Otherwise tombstones are the `removed` entries' `old.path` that are `InScope(base.Paths)`, pass `c.importable` and are absent from `next`; items are the `added` and `modified` entries' `new.path` present in `next`. When `!slices.Equal(base.Paths, roots)`: the base listing `c.files(key, base.Head, base.Paths)` is taken (an `errMissingCommit` falls back to the unknown change set above), its paths not `InScope(roots)` join the tombstones, and the `next` paths not `InScope(base.Paths)` join the items. Both groups deduplicated.
- Produces in `connector.go`:
  - `NewConnector() *Connector`; `Type()` returns `types.ConnectorTypeBitbucket`; `var _ datasource.StreamingConnector = (*Connector)(nil)`; `configured(ds)` reads `credentials.username` and `credentials.access_token` and calls `newClient(apiBaseURL, username, token)`; `Connector` fields `client *client`, `imports utils.ImportCapabilities`, `importable func(string) bool`.
  - `Validate`: `configured`; when `ds.Settings["repositories"]` exists, `parseConfig` then `client.repository(cfg.Repositories[0].Repository)`; otherwise `client.ping`.
  - `ListResources`: root is every repository of every `client.workspaces()` entry as `{ExternalID: strings.ToLower(FullName), Name: FullName, Type: "repository", URL: Links.HTML.Href, HasChildren: true}`; a parent `key[:dir]` resolves `repository` then `headCommit(key, MainBranch.Name)` (a nil `MainBranch` lists nothing) then `client.listing(key, hash, dir, 1)` keeping `commit_directory` entries as `{ExternalID: key + ":" + path, Name: path.Base, Type: "directory", ParentID: parent, HasChildren: true}`.
  - `ResolveResourceAncestors` returns `[]string{}`; `FetchAll` and `FetchIncremental` collect `FetchStream` as GitHub's do.
  - `type repositoryCursor struct{ Head, Ref string; Paths []string; Pending *pendingCursor }` (JSON `head`, `ref`, `paths`, `pending,omitempty`); `type pendingCursor struct{ Head, Ref string; Paths []string; Done int }` (JSON `head`, `ref`, `paths`, `done`); `type cursor struct{ Repositories map[string]repositoryCursor }`; `readCursor(old *types.SyncCursor) cursor` and `bitbucketCursor(value cursor, complete bool) *types.SyncCursor` as GitHub's (`{"repositories": ..., "raw": <json>}`, `LastSyncTime` now).
  - `const checkpointEvery = 50`; `const reasonFileTooLarge = "bitbucket_file_too_large"`, `reasonLFSUnsupported = "bitbucket_lfs_unsupported"`; `var SyncReasonTexts = map[string]string{reasonFileTooLarge: "The Bitbucket file exceeds the 100 MB import cap", reasonLFSUnsupported: "The Bitbucket file is stored in Git LFS, which is not imported"}`.
  - `externalID(key, ref, file)` is `"bitbucket:" + c.client.baseURL + ":" + key + ":" + ref + ":" + file`; `item`, `deleted` and `errorItem` per the spec's Data shapes (`FileName` is `repofile.KnowledgeRelativePath(repo.slug(), ref, file)`; `bitbucket_url` is `Links.HTML.Href + "/src/" + ref + "/" + file`); the zero-byte body rule and its comment from `github/connector.go:item`.

Algorithm for `FetchStream` (the one block the signatures do not determine). `complete := old == nil`; `next := readCursor(old)`; when `old != nil && old.RecordedImports() != c.imports`, every `Pending` is dropped with an Info log (its `Done` indexes a list computed under another filter). For each selection, `streamRepository`:

1. Resolve `repo`, `ref` (empty means `repo.MainBranch.Name`; a nil `MainBranch` means the repository lists as empty: `head` is `""` and the listing is empty), `head := headCommit(key, ref)`; `rc := next.Repositories[key]`.
2. Drain the pending (spec decision 2). If `rc.Pending != nil` (`p`): `pendingFiles, err := c.files(key, p.Head, p.Paths)`; on `errMissingCommit` log at Warn, set `rc.Pending = nil` and go to step 3. Otherwise `list, gone, err := c.changeList(key, rc, p.Head, p.Ref, p.Paths, pendingFiles)`; `start := p.Done`, or `0` when `gone`. Emit `list[start:]` through `emit` (below) with checkpoints that write `rc.Pending = &pendingCursor{p.Head, p.Ref, p.Paths, n}`. When done, `rc = repositoryCursor{Head: p.Head, Ref: p.Ref, Paths: p.Paths}`, store and checkpoint.
3. Fast path: `rc.Head == head && rc.Ref == ref && slices.Equal(rc.Paths, roots)`: store `rc` and checkpoint, return.
4. `nextFiles := c.files(key, head, roots)`; `list, _, err := c.changeList(key, rc, head, ref, roots, nextFiles)`; emit all of it with checkpoints writing `rc.Pending = &pendingCursor{head, ref, roots, n}`; when done `rc = repositoryCursor{Head: head, Ref: ref, Paths: roots}`, store and checkpoint.

`emit(ctx, repo, key, ref, prevRef string, list []change, sizes files, start int, h, checkpoint func(done int) error) error`: for `i := start; i < len(list)`: a deletion is `c.deleted(key, prevRef, path)` where `prevRef` is the cursor's recorded ref for the current-selection list and `p.Ref` for a pending drain (both equal `ref` when the ref did not change); `sizes[path] > c.client.rawLimit` is `errorItem(reasonFileTooLarge)` with no request; otherwise `c.item(...)`, where `client.raw` returning `errLFSRedirect` becomes `errorItem(reasonLFSUnsupported)` and `readCapped`'s overrun becomes `errorItem(reasonFileTooLarge)`. After every `checkpointEvery` emitted entries call `checkpoint(i + 1)`.

Removed repositories (spec decision 9), after the selections, in sorted key order: `baseFiles, err := c.files(key, rc.Head, rc.Paths)` (skipped when `rc.Head == ""`); tombstone every path under `rc.Ref`; when `rc.Pending != nil`, `c.files(key, p.Head, p.Paths)` and tombstone every path under `p.Ref`, skipping paths already tombstoned when `p.Ref == rc.Ref`; delete the entry and checkpoint. An error that `Is` `ErrPermissionDenied`, `ErrResourceNotFound` or `errMissingCommit` on any of these calls drops the entry with a Warn log and checkpoints; any other error is returned.

- [ ] **Step 1: Write the failing tests**

A test server `fakeBitbucket(t, repo fakeRepo)` serving `/user`, `/user/workspaces`, `/repositories/{ws}`, `/repositories/ws/r`, `/repositories/ws/r/refs/branches/{name}`, `/refs/tags/{name}`, `/commit/{hash}`, `/src/{commit}/{dir}/` (paged by `pagelen`, honouring `max_depth` breadth-first, 404 for a missing directory or commit, 555 when the commit is in `repo.timeout`), `/src/{commit}/{path}` (raw; 301 for paths in `repo.lfs`), `/diffstat/{new}..{old}` computed from the two commits' file maps in the `new..old` direction (`removed` for a path only at `old`, `added` for a path only at `new`, `modified` for a changed body; 555 when either commit is in `repo.timeout`; 404 when either is unknown), recording request paths. `fakeRepo{ commits map[string]map[string]string; attributes map[string]map[string][]string; sizes map[string]map[string]int64; refs map[string]string; tags map[string]string; timeout map[string]bool; lfs map[string]bool; mainBranch string; status map[string]int }`. A `streamRecorder` copied from `github/connector_test.go` with `failAt`. `base := server.URL`.

```go
func TestValidateChecksTheFirstRepositoryOrTheUser(t *testing.T) // with a selection: one request to /repositories/ws/r and none to /user; without: /user only; 401 -> ErrInvalidCredentials

func TestListResourcesListsWorkspacesRepositoriesThenDirectories(t *testing.T) {
	// root -> every repository of workspaces ["ws", "other"], ExternalID lowercased, URL links.html.href
	// parent "ws/r" -> the branch is resolved through /refs/branches/main first, then "/src/<hash>/?max_depth=1&..."; directories only
}

func TestFetchStreamFullSyncEmitsImportableRegularFilesInScope(t *testing.T) {
	// head: docs/a.md, docs/b.go, docs/link.md (attributes ["link"]), docs/big.bin (attributes ["lfs"]), vendor/sub (attributes ["subrepository"]), src/c.md, README.md; roots ["docs"]
	// expects exactly one item: ExternalID "bitbucket:"+base+":ws/r:main:docs/a.md", Title "ws/R/docs/a.md", FileName "R-main/docs/a.md",
	// SourceResourceID "ws/r", Metadata bitbucket_url "https://bitbucket.org/ws/R/src/main/docs/a.md", bitbucket_repository_uuid "{u1}";
	// CompleteListing true; cursor {"ws/r": {head, ref "main", paths ["docs"]}} with no pending; no raw request for the skipped entries
}

func TestFetchStreamIncrementalUsesDiffstatDirection(t *testing.T) {
	// cursor head c1 (docs/a.md "v1", docs/b.md); head c2 (docs/a.md "v2", docs/c.md)
	// expects emits in order: tombstone docs/b.md, item docs/a.md, item docs/c.md; the diffstat request path is "/diffstat/c2..c1"; CompleteListing false
}

func TestFetchStreamUnchangedHeadEmitsNothing(t *testing.T) // one Checkpoint, no Emit, no listing request

func TestFetchStreamRefChangeTombstonesOldRefAndEmitsNewRoots(t *testing.T) // cursor ref "main"; selection ref "release": tombstones with ":main:" for every indexed docs file, then items with ":release:"; no diffstat request

func TestFetchStreamPathsChangeTombstonesLostRootAndEmitsGainedRoot(t *testing.T) {
	// same head, paths ["docs"] -> ["guides"]: tombstones for every docs file, items for every guides file, one diffstat request answering empty
	// paths ["docs"] -> ["docs", "guides"]: items for guides only, no tombstones
}

func TestFetchStreamUnknownChangeSetAfterDiffstatTimeout(t *testing.T) // diffstat 555: every in-scope file of the new head is emitted and the files only at the old head are tombstoned; diffstat 404: the same emits, no tombstones, cursor advanced

func TestListingFailsOnDirectoryAtDepthLimit(t *testing.T) // a commit_directory at depth 50 below the root -> FetchStream returns errDepthExceeded and nil cursor, no Emit

func TestListingFallsBackToPerDirectoryWalkOnTimeout(t *testing.T) // the depth-50 listing answers 555; depth-10 listings succeed; expects docs/a.md and docs/sub/b.md, requests for "/src/c1/?max_depth=10", "/src/c1/docs/?max_depth=10", and none for "src/"; a 555 on "docs/sub" fails the sync

func TestMissingRootIsEmptyAndMissingCommitIsGone(t *testing.T) {
	// root "docs" 404 but /commit/c1 200: no items, no error, cursor advanced
	// cursor head c0 unknown to the fake: no tombstones, every new file emitted, Info log path (cursor advanced)
}

func TestFetchStreamResumesInsideRepositoryFromPendingCursor(t *testing.T) // 120 items; failAt 101 -> error, last checkpoint pending {head, ref, paths, done 100}; resume emits items 101..120 only, final cursor has no pending

func TestFetchStreamDrainsPendingBeforeMovedHead(t *testing.T) {
	// cursor {head c1, pending {head c2, done 50}} where c2 adds docs/x.md and docs/y.md after 50 unchanged files; head is now c3 which drops docs/x.md
	// expects: the remaining c2 entries emitted first (docs/x.md and docs/y.md among them), a checkpoint whose cursor is {head c2, no pending}, then tombstone docs/x.md, and no tombstone for docs/y.md
}

func TestFetchStreamPendingWithGoneBaselineRestartsAtZero(t *testing.T) // cursor {head c0 (unknown), pending {head c2, done 50}}: every c2 file is emitted, starting from the first

func TestFetchStreamGonePendingHeadIsDropped(t *testing.T) // pending head unknown: Warn, pending dropped, decision 1 against the recorded baseline

func TestPendingIsDroppedWhenImportCapabilitiesChange(t *testing.T) // old cursor recorded other imports: the pending is dropped and the current head is diffed against the baseline (copy the GitHub test's setup)

func TestRemovedRepositoryIsTombstonedOrDropped(t *testing.T) {
	// "ws/gone" in the cursor, not in the selection, listing readable: a tombstone per recorded file under the recorded ref, then the pending listing's extra paths under the pending ref, entry dropped
	// 404 on the repository, 403, or a gone recorded commit: entry dropped with no tombstones; 500 or 555: FetchStream returns the error and the entry stays
}

func TestOversizedAndLFSFilesBecomeErrorItems(t *testing.T) {
	// size 101<<20 from the listing -> error_reason_code "bitbucket_file_too_large", no raw request
	// raw answers 301 -> "bitbucket_lfs_unsupported", one raw request, nothing followed
}

func TestRateLimitEndsTheRunAfterACheckpoint(t *testing.T) // raw for the 51st file answers 429 with reset 3600: FetchStream returns errRateLimited, the last checkpoint holds pending done 50

func TestEmptyFileSyncsItsPath(t *testing.T) // zero-byte body -> Content "# ws/R/docs/empty.md\n"

func TestPathsAreEscapedInSrcURLs(t *testing.T) // "docs/a b#c.md" -> request path "/src/c1/docs/a%20b%23c.md"

func TestRawRedirectBecomesLFSErrorItem(t *testing.T) // listed without the lfs attribute, raw 301: the item is the LFS error item and the run continues to the next file
```

`listing_test.go` holds the pure helpers: `changeList` ordering on a hand-built fake (tombstones sorted, then items sorted; the same ref with equal paths sends one diffstat and no base listing) and `leadsToRoot`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/bitbucket/`
Expected: build failure, undefined `NewConnector`.

- [ ] **Step 3: Implement `listing.go` and `connector.go`, and add the type constant**

Write spec decision 1 as the package comment on `files` and `changeList`, and decision 2 as the comment on `pendingCursor` (why there is no chain). Keep GitHub's comments where the reasoning is the same (empty file, unset `UpdatedAt`, streaming memory bound, dropped pending on a capability change).

- [ ] **Step 4: Run the package tests with the race detector**

Run: `GOTOOLCHAIN=go1.26.10 go test -race ./internal/datasource/connector/bitbucket/ && GOTOOLCHAIN=go1.26.10 go vet ./internal/datasource/connector/bitbucket/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/bitbucket internal/types/datasource.go
git commit -m "feat(datasource/bitbucket): sync repository files from listings and diffstats"
```

---

### Task 5: Register the type, enable it by default and wire the service

**Files:**

- Modify: `internal/datasource/connector.go:295-302` (metadata entry after GitHub's), `:406-408` (`DefaultConnectorTypes`)
- Modify: `internal/datasource/connector_test.go:19,26,62,84` (the `reconciles` map, the tracks-source exclusion, the default list `[notion confluence github bitbucket]`, `TestConnectorTypeEnabled`)
- Modify: `internal/container/container.go` (import `bitbucketConnector` and `registry.Register(bitbucketConnector.NewConnector())` after the GitHub line, same `errors.Join` message shape)
- Modify: `internal/application/service/knowledge_create.go:1157` (`usesSourceIdentityDuplicateCheck` case)
- Modify: `internal/application/service/knowledge_create_gitlab_test.go:84-117` (a Bitbucket sibling)
- Modify: `internal/application/repository/knowledge_duplicate_test.go:92` (a row)
- Modify: `internal/datasource/syncreason/parity_test.go:27`
- Modify: `frontend/src/i18n/locales/en-US.ts:7048` (`datasource.syncError` keys)
- Modify: `internal/datasource/README.md:7`, `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md:14`

**Interfaces:**

- Consumes: `bitbucket.NewConnector`, `bitbucket.SyncReasonTexts`, `types.ConnectorTypeBitbucket`.
- Produces: metadata entry `{Type: bitbucket, Name: "Bitbucket", Description: "Sync files from Bitbucket Cloud repositories", Priority: 4, AuthType: "token", Capabilities: {"incremental", "hierarchical", "deletion_sync", CapabilityFullSyncReconcile}}`; `DefaultConnectorTypes` ends `..., types.ConnectorTypeGitHub, types.ConnectorTypeBitbucket`; i18n `datasource.syncError.bitbucket_file_too_large: 'The Bitbucket file exceeds the 100 MB import cap.'` and `datasource.syncError.bitbucket_lfs_unsupported: 'The Bitbucket file is stored in Git LFS; AI KB imports repository files only.'`.

- [ ] **Step 1: Write the failing tests**

```go
func TestCreateKnowledgeFromFileBitbucketUsesSourceIdentity(t *testing.T) // copy of the GitHub test with types.ConnectorTypeBitbucket and external_id "bitbucket:https://api.bitbucket.org/2.0:ws/r:main:" + file
```

Row in `TestCheckKnowledgeExists_FileSourceIdentity`: `{"bitbucket item of another source", "ds-4", "bitbucket:https://api.bitbucket.org/2.0:ws/r:main:README.md", "same-hash", "md", false}`. Add `bitbucket.SyncReasonTexts` to the parity list. Change the default-list assertions in `connector_test.go` to include `"bitbucket"` and add it to the `reconciles` map and the tracks-source exclusion.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/application/service/ ./internal/datasource/ ./internal/datasource/syncreason/ -run 'Bitbucket|Parity|ConnectorType|Default'`
Expected: FAIL (two sources collapse into one row; the parity test names the missing keys; the default list lacks bitbucket).

- [ ] **Step 3: Make the edits**

README: add Bitbucket to the implemented list. Guide: "Eleven connector packages plus the shared `repofile` helper package", the package list with `bitbucket`, "fourteen types" with `bitbucket` in the list.

- [ ] **Step 4: Run the tests to verify they pass, then build the server**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/application/service/ ./internal/application/repository/ ./internal/datasource/... && GOTOOLCHAIN=go1.26.10 go build ./cmd/server`
Expected: PASS, build succeeds.

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector.go internal/datasource/connector_test.go internal/container/container.go internal/application/service internal/application/repository/knowledge_duplicate_test.go internal/datasource/syncreason/parity_test.go frontend/src/i18n/locales/en-US.ts internal/datasource/README.md internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md
git commit -m "feat(datasource): register the Bitbucket connector and offer it by default"
```

---

### Task 6: Frontend form, labels and icon

**Files:**

- Create: `frontend/src/views/knowledge/settings/gitConnectorKeys.ts`
- Create: `frontend/src/views/knowledge/settings/gitConnectorKeys.test.ts`
- Modify: `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue:269,286-288,749-756` (the predicate and the three computed keys read the table; a `bitbucket` entry in `connectorDefs` after `github`)
- Modify: `frontend/src/views/knowledge/settings/DataSourceEditorDialog.test.ts`
- Modify: `frontend/src/i18n/localeKeyAudit.test.ts:225`
- Modify: `frontend/src/views/knowledge/settings/datasourceIcons.ts:2,25`
- Create: `frontend/src/assets/img/datasource-bitbucket.svg`; Modify: `frontend/src/assets/img/ICONS.md` (a row)
- Modify: `frontend/src/views/knowledge/components/DocumentListView.vue:106`, `DocumentCardView.vue:157`, `frontend/src/views/knowledge/KnowledgeBase.vue:693`, `frontend/src/views/chat/components/tool-results/DocumentInfo.vue:112`, `frontend/src/components/doc-content.vue:989`
- Modify: `frontend/src/i18n/locales/en-US.ts:803,6772-6781,6868,6883`

**Interfaces:**

- `gitConnectorKeys.ts` exports `type GitConnectorType = 'gitlab' | 'github' | 'bitbucket'`, `const GIT_CONNECTOR_KEYS: Record<GitConnectorType, { settings: string; item: string; labels: string }>` with `gitlab: { settings: 'projects', item: 'project_id', labels: 'datasource.gitlab' }`, `github: { settings: 'repositories', item: 'repository', labels: 'datasource.github' }`, `bitbucket: { settings: 'repositories', item: 'repository', labels: 'datasource.bitbucket' }`, and `isGitConnector(type: string): type is GitConnectorType`. The dialog's `gitSettingsKey`, `gitItemKey` and `gitLabelsKey` become `computed(() => GIT_CONNECTOR_KEYS[form.value.type as GitConnectorType]?.settings)` and so on; `selectType` for a git connector still deletes both `settings.projects` and `settings.repositories`.
- `connectorDefs` entry: `{ type: 'bitbucket', available: true, docUrl: 'https://support.atlassian.com/bitbucket-cloud/docs/api-tokens/', permissionDocUrl: '', permissionPageUrl: '', requiredPermissions: [], fields: [ { key: 'username', labelKey: 'datasource.bitbucket.username', placeholder: 'you@example.com', optional: true, hintKey: 'datasource.bitbucket.usernameHint' }, { key: 'access_token', labelKey: 'datasource.bitbucket.accessToken', placeholder: '', secret: true, hintKey: 'datasource.bitbucket.accessTokenHint' } ] }` (confirm `hintKey` renders on a secret field in the template; if not, show the hint under the username field only and fold the scope sentence into `usernameHint`).
- i18n: `knowledgeBase.channelBitbucket: 'Bitbucket'`; `datasource.connector.bitbucket: 'Bitbucket'`; `datasource.connectorDesc.bitbucket: 'Sync files from Bitbucket Cloud repositories'`; group `datasource.bitbucket`: `username: 'Atlassian account email'`, `usernameHint: 'Fill in the email for an API token; leave empty for a workspace, project or repository access token.'`, `accessToken: 'API token or access token'`, `accessTokenHint: 'An API token needs the read:repository:bitbucket scope, plus read:workspace:bitbucket to browse repositories.'`, `projects: 'Bitbucket repositories'`, `projectsHint: 'Enter a repository as workspace/repository, with optional branch and directories.'`, `project: 'Repository'`, `projectId: 'Repository'`, `projectIdPlaceholder: 'For example: workspace/repository'`, `ref: 'Branch'`, `refPlaceholder: 'Leave empty to use the main branch'`, `paths: 'Directories'`, `pathsPlaceholder: 'One directory per line; leave empty to sync the whole repository'`, `addProject: 'Add repository'`, `projectRequired: 'Add at least one Bitbucket repository'`.
- Icon: the isolated Bitbucket logomark (blue tile, SVG) from `https://atlassian.design/resources/logo-library`, saved unmodified; `ICONS.md` row: source URL and "Atlassian trademark guidelines: product logos from an official Atlassian site, reproduced without modification, for a related product". `datasourceIconMap.bitbucket = bitbucketIcon`; no dark-mode rule.
- Channel maps: `bitbucket` next to `github` in each of the five files, label `knowledgeBase.channelBitbucket`.

- [ ] **Step 1: Write the failing tests**

`gitConnectorKeys.test.ts`:

```ts
test('the git connector table covers every git type', () => {
  assert.deepEqual(Object.keys(GIT_CONNECTOR_KEYS).sort(), ['bitbucket', 'github', 'gitlab'])
  assert.equal(isGitConnector('bitbucket'), true)
  assert.equal(isGitConnector('seafile'), false)
  assert.deepEqual(GIT_CONNECTOR_KEYS.bitbucket, { settings: 'repositories', item: 'repository', labels: 'datasource.bitbucket' })
})
```

In `DataSourceEditorDialog.test.ts`, next to the GitHub tests:

```ts
test('saved Bitbucket repositories hydrate the shared git form', async () => {
  const f = await fixture({ type: 'bitbucket', settings: { repositories: [{ repository: 'ws/r', ref: 'main', paths: ['docs'] }] } })
  try {
    assert.deepEqual(f.vm.gitProjects, [{ project_id: 'ws/r', ref: 'main', pathsText: 'docs' }])
    f.vm.syncGitProjectsToSettings()
    assert.deepEqual(f.vm.form.config.settings, { repositories: [{ repository: 'ws/r', ref: 'main', paths: ['docs'] }] })
    assert.equal(f.vm.gitLabelsKey, 'datasource.bitbucket')
  } finally { f.close() }
})
```

In `localeKeyAudit.test.ts` after the GitHub assertion:

```ts
test('the Bitbucket form label group matches GitLab plus its own credential keys', () => {
  const en = LOCALE_BUNDLES["en-US"] as any
  const gitlab = Object.keys(en.datasource.gitlab).filter(key => key !== 'baseUrl')
  assert.deepEqual(Object.keys(en.datasource.bitbucket).sort(), [...gitlab, 'username', 'usernameHint', 'accessTokenHint'].sort())
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `npm --prefix frontend test -- src/views/knowledge/settings/gitConnectorKeys.test.ts src/views/knowledge/settings/DataSourceEditorDialog.test.ts src/i18n/localeKeyAudit.test.ts`
Expected: FAIL, module not found and `en.datasource.bitbucket` undefined.

- [ ] **Step 3: Fetch the icon, add the table module, the definition, i18n keys and channel labels**

Download the Bitbucket logomark SVG from the logo library (ask the operator once; name the file, source and size) into an empty scratch directory and copy it unmodified.

- [ ] **Step 4: Run the frontend checks**

Run: `npm --prefix frontend test && npm --prefix frontend run type-check`
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(ui): add the Bitbucket data source form"
```

---

### Task 7: Documentation, configuration, changelog and end-to-end check

**Files:**

- Modify: `website-docs/03-features/10-datasource.md:41,45,55,112-120,147-153,322,437,455,459`
- Modify: `website-docs/01-getting-started/01-introduction.md:50,110`, `website-docs/01-getting-started/04-configuration.md:144`, `website-docs/04-api/02-api-infra.md:309,313`, `website-docs/05-clients/01-frontend.md:194`, `website-docs/06-development/02-database-schema.md:250`, `website-docs/06-development/03-extension-points.md:502`, `README.md:51,233`, `CHANGELOG.md` (Unreleased, `### Changes`)
- Modify: `.env.example:129-134`, `docker-compose.yml:117`, `helm/README.md:507,520`, `.claude/skills/upstream-review/SKILL.md:126,132` (the allow-list default and a `no Bitbucket data source` deviations row)

**Interfaces:**

- Consumes: the JSON shapes from the spec; the registered-type count is now 14; the default allow-list is `notion,confluence,github,bitbucket`.

- [ ] **Step 1: Write the docs**

A level-four `#### Bitbucket (connector/bitbucket/)` heading after the GitHub section in `10-datasource.md` with four paragraphs: credentials (`credentials.access_token` and optional `credentials.username`: an Atlassian account email plus an API token with the `read:repository:bitbucket` scope, plus `read:workspace:bitbucket` to browse repositories in the picker, or a workspace, project or repository access token with no username; Bitbucket Cloud only, no Data Center), the `settings.repositories` shape with the lowercase `workspace/slug` rule and the rename note, sync behaviour (every run lists the selected directories at the new head and, under an unchanged branch, asks Bitbucket for the changed files between the two commits; a changed branch or directory selection, a force-push and a garbage-collected previous commit are handled; progress is checkpointed every 50 files; a run that hits the 1,000 requests per hour limit waits for the reset when it is at most 30 minutes away, otherwise it ends and the next sync resumes, so a first sync of more than about a thousand files spans several runs; a file over 100 MB, a Git LFS file, a symlink and a submodule are not imported, the first two reported per item; a forced full sync removes items the run did not list, except a forced full sync that the hourly limit cut, which never reconciles; a listing deeper than 50 directories fails the sync instead of silently truncating it), and the example `{"credentials":{"username":"you@example.com","access_token":"<token>"},"settings":{"repositories":[{"repository":"workspace/repository","ref":"main","paths":["docs"]}]}}`. Add a Bitbucket row to the connector table, `bitbucket` to every type list, count and default allow-list named above, the reconcile list, the streaming lists, the sequence-diagram label, the registration line, the capability paragraphs at lines 147-153 (Bitbucket behaves as GitHub there) and the deviations row. CHANGELOG under `## [Unreleased]` `### Changes`: an entry starting `- **NEW**: **Bitbucket data source**:` that says: sync document files from the directories of Bitbucket Cloud repositories with an API token or an access token; incremental sync asks Bitbucket for the changed files, so a force-push is handled; progress resumes after the hourly rate limit; files over 100 MB and Git LFS files are reported per item; offered by default next to Notion, Confluence and GitHub (a deployment that sets `AIKB_DATASOURCE_TYPES` must list it); and a `- **CHANGED**:` entry that the GitHub data source icon is now GitHub's own Invertocat.

- [ ] **Step 2: Check for stale claims**

Run: `git grep -n -i 'thirteen types\|13 connector types\|notion,confluence,github\b' -- '*.md' '.env.example' 'docker-compose.yml' internal | grep -v bitbucket; git grep -n 'Notion, Confluence and GitHub' -- '*.md' '.env.example' | grep -v Bitbucket`
Expected: no output. Then run the `docs-drift-reviewer` agent on `git diff main...HEAD`.

- [ ] **Step 3: Run the repository gates**

Run: `scripts/validate.sh --base main`
Expected: every area it lists passes, including the English-only, em dash and version gates.

- [ ] **Step 4: Prove it against the real API**

Start the stack per the dev recipe (`./scripts/dev.sh start --minio`, `SERVER_PORT=8081 AIKB_DATASOURCE_TYPES=all make dev-app`, the embedding model on `:8092`, `make dev-frontend`), create a Bitbucket data source for a workspace repository with a `docs` directory using an API token pasted into the form, and run a sync. Then push a commit that adds one file and removes another and run an incremental sync (the diffstat direction tripwire). Then a rename plus an emptied file, a branch change, a directory-list change, a forced full sync, and the same token as a bare Bearer value (empty username). Expected: the first sync imports the `docs` files; the second removes the deleted file and imports the added one; the third removes the old name, imports the new one, and the emptied file's item holds its path; the branch change tombstones the old branch's items and imports the new branch's; the directory change tombstones the lost root; the forced full sync reconciles; the Bearer form either validates (record it in the docs) or fails with 401 (record that an API token needs the email). Record the sync logs from `GET /api/v1/datasource?kb_id=...` in the PR description. This step needs a token the operator provides; without one, say so in the PR and leave the step open.

- [ ] **Step 5: Commit and open the PR**

```bash
git add website-docs README.md CHANGELOG.md .env.example docker-compose.yml helm/README.md .claude/skills/upstream-review/SKILL.md
git commit -m "docs(datasource): document the Bitbucket connector"
git push -u origin feat/bitbucket-datasource
```

Open the PR against `main` with the spec link, the sync-log evidence from Step 4, and the attribution line `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
