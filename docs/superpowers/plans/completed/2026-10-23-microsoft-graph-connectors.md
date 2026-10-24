# OneDrive and SharePoint Data Source Connectors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `onedrive` and `sharepoint` data source connectors that sync document files from Microsoft 365 over Microsoft Graph with app-only credentials, a folder tree picker, delta-based incremental sync with checkpoints and resume, deletion tombstones and full-sync reconcile, enabled by default next to GitHub and Bitbucket.

**Architecture:** One Go package `internal/datasource/connector/msgraph` exports `NewConnector(kind)` for the two types (the Feishu/Lark shape). A per-run client mints a client-credentials token and caches it in memory. Each selected root (a drive or a folder) is one Graph `delta` stream whose `nextLink` and `deltaLink` live in the cursor, next to a fingerprint map of every known file and folder, so unchanged files are not re-ingested, renames re-emit, folder deletions fan out to their descendants, and a 410 resync still tombstones what vanished. The frontend reuses the lazy tree picker and generalises the Confluence public-field mirroring into a table.

**Tech Stack:** Go 1.26 (`GOTOOLCHAIN=go1.26.10` to match CI), `net/http/httptest` and `testify`; Vue 3 + TypeScript, vue-i18n, node test runner via tsx (`npm --prefix frontend test`), vue-tsc (`npm --prefix frontend run type-check`).

**Spec:** `docs/superpowers/specs/completed/2026-10-23-microsoft-graph-connectors-design.md`

## Global Constraints

- English only, no em dashes anywhere (`scripts/check-english-only.sh`, `scripts/check-emdash.sh`; the pre-commit hook enforces both).
- Commit subjects: `type(scope): imperative subject`, for example `feat(datasource/msgraph): ...`. Every commit message ends with `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`.
- Before each commit: run the built-in `/simplify` and `/code-review` on the diff. golangci-lint runs over the whole root module on commit, so keep unrelated Go files clean. Lines at most 120 characters, gofumpt.
- No UI text outside `frontend/src/i18n/locales/en-US.ts`; product names in copy are "Microsoft OneDrive (work or school)" and "Microsoft SharePoint"; the AI KB product name is "AI KB".
- A doc claim cites code that matches it; after a code change, `git grep -n '<old value>' -- '*.md'` for the old value and run the `docs-drift-reviewer` agent on the diff.
- Hosts are fixed: `https://graph.microsoft.com/v1.0` and `https://login.microsoftonline.com`; no `base_url`, no national clouds. Token scope `https://graph.microsoft.com/.default`. Access token cached until `expires_in` minus 5 minutes, re-minted once on a 401. JSON response cap 16 MiB; file cap `utils.GetMaxFileSize()`; children page size 200; delta `$select=id,name,size,file,folder,root,deleted,parentReference,eTag,lastModifiedDateTime,webUrl,package,remoteItem,malware` (`deltaSelect` in `client.go`); checkpoint after every delta page and every 50 emits; rate-limit wait at most 30 minutes, once per run; pagination guards: repeated link, 10,000 hops, link off the Graph host.
- Credentials are `tenant_id`, `client_id`, `client_secret`; public settings are `user` (OneDrive) and `site_url` (SharePoint), read from `Settings` with a fallback to `Credentials`.
- Resource ids are `<driveId>` or `<driveId>:<itemId>`; external ids are `<driveId>:<itemId>`; `metadata["channel"]` is the connector's own type string.
- The package copies the capped reader, the status-to-sentinel error type, the `collector` and the `allowLocalServer` test helper from `internal/datasource/connector/bitbucket/` (spec decision 12); nothing is extracted into a shared package.
- Tests build `&Connector{kind: ..., graphBaseURL: server.URL, loginBaseURL: server.URL}` against one `httptest` fake Graph; the fake is whitelisted with `allowLocalServer(t)`.
- No Microsoft artwork: the two icons are original tiles recorded in `frontend/src/assets/img/ICONS.md` as AI KB artwork (spec decision 15).
- Work on a branch `feat/microsoft-graph-datasources` cut from `main`; never commit to `main`.

## Review Focus

1. A delta page in which a folder appears after its files: the files must still get the folder's name in their knowledge path, so the page is applied to the maps before any item is emitted (Task 6, `TestFetchStreamFolderAfterChildrenKeepsPath`).
2. A rename of a file whose bytes did not change: the fingerprint must differ (name is part of it) so the item is re-emitted under the new path, while a sharing-only change that re-sends the item with the same fingerprint must not re-emit (Task 6, `TestFetchStreamRenameReemitsAndMetadataChangeDoesNot`).
3. A resumed run must not report a complete listing, and a checkpoint taken mid-page must still point at the page being processed, so the retry re-walks that page and skips what was emitted by fingerprint (Task 6, `TestFetchStreamResumesFromPageLink`).
4. A 410 on a stored link: the enumeration restarts from the `Location` link, every known item under that root that the restart did not list is tombstoned, and nothing is tombstoned before the restart reaches its `deltaLink` (Task 6, `TestFetchStreamResyncTombstonesUnseen`).
5. The `/content` redirect: the bearer token must not travel to the download host, a `Location` that is not `https` is a per-item failure, and a body whose length differs from the listed size is `msgraph_source_changed` (Task 3, `TestDownloadFollowsRedirectWithoutToken`, `TestDownloadRejectsInsecureLocation`, `TestDownloadSizeMismatch`).

---

### Task 1: Type constant, catalog entries, default allow-list and channel labels

**Files:**

- Modify: `internal/types/datasource.go:35` (add `ConnectorTypeSharePoint` after `ConnectorTypeOneDrive`)
- Modify: `internal/datasource/connector.go:332-339` (OneDrive entry), add a SharePoint entry after it, `DefaultConnectorTypes` at line 429
- Modify: `internal/datasource/connector_test.go:32` (capability rows), lines 98-99 and 120 (allow-list expectations)
- Modify: `frontend/src/utils/channelLabel.ts` (two keys after `seafile`)
- Modify: `frontend/src/utils/channelLabel.test.ts:14` (exempt set) and lines 37-39 (order array)
- Modify: `frontend/src/i18n/locales/en-US.ts:805` (two `knowledgeBase.channel*` keys after `channelSeafile`)

**Interfaces:**

- Produces: `types.ConnectorTypeSharePoint = "sharepoint"`; registry entries for both types with capabilities `incremental, hierarchical, deletion_sync, full_sync_reconcile, source_identity_dedup`; `DefaultConnectorTypes` lists both; channel labels `knowledgeBase.channelOneDrive` and `knowledgeBase.channelSharePoint`.

- [ ] **Step 1: Write the failing Go tests**

In `internal/datasource/connector_test.go` change the `want` map rows and the allow-list expectations:

```go
		types.ConnectorTypeOneDrive:    {reconcile: true, sourceIdentityDedup: true},
		types.ConnectorTypeSharePoint:  {reconcile: true, sourceIdentityDedup: true},
```

```go
	if got := enabled(); !slices.Equal(got, []string{"notion", "confluence", "github", "bitbucket", "onedrive", "sharepoint"}) {
		t.Fatalf("blank allow-list enabled %v, want [notion confluence github bitbucket onedrive sharepoint]", got)
	}
```

In `TestConnectorTypeEnabled` extend the condition:

```go
		if !ConnectorTypeEnabled(types.ConnectorTypeNotion) || !ConnectorTypeEnabled(types.ConnectorTypeConfluence) ||
			!ConnectorTypeEnabled(types.ConnectorTypeGitHub) || !ConnectorTypeEnabled(types.ConnectorTypeBitbucket) ||
			!ConnectorTypeEnabled(types.ConnectorTypeOneDrive) || !ConnectorTypeEnabled(types.ConnectorTypeSharePoint) ||
			ConnectorTypeEnabled(types.ConnectorTypeFeishu) || ConnectorTypeEnabled("fake") {
			t.Fatalf("allow-list %q must admit only notion, confluence, github, bitbucket, onedrive and sharepoint", value)
		}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/ -run 'TestCapabilityTableMatchesRegistry|TestListAvailableConnectorsMarksAllowList|TestConnectorTypeEnabled' 2>&1 | head -20`
Expected: FAIL with `undefined: types.ConnectorTypeSharePoint`.

- [ ] **Step 3: Add the constant and the catalog entries**

`internal/types/datasource.go`, after line 35:

```go
	ConnectorTypeOneDrive    = "onedrive"
	ConnectorTypeSharePoint  = "sharepoint"
```

`internal/datasource/connector.go`, replace the OneDrive entry and add SharePoint:

```go
	types.ConnectorTypeOneDrive: {
		Type:        types.ConnectorTypeOneDrive,
		Name:        "OneDrive (work or school)",
		Description: "Sync folders and files from a user's Microsoft OneDrive",
		Priority:    6,
		AuthType:    "client_credentials",
		Capabilities: []string{
			"incremental", "hierarchical", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySourceIdentityDedup,
		},
	},
	types.ConnectorTypeSharePoint: {
		Type:        types.ConnectorTypeSharePoint,
		Name:        "SharePoint",
		Description: "Sync document libraries and folders from a Microsoft SharePoint site",
		Priority:    6,
		AuthType:    "client_credentials",
		Capabilities: []string{
			"incremental", "hierarchical", "deletion_sync", CapabilityFullSyncReconcile, CapabilitySourceIdentityDedup,
		},
	},
```

`DefaultConnectorTypes`:

```go
var DefaultConnectorTypes = []string{
	types.ConnectorTypeNotion, types.ConnectorTypeConfluence, types.ConnectorTypeGitHub, types.ConnectorTypeBitbucket,
	types.ConnectorTypeOneDrive, types.ConnectorTypeSharePoint,
}
```

- [ ] **Step 4: Run the Go tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/ 2>&1 | tail -3`
Expected: `ok`.

- [ ] **Step 5: Write the failing frontend test change**

`frontend/src/utils/channelLabel.test.ts`: remove `'onedrive'` from `TYPES_WITHOUT_A_CHANNEL` (the comment above it becomes "the Lark wiki emits "feishu", and the other three have no connector") and extend the order array:

```ts
    'feishu', 'feishu_drive', 'lark_drive', 'notion', 'yuque', 'confluence', 'gitlab', 'github', 'bitbucket',
    'seafile', 'onedrive', 'sharepoint', 'ima', 'rss', 'wechat', 'wecom', 'dingtalk', 'slack', 'im',
```

Run: `npm --prefix frontend test -- src/utils/channelLabel.test.ts 2>&1 | tail -15`
Expected: FAIL, `onedrive` and `sharepoint` have no label.

- [ ] **Step 6: Add the labels**

`frontend/src/utils/channelLabel.ts`, after the `seafile` line:

```ts
  onedrive: 'knowledgeBase.channelOneDrive',
  sharepoint: 'knowledgeBase.channelSharePoint',
```

`frontend/src/i18n/locales/en-US.ts`, after `channelSeafile: 'Seafile',`:

```ts
    channelOneDrive: 'OneDrive',
    channelSharePoint: 'SharePoint',
```

- [ ] **Step 7: Run the frontend tests and the Go syncreason test**

Run: `npm --prefix frontend test -- src/utils/channelLabel.test.ts && GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/syncreason/`
Expected: both PASS (an `en-US.ts` edit runs the Go parity test; no reason codes changed yet).

- [ ] **Step 8: Commit**

```bash
git add internal/types/datasource.go internal/datasource/connector.go internal/datasource/connector_test.go frontend/src/utils/channelLabel.ts frontend/src/utils/channelLabel.test.ts frontend/src/i18n/locales/en-US.ts
git commit -m "feat(datasource): catalog entries and channel labels for OneDrive and SharePoint"
```

---

### Task 2: Settings parser, resource ids and reason codes

**Files:**

- Create: `internal/datasource/connector/msgraph/types.go`
- Create: `internal/datasource/connector/msgraph/types_test.go`

**Interfaces:**

- Produces: `type Kind string` with `KindOneDrive`, `KindSharePoint` and `(Kind).connectorType() string`; `type config struct { TenantID, ClientID, ClientSecret, User, SiteHost, SitePath string; Roots []root }`; `type root struct { DriveID, ItemID string }` with `(root).id() string`; `parseConfig(kind Kind, ds *types.DataSourceConfig) (*config, error)`; `parseResourceID(id string) (root, error)`; `parseSiteURL(raw string) (host, path string, err error)`; `publicSetting(ds, key) string`; the eight reason constants and `SyncReasonTexts`.

- [ ] **Step 1: Write the failing tests**

```go
package msgraph

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func baseConfig(settings map[string]interface{}, ids ...string) *types.DataSourceConfig {
	return &types.DataSourceConfig{
		Credentials: map[string]interface{}{
			"tenant_id":     "contoso.onmicrosoft.com",
			"client_id":     "app",
			"client_secret": "s3cret",
		},
		Settings:    settings,
		ResourceIDs: ids,
	}
}

func TestParseConfigOneDrive(t *testing.T) {
	cfg, err := parseConfig(
		KindOneDrive,
		baseConfig(map[string]interface{}{"user": "alice@contoso.com"}, "b!abc", "b!abc:01F"),
	)
	require.NoError(t, err)
	require.Equal(t, "alice@contoso.com", cfg.User)
	require.Equal(t, []root{{DriveID: "b!abc"}, {DriveID: "b!abc", ItemID: "01F"}}, cfg.Roots)
}

func TestParseConfigSharePointSiteURL(t *testing.T) {
	cfg, err := parseConfig(
		KindSharePoint,
		baseConfig(map[string]interface{}{"site_url": "https://contoso.sharepoint.com/sites/hr/"}, "b!abc"),
	)
	require.NoError(t, err)
	require.Equal(t, "contoso.sharepoint.com", cfg.SiteHost)
	require.Equal(t, "sites/hr", cfg.SitePath)

	cfg, err = parseConfig(
		KindSharePoint,
		baseConfig(map[string]interface{}{"site_url": "https://contoso.sharepoint.com"}, "b!abc"),
	)
	require.NoError(t, err)
	require.Equal(t, "", cfg.SitePath)
}

func TestParseConfigPublicFieldFallsBackToCredentials(t *testing.T) {
	ds := baseConfig(nil)
	ds.Credentials["site_url"] = "https://contoso.sharepoint.com/sites/hr"
	cfg, err := parseConfig(KindSharePoint, ds)
	require.NoError(t, err)
	require.Equal(t, "sites/hr", cfg.SitePath)
}

func TestParseConfigRejects(t *testing.T) {
	cases := map[string]*types.DataSourceConfig{
		"missing secret": {Credentials: map[string]interface{}{"tenant_id": "t", "client_id": "c"}},
		"blank tenant": {
			Credentials: map[string]interface{}{"tenant_id": " ", "client_id": "c", "client_secret": "s"},
		},
		"bad site url":  baseConfig(map[string]interface{}{"site_url": "contoso.sharepoint.com/sites/hr"}),
		"http site url": baseConfig(map[string]interface{}{"site_url": "http://contoso.sharepoint.com/sites/hr"}),
		"bad root":      baseConfig(map[string]interface{}{"site_url": "https://c.sharepoint.com/sites/hr"}, ":01F"),
	}
	for name, ds := range cases {
		_, err := parseConfig(KindSharePoint, ds)
		require.ErrorIs(t, err, datasource.ErrInvalidConfig, name)
	}
}

func TestParseConfigDropsDuplicateRoots(t *testing.T) {
	cfg, err := parseConfig(KindSharePoint, sharePointConfigIDs("b!abc", "b!abc:01F", "b!abc"))
	require.NoError(t, err)
	require.Equal(t, []root{{DriveID: "b!abc"}, {DriveID: "b!abc", ItemID: "01F"}}, cfg.Roots)
}

func sharePointConfigIDs(ids ...string) *types.DataSourceConfig {
	return baseConfig(map[string]interface{}{"site_url": "https://c.sharepoint.com/sites/hr"}, ids...)
}

func TestParseConfigWithoutSelectionIsAllowed(t *testing.T) {
	// Validate runs before anything is selected; the sync requires roots itself.
	cfg, err := parseConfig(KindOneDrive, baseConfig(map[string]interface{}{"user": "alice@contoso.com"}))
	require.NoError(t, err)
	require.Empty(t, cfg.Roots)
}

func TestKindConnectorTypes(t *testing.T) {
	require.Equal(t, types.ConnectorTypeOneDrive, KindOneDrive.connectorType())
	require.Equal(t, types.ConnectorTypeSharePoint, KindSharePoint.connectorType())
}

func TestRootID(t *testing.T) {
	require.Equal(t, "b!abc", root{DriveID: "b!abc"}.id())
	require.Equal(t, "b!abc:01F", root{DriveID: "b!abc", ItemID: "01F"}.id())
}

func TestSyncReasonTextsCoverEveryCode(t *testing.T) {
	for _, code := range []string{
		reasonFileTooLarge, reasonEmptyFile, reasonSourceChanged, reasonFetchFailed,
		reasonPermissionDenied, reasonNotFound, reasonMalware, reasonInvalidResponse,
	} {
		require.NotEmpty(t, SyncReasonTexts[code], code)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ 2>&1 | head -5`
Expected: build failure, `undefined: parseConfig`.

- [ ] **Step 3: Write `types.go`**

```go
// Package msgraph is the Microsoft OneDrive and SharePoint data source
// connector over the Microsoft Graph API; one package serves both types.
package msgraph

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// Kind selects the connector type a Connector instance registers as.
type Kind string

// The two kinds, one per registered connector type.
const (
	KindOneDrive   Kind = "onedrive"
	KindSharePoint Kind = "sharepoint"
)

func (k Kind) connectorType() string {
	if k == KindSharePoint {
		return types.ConnectorTypeSharePoint
	}
	return types.ConnectorTypeOneDrive
}

// root is one selected drive (ItemID empty) or folder.
type root struct {
	DriveID string
	ItemID  string
}

func (r root) id() string {
	if r.ItemID == "" {
		return r.DriveID
	}
	return r.DriveID + ":" + r.ItemID
}

type config struct {
	TenantID, ClientID, ClientSecret string
	// User is the OneDrive owner (principal name or object id).
	User string
	// SiteHost and SitePath address the SharePoint site: /sites/{host}:/{path}.
	SiteHost, SitePath string
	Roots              []root
}

const (
	settingUser    = "user"
	settingSiteURL = "site_url"
)

// publicSetting reads a non-secret field from settings, or from the
// credentials map where the editor sends it during a connection test.
func publicSetting(ds *types.DataSourceConfig, key string) string {
	if v, ok := ds.Settings[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	v, _ := ds.Credentials[key].(string)
	return strings.TrimSpace(v)
}

func credential(ds *types.DataSourceConfig, key string) (string, error) {
	v, _ := ds.Credentials[key].(string)
	if strings.TrimSpace(v) == "" {
		return "", fmt.Errorf("%w: %s is required", datasource.ErrInvalidConfig, key)
	}
	return strings.TrimSpace(v), nil
}

func parseConfig(kind Kind, ds *types.DataSourceConfig) (*config, error) {
	if ds == nil {
		return nil, datasource.ErrInvalidConfig
	}
	cfg := &config{}
	var err error
	if cfg.TenantID, err = credential(ds, "tenant_id"); err != nil {
		return nil, err
	}
	if cfg.ClientID, err = credential(ds, "client_id"); err != nil {
		return nil, err
	}
	if cfg.ClientSecret, err = credential(ds, "client_secret"); err != nil {
		return nil, err
	}
	switch kind {
	case KindSharePoint:
		if raw := publicSetting(ds, settingSiteURL); raw != "" {
			if cfg.SiteHost, cfg.SitePath, err = parseSiteURL(raw); err != nil {
				return nil, err
			}
		}
	default:
		cfg.User = publicSetting(ds, settingUser)
	}
	seen := map[string]bool{}
	for _, id := range ds.ResourceIDs {
		r, err := parseResourceID(id)
		if err != nil {
			return nil, err
		}
		if seen[r.id()] {
			continue
		}
		seen[r.id()] = true
		cfg.Roots = append(cfg.Roots, r)
	}
	return cfg, nil
}

// parseSiteURL splits https://host/path into the Graph site address parts.
func parseSiteURL(raw string) (host, sitePath string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return "", "", fmt.Errorf("%w: site_url must be an https URL such as https://contoso.sharepoint.com/sites/hr",
			datasource.ErrInvalidConfig)
	}
	return u.Hostname(), strings.Trim(u.Path, "/"), nil
}

// parseResourceID accepts "<driveId>" or "<driveId>:<itemId>"; drive ids
// carry no colon, so the split is at the first one.
func parseResourceID(id string) (root, error) {
	driveID, itemID, _ := strings.Cut(strings.TrimSpace(id), ":")
	if driveID == "" || strings.ContainsAny(driveID+itemID, "/ ") {
		return root{}, fmt.Errorf("%w: invalid resource id %q", datasource.ErrInvalidConfig, id)
	}
	return root{DriveID: driveID, ItemID: itemID}, nil
}

// Error reason codes on error items; the frontend localises them as
// datasource.syncError.<code> and SyncReasonTexts holds the English fallbacks.
const (
	reasonFileTooLarge     = "msgraph_file_too_large"
	reasonEmptyFile        = "msgraph_empty_file"
	reasonSourceChanged    = "msgraph_source_changed"
	reasonFetchFailed      = "msgraph_fetch_failed"
	reasonPermissionDenied = "msgraph_permission_denied"
	reasonNotFound         = "msgraph_not_found"
	reasonMalware          = "msgraph_malware_detected"
	reasonInvalidResponse  = "msgraph_invalid_response"
)

// SyncReasonTexts maps every reason code to its English fallback.
var SyncReasonTexts = map[string]string{
	reasonFileTooLarge:     "The file exceeds the configured size limit",
	reasonEmptyFile:        "The file is empty",
	reasonSourceChanged:    "The file changed while it was being fetched",
	reasonFetchFailed:      "Failed to fetch the file from Microsoft Graph",
	reasonPermissionDenied: "Permission to read the file was denied",
	reasonNotFound:         "The file was not found",
	reasonMalware:          "Microsoft flagged the file as malware; it was not imported",
	reasonInvalidResponse:  "Microsoft Graph returned an unexpected response",
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/msgraph/types.go internal/datasource/connector/msgraph/types_test.go
git commit -m "feat(datasource/msgraph): settings parser, resource ids and reason codes"
```

---

### Task 3: Graph client with token exchange, paging, delta, children and download

**Files:**

- Create: `internal/datasource/connector/msgraph/client.go`
- Create: `internal/datasource/connector/msgraph/fake_graph_test.go` (the fake used by every later task)
- Create: `internal/datasource/connector/msgraph/client_test.go`

**Interfaces:**

- Consumes: `config` from Task 2.
- Produces: `newClient(graphBaseURL, loginBaseURL string, cfg *config) *client`; `(*client).accessToken(ctx) (string, error)`; `(*client).site(ctx, host, path) (*siteInfo, error)`; `(*client).siteDrives(ctx, siteID) ([]driveInfo, error)`; `(*client).userDrive(ctx, user) (*driveInfo, error)`; `(*client).drive(ctx, driveID) (*driveInfo, error)`; `(*client).item(ctx, driveID, itemID) (*driveItem, error)`; `(*client).childFolders(ctx, driveID, itemID) ([]driveItem, error)`; `(*client).deltaStart(driveID, itemID) string` (selects the `root` and `malware` facets the sync branches on); `(*client).deltaPage(ctx, link) (*deltaPage, error)` returning `*resyncError{Location}` on 410 (its message never includes the link); `(*client).download(ctx, driveID, itemID string, expectedSize, limit int64) ([]byte, error)` returning `errEmptyFile`, `errSourceChanged`, `errTooLarge`, `errInsecureDownload` or `*apiError`; `*apiError` with `Is` mapping 401, 403, 404; `*rateLimitError` with `Is(errRateLimited)`; the `driveItem` struct with `ID, Name, Size, ETag, WebURL, LastModified, File (MimeType, Hashes), Folder, Deleted, Package, RemoteItem, Malware, ParentReference (DriveID, ID)` and `(driveItem).isFile() bool`; `driveItem.Root` marks the drive root item. A 429 or 503 without `Retry-After` backs off 2, 4 and 8 seconds, then surfaces; error strings carry the request path only, never a query string or link.

- [ ] **Step 1: Write the fake Graph server**

`fake_graph_test.go`:

```go
package msgraph

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

func allowLocalServer(t *testing.T) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1,::1,localhost")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
}

// fakeGraph serves the token endpoint and a map of Graph paths. Handlers are
// keyed by "METHOD path" without the query string; "GET /drives/b!abc/root/delta"
// and the download host live on the same server.
type fakeGraph struct {
	t        *testing.T
	server   *httptest.Server
	mu       sync.Mutex
	handlers map[string]http.HandlerFunc
	tokens   atomic.Int32
	requests []string
	// token is what the token endpoint hands out; tests change it to check re-minting.
	token string
	// tokenStatus lets a test fail the token endpoint.
	tokenStatus int
}

func newFakeGraph(t *testing.T) *fakeGraph {
	t.Helper()
	allowLocalServer(t)
	f := &fakeGraph{t: t, handlers: map[string]http.HandlerFunc{}, token: "tok-1", tokenStatus: http.StatusOK}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGraph) handle(key string, h http.HandlerFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[key] = h
}

func (f *fakeGraph) json(key string, status int, body any) {
	f.handle(key, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
}

func (f *fakeGraph) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	// Recorded decoded, so a drive id such as b!abc matches the handler key and f.seen alike.
	f.requests = append(f.requests, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
	h, ok := f.handlers[r.Method+" "+r.URL.Path]
	f.mu.Unlock()
	if strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token") {
		f.tokens.Add(1)
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "client_credentials" ||
			r.Form.Get("scope") != "https://graph.microsoft.com/.default" || r.Form.Get("client_secret") == "" {
			http.Error(w, `{"error":"invalid_request"}`, http.StatusBadRequest)
			return
		}
		if f.tokenStatus != http.StatusOK {
			w.WriteHeader(f.tokenStatus)
			_, _ = w.Write(
				[]byte(`{"error":"invalid_client","error_description":"AADSTS7000215: Invalid client secret"}`),
			)
			return
		}
		_ = json.NewEncoder(w).
			Encode(map[string]any{"token_type": "Bearer", "expires_in": 3599, "access_token": f.token})
		return
	}
	if !ok {
		http.Error(w, `{"error":{"code":"itemNotFound"}}`, http.StatusNotFound)
		return
	}
	h(w, r)
}

// requireBearer fails the request unless it carries the current token.
func (f *fakeGraph) requireBearer(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			http.Error(w, `{"error":{"code":"InvalidAuthenticationToken"}}`, http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func (f *fakeGraph) seen(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, prefix) {
			n++
		}
	}
	return n
}

func (f *fakeGraph) client(t *testing.T) *client {
	t.Helper()
	cfg := &config{TenantID: "contoso.onmicrosoft.com", ClientID: "app", ClientSecret: "s3cret"}
	return newClient(f.server.URL, f.server.URL, cfg)
}
```

- [ ] **Step 2: Write the failing client tests**

`client_test.go`:

```go
package msgraph

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenIsCachedAndRemintedOn401(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc", f.requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"b!abc","name":"Documents","webUrl":"https://x"}`))
	}))
	c := f.client(t)
	_, err := c.drive(context.Background(), "b!abc")
	require.NoError(t, err)
	_, err = c.drive(context.Background(), "b!abc")
	require.NoError(t, err)
	require.Equal(t, int32(1), f.tokens.Load(), "second call reuses the cached token")
	// The server rotates the accepted token: the cached one is rejected once, re-minted, and the call succeeds.
	f.token = "tok-2"
	_, err = c.drive(context.Background(), "b!abc")
	require.NoError(t, err)
	require.Equal(t, int32(2), f.tokens.Load(), "a 401 re-mints once")
	require.Equal(t, 4, f.seen("GET /drives/b!abc"), "two cached calls, one rejected, one retried")
}

func TestAccessTokenBadSecretIsInvalidCredentials(t *testing.T) {
	f := newFakeGraph(t)
	f.tokenStatus = http.StatusUnauthorized
	_, err := f.client(t).accessToken(context.Background())
	require.ErrorIs(t, err, datasource.ErrInvalidCredentials)
}

func TestSiteAddressesByHostAndPath(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /sites/contoso.sharepoint.com:/sites/hr",
		http.StatusOK,
		map[string]any{
			"id":          "contoso.sharepoint.com,1,2",
			"displayName": "HR",
			"webUrl":      "https://contoso.sharepoint.com/sites/hr",
		},
	)
	f.json("GET /sites/contoso.sharepoint.com", http.StatusOK, map[string]any{"id": "root-site", "displayName": "Root"})
	c := f.client(t)
	site, err := c.site(context.Background(), "contoso.sharepoint.com", "sites/hr")
	require.NoError(t, err)
	require.Equal(t, "contoso.sharepoint.com,1,2", site.ID)
	site, err = c.site(context.Background(), "contoso.sharepoint.com", "")
	require.NoError(t, err)
	require.Equal(t, "root-site", site.ID)
}

func TestStatusMapping(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /drives/forbidden",
		http.StatusForbidden,
		map[string]any{"error": map[string]any{"code": "accessDenied"}},
	)
	c := f.client(t)
	_, err := c.drive(context.Background(), "forbidden")
	require.ErrorIs(t, err, datasource.ErrPermissionDenied)
	_, err = c.drive(context.Background(), "missing")
	require.ErrorIs(t, err, datasource.ErrResourceNotFound)
}

func TestChildFoldersFollowsNextLinkAndKeepsFoldersOnly(t *testing.T) {
	f := newFakeGraph(t)
	f.json("GET /drives/b!abc/items/01F/children", http.StatusOK, map[string]any{
		"value": []any{
			map[string]any{"id": "01A", "name": "Policies", "folder": map[string]any{}},
			map[string]any{"id": "01B", "name": "note.txt", "file": map[string]any{}},
		},
		"@odata.nextLink": f.server.URL + "/drives/b!abc/items/01F/children?$skiptoken=2",
	})
	f.handle("GET /drives/b!abc/items/01F/children", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("$skiptoken") == "2" {
			_, _ = w.Write([]byte(`{"value":[{"id":"01C","name":"Archive","folder":{}}]}`))
			return
		}
		_, _ = w.Write(
			[]byte(`{"value":[{"id":"01A","name":"Policies","folder":{}},{"id":"01B","name":"note.txt","file":{}}],` +
				`"@odata.nextLink":"` + f.server.URL + `/drives/b!abc/items/01F/children?$skiptoken=2"}`),
		)
	})
	folders, err := f.client(t).childFolders(context.Background(), "b!abc", "01F")
	require.NoError(t, err)
	require.Len(t, folders, 2)
	require.Equal(t, "Policies", folders[0].Name)
	require.Equal(t, "Archive", folders[1].Name)
}

func TestPagesRejectOffHostLink(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/root/children", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"value":[],"@odata.nextLink":"https://evil.example/next"}`))
	})
	_, err := f.client(t).childFolders(context.Background(), "b!abc", "")
	require.ErrorContains(t, err, "outside the Graph host")
}

func TestDeltaPageReturnsResyncOn410(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/root/delta", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", f.server.URL+"/drives/b!abc/root/delta?token=fresh")
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write(
			[]byte(`{"error":{"code":"resyncRequired","innerError":{"code":"resyncChangesApplyDifferences"}}}`),
		)
	})
	c := f.client(t)
	_, err := c.deltaPage(context.Background(), c.deltaStart("b!abc", ""))
	var resync *resyncError
	require.ErrorAs(t, err, &resync)
	require.Equal(t, f.server.URL+"/drives/b!abc/root/delta?token=fresh", resync.Location)
}

func TestRateLimitWaitsOnceThenSurfaces(t *testing.T) {
	f := newFakeGraph(t)
	hits := 0
	f.handle("GET /drives/b!abc", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"id":"b!abc","name":"Documents"}`))
	})
	c := f.client(t)
	start := time.Now()
	_, err := c.drive(context.Background(), "b!abc")
	require.ErrorIs(t, err, errRateLimited, "the second 429 in a run is not waited for")
	require.GreaterOrEqual(t, time.Since(start), time.Second, "the first 429 was waited for")
	require.Equal(t, 2, hits)
}

func TestDownloadFollowsRedirectWithoutToken(t *testing.T) {
	f := newFakeGraph(t)
	body := []byte("%PDF-1.4 hello")
	f.handle("GET /drives/b!abc/items/01B/content", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer tok-1", r.Header.Get("Authorization"))
		http.Redirect(w, r, f.server.URL+"/download/01B?tempauth=x", http.StatusFound)
	})
	f.handle("GET /download/01B", func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Authorization"), "the pre-authenticated URL gets no bearer token")
		_, _ = w.Write(body)
	})
	got, err := f.client(t).download(context.Background(), "b!abc", "01B", int64(len(body)), 1<<20)
	require.NoError(t, err)
	require.Equal(t, body, got)
}

func TestDownloadRejectsInsecureLocation(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/items/01B/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://downloads.example/01B", http.StatusFound)
	})
	_, err := f.client(t).download(context.Background(), "b!abc", "01B", 10, 1<<20)
	require.ErrorIs(t, err, errInsecureDownload)
}

func TestDownloadSizeMismatchAndEmptyAndTooLarge(t *testing.T) {
	f := newFakeGraph(t)
	f.handle("GET /drives/b!abc/items/01B/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, f.server.URL+"/download/01B", http.StatusFound)
	})
	f.handle("GET /download/01B", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("abc")) })
	c := f.client(t)
	_, err := c.download(context.Background(), "b!abc", "01B", 5, 1<<20)
	require.ErrorIs(t, err, errSourceChanged)
	_, err = c.download(context.Background(), "b!abc", "01B", 3, 2)
	require.ErrorIs(t, err, errTooLarge)
	f.handle("GET /download/01B", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	_, err = c.download(context.Background(), "b!abc", "01B", 0, 1<<20)
	require.ErrorIs(t, err, errEmptyFile)
}

func TestNewClientRejectsPrivateHostWithoutWhitelist(t *testing.T) {
	cfg := &config{TenantID: "t", ClientID: "c", ClientSecret: "s"}
	c := newClient("http://127.0.0.1:1", "http://127.0.0.1:1", cfg)
	_, err := c.accessToken(context.Background())
	require.ErrorContains(t, err, "SSRF", "the SSRF dial guard refuses a loopback host unless whitelisted")
	require.False(t, errors.Is(err, datasource.ErrInvalidCredentials))
}

func TestThrottleWithoutRetryAfterBacksOffThreeTimes(t *testing.T) {
	f := newFakeGraph(t)
	hits := 0
	f.handle("GET /drives/b!abc", func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	c := f.client(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	start := time.Now()
	_, err := c.drive(ctx, "b!abc")
	require.ErrorIs(t, err, errRateLimited)
	require.Equal(t, 4, hits, "the first answer plus three backed-off retries")
	require.GreaterOrEqual(t, time.Since(start), 14*time.Second, "2 + 4 + 8 seconds of backoff")
}

func TestErrorsCarryNoQueryStringOrLink(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /drives/b!abc/root/delta",
		http.StatusInternalServerError,
		map[string]any{"error": map[string]any{"code": "x"}},
	)
	c := f.client(t)
	_, err := c.deltaPage(context.Background(), c.deltaStart("b!abc", "")+"&token=SECRET")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET")
	require.NotContains(t, err.Error(), "$select")
	require.NotContains(t, (&resyncError{Location: "https://x/delta?token=SECRET"}).Error(), "SECRET")
}
```

Note: the download tests redirect to the fake's own `http` URL. `download` requires `https` on the `Location` in production; the one `http` host it accepts is the client's own `graphBaseURL` host when that is `http` (the test override), so `http://downloads.example` is still refused as insecure.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ 2>&1 | head -5`
Expected: build failure, `undefined: newClient`.

- [ ] **Step 4: Write `client.go`**

```go
package msgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

const (
	graphBaseURL = "https://graph.microsoft.com/v1.0"
	loginBaseURL = "https://login.microsoftonline.com"
	graphScope   = "https://graph.microsoft.com/.default"

	apiTimeout      = 30 * time.Second
	downloadTimeout = 120 * time.Second

	maxJSONResponseBytes int64 = 16 << 20
	maxPaginationHops          = 10000
	childrenPageSize           = 200
	tokenSkew                  = 5 * time.Minute
	// rateLimitWaitMax is the longest Retry-After a client sits out, once, before
	// retrying; a longer one, or a second one, ends the run.
	rateLimitWaitMax = 30 * time.Minute
	// missingRetryAfterBackoff is the backoff for a 429 or 503 without Retry-After:
	// 2, 4 and 8 seconds, then the run ends.
	missingRetryAfterBackoff = 2 * time.Second
	missingRetryAfterTries   = 3

	deltaSelect = "id,name,size,file,folder,root,deleted,parentReference,eTag,lastModifiedDateTime,webUrl," +
		"package,remoteItem,malware"
)

var (
	errRateLimited      = errors.New("microsoft graph rate limit exhausted")
	errTooLarge         = errors.New("response exceeds maximum size")
	errEmptyFile        = errors.New("file is empty")
	errSourceChanged    = errors.New("file size changed between listing and download")
	errInsecureDownload = errors.New("download location is not an https URL")
	errNoRedirect       = errors.New("content endpoint did not redirect to a download URL")
)

type client struct {
	graphBaseURL, loginBaseURL string
	cfg                        *config
	// http carries the bearer token; redirectHTTP is the same client that stops at
	// the /content 302; files fetches pre-authenticated URLs without any token.
	http, redirectHTTP, files *http.Client
	jsonLimit                 int64

	tokenMu    sync.Mutex
	token      string
	tokenExpAt time.Time
	waited     bool
}

type apiError struct {
	endpoint string
	status   int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("microsoft graph %s: status %d", e.endpoint, e.status)
}

func (e *apiError) Is(target error) bool {
	switch target {
	case datasource.ErrInvalidCredentials:
		return e.status == http.StatusUnauthorized
	case datasource.ErrPermissionDenied:
		return e.status == http.StatusForbidden
	case datasource.ErrResourceNotFound:
		return e.status == http.StatusNotFound
	}
	return false
}

type rateLimitError struct{ retryAfter time.Duration }

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("%s, retry after %s", errRateLimited, e.retryAfter)
}
func (e *rateLimitError) Is(target error) bool { return target == errRateLimited }

// resyncError is a 410 on a delta link: the enumeration restarts from Location.
// The link carries the delta token, so the message never includes it.
type resyncError struct{ Location string }

func (e *resyncError) Error() string { return "microsoft graph delta token expired; resync required" }

type siteInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	WebURL      string `json:"webUrl"`
}

type driveInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	WebURL string `json:"webUrl"`
}

type driveItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ETag         string    `json:"eTag"`
	WebURL       string    `json:"webUrl"`
	LastModified time.Time `json:"lastModifiedDateTime"`
	File         *struct {
		MimeType string `json:"mimeType"`
		Hashes   *struct {
			QuickXorHash string `json:"quickXorHash"`
			Sha1Hash     string `json:"sha1Hash"`
			Sha256Hash   string `json:"sha256Hash"`
		} `json:"hashes"`
	} `json:"file"`
	Folder          *struct{} `json:"folder"`
	Root            *struct{} `json:"root"`
	Deleted         *struct{} `json:"deleted"`
	Package         *struct{} `json:"package"`
	RemoteItem      *struct{} `json:"remoteItem"`
	Malware         *struct{} `json:"malware"`
	ParentReference *struct {
		DriveID string `json:"driveId"`
		ID      string `json:"id"`
	} `json:"parentReference"`
}

func (it driveItem) isFile() bool {
	return it.File != nil && it.Deleted == nil && it.RemoteItem == nil && it.Package == nil
}

type deltaPage struct {
	Items []driveItem `json:"value"`
	Next  string      `json:"@odata.nextLink"`
	Delta string      `json:"@odata.deltaLink"`
}

func newClient(graphBase, loginBase string, cfg *config) *client {
	redirect := datasource.NewConnectorHTTPClient(apiTimeout)
	// The /content answer is a 302 to a pre-authenticated host; the caller
	// resolves it so the bearer token never leaves the Graph host.
	redirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client{
		graphBaseURL: strings.TrimRight(graphBase, "/"),
		loginBaseURL: strings.TrimRight(loginBase, "/"),
		cfg:          cfg,
		http:         datasource.NewConnectorHTTPClient(apiTimeout),
		redirectHTTP: redirect,
		files:        datasource.NewConnectorHTTPClient(downloadTimeout),
		jsonLimit:    maxJSONResponseBytes,
	}
}

// readCapped reads a body, refusing anything larger than limit instead of
// buffering it; a truncated body would be indexed as if it were the whole document.
func readCapped(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w (%d bytes)", errTooLarge, limit)
	}
	return data, nil
}

// accessToken returns the cached app-only token or mints one with the client
// credentials grant. The token endpoint's 400 and 401 mean the credentials
// are wrong; anything else is a transport or service failure.
func (c *client) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpAt) {
		return c.token, nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"scope":         {graphScope},
	}
	endpoint := c.loginBaseURL + "/" + url.PathEscape(c.cfg.TenantID) + "/oauth2/v2.0/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("microsoft identity token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readCapped(resp.Body, c.jsonLimit)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		return "", fmt.Errorf("%w: microsoft identity rejected the app credentials (%s)",
			datasource.ErrInvalidCredentials, e.Error)
	}
	if resp.StatusCode != http.StatusOK {
		return "", &apiError{endpoint: "token", status: resp.StatusCode}
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("microsoft identity token response: %w", errors.Join(err, errors.New("no access_token")))
	}
	c.token = tok.AccessToken
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl > tokenSkew {
		ttl -= tokenSkew
	} else if ttl <= 0 {
		ttl = tokenSkew // a missing expires_in is not a reason to mint a token per call
	}
	c.tokenExpAt = time.Now().Add(ttl)
	return c.token, nil
}

func (c *client) invalidateToken(rejected string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token == rejected {
		c.token = ""
	}
}

// do GETs rawURL with the bearer token through hc. A 401 re-mints the token
// once; a 429 or 503 with a short Retry-After is sat out once per client.
func (c *client) do(ctx context.Context, hc *http.Client, rawURL string) (*http.Response, error) {
	reminted := false
	backoffs := 0
	for {
		token, err := c.accessToken(ctx)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			if reminted {
				return resp, nil
			}
			_ = resp.Body.Close()
			c.invalidateToken(token)
			reminted = true
			continue
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			_ = resp.Body.Close()
			limited := &rateLimitError{}
			if sec, err := strconv.ParseInt(resp.Header.Get("Retry-After"), 10, 64); err == nil && sec > 0 {
				limited.retryAfter = time.Duration(sec) * time.Second
			}
			var wait time.Duration
			switch {
			case limited.retryAfter <= 0 && backoffs < missingRetryAfterTries:
				wait = missingRetryAfterBackoff << backoffs
				backoffs++
			case limited.retryAfter > 0 && !c.waited && limited.retryAfter <= rateLimitWaitMax:
				wait = limited.retryAfter
				c.waited = true
			default:
				return nil, limited
			}
			logger.Warnf(ctx, "[MSGraph] throttled; waiting %s before retrying", wait)
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}
		return resp, nil
	}
}

// endpointLabel names a Graph call for an error without its query string,
// which carries delta and paging tokens.
func endpointLabel(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "request"
	}
	return u.Path
}

func (c *client) getJSON(ctx context.Context, rawURL string, out any) error {
	endpoint := endpointLabel(rawURL)
	resp, err := c.do(ctx, c.http, rawURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusGone {
		loc := resp.Header.Get("Location")
		if loc == "" || !c.onGraphHost(loc) {
			return &apiError{endpoint: endpoint, status: resp.StatusCode}
		}
		return &resyncError{Location: loc}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{endpoint: endpoint, status: resp.StatusCode}
	}
	body, err := readCapped(resp.Body, c.jsonLimit)
	if err != nil {
		return fmt.Errorf("microsoft graph %s: %w", endpoint, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("microsoft graph %s: %w: %w", endpoint, datasource.ErrFetchFailed, err)
	}
	return nil
}

func (c *client) onGraphHost(link string) bool { return strings.HasPrefix(link, c.graphBaseURL+"/") }

// pages walks a Graph collection by its nextLink with the git connectors'
// guards; the token goes with every request, so a link may only point back at
// the Graph host.
func (c *client) pages(ctx context.Context, first string, each func(values json.RawMessage) error) error {
	seen := map[string]struct{}{}
	for next := first; next != ""; {
		if _, dup := seen[next]; dup {
			return fmt.Errorf("microsoft graph pagination repeated a page link")
		}
		if len(seen) >= maxPaginationHops {
			return fmt.Errorf("microsoft graph pagination exceeded %d pages", maxPaginationHops)
		}
		seen[next] = struct{}{}
		var page struct {
			Value json.RawMessage `json:"value"`
			Next  string          `json:"@odata.nextLink"`
		}
		if err := c.getJSON(ctx, next, &page); err != nil {
			return err
		}
		if err := each(page.Value); err != nil {
			return err
		}
		next = page.Next
		if next != "" && !c.onGraphHost(next) {
			return fmt.Errorf("microsoft graph pagination link is outside the Graph host")
		}
	}
	return nil
}

func collect[T any](ctx context.Context, c *client, first string) ([]T, error) {
	var all []T
	err := c.pages(ctx, first, func(values json.RawMessage) error {
		var page []T
		if err := json.Unmarshal(values, &page); err != nil {
			return fmt.Errorf("%w: %w", datasource.ErrFetchFailed, err)
		}
		all = append(all, page...)
		return nil
	})
	return all, err
}

func escapeSegments(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

func (c *client) site(ctx context.Context, host, sitePath string) (*siteInfo, error) {
	endpoint := c.graphBaseURL + "/sites/" + url.PathEscape(host)
	if sitePath != "" {
		endpoint += ":/" + escapeSegments(sitePath)
	}
	var s siteInfo
	if err := c.getJSON(ctx, endpoint+"?$select=id,displayName,webUrl", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *client) siteDrives(ctx context.Context, siteID string) ([]driveInfo, error) {
	return collect[driveInfo](ctx, c, c.graphBaseURL+"/sites/"+url.PathEscape(siteID)+"/drives?$select=id,name,webUrl")
}

func (c *client) userDrive(ctx context.Context, user string) (*driveInfo, error) {
	var d driveInfo
	if err := c.getJSON(
		ctx,
		c.graphBaseURL+"/users/"+url.PathEscape(user)+"/drive?$select=id,name,webUrl",
		&d,
	); err != nil {
		return nil, err
	}
	return &d, nil
}

func (c *client) drive(ctx context.Context, driveID string) (*driveInfo, error) {
	var d driveInfo
	if err := c.getJSON(
		ctx,
		c.graphBaseURL+"/drives/"+url.PathEscape(driveID)+"?$select=id,name,webUrl",
		&d,
	); err != nil {
		return nil, err
	}
	return &d, nil
}

func (c *client) itemPath(driveID, itemID string) string {
	base := c.graphBaseURL + "/drives/" + url.PathEscape(driveID)
	if itemID == "" {
		return base + "/root"
	}
	return base + "/items/" + url.PathEscape(itemID)
}

func (c *client) item(ctx context.Context, driveID, itemID string) (*driveItem, error) {
	var it driveItem
	if err := c.getJSON(ctx, c.itemPath(driveID, itemID)+"?$select="+deltaSelect, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

// childFolders lists the direct child folders of a drive root or folder.
func (c *client) childFolders(ctx context.Context, driveID, itemID string) ([]driveItem, error) {
	first := fmt.Sprintf("%s/children?$select=id,name,folder,lastModifiedDateTime,webUrl&$top=%d",
		c.itemPath(driveID, itemID), childrenPageSize)
	all, err := collect[driveItem](ctx, c, first)
	if err != nil {
		return nil, err
	}
	folders := all[:0]
	for _, it := range all {
		if it.Folder != nil && it.Deleted == nil && it.RemoteItem == nil {
			folders = append(folders, it)
		}
	}
	return folders, nil
}

func (c *client) deltaStart(driveID, itemID string) string {
	return c.itemPath(driveID, itemID) + "/delta?$select=" + deltaSelect
}

// deltaPage fetches one page of a delta enumeration. A 410 is a *resyncError
// carrying the fresh start link.
func (c *client) deltaPage(ctx context.Context, link string) (*deltaPage, error) {
	if !c.onGraphHost(link) {
		return nil, fmt.Errorf("microsoft graph delta link is outside the Graph host")
	}
	var page deltaPage
	if err := c.getJSON(ctx, link, &page); err != nil {
		return nil, err
	}
	if page.Next != "" && !c.onGraphHost(page.Next) || page.Delta != "" && !c.onGraphHost(page.Delta) {
		return nil, fmt.Errorf("microsoft graph delta answered a link outside the Graph host")
	}
	return &page, nil
}

// download resolves the /content redirect itself, validates the pre-authenticated
// URL and fetches it without the bearer token through a capped reader. The body
// must be exactly expectedSize bytes and at most limit bytes.
func (c *client) download(ctx context.Context, driveID, itemID string, expectedSize, limit int64) ([]byte, error) {
	endpoint := c.itemPath(driveID, itemID) + "/content"
	resp, err := c.do(ctx, c.redirectHTTP, endpoint)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil, errNoRedirect
		}
		return nil, &apiError{endpoint: endpointLabel(endpoint), status: resp.StatusCode}
	}
	location := resp.Header.Get("Location")
	u, err := url.Parse(location)
	// Production download hosts are https; the only http host accepted is the
	// test fake serving as the Graph host itself.
	fakeHost := strings.TrimPrefix(c.graphBaseURL, "http://")
	plainFake := u != nil && u.Scheme == "http" && strings.HasPrefix(c.graphBaseURL, "http://") && u.Host == fakeHost
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && !plainFake) {
		return nil, errInsecureDownload
	}
	if err := utils.ValidateURLForSSRF(location); err != nil {
		return nil, fmt.Errorf("%w: %w", datasource.ErrFetchFailed, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	file, err := c.files.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: download failed", datasource.ErrFetchFailed)
	}
	defer func() { _ = file.Body.Close() }()
	if file.StatusCode < 200 || file.StatusCode >= 300 {
		return nil, &apiError{endpoint: "download", status: file.StatusCode}
	}
	body, err := readCapped(file.Body, limit)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errEmptyFile
	}
	if int64(len(body)) != expectedSize {
		return nil, errSourceChanged
	}
	return body, nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
```

The download error messages never include `location`: a signed URL in a log or a sync result would be a leaked capability.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ 2>&1 | tail -5`
Expected: `ok`.

- [ ] **Step 6: Lint and commit**

Run: `golangci-lint run ./internal/datasource/connector/msgraph/...`
Expected: no findings.

```bash
git add internal/datasource/connector/msgraph/client.go internal/datasource/connector/msgraph/client_test.go internal/datasource/connector/msgraph/fake_graph_test.go
git commit -m "feat(datasource/msgraph): Graph client with app-only token, delta paging and capped download"
```

---

### Task 4: Cursor, fingerprints and knowledge file names

**Files:**

- Create: `internal/datasource/connector/msgraph/cursor.go`
- Create: `internal/datasource/connector/msgraph/filename.go`
- Create: `internal/datasource/connector/msgraph/cursor_test.go`

**Interfaces:**

- Consumes: `driveItem` from Task 3.
- Produces: `type cursor struct { SchemaVersion int; Roots map[string]*rootState; Folders map[string]folderState; Items map[string]itemState; Drives map[string]string }`; `rootState{Delta, Next string; Resync bool; Gen int}` (Gen counts 410 restarts and stamps what each restart listed, so a restart cut by a checkpoint still knows what it has seen); `folderState{Parent, Name, Root string; Gen int}`; `itemState{FP, Failed, Parent, Name, Root string; Gen int}` (empty FP with no Failed code is a transient failure retried next run; Failed records a deterministic one reported once); `decodeCursor(old *types.SyncCursor) (*cursor, error)`; `(*cursor).toSyncCursor() *types.SyncCursor`; `(*cursor).path(externalID string) string`; `(*cursor).descendants(folderExternalID string) []string` (item ids, sorted); `(*cursor).itemsUnderRoot(rootID string) []string` (sorted); `(*cursor).dropSubtree(folderExternalID string)` (collects before deleting); `(*cursor).underRoot(id, parent string, r root) bool`; `fingerprint(it driveItem) string`; `externalID(driveID, itemID string) string`; `knowledgeFileName(segments []string) string`.

- [ ] **Step 1: Write the failing tests**

```go
package msgraph

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func sampleCursor() *cursor {
	c, _ := decodeCursor(nil)
	c.Drives["b!abc"] = "Documents"
	c.Roots["b!abc:01ROOT"] = &rootState{
		Delta: "https://graph.microsoft.com/v1.0/drives/b!abc/items/01ROOT/delta?token=t1",
	}
	c.Folders["b!abc:01ROOT"] = folderState{Parent: "b!abc:top", Name: "Policies", Root: "b!abc:01ROOT"}
	c.Folders["b!abc:01SUB"] = folderState{Parent: "b!abc:01ROOT", Name: "2026", Root: "b!abc:01ROOT"}
	c.Items["b!abc:01A"] = itemState{
		FP:     "qx:a|10|b!abc:01SUB|a.docx",
		Parent: "b!abc:01SUB",
		Name:   "a.docx",
		Root:   "b!abc:01ROOT",
	}
	c.Items["b!abc:01B"] = itemState{
		FP:     "qx:b|20|b!abc:01ROOT|b:c.pdf",
		Parent: "b!abc:01ROOT",
		Name:   "b:c.pdf",
		Root:   "b!abc:01ROOT",
	}
	return c
}

func TestCursorRoundTrip(t *testing.T) {
	c := sampleCursor()
	sc := c.toSyncCursor()
	require.Equal(t, float64(1), sc.ConnectorCursor["schema_version"])
	back, err := decodeCursor(sc)
	require.NoError(t, err)
	require.Equal(t, c.Items, back.Items)
	require.Equal(t, c.Roots["b!abc:01ROOT"].Delta, back.Roots["b!abc:01ROOT"].Delta)
}

func TestCursorRejectsUnknownSchema(t *testing.T) {
	_, err := decodeCursor(&types.SyncCursor{ConnectorCursor: map[string]interface{}{"schema_version": 7}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
}

func TestCursorPathWalksFoldersAndSanitizes(t *testing.T) {
	c := sampleCursor()
	require.Equal(t, "Documents/Policies/2026/a.docx", c.path("b!abc:01A"))
	require.Equal(t, "Documents/Policies/b_c.pdf", c.path("b!abc:01B"))
	require.Equal(t, "", c.path("b!abc:missing"))
}

func TestCursorDropSubtreeForgetsNestedFoldersWhateverTheIterationOrder(t *testing.T) {
	for i := 0; i < 20; i++ {
		c, _ := decodeCursor(nil)
		c.Folders["d:F"] = folderState{Parent: "d:top", Name: "F", Root: "d"}
		c.Folders["d:G"] = folderState{Parent: "d:F", Name: "G", Root: "d"}
		c.Folders["d:H"] = folderState{Parent: "d:G", Name: "H", Root: "d"}
		c.Items["d:x"] = itemState{FP: "x", Parent: "d:H", Name: "x.pdf", Root: "d"}
		c.dropSubtree("d:F")
		require.Empty(t, c.Folders, "run %d", i)
		require.Empty(t, c.Items, "run %d", i)
	}
}

func TestCursorPathSkipsTheDriveRootItem(t *testing.T) {
	c, _ := decodeCursor(nil)
	c.Drives["d"] = "Documents"
	c.Folders["d:root"] = folderState{Parent: "", Name: "", Root: "d"}
	c.Folders["d:P"] = folderState{Parent: "d:root", Name: "Policies", Root: "d"}
	c.Items["d:a"] = itemState{FP: "a", Parent: "d:P", Name: "a.docx", Root: "d"}
	require.Equal(t, "Documents/Policies/a.docx", c.path("d:a"))
}

func TestCursorDescendantsAndDrop(t *testing.T) {
	c := sampleCursor()
	require.Equal(t, []string{"b!abc:01A"}, c.descendants("b!abc:01SUB"))
	require.Equal(t, []string{"b!abc:01A", "b!abc:01B"}, c.descendants("b!abc:01ROOT"))
	require.Equal(t, []string{"b!abc:01A", "b!abc:01B"}, c.itemsUnderRoot("b!abc:01ROOT"))
	c.dropSubtree("b!abc:01SUB")
	require.NotContains(t, c.Items, "b!abc:01A")
	require.NotContains(t, c.Folders, "b!abc:01SUB")
	require.Contains(t, c.Items, "b!abc:01B")
}

func TestFingerprintPrefersHashesAndIncludesNameAndParent(t *testing.T) {
	it := driveItem{ID: "01A", Name: "a.docx", Size: 10, ETag: `"e1"`}
	it.ParentReference = &struct {
		DriveID string `json:"driveId"`
		ID      string `json:"id"`
	}{DriveID: "b!abc", ID: "01SUB"}
	require.Equal(t, `etag:"e1"|10|b!abc:01SUB|a.docx`, fingerprint(it))
	it.File = &struct {
		MimeType string `json:"mimeType"`
		Hashes   *struct {
			QuickXorHash string `json:"quickXorHash"`
			Sha1Hash     string `json:"sha1Hash"`
			Sha256Hash   string `json:"sha256Hash"`
		} `json:"hashes"`
	}{}
	it.File.Hashes = &struct {
		QuickXorHash string `json:"quickXorHash"`
		Sha1Hash     string `json:"sha1Hash"`
		Sha256Hash   string `json:"sha256Hash"`
	}{QuickXorHash: "QX"}
	require.Equal(t, "qx:QX|10|b!abc:01SUB|a.docx", fingerprint(it))
	renamed := it
	renamed.Name = "renamed.docx"
	require.NotEqual(t, fingerprint(it), fingerprint(renamed))
}

func TestKnowledgeFileName(t *testing.T) {
	require.Equal(
		t,
		"Documents/Policies/hand_book.docx",
		knowledgeFileName([]string{"Documents", "Policies", "hand?book.docx"}),
	)
	require.Equal(t, "untitled", knowledgeFileName([]string{""}))
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ 2>&1 | head -5`
Expected: build failure, `undefined: decodeCursor`.

- [ ] **Step 3: Write `cursor.go`**

```go
package msgraph

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
)

const cursorSchemaVersion = 1

// rootState is one selected drive or folder's delta position. Next is the
// page link being processed (resume point); Delta is the link for the next
// incremental run; Resync marks an enumeration restarted after a 410. Gen
// counts restarts: every item the restart lists is stamped with it, so a
// restart interrupted by a checkpoint still knows what it has seen, and its
// end tombstones every known item under the root with an older stamp.
type rootState struct {
	Delta  string `json:"delta,omitempty"`
	Next   string `json:"next,omitempty"`
	Resync bool   `json:"resync,omitempty"`
	Gen    int    `json:"gen,omitempty"`
}

type folderState struct {
	Parent string `json:"parent"`
	Name   string `json:"name"`
	Root   string `json:"root"`
	Gen    int    `json:"gen,omitempty"`
}

// itemState is a known file. An empty FP with no Failed code marks a file
// whose fetch failed for a transient reason and is retried before the next
// run's delta walk; Failed records a deterministic failure (too large, empty,
// malware) that is reported once and re-checked only when the file changes.
type itemState struct {
	FP     string `json:"fp"`
	Failed string `json:"failed,omitempty"`
	Parent string `json:"parent"`
	Name   string `json:"name"`
	Root   string `json:"root"`
	Gen    int    `json:"gen,omitempty"`
}

type cursor struct {
	SchemaVersion int                    `json:"schema_version"`
	Roots         map[string]*rootState  `json:"roots"`
	Folders       map[string]folderState `json:"folders"`
	Items         map[string]itemState   `json:"items"`
	Drives        map[string]string      `json:"drives"`
}

func decodeCursor(old *types.SyncCursor) (*cursor, error) {
	c := &cursor{SchemaVersion: cursorSchemaVersion}
	if old != nil && len(old.ConnectorCursor) > 0 {
		c.SchemaVersion = 0
		raw, err := json.Marshal(old.ConnectorCursor)
		if err == nil {
			err = json.Unmarshal(raw, c)
		}
		if err != nil {
			return nil, fmt.Errorf("%w: invalid Microsoft Graph cursor: %w", datasource.ErrInvalidConfig, err)
		}
		if c.SchemaVersion != cursorSchemaVersion {
			return nil, fmt.Errorf("%w: unsupported Microsoft Graph cursor schema %d",
				datasource.ErrInvalidConfig, c.SchemaVersion)
		}
	}
	if c.Roots == nil {
		c.Roots = map[string]*rootState{}
	}
	if c.Folders == nil {
		c.Folders = map[string]folderState{}
	}
	if c.Items == nil {
		c.Items = map[string]itemState{}
	}
	if c.Drives == nil {
		c.Drives = map[string]string{}
	}
	return c, nil
}

// toSyncCursor snapshots the maps through a JSON round trip so the service can
// serialize it after the run moves on.
func (c *cursor) toSyncCursor() *types.SyncCursor {
	raw, _ := json.Marshal(c)
	fields := map[string]interface{}{}
	_ = json.Unmarshal(raw, &fields)
	return &types.SyncCursor{LastSyncTime: time.Now().UTC(), ConnectorCursor: fields}
}

func externalID(driveID, itemID string) string { return driveID + ":" + itemID }

func driveOf(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] == ':' {
			return id[:i]
		}
	}
	return id
}

// path builds "<drive name>/<folders>/<file>" for a known item by walking
// parent ids; it stops at the first parent the maps do not know. The drive
// root item is recorded with an empty name and contributes no segment. An
// unknown item yields "".
func (c *cursor) path(id string) string {
	it, ok := c.Items[id]
	if !ok {
		return ""
	}
	segments := []string{it.Name}
	for parent := it.Parent; parent != ""; {
		f, ok := c.Folders[parent]
		if !ok {
			break
		}
		if f.Name != "" {
			segments = append([]string{f.Name}, segments...)
		}
		parent = f.Parent
	}
	if name := c.Drives[driveOf(id)]; name != "" {
		segments = append([]string{name}, segments...)
	}
	return knowledgeFileName(segments)
}

// isUnder reports whether the chain of parents starting at parent reaches folderID.
func (c *cursor) isUnder(parent, folderID string) bool {
	for seen := 0; parent != "" && seen < len(c.Folders)+1; seen++ {
		if parent == folderID {
			return true
		}
		f, ok := c.Folders[parent]
		if !ok {
			return false
		}
		parent = f.Parent
	}
	return false
}

// underRoot reports whether an item or folder with the given parent lies in
// the selected root r: a whole drive covers everything in that drive, a
// folder covers its subtree.
func (c *cursor) underRoot(id, parent string, r root) bool {
	if r.ItemID == "" {
		return driveOf(id) == r.DriveID
	}
	return c.isUnder(parent, r.id())
}

// descendants lists the known items inside folderID, sorted for deterministic emission.
func (c *cursor) descendants(folderID string) []string {
	var out []string
	for id, it := range c.Items {
		if c.isUnder(it.Parent, folderID) {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func (c *cursor) itemsUnderRoot(rootID string) []string {
	var out []string
	for id, it := range c.Items {
		if it.Root == rootID {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// dropSubtree forgets a folder, its subfolders and their items. The folder
// ids are collected before anything is deleted: isUnder walks parent chains
// through the same map, so deleting while iterating would orphan subfolders
// whose parent was removed first.
func (c *cursor) dropSubtree(folderID string) {
	for _, id := range c.descendants(folderID) {
		delete(c.Items, id)
	}
	var folders []string
	for id, f := range c.Folders {
		if id == folderID || c.isUnder(f.Parent, folderID) {
			folders = append(folders, id)
		}
	}
	for _, id := range folders {
		delete(c.Folders, id)
	}
}

// fingerprint identifies a file version plus its place and name: a content
// change, a rename or a move re-emits, a sharing or metadata change does not.
func fingerprint(it driveItem) string {
	content := "etag:" + it.ETag
	if it.File != nil && it.File.Hashes != nil {
		switch h := it.File.Hashes; {
		case h.QuickXorHash != "":
			content = "qx:" + h.QuickXorHash
		case h.Sha1Hash != "":
			content = "sha1:" + h.Sha1Hash
		case h.Sha256Hash != "":
			content = "sha256:" + h.Sha256Hash
		}
	}
	parent := ""
	if it.ParentReference != nil {
		parent = externalID(it.ParentReference.DriveID, it.ParentReference.ID)
	}
	return fmt.Sprintf("%s|%d|%s|%s", content, it.Size, parent, it.Name)
}
```

- [ ] **Step 4: Write `filename.go`**

```go
package msgraph

import (
	"path"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
)

// knowledgeFileName joins sanitized path segments into the knowledge base
// folder convention "<drive>/<folders>/<file>"; the extension survives so the
// parser is chosen from it. Depth and length limits are applied by the service.
func knowledgeFileName(segments []string) string {
	if len(segments) == 0 {
		return datasource.SanitizeFileName("")
	}
	out := make([]string, len(segments))
	last := len(segments) - 1
	for i := 0; i < last; i++ {
		out[i] = datasource.SanitizeFileName(segments[i])
	}
	ext := path.Ext(segments[last])
	out[last] = datasource.SanitizeFileName(strings.TrimSuffix(segments[last], ext)) + ext
	return strings.Join(out, "/")
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ -run 'TestCursor|TestFingerprint|TestKnowledgeFileName' -v 2>&1 | tail -15`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/datasource/connector/msgraph/cursor.go internal/datasource/connector/msgraph/filename.go internal/datasource/connector/msgraph/cursor_test.go
git commit -m "feat(datasource/msgraph): cursor with per-root delta links and file fingerprints"
```

---

### Task 5: Connector shell: Validate, resource tree, ancestors and the compatibility methods

**Files:**

- Create: `internal/datasource/connector/msgraph/connector.go`
- Create: `internal/datasource/connector/msgraph/connector_test.go`

**Interfaces:**

- Consumes: Tasks 2 to 4.
- Produces: `type Connector struct { kind Kind; graphBaseURL, loginBaseURL string }`; `NewConnector(kind Kind) *Connector`; `Type`, `Validate`, `ListResources`, `ResolveResourceAncestors`, `FetchAll`, `FetchIncremental`; `(*Connector).newClient(cfg *config) *client`; `(*Connector).rootDrives(ctx, cl, cfg) ([]driveInfo, error)` (the site's libraries or the user's drive); `(*Connector).errorItem(id, title, path, code string) types.FetchedItem`; `(*Connector).tombstone(id, title, path string) types.FetchedItem`; `var _ datasource.StreamingConnector = (*Connector)(nil)` (satisfied in Task 6, so this task adds the stub `FetchStream` returning `errors.New("not implemented")` and Task 6 replaces it).

- [ ] **Step 1: Write the failing tests**

```go
package msgraph

import (
	"context"
	"net/http"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func testConnector(f *fakeGraph, kind Kind) *Connector {
	return &Connector{kind: kind, graphBaseURL: f.server.URL, loginBaseURL: f.server.URL}
}

func sharePointConfig(ids ...string) *types.DataSourceConfig {
	return baseConfig(map[string]interface{}{"site_url": "https://contoso.sharepoint.com/sites/hr"}, ids...)
}

func oneDriveConfig(ids ...string) *types.DataSourceConfig {
	return baseConfig(map[string]interface{}{"user": "alice@contoso.com"}, ids...)
}

func serveSite(f *fakeGraph) {
	f.json("GET /sites/contoso.sharepoint.com:/sites/hr", http.StatusOK,
		map[string]any{"id": "site1", "displayName": "HR", "webUrl": "https://contoso.sharepoint.com/sites/hr"})
	f.json("GET /sites/site1/drives", http.StatusOK, map[string]any{"value": []any{
		map[string]any{"id": "b!docs", "name": "Documents", "webUrl": "https://contoso.sharepoint.com/sites/hr/Shared"},
		map[string]any{"id": "b!pol", "name": "Policies", "webUrl": "https://contoso.sharepoint.com/sites/hr/Policies"},
	}})
}

func TestTypes(t *testing.T) {
	require.Equal(t, "onedrive", NewConnector(KindOneDrive).Type())
	require.Equal(t, "sharepoint", NewConnector(KindSharePoint).Type())
	require.Equal(t, graphBaseURL, NewConnector(KindOneDrive).graphBaseURL)
}

func TestValidateCredentialsOnlyMintsAToken(t *testing.T) {
	f := newFakeGraph(t)
	require.NoError(t, testConnector(f, KindSharePoint).Validate(context.Background(), baseConfig(nil)))
	require.Equal(t, int32(1), f.tokens.Load())
	require.Equal(t, 0, f.seen("GET /sites"))
}

func TestValidateResolvesTheRoot(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	require.NoError(t, testConnector(f, KindSharePoint).Validate(context.Background(), sharePointConfig()))
	require.Equal(t, 1, f.seen("GET /sites/contoso.sharepoint.com:/sites/hr"))

	f.json("GET /users/alice@contoso.com/drive", http.StatusOK, map[string]any{"id": "b!alice", "name": "OneDrive"})
	require.NoError(t, testConnector(f, KindOneDrive).Validate(context.Background(), oneDriveConfig()))
	err := testConnector(f, KindOneDrive).Validate(context.Background(),
		baseConfig(map[string]interface{}{"user": "nobody@contoso.com"}))
	require.ErrorIs(t, err, datasource.ErrResourceNotFound)
}

func TestListResourcesSharePointLibrariesThenFolders(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	f.json("GET /drives/b!docs/root/children", http.StatusOK, map[string]any{"value": []any{
		map[string]any{"id": "01F", "name": "Finance", "folder": map[string]any{}, "webUrl": "https://x/Finance"},
		map[string]any{"id": "01X", "name": "readme.md", "file": map[string]any{}},
	}})
	c := testConnector(f, KindSharePoint)
	top, err := c.ListResources(context.Background(), sharePointConfig(), "")
	require.NoError(t, err)
	require.Equal(t, []types.Resource{
		{
			ExternalID:  "b!docs",
			Name:        "Documents",
			Type:        "drive",
			URL:         "https://contoso.sharepoint.com/sites/hr/Shared",
			HasChildren: true,
		},
		{
			ExternalID:  "b!pol",
			Name:        "Policies",
			Type:        "drive",
			URL:         "https://contoso.sharepoint.com/sites/hr/Policies",
			HasChildren: true,
		},
	}, top)
	children, err := c.ListResources(context.Background(), sharePointConfig(), "b!docs")
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, "b!docs:01F", children[0].ExternalID)
	require.Equal(t, "b!docs", children[0].ParentID)
	require.Equal(t, "folder", children[0].Type)
	require.True(t, children[0].HasChildren)
}

func TestListResourcesOneDriveSingleDrive(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /users/alice@contoso.com/drive",
		http.StatusOK,
		map[string]any{"id": "b!alice", "name": "OneDrive", "webUrl": "https://x"},
	)
	top, err := testConnector(f, KindOneDrive).ListResources(context.Background(), oneDriveConfig(), "")
	require.NoError(t, err)
	require.Equal(
		t,
		[]types.Resource{{ExternalID: "b!alice", Name: "OneDrive", Type: "drive", URL: "https://x", HasChildren: true}},
		top,
	)
}

func TestListResourcesWithoutPublicSettingIsInvalidConfig(t *testing.T) {
	f := newFakeGraph(t)
	_, err := testConnector(f, KindSharePoint).ListResources(context.Background(), baseConfig(nil), "")
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
}

func TestResolveResourceAncestorsWalksParents(t *testing.T) {
	f := newFakeGraph(t)
	f.json(
		"GET /drives/b!docs/items/01C",
		http.StatusOK,
		map[string]any{
			"id": "01C", "name": "c", "folder": map[string]any{},
			"parentReference": map[string]any{"driveId": "b!docs", "id": "01B"},
		},
	)
	f.json(
		"GET /drives/b!docs/items/01B",
		http.StatusOK,
		map[string]any{
			"id": "01B", "name": "b", "folder": map[string]any{},
			"parentReference": map[string]any{"driveId": "b!docs", "id": "01ROOT"},
		},
	)
	f.json(
		"GET /drives/b!docs/items/01ROOT",
		http.StatusOK,
		map[string]any{
			"id": "01ROOT", "name": "root", "root": map[string]any{},
			"folder": map[string]any{}, "parentReference": map[string]any{"driveId": "b!docs"},
		},
	)
	got, err := testConnector(
		f,
		KindSharePoint,
	).ResolveResourceAncestors(context.Background(), sharePointConfig(), []string{"b!docs:01C", "b!docs"})
	require.NoError(t, err)
	require.Equal(t, []string{"b!docs", "b!docs:01B"}, got)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ 2>&1 | head -5`
Expected: build failure, `undefined: Connector`.

- [ ] **Step 3: Write `connector.go`**

```go
package msgraph

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
)

var _ datasource.StreamingConnector = (*Connector)(nil)

// Connector syncs files from Microsoft OneDrive or SharePoint; kind decides
// which type it registers as and where the resource tree starts.
type Connector struct {
	kind Kind
	// The hosts are fixed in production; tests point them at a fake.
	graphBaseURL, loginBaseURL string
}

// NewConnector creates a stateless connector for one kind. Each data source
// carries its own app registration in its encrypted credentials.
func NewConnector(kind Kind) *Connector {
	return &Connector{kind: kind, graphBaseURL: graphBaseURL, loginBaseURL: loginBaseURL}
}

// Type returns the connector type.
func (c *Connector) Type() string { return c.kind.connectorType() }

func (c *Connector) newClient(cfg *config) *client {
	return newClient(c.graphBaseURL, c.loginBaseURL, cfg)
}

func (c *Connector) requireTarget(cfg *config) error {
	switch c.kind {
	case KindSharePoint:
		if cfg.SiteHost == "" {
			return fmt.Errorf("%w: site_url is required", datasource.ErrInvalidConfig)
		}
	default:
		if cfg.User == "" {
			return fmt.Errorf("%w: user is required", datasource.ErrInvalidConfig)
		}
	}
	return nil
}

// rootDrives lists the drives the tree starts from: the site's document
// libraries, or the user's single drive.
func (c *Connector) rootDrives(ctx context.Context, cl *client, cfg *config) ([]driveInfo, error) {
	if c.kind == KindSharePoint {
		site, err := cl.site(ctx, cfg.SiteHost, cfg.SitePath)
		if err != nil {
			return nil, err
		}
		return cl.siteDrives(ctx, site.ID)
	}
	d, err := cl.userDrive(ctx, cfg.User)
	if err != nil {
		return nil, err
	}
	return []driveInfo{*d}, nil
}

// Validate mints a token with the credentials alone, and resolves the site or
// drive when the public setting is present (the call every sync opens with).
func (c *Connector) Validate(ctx context.Context, ds *types.DataSourceConfig) error {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return err
	}
	cl := c.newClient(cfg)
	if c.requireTarget(cfg) != nil {
		_, err := cl.accessToken(ctx)
		return err
	}
	_, err = c.rootDrives(ctx, cl, cfg)
	return err
}

// ListResources returns the root drives for an empty parent and one level of
// child folders otherwise. Files are not pickable: a sync root is a folder.
func (c *Connector) ListResources(
	ctx context.Context, ds *types.DataSourceConfig, parentID string,
) ([]types.Resource, error) {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return nil, err
	}
	if err := c.requireTarget(cfg); err != nil {
		return nil, err
	}
	cl := c.newClient(cfg)
	out := []types.Resource{}
	if parentID == "" {
		drives, err := c.rootDrives(ctx, cl, cfg)
		if err != nil {
			return nil, err
		}
		for _, d := range drives {
			out = append(
				out,
				types.Resource{ExternalID: d.ID, Name: d.Name, Type: "drive", URL: d.WebURL, HasChildren: true},
			)
		}
		return out, nil
	}
	parent, err := parseResourceID(parentID)
	if err != nil {
		return nil, err
	}
	folders, err := cl.childFolders(ctx, parent.DriveID, parent.ItemID)
	if err != nil {
		return nil, err
	}
	for _, f := range folders {
		out = append(out, types.Resource{
			ExternalID: externalID(parent.DriveID, f.ID), Name: f.Name, Type: "folder", URL: f.WebURL,
			ParentID: parentID, HasChildren: true, ModifiedAt: f.LastModified,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ResolveResourceAncestors walks parentReference ids up to the drive root so
// the lazy picker can reveal a saved folder selection: one request per hop,
// the drive root reported as the drive resource itself. A saved folder that
// no longer exists contributes nothing rather than failing the picker.
func (c *Connector) ResolveResourceAncestors(
	ctx context.Context, ds *types.DataSourceConfig, ids []string,
) ([]string, error) {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return nil, err
	}
	cl := c.newClient(cfg)
	seen := map[string]bool{}
	out := []string{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, id := range ids {
		r, err := parseResourceID(id)
		if err != nil || r.ItemID == "" {
			continue
		}
		it, err := cl.item(ctx, r.DriveID, r.ItemID)
		if errors.Is(err, datasource.ErrResourceNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for it.ParentReference != nil && it.ParentReference.ID != "" {
			parent, err := cl.item(ctx, r.DriveID, it.ParentReference.ID)
			if err != nil {
				return nil, err
			}
			if parent.Root != nil || parent.ParentReference == nil || parent.ParentReference.ID == "" {
				add(r.DriveID)
				break
			}
			add(externalID(r.DriveID, parent.ID))
			it = parent
		}
	}
	sort.Strings(out)
	return out, nil
}

// collector gathers a stream for the compatibility methods required by the
// base Connector interface.
type collector struct{ items []types.FetchedItem }

func (h *collector) Emit(_ context.Context, item types.FetchedItem) error {
	h.items = append(h.items, item)
	return nil
}

func (*collector) Checkpoint(context.Context, *types.SyncCursor) error { return nil }

// FetchAll collects what a full FetchStream would emit.
func (c *Connector) FetchAll(ctx context.Context, ds *types.DataSourceConfig, _ []string) ([]types.FetchedItem, error) {
	h := &collector{}
	_, err := c.FetchStream(ctx, ds, nil, h)
	return h.items, err
}

// FetchIncremental collects what FetchStream would emit from old.
func (c *Connector) FetchIncremental(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	h := &collector{}
	next, err := c.FetchStream(ctx, ds, old, h)
	if err != nil {
		return nil, nil, err
	}
	return h.items, next, nil
}

// FetchStream is implemented in sync.go.
func (c *Connector) FetchStream(
	context.Context, *types.DataSourceConfig, *types.SyncCursor, datasource.StreamHandler,
) (*types.SyncCursor, error) {
	return nil, errors.New("msgraph: FetchStream not implemented")
}

func (c *Connector) metadata(id, p string) map[string]string {
	driveID := driveOf(id)
	return map[string]string{
		"channel":          c.Type(),
		"msgraph_drive_id": driveID,
		"msgraph_item_id":  id[len(driveID)+1:],
		"msgraph_path":     p,
	}
}

// errorItem follows the service contract for failed fetches: no content, a
// stable reason code and English fallback text.
func (c *Connector) errorItem(id, title, p, code string) types.FetchedItem {
	m := c.metadata(id, p)
	m["error"] = SyncReasonTexts[code]
	m["error_reason_code"] = code
	m["error_reason"] = SyncReasonTexts[code]
	return types.FetchedItem{ExternalID: id, Title: title, Metadata: m}
}

func (c *Connector) tombstone(id, title, p string) types.FetchedItem {
	return types.FetchedItem{ExternalID: id, Title: title, IsDeleted: true, Metadata: c.metadata(id, p)}
}
```

In the ancestors test, the fake returns `"root": {}` on the drive root item; the code does not read it, it treats "parent has no parent" as the drive root. Delete the `"root"` key from the test fixture if the Go struct's strictness ever complains (it will not: unknown JSON keys are ignored).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ 2>&1 | tail -5`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector/msgraph/connector.go internal/datasource/connector/msgraph/connector_test.go
git commit -m "feat(datasource/msgraph): connector shell with validation and the folder tree"
```

---

### Task 6: Delta streaming sync

**Files:**

- Create: `internal/datasource/connector/msgraph/sync.go`
- Create: `internal/datasource/connector/msgraph/sync_test.go`
- Modify: `internal/datasource/connector/msgraph/connector.go` (delete the stub `FetchStream`)

**Interfaces:**

- Consumes: everything above.
- Produces: `(*Connector).FetchStream(ctx, ds, old, h) (*types.SyncCursor, error)` per spec decisions 5 to 11; `const checkpointEvery = 50`. The `run` type carries the pass: `dropDeselected` (items another selected root still covers are re-homed, not tombstoned), `retryFailed` (per-item errors except credentials, rate limit and cancellation), `walkRoot` (repeated-link and hop guards, a best-effort checkpoint before a failing page is surfaced, generation stamps across a resync), `applyPage` (the drive root item contributes no path segment; a `deleted` entry owned by another root is ignored; a rename to an unimportable name tombstones), `ensureAncestors` (a parent that arrives on a later page is fetched by id), `fetchFile` (deterministic failures keep their fingerprint and a `Failed` code; a credential or transport failure ends the run instead of becoming an error item).

- [ ] **Step 1: Write the failing tests**

The fake serves delta pages from a per-root script so each test states the exact pages. Add this helper at the top of `sync_test.go`:

```go
package msgraph

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
)

func jsonEncode(w io.Writer, body any) error { return json.NewEncoder(w).Encode(body) }

func secutilsAll() utils.ImportCapabilities { return utils.AllImportCapabilities }

// deltaScript serves the pages of one root in order: the first request gets
// pages[0], the next link "?page=1" gets pages[1], and so on. The last page
// carries the deltaLink "?token=done-<n>" where n counts enumerations.
func deltaScript(f *fakeGraph, path string, pages ...[]map[string]any) {
	runs := 0
	f.handle("GET "+path, func(w http.ResponseWriter, r *http.Request) {
		idx := 0
		if p := r.URL.Query().Get("page"); p != "" {
			idx = int(p[0] - '0')
		}
		if r.URL.Query().Get("token") != "" && idx == 0 {
			// an incremental call: answer the change page registered under "changes"
			f.mu.Lock()
			h, ok := f.handlers["GET "+path+"#changes"]
			f.mu.Unlock()
			if ok {
				h(w, r)
				return
			}
			_, _ = w.Write(
				[]byte(
					`{"value":[],"@odata.deltaLink":"` + f.server.URL + path + `?token=` + r.URL.Query().
						Get("token") +
						`"}`,
				),
			)
			return
		}
		body := map[string]any{"value": pages[idx]}
		if idx+1 < len(pages) {
			body["@odata.nextLink"] = f.server.URL + path + "?page=" + string(rune('0'+idx+1))
		} else {
			runs++
			body["@odata.deltaLink"] = f.server.URL + path + "?token=done-" + string(rune('0'+runs))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = jsonEncode(w, body)
	})
}

func serveFile(f *fakeGraph, driveID, itemID string, body string) {
	f.handle("GET /drives/"+driveID+"/items/"+itemID+"/content", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, f.server.URL+"/download/"+itemID, http.StatusFound)
	})
	f.handle("GET /download/"+itemID, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
}

func file(id, name, parent string, body string, hash string) map[string]any {
	return map[string]any{
		"id":                   id,
		"name":                 name,
		"size":                 len(body),
		"eTag":                 `"e-` + id + `"`,
		"webUrl":               "https://x/" + name,
		"lastModifiedDateTime": "2026-10-23T01:00:00Z",
		"file": map[string]any{
			"mimeType": "application/octet-stream",
			"hashes":   map[string]any{"quickXorHash": hash},
		},
		"parentReference": map[string]any{"driveId": "b!docs", "id": parent},
	}
}

func rootItem(id string) map[string]any {
	return map[string]any{
		"id": id, "name": "root", "folder": map[string]any{}, "root": map[string]any{},
		"parentReference": map[string]any{"driveId": "b!docs"},
	}
}

func folder(id, name, parent string) map[string]any {
	return map[string]any{
		"id": id, "name": name, "folder": map[string]any{},
		"parentReference": map[string]any{"driveId": "b!docs", "id": parent},
	}
}

func deleted(id string) map[string]any {
	return map[string]any{"id": id, "deleted": map[string]any{}, "parentReference": map[string]any{"driveId": "b!docs"}}
}

type recorder struct {
	items       []types.FetchedItem
	checkpoints []*types.SyncCursor
}

func (r *recorder) Emit(_ context.Context, item types.FetchedItem) error {
	r.items = append(r.items, item)
	return nil
}

func (r *recorder) Checkpoint(_ context.Context, c *types.SyncCursor) error {
	r.checkpoints = append(r.checkpoints, c)
	return nil
}

func (r *recorder) names() []string {
	var out []string
	for _, it := range r.items {
		switch {
		case it.IsDeleted:
			out = append(out, "-"+it.ExternalID)
		case it.Metadata["error_reason_code"] != "":
			out = append(out, "!"+it.Metadata["error_reason_code"]+":"+it.FileName+it.Title)
		default:
			out = append(out, it.FileName)
		}
	}
	return out
}

func importsAll() *types.DataSourceConfig {
	ds := sharePointConfig("b!docs")
	ds.Imports = secutilsAll()
	return ds
}

func TestFetchStreamInitialEnumerationEmitsFilesAndCompleteListing(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			folder("01P", "Policies", "01ROOT"),
			file("01A", "a.docx", "01P", "AAAA", "qa"),
		},
		[]map[string]any{file("01B", "b.pdf", "01ROOT", "BBBBBB", "qb"), file("01N", "notes.xyz", "01ROOT", "N", "qn")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BBBBBB")
	rec := &recorder{}
	next, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(
		t,
		[]string{"Documents/Policies/a.docx", "Documents/b.pdf"},
		rec.names(),
		"notes.xyz is not importable",
	)
	require.True(t, next.CompleteListing)
	require.Equal(t, "sharepoint", rec.items[0].Metadata["channel"])
	require.Equal(t, "b!docs:01A", rec.items[0].ExternalID)
	require.Equal(t, "b!docs", rec.items[0].SourceResourceID)
	require.Len(t, rec.checkpoints, 2, "one checkpoint per page")
	cur, err := decodeCursor(next)
	require.NoError(t, err)
	require.Contains(t, cur.Roots["b!docs"].Delta, "token=done-1")
	require.Empty(t, cur.Roots["b!docs"].Next)
	require.NotContains(t, cur.Items, "b!docs:01N", "unimportable files are not tracked")
}

func TestFetchStreamFolderAfterChildrenKeepsPath(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			file("01A", "a.docx", "01P", "AAAA", "qa"),
			folder("01P", "Policies", "01ROOT"),
			rootItem("01ROOT"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	rec := &recorder{}
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/Policies/a.docx"}, rec.names())
}

func TestFetchStreamRenameReemitsAndMetadataChangeDoesNot(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err)
	// The incremental page: a.docx renamed (same hash), b.pdf re-sent unchanged.
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(
			w,
			map[string]any{
				"value": []any{
					file("01A", "renamed.docx", "01ROOT", "AAAA", "qa"),
					file("01B", "b.pdf", "01ROOT", "BB", "qb"),
				},
				"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-2",
			},
		)
	})
	second := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/renamed.docx"}, second.names())
	require.False(t, next.CompleteListing)
}

func TestFetchStreamDeletedFolderTombstonesDescendants(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			folder("01P", "Policies", "01ROOT"),
			file("01A", "a.docx", "01P", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{deleted("01P")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-2",
		})
	})
	rec := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!docs:01A"}, rec.names())
	require.Equal(t, "Documents/Policies/a.docx", rec.items[0].Metadata["msgraph_path"])
	after, _ := decodeCursor(next)
	require.NotContains(t, after.Items, "b!docs:01A")
	require.NotContains(t, after.Folders, "b!docs:01P")
	require.Contains(t, after.Items, "b!docs:01B")
}

func TestFetchStreamDeselectedRootTombstonesItsItems(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	deltaScript(
		f,
		"/drives/b!pol/root/delta",
		[]map[string]any{rootItem("02ROOT"), file("02A", "p.pdf", "02ROOT", "PP", "qp")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.json("GET /drives/b!pol", http.StatusOK, map[string]any{"id": "b!pol", "name": "Policies"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!pol", "02A", "PP")
	c := testConnector(f, KindSharePoint)
	both := sharePointConfig("b!docs", "b!pol")
	both.Imports = secutilsAll()
	cur, err := c.FetchStream(context.Background(), both, nil, &recorder{})
	require.NoError(t, err)
	rec := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!pol:02A"}, rec.names())
}

func TestFetchStreamResumesFromPageLink(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
		[]map[string]any{file("01B", "b.pdf", "01ROOT", "BB", "qb")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	// Run 1 is cut after the first page: the second page's download fails the run.
	f.handle(
		"GET /download/01B",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	)
	first := &recorder{}
	_, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err, "a download failure is a per-item error item, not a run failure")
	require.Equal(t, []string{"Documents/a.docx", "!msgraph_fetch_failed:b.pdf"}, first.names())
	// The checkpoint after page 1 points at page 2.
	mid, err := decodeCursor(first.checkpoints[0])
	require.NoError(t, err)
	require.Contains(t, mid.Roots["b!docs"].Next, "page=1")
	// Resume from that checkpoint: page 2 is re-walked, a.docx is not re-emitted, b.pdf succeeds now.
	serveFile(f, "b!docs", "01B", "BB")
	second := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), first.checkpoints[0], second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/b.pdf"}, second.names())
	require.False(t, next.CompleteListing, "a resumed run is not a complete listing")
}

func TestFetchStreamRetriesFailedDownloadsBeforeDelta(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle(
		"GET /drives/b!docs/items/01A/content",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
	)
	c := testConnector(f, KindSharePoint)
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_fetch_failed:a.docx"}, first.names())
	// Next run: delta has no changes, but the failed item is re-fetched by id.
	f.json("GET /drives/b!docs/items/01A", http.StatusOK, file("01A", "a.docx", "01ROOT", "AAAA", "qa"))
	serveFile(f, "b!docs", "01A", "AAAA")
	second := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/a.docx"}, second.names())
}

func TestFetchStreamResyncTombstonesUnseen(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	// The stored deltaLink is gone; the fresh enumeration no longer lists b.pdf.
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
		w.WriteHeader(http.StatusGone)
	})
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fresh") == "1" {
			_ = jsonEncode(
				w,
				map[string]any{
					"value": []any{
						rootItem("01ROOT"),
						file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
					},
					"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-9",
				},
			)
			return
		}
		w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
		w.WriteHeader(http.StatusGone)
	})
	rec := &recorder{}
	next, err := c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!docs:01B"}, rec.names(), "a.docx is unchanged, b.pdf vanished during the gap")
	require.False(t, next.CompleteListing, "a resync is not a fresh enumeration from a nil cursor")
	after, _ := decodeCursor(next)
	require.False(t, after.Roots["b!docs"].Resync)
	require.Contains(t, after.Roots["b!docs"].Delta, "done-9")
}

func TestFetchStreamFileTooLargeAndMalwareAreErrorItemsWithoutDownload(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	big := file("01A", "big.pdf", "01ROOT", "", "qa")
	big["size"] = 1 << 40
	bad := file("01M", "bad.docx", "01ROOT", "MM", "qm")
	bad["malware"] = map[string]any{"description": "eicar"}
	deltaScript(f, "/drives/b!docs/root/delta", []map[string]any{rootItem("01ROOT"), big, bad})
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	rec := &recorder{}
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_file_too_large:big.pdf", "!msgraph_malware_detected:bad.docx"}, rec.names())
	require.Equal(t, 0, f.seen("GET /drives/b!docs/items/01A/content"))
	require.Equal(t, 0, f.seen("GET /drives/b!docs/items/01M/content"))
}

func TestFetchStreamNoRootsIsInvalidConfig(t *testing.T) {
	f := newFakeGraph(t)
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), sharePointConfig(), nil, &recorder{})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
}

func TestFetchStreamResumedResyncKeepsItemsListedBeforeTheCut(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{
			rootItem("01ROOT"),
			file("01A", "a.docx", "01ROOT", "AAAA", "qa"),
			file("01B", "b.pdf", "01ROOT", "BB", "qb"),
		},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	// The stored deltaLink is gone. The restart has two pages: a.docx on page 1, b.pdf on page 2. The first
	// attempt is cancelled when it asks for page 2, so the second attempt resumes there with Resync still set.
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
		w.WriteHeader(http.StatusGone)
	})
	first := &recorder{}
	ctx, cancel := context.WithCancel(context.Background())
	pageTwo := false
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("fresh") {
		case "1":
			_ = jsonEncode(
				w,
				map[string]any{
					"value":           []any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
					"@odata.nextLink": f.server.URL + "/drives/b!docs/root/delta?fresh=2",
				},
			)
		case "2":
			if !pageTwo {
				pageTwo = true
				cancel() // the first attempt is cut when it asks for page 2
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = jsonEncode(w, map[string]any{
				"value":            []any{file("01B", "b.pdf", "01ROOT", "BB", "qb")},
				"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-9",
			})
		default:
			w.Header().Set("Location", f.server.URL+"/drives/b!docs/root/delta?fresh=1")
			w.WriteHeader(http.StatusGone)
		}
	})
	_, err = c.FetchStream(ctx, importsAll(), cur, first)
	require.Error(t, err)
	require.NotEmpty(t, first.checkpoints)
	mid, _ := decodeCursor(first.checkpoints[len(first.checkpoints)-1])
	require.True(t, mid.Roots["b!docs"].Resync, "the checkpoint sits inside the resync")
	require.Contains(t, mid.Roots["b!docs"].Next, "fresh=2")
	second := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), first.checkpoints[len(first.checkpoints)-1], second)
	require.NoError(t, err)
	require.Empty(t, second.names(), "nothing changed, nothing vanished: no tombstones for a.docx or b.pdf")
}

func TestFetchStreamParentOnLaterPageIsFetchedForThePath(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(f, "/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01P", "AAAA", "qa")},
		[]map[string]any{folder("01P", "Policies", "01ROOT")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.json("GET /drives/b!docs/items/01P", http.StatusOK, folder("01P", "Policies", "01ROOT"))
	serveFile(f, "b!docs", "01A", "AAAA")
	rec := &recorder{}
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"Documents/Policies/a.docx"}, rec.names())
}

func TestFetchStreamPermanentFailuresAreReportedOnceAndNotRefetched(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	big := file("01A", "big.pdf", "01ROOT", "", "qa")
	big["size"] = 1 << 40
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), big, file("01B", "b.pdf", "01ROOT", "BB", "qb")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01B", "BB")
	c := testConnector(f, KindSharePoint)
	first := &recorder{}
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, first)
	require.NoError(t, err)
	require.Equal(t, []string{"!msgraph_file_too_large:big.pdf", "Documents/b.pdf"}, first.names())
	second := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, second)
	require.NoError(t, err)
	require.Empty(t, second.names(), "a quiet run emits nothing, so the service does not count it as all-failed")
	require.Equal(t, 0, f.seen("GET /drives/b!docs/items/01A?"), "the oversized file is not re-fetched by id")
}

func TestFetchStreamRenameToUnimportableTombstones(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	serveFile(f, "b!docs", "01A", "AAAA")
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	f.handle("GET /drives/b!docs/root/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{file("01A", "a.xyz", "01ROOT", "AAAA", "qa")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/root/delta?token=done-2",
		})
	})
	rec := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err)
	require.Equal(t, []string{"-b!docs:01A"}, rec.names())
}

func TestFetchStreamMoveBetweenRootsKeepsTheFile(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	// Two folder roots in one drive; a.docx moves from A to B. Root A's stream reports the deletion,
	// root B's stream reports the arrival; whichever order the streams run in, the file stays.
	deltaScript(
		f,
		"/drives/b!docs/items/01A/delta",
		[]map[string]any{folder("01A", "A", "01ROOT"), file("01X", "a.docx", "01A", "AAAA", "qa")},
	)
	deltaScript(f, "/drives/b!docs/items/01B/delta", []map[string]any{folder("01B", "B", "01ROOT")})
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.json("GET /drives/b!docs/items/01ROOT", http.StatusOK, rootItem("01ROOT"))
	serveFile(f, "b!docs", "01X", "AAAA")
	c := testConnector(f, KindSharePoint)
	both := sharePointConfig("b!docs:01A", "b!docs:01B")
	both.Imports = secutilsAll()
	cur, err := c.FetchStream(context.Background(), both, nil, &recorder{})
	require.NoError(t, err)
	f.handle("GET /drives/b!docs/items/01A/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{deleted("01X")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01A/delta?token=done-2",
		})
	})
	f.handle("GET /drives/b!docs/items/01B/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{file("01X", "a.docx", "01B", "AAAA", "qa")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01B/delta?token=done-2",
		})
	})
	rec := &recorder{}
	next, err := c.FetchStream(context.Background(), both, cur, rec)
	require.NoError(t, err)
	// Roots run in id order: A (tombstone) then B (live). The service keeps the file because live follows
	// the tombstone, and the cursor records it under B.
	require.Equal(t, []string{"-b!docs:01X", "Documents/B/a.docx"}, rec.names())
	after, _ := decodeCursor(next)
	require.Equal(t, "b!docs:01B", after.Items["b!docs:01X"].Root)
	// A later deletion entry from A's stream for an item B now owns is ignored.
	f.handle("GET /drives/b!docs/items/01A/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{deleted("01X")},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01A/delta?token=done-3",
		})
	})
	f.handle("GET /drives/b!docs/items/01B/delta#changes", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(w, map[string]any{
			"value":            []any{},
			"@odata.deltaLink": f.server.URL + "/drives/b!docs/items/01B/delta?token=done-3",
		})
	})
	third := &recorder{}
	_, err = c.FetchStream(context.Background(), both, next, third)
	require.NoError(t, err)
	require.Empty(t, third.names())
}

func TestFetchStreamRepeatedPageLinkIsAnError(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	f.handle("GET /drives/b!docs/root/delta", func(w http.ResponseWriter, _ *http.Request) {
		_ = jsonEncode(
			w,
			map[string]any{"value": []any{}, "@odata.nextLink": f.server.URL + "/drives/b!docs/root/delta?page=loop"},
		)
	})
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.ErrorContains(t, err, "repeated a page link")
}

func TestFetchStreamRetryPassTurnsAForbiddenItemIntoAnErrorItem(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle(
		"GET /drives/b!docs/items/01A/content",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadGateway) },
	)
	c := testConnector(f, KindSharePoint)
	cur, err := c.FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.NoError(t, err)
	f.json(
		"GET /drives/b!docs/items/01A",
		http.StatusForbidden,
		map[string]any{"error": map[string]any{"code": "accessDenied"}},
	)
	rec := &recorder{}
	_, err = c.FetchStream(context.Background(), importsAll(), cur, rec)
	require.NoError(t, err, "one unreadable file does not stop the delta walk")
	require.Equal(t, []string{"!msgraph_permission_denied:a.docx"}, rec.names())
}

func TestFetchStreamRotatedSecretFailsTheRunNotTheFile(t *testing.T) {
	f := newFakeGraph(t)
	serveSite(f)
	deltaScript(
		f,
		"/drives/b!docs/root/delta",
		[]map[string]any{rootItem("01ROOT"), file("01A", "a.docx", "01ROOT", "AAAA", "qa")},
	)
	f.json("GET /drives/b!docs", http.StatusOK, map[string]any{"id": "b!docs", "name": "Documents"})
	f.handle("GET /drives/b!docs/items/01A/content", func(w http.ResponseWriter, _ *http.Request) {
		f.tokenStatus = http.StatusUnauthorized // the secret is revoked while the run is in flight
		http.Error(w, `{"error":{"code":"InvalidAuthenticationToken"}}`, http.StatusUnauthorized)
	})
	_, err := testConnector(f, KindSharePoint).FetchStream(context.Background(), importsAll(), nil, &recorder{})
	require.ErrorIs(t, err, datasource.ErrInvalidCredentials)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ -run TestFetchStream 2>&1 | tail -5`
Expected: FAIL with `msgraph: FetchStream not implemented`.

- [ ] **Step 3: Write `sync.go` and delete the stub in `connector.go`**

```go
package msgraph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

const checkpointEvery = 50

// run is one FetchStream invocation's working state.
type run struct {
	c        *Connector
	cl       *client
	cfg      *config
	ds       *types.DataSourceConfig
	cur      *cursor
	h        datasource.StreamHandler
	limit    int64
	emitted  int
	fresh    bool // started from a nil cursor
	resumed  bool // some root had a page link in progress
	resynced bool // some root restarted after a 410
}

// FetchStream walks every selected root's delta stream, emitting changed
// files, tombstones and per-file failures, and checkpoints after every page
// and every 50 emits. See the design's decisions 5 to 11.
func (c *Connector) FetchStream(
	ctx context.Context, ds *types.DataSourceConfig, old *types.SyncCursor, h datasource.StreamHandler,
) (*types.SyncCursor, error) {
	cfg, err := parseConfig(c.kind, ds)
	if err != nil {
		return nil, err
	}
	if len(cfg.Roots) == 0 {
		return nil, fmt.Errorf("%w: select at least one library or folder", datasource.ErrInvalidConfig)
	}
	cur, err := decodeCursor(old)
	if err != nil {
		return nil, err
	}
	r := &run{
		c:     c,
		cl:    c.newClient(cfg),
		cfg:   cfg,
		ds:    ds,
		cur:   cur,
		h:     h,
		limit: utils.GetMaxFileSize(),
		fresh: old == nil,
	}

	selected := map[string]root{}
	for _, rt := range cfg.Roots {
		selected[rt.id()] = rt
	}
	if err := r.dropDeselected(ctx, selected); err != nil {
		return nil, err
	}
	// Drive names for the knowledge path.
	for _, rt := range cfg.Roots {
		if _, ok := cur.Drives[rt.DriveID]; ok {
			continue
		}
		d, err := r.cl.drive(ctx, rt.DriveID)
		if err != nil {
			return nil, err
		}
		cur.Drives[rt.DriveID] = d.Name
	}
	if err := r.retryFailed(ctx); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(cfg.Roots))
	for _, rt := range cfg.Roots {
		ids = append(ids, rt.id())
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := r.walkRoot(ctx, selected[id]); err != nil {
			return nil, err
		}
	}
	out := cur.toSyncCursor()
	out.CompleteListing = r.fresh && !r.resumed && !r.resynced
	return out, nil
}

// dropDeselected tombstones what the roots that left the selection held,
// except items another selected root still covers, and forgets those roots.
func (r *run) dropDeselected(ctx context.Context, selected map[string]root) error {
	for id := range r.cur.Roots {
		if _, ok := selected[id]; ok {
			continue
		}
		for _, itemID := range r.cur.itemsUnderRoot(id) {
			if other := r.coveringRoot(itemID, r.cur.Items[itemID].Parent, selected); other != "" {
				st := r.cur.Items[itemID]
				st.Root = other
				r.cur.Items[itemID] = st
				continue
			}
			if err := r.emitTombstone(ctx, itemID); err != nil {
				return err
			}
		}
		for fid, f := range r.cur.Folders {
			if f.Root != id {
				continue
			}
			if other := r.coveringRoot(fid, f.Parent, selected); other != "" {
				f.Root = other
				r.cur.Folders[fid] = f
			} else {
				delete(r.cur.Folders, fid)
			}
		}
		delete(r.cur.Roots, id)
		if err := r.checkpoint(ctx); err != nil {
			return err
		}
	}
	return nil
}

// coveringRoot returns a selected root that still covers the entry, or "".
func (r *run) coveringRoot(id, parent string, selected map[string]root) string {
	for rootID, rt := range selected {
		if r.cur.underRoot(id, parent, rt) {
			return rootID
		}
	}
	return ""
}

func (r *run) checkpoint(ctx context.Context) error {
	r.emitted = 0
	return r.h.Checkpoint(ctx, r.cur.toSyncCursor())
}

// emit hands one item to the service and checkpoints every 50 emits. The
// caller has already recorded the item's state, so the checkpoint is a
// faithful snapshot.
func (r *run) emit(ctx context.Context, item types.FetchedItem) error {
	if err := r.h.Emit(ctx, item); err != nil {
		return err
	}
	r.emitted++
	if r.emitted >= checkpointEvery {
		return r.checkpoint(ctx)
	}
	return nil
}

func (r *run) emitTombstone(ctx context.Context, id string) error {
	it, ok := r.cur.Items[id]
	if !ok {
		return nil
	}
	p := r.cur.path(id)
	delete(r.cur.Items, id)
	return r.emit(ctx, r.c.tombstone(id, it.Name, p))
}

// retryFailed re-fetches items whose last download failed for a transient
// reason; delta will not re-send an unchanged item, so this pass is the only
// retry they get. A deterministic failure (Failed set) waits for a change.
func (r *run) retryFailed(ctx context.Context) error {
	var failed []string
	for id, it := range r.cur.Items {
		if it.FP == "" && it.Failed == "" {
			failed = append(failed, id)
		}
	}
	sort.Strings(failed)
	for _, id := range failed {
		st := r.cur.Items[id]
		item, err := r.cl.item(ctx, driveOf(id), id[len(driveOf(id))+1:])
		switch {
		case errors.Is(err, datasource.ErrResourceNotFound):
			if err := r.emitTombstone(ctx, id); err != nil {
				return err
			}
			continue
		case err != nil && !r.perItem(err):
			return err
		case err != nil:
			if err := r.emit(ctx, r.c.errorItem(id, st.Name, r.cur.path(id), downloadReason(err))); err != nil {
				return err
			}
			continue
		}
		if !item.isFile() || !r.ds.Importable(item.Name) {
			delete(r.cur.Items, id)
			continue
		}
		if err := r.fetchFile(ctx, *item, st.Root); err != nil {
			return err
		}
	}
	if len(failed) > 0 {
		return r.checkpoint(ctx)
	}
	return nil
}

// perItem reports whether a fetch error is confined to one file. Credential,
// rate-limit, cancellation and transport failures affect the whole run and
// are not hidden in error items.
func (r *run) perItem(err error) bool {
	var limited *rateLimitError
	var api *apiError
	switch {
	case errors.As(err, &limited), errors.Is(err, datasource.ErrInvalidCredentials), errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return false
	case errors.As(err, &api):
		return api.status != http.StatusUnauthorized
	}
	return errors.Is(err, errEmptyFile) || errors.Is(err, errSourceChanged) || errors.Is(err, errTooLarge) ||
		errors.Is(
			err,
			errInsecureDownload,
		) || errors.Is(err, errNoRedirect) || errors.Is(err, datasource.ErrFetchFailed)
}

// walkRoot runs one root's delta stream from its stored position.
func (r *run) walkRoot(ctx context.Context, rt root) error {
	id := rt.id()
	st := r.cur.Roots[id]
	if st == nil {
		st = &rootState{}
		r.cur.Roots[id] = st
	}
	link := st.Next
	switch {
	case link != "":
		r.resumed = true
	case st.Delta != "":
		link = st.Delta
	default:
		link = r.cl.deltaStart(rt.DriveID, rt.ItemID)
	}
	visited := map[string]struct{}{}
	for link != "" {
		if _, dup := visited[link]; dup {
			return fmt.Errorf("microsoft graph delta for %s repeated a page link", id)
		}
		if len(visited) >= maxPaginationHops {
			return fmt.Errorf("microsoft graph delta for %s exceeded %d pages", id, maxPaginationHops)
		}
		visited[link] = struct{}{}
		st.Next = link
		page, err := r.cl.deltaPage(ctx, link)
		var resync *resyncError
		if errors.As(err, &resync) {
			logger.Warnf(ctx, "[MSGraph] delta token for %s expired; re-enumerating", id)
			st.Delta, st.Resync, r.resynced = "", true, true
			st.Gen++
			link = resync.Location
			continue
		}
		if err != nil {
			return err
		}
		if page.Next == "" && page.Delta == "" {
			return fmt.Errorf("%w: delta page for %s carried neither a next link nor a delta link",
				datasource.ErrFetchFailed, id)
		}
		if err := r.applyPage(ctx, rt, st, page.Items); err != nil {
			// Best effort: the fingerprints recorded so far survive, and st.Next
			// still points at this page, so the retry re-walks it.
			_ = r.checkpoint(ctx)
			return err
		}
		if page.Delta != "" {
			if st.Resync {
				if err := r.tombstoneUnseen(ctx, id, st.Gen); err != nil {
					return err
				}
				st.Resync = false
			}
			st.Delta, st.Next = page.Delta, ""
			link = ""
		} else {
			link = page.Next
			st.Next = link
		}
		if err := r.checkpoint(ctx); err != nil {
			return err
		}
	}
	return nil
}

// applyPage records the page's folders first (a folder may follow its files
// in the same page), then handles each item with the last occurrence of a
// repeated id winning.
func (r *run) applyPage(ctx context.Context, rt root, st *rootState, items []driveItem) error {
	rootID := rt.id()
	last := map[string]int{}
	for i, it := range items {
		last[it.ID] = i
	}
	for i, it := range items {
		if last[it.ID] != i || it.Folder == nil || it.Deleted != nil || it.RemoteItem != nil {
			continue
		}
		r.recordFolder(rt, st, it)
	}
	for i, it := range items {
		if last[it.ID] != i {
			continue
		}
		id := externalID(rt.DriveID, it.ID)
		switch {
		case it.Deleted != nil:
			if f, isFolder := r.cur.Folders[id]; isFolder {
				if f.Root != rootID {
					continue // another selected root owns it now (a move between roots)
				}
				for _, child := range r.cur.descendants(id) {
					if err := r.emitTombstone(ctx, child); err != nil {
						return err
					}
				}
				r.cur.dropSubtree(id)
				continue
			}
			if known, ok := r.cur.Items[id]; ok && known.Root != rootID {
				continue
			}
			if err := r.emitTombstone(ctx, id); err != nil {
				return err
			}
		case it.Folder != nil, it.Package != nil, it.RemoteItem != nil, it.File == nil:
			continue
		case !r.ds.Importable(it.Name):
			// A file renamed to an extension the knowledge base cannot import
			// leaves the index; one that was never imported leaves no trace.
			if err := r.emitTombstone(ctx, id); err != nil {
				return err
			}
		default:
			known, ok := r.cur.Items[id]
			if ok && known.FP != "" && known.FP == fingerprint(it) {
				known.Gen, known.Root = st.Gen, rootID
				r.cur.Items[id] = known
				continue
			}
			if err := r.ensureAncestors(ctx, rt, st, it); err != nil {
				return err
			}
			if err := r.fetchFile(ctx, it, rootID); err != nil {
				return err
			}
		}
	}
	return nil
}

// recordFolder stores a folder for path building. The drive root item
// contributes no path segment, so it is stored with an empty name.
func (r *run) recordFolder(rt root, st *rootState, it driveItem) {
	parent, name := "", it.Name
	if it.ParentReference != nil && it.ParentReference.ID != "" {
		parent = externalID(rt.DriveID, it.ParentReference.ID)
	}
	if it.Root != nil || parent == "" {
		name = ""
	}
	r.cur.Folders[externalID(rt.DriveID, it.ID)] = folderState{Parent: parent, Name: name, Root: rt.id(), Gen: st.Gen}
}

// ensureAncestors fetches the folders between a file and its root that no
// delta page has delivered yet (a parent can arrive on a later page), so the
// knowledge path is complete when the file is emitted.
func (r *run) ensureAncestors(ctx context.Context, rt root, st *rootState, it driveItem) error {
	if it.ParentReference == nil {
		return nil
	}
	parentItem := it.ParentReference.ID
	for hops := 0; parentItem != "" && hops < maxPaginationHops; hops++ {
		parentID := externalID(rt.DriveID, parentItem)
		if _, known := r.cur.Folders[parentID]; known {
			return nil
		}
		if rt.ItemID != "" && parentItem == rt.ItemID {
			// The selected folder itself; its own name belongs in the path.
			folder, err := r.cl.item(ctx, rt.DriveID, parentItem)
			if err != nil {
				return err
			}
			r.recordFolder(rt, st, *folder)
			return nil
		}
		folder, err := r.cl.item(ctx, rt.DriveID, parentItem)
		if err != nil {
			return err
		}
		r.recordFolder(rt, st, *folder)
		if folder.Root != nil || folder.ParentReference == nil {
			return nil
		}
		parentItem = folder.ParentReference.ID
	}
	return nil
}

// tombstoneUnseen ends a resync: every known item under the root that the
// fresh enumeration did not stamp with the current generation is gone.
func (r *run) tombstoneUnseen(ctx context.Context, rootID string, gen int) error {
	for _, id := range r.cur.itemsUnderRoot(rootID) {
		if r.cur.Items[id].Gen == gen {
			continue
		}
		if err := r.emitTombstone(ctx, id); err != nil {
			return err
		}
	}
	for id, f := range r.cur.Folders {
		if f.Root == rootID && f.Gen != gen {
			delete(r.cur.Folders, id)
		}
	}
	return nil
}

// fetchFile downloads one file and emits it, or emits an error item. Size and
// malware checks happen before any request and are deterministic, so they are
// recorded with the fingerprint and not retried until the file changes; a
// transient failure leaves the entry empty for the retry pass.
func (r *run) fetchFile(ctx context.Context, it driveItem, rootID string) error {
	id := externalID(driveOf(rootID), it.ID)
	parent := ""
	if it.ParentReference != nil && it.ParentReference.ID != "" {
		parent = externalID(driveOf(rootID), it.ParentReference.ID)
	}
	gen := r.cur.Roots[rootID].Gen
	state := itemState{Parent: parent, Name: it.Name, Root: rootID, Gen: gen}
	r.cur.Items[id] = state
	p := r.cur.path(id)
	fail := func(code string, permanent bool) error {
		if permanent {
			state.FP, state.Failed = fingerprint(it), code
			r.cur.Items[id] = state
		}
		return r.emit(ctx, r.c.errorItem(id, it.Name, p, code))
	}
	if it.Malware != nil {
		return fail(reasonMalware, true)
	}
	if it.Size > r.limit {
		return fail(reasonFileTooLarge, true)
	}
	body, err := r.cl.download(ctx, driveOf(rootID), it.ID, it.Size, r.limit)
	if err != nil {
		if !r.perItem(err) {
			return err
		}
		return fail(downloadReason(err), errors.Is(err, errEmptyFile) || errors.Is(err, errTooLarge))
	}
	m := r.c.metadata(id, p)
	m["msgraph_web_url"] = it.WebURL
	state.FP = fingerprint(it)
	r.cur.Items[id] = state
	item := types.FetchedItem{
		ExternalID: id, Title: it.Name, FileName: p, Content: body, ContentType: http.DetectContentType(body),
		URL: it.WebURL, UpdatedAt: it.LastModified, SourceResourceID: rootID, Metadata: m,
	}
	return r.emit(ctx, item)
}

func downloadReason(err error) string {
	switch {
	case errors.Is(err, errEmptyFile):
		return reasonEmptyFile
	case errors.Is(err, errSourceChanged):
		return reasonSourceChanged
	case errors.Is(err, errTooLarge):
		return reasonFileTooLarge
	case errors.Is(err, datasource.ErrPermissionDenied):
		return reasonPermissionDenied
	case errors.Is(err, datasource.ErrResourceNotFound):
		return reasonNotFound
	case errors.Is(err, errInsecureDownload), errors.Is(err, errNoRedirect):
		return reasonInvalidResponse
	}
	return reasonFetchFailed
}
```

Then delete the stub `FetchStream` from `connector.go` (the `errors` import stays: `ResolveResourceAncestors` uses it).

Note on `TestFetchStreamResumesFromPageLink`: a download failure yields an error item and does not end the run, so the first run reaches the deltaLink. The test asserts the first checkpoint's `Next` points at page 2 and resumes from that checkpoint deliberately, which is what the service does after a timeout: the stored checkpoint, not the final cursor. `TestFetchStreamResumedResyncKeepsItemsListedBeforeTheCut` is the Review Focus 4 case: it cancels the first attempt between the two pages of a restarted enumeration and asserts the resumed attempt tombstones nothing.

- [ ] **Step 4: Run the whole package**

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/connector/msgraph/ 2>&1 | tail -20`
Expected: `ok`.

- [ ] **Step 5: Race and lint**

Run: `GOTOOLCHAIN=go1.26.10 go test -race ./internal/datasource/connector/msgraph/ && golangci-lint run ./internal/datasource/connector/msgraph/...`
Expected: both clean.

- [ ] **Step 6: Commit**

```bash
git add internal/datasource/connector/msgraph/
git commit -m "feat(datasource/msgraph): delta streaming sync with fingerprints, tombstones and resync"
```

---

### Task 7: Register both types, parity test and the default allow-list config

**Files:**

- Modify: `internal/container/container.go:61` (import), `:1906-1911` (two `Register` calls after Seafile)
- Modify: `internal/datasource/syncreason/parity_test.go:26-27` (add `msgraph.SyncReasonTexts`)
- Modify: `frontend/src/i18n/locales/en-US.ts:7066` (eight `datasource.syncError.msgraph_*` keys after `bitbucket_lfs_unsupported`)
- Modify: `.env.example:130-134`, `docker-compose.yml:117`, `helm/README.md:507,520`
- Modify: `internal/datasource/README.md:7`

**Interfaces:**

- Consumes: `msgraph.NewConnector`, `msgraph.KindOneDrive`, `msgraph.KindSharePoint`, `msgraph.SyncReasonTexts`.

- [ ] **Step 1: Write the failing parity test change**

`internal/datasource/syncreason/parity_test.go`: add the import `"github.com/jrmatherly/ai-kb/internal/datasource/connector/msgraph"` and extend the list:

```go
		github.SyncReasonTexts, bitbucket.SyncReasonTexts, msgraph.SyncReasonTexts,
```

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/syncreason/ 2>&1 | tail -12`
Expected: FAIL, eight `backend emits "msgraph_..." but en-US.ts has no datasource.syncError.msgraph_...` lines.

- [ ] **Step 2: Add the frontend strings**

`frontend/src/i18n/locales/en-US.ts`, after `bitbucket_lfs_unsupported`:

```ts
      msgraph_file_too_large: 'The file exceeds the size limit; raise MAX_FILE_SIZE_MB or shrink the file.',
      msgraph_empty_file: 'The file is empty and was skipped.',
      msgraph_source_changed: 'The file changed while it was being fetched; the next sync retries it.',
      msgraph_fetch_failed: 'Failed to fetch the file from Microsoft Graph; the next sync retries it.',
      msgraph_permission_denied: 'Access to the file was denied; check the app registration permissions and the site grant.',
      msgraph_not_found: 'The file no longer exists or was moved; the next sync retries it.',
      msgraph_malware_detected: 'Microsoft flagged the file as malware; AI KB does not import it.',
      msgraph_invalid_response: 'Microsoft Graph returned a response that could not be used; the next sync retries it.',
```

Run: `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/syncreason/`
Expected: PASS.

- [ ] **Step 3: Register the connectors**

`internal/container/container.go` imports, after the `bitbucketConnector` line:

```go
	msgraphConnector "github.com/jrmatherly/ai-kb/internal/datasource/connector/msgraph"
```

After the Seafile registration:

```go
	if err := registry.Register(msgraphConnector.NewConnector(msgraphConnector.KindOneDrive)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register onedrive connector: %w", err))
	}
	if err := registry.Register(msgraphConnector.NewConnector(msgraphConnector.KindSharePoint)); err != nil {
		errs = errors.Join(errs, fmt.Errorf("register sharepoint connector: %w", err))
	}
```

Run: `GOTOOLCHAIN=go1.26.10 go build ./... && GOTOOLCHAIN=go1.26.10 go test ./internal/container/ ./internal/datasource/... 2>&1 | tail -5`
Expected: build OK, tests `ok`.

- [ ] **Step 4: Update the default allow-list surfaces**

`.env.example` lines 130-134: change every "Notion, Confluence, GitHub and Bitbucket" to "Notion, Confluence, GitHub, Bitbucket, OneDrive and SharePoint" and the example to `# AIKB_DATASOURCE_TYPES=notion,confluence,github,bitbucket,onedrive,sharepoint`.

`docker-compose.yml:117`: the comment becomes `# Data source connector types offered (allow-list; empty means notion,confluence,github,bitbucket,onedrive,sharepoint, all means every type, see .env.example)`.

`helm/README.md:507`: "unset means Notion, Confluence, GitHub, Bitbucket, OneDrive and SharePoint"; line 520: `value: "confluence,notion,github,bitbucket,onedrive,sharepoint,rss"`.

`internal/datasource/README.md:7`: add "OneDrive, SharePoint" to the implemented list after Bitbucket and drop OneDrive from the metadata-only list ("Google Drive, Web Crawler, Slack and IMAP have metadata entries...").

Run: `git grep -n 'notion,confluence,github,bitbucket[^,]' -- '*.md' '*.yml' '*.example'`
Expected: no output (every default list carries the two new types).

- [ ] **Step 5: Commit**

```bash
git add internal/container/container.go internal/datasource/syncreason/parity_test.go frontend/src/i18n/locales/en-US.ts .env.example docker-compose.yml helm/README.md internal/datasource/README.md
git commit -m "feat(datasource): register the OneDrive and SharePoint connectors and enable them by default"
```

---

### Task 8: Frontend form, public-field table, icons and strings

**Files:**

- Modify: `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue:160-185` (generalise the two Confluence functions), `:759-767` (two `connectorDefs` entries after Bitbucket)
- Modify: `frontend/src/views/knowledge/settings/datasourceIcons.ts`
- Create: `frontend/src/assets/img/datasource-onedrive.svg`, `frontend/src/assets/img/datasource-sharepoint.svg`
- Modify: `frontend/src/assets/img/ICONS.md`
- Modify: `frontend/src/i18n/locales/en-US.ts:6796` (two field groups after `bitbucket`), `:6884` (two `connector` names), `:6900` (two `connectorDesc` entries)
- Modify: `frontend/src/views/knowledge/settings/DataSourceEditorDialog.test.ts:269` (two hydration tests)

**Interfaces:**

- Produces: `PUBLIC_SETTINGS_FIELDS: Record<string, string[]>` in the dialog's script; `hydratePublicFieldsFromSettings()` and `syncPublicFieldsToSettings()` replacing the Confluence pair at every call site (lines 931, 979, 1203, 1217 and `enterReplaceCredentials`); i18n groups `datasource.onedrive.{tenantId, tenantIdHint, clientId, clientSecret, clientSecretHint, user, userHint}` and `datasource.sharepoint.{tenantId, tenantIdHint, clientId, clientSecret, clientSecretHint, siteUrl, siteUrlHint}`.

- [ ] **Step 1: Write the failing hydration tests**

Append to `DataSourceEditorDialog.test.ts` after the Bitbucket test:

```ts
test('saved OneDrive and SharePoint public settings hydrate the credential form', async () => {
  const od = await fixture({ type: 'onedrive', settings: { user: 'alice@contoso.com' } })
  try {
    od.vm.enterReplaceCredentials()
    assert.equal(od.vm.form.config.credentials.user, 'alice@contoso.com')
    od.vm.form.config.credentials.user = 'bob@contoso.com'
    od.vm.syncPublicFieldsToSettings()
    assert.equal(od.vm.form.config.settings.user, 'bob@contoso.com')
  } finally { od.close() }
  const sp = await fixture({ type: 'sharepoint', settings: { site_url: 'https://contoso.sharepoint.com/sites/hr' } })
  try {
    sp.vm.enterReplaceCredentials()
    assert.equal(sp.vm.form.config.credentials.site_url, 'https://contoso.sharepoint.com/sites/hr')
    assert.deepEqual(sp.vm.currentDef.fields.map((f: any) => f.key), ['tenant_id', 'client_id', 'client_secret', 'site_url'])
  } finally { sp.close() }
})

test('Confluence public fields still round-trip through the table', async () => {
  const f = await fixture({ type: 'confluence', settings: { edition: 'cloud', base_url: 'https://c.atlassian.net', username: 'u' } })
  try {
    f.vm.enterReplaceCredentials()
    assert.equal(f.vm.form.config.credentials.base_url, 'https://c.atlassian.net')
    assert.equal(f.vm.form.config.credentials.edition, 'cloud')
  } finally { f.close() }
})
```

The fixture's `replace()` helper overwrites the credentials with a GitLab fixture, so these tests call `enterReplaceCredentials()` directly; the fixture exposes every script-setup binding, so no `defineExpose` change is needed.

Run: `npm --prefix frontend test -- src/views/knowledge/settings/DataSourceEditorDialog.test.ts 2>&1 | tail -20`
Expected: the two new tests FAIL (`syncPublicFieldsToSettings is not a function`, no `onedrive` def).

- [ ] **Step 2: Generalise the public-field pair**

Replace `hydrateConfluenceCredentialsFromSettings` and `syncConfluencePublicFieldsToSettings` with:

```ts
// Non-secret fields the connector reads from settings but the editor collects
// in the credentials step: mirrored into settings on every submit, test and
// listing, and hydrated back when the user replaces credentials.
const PUBLIC_SETTINGS_FIELDS: Record<string, { key: string; fallback: string }[]> = {
  confluence: [{ key: 'edition', fallback: 'server' }, { key: 'base_url', fallback: '' }, { key: 'username', fallback: '' }],
  onedrive: [{ key: 'user', fallback: '' }],
  sharepoint: [{ key: 'site_url', fallback: '' }],
}

function hydratePublicFieldsFromSettings() {
  const fields = PUBLIC_SETTINGS_FIELDS[form.value.type]
  if (!fields) return
  const settings = form.value.config.settings || {}
  const creds = form.value.config.credentials || {}
  const hydrated: Record<string, any> = { ...creds }
  for (const { key, fallback } of fields) {
    hydrated[key] = creds[key] || settings[key] || fallback
  }
  form.value.config.credentials = hydrated
}

function syncPublicFieldsToSettings() {
  const fields = PUBLIC_SETTINGS_FIELDS[form.value.type]
  if (!fields) return
  const creds = form.value.config.credentials || {}
  const mirrored: Record<string, any> = {}
  for (const { key } of fields) {
    if (creds[key]) mirrored[key] = creds[key]
  }
  form.value.config.settings = { ...(form.value.config.settings || {}), ...mirrored }
}
```

Rename every call site (`grep -n 'Confluence.*Settings\|ConfluenceCredentials' frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue` lists them: `enterReplaceCredentials`, `testConnection`, `loadResources`, `buildConfigPayload`, `commitCredentialsIfNeeded`).

- [ ] **Step 3: Add the connector definitions**

After the Bitbucket entry in `connectorDefs`:

```ts
  {
    type: 'onedrive', available: true,
    docUrl: 'https://learn.microsoft.com/graph/auth-v2-service',
    permissionDocUrl: 'https://learn.microsoft.com/graph/permissions-reference#filesreadall',
    permissionPageUrl: 'https://entra.microsoft.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade',
    requiredPermissions: ['Files.Read.All (application, admin consent)'],
    fields: [
      { key: 'tenant_id', labelKey: 'datasource.onedrive.tenantId', placeholder: 'contoso.onmicrosoft.com', hintKey: 'datasource.onedrive.tenantIdHint' },
      { key: 'client_id', labelKey: 'datasource.onedrive.clientId', placeholder: '00000000-0000-0000-0000-000000000000' },
      { key: 'client_secret', labelKey: 'datasource.onedrive.clientSecret', placeholder: '', secret: true, hintKey: 'datasource.onedrive.clientSecretHint' },
      { key: 'user', labelKey: 'datasource.onedrive.user', placeholder: 'alice@contoso.com', hintKey: 'datasource.onedrive.userHint' },
    ],
  },
  {
    type: 'sharepoint', available: true,
    docUrl: 'https://learn.microsoft.com/graph/auth-v2-service',
    permissionDocUrl: 'https://learn.microsoft.com/graph/permissions-selected-overview',
    permissionPageUrl: 'https://entra.microsoft.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade',
    requiredPermissions: ['Sites.Selected (application) plus a read grant on the site, or Sites.Read.All'],
    fields: [
      { key: 'tenant_id', labelKey: 'datasource.sharepoint.tenantId', placeholder: 'contoso.onmicrosoft.com', hintKey: 'datasource.sharepoint.tenantIdHint' },
      { key: 'client_id', labelKey: 'datasource.sharepoint.clientId', placeholder: '00000000-0000-0000-0000-000000000000' },
      { key: 'client_secret', labelKey: 'datasource.sharepoint.clientSecret', placeholder: '', secret: true, hintKey: 'datasource.sharepoint.clientSecretHint' },
      { key: 'site_url', labelKey: 'datasource.sharepoint.siteUrl', placeholder: 'https://contoso.sharepoint.com/sites/hr', hintKey: 'datasource.sharepoint.siteUrlHint' },
    ],
  },
```

- [ ] **Step 4: Add the strings**

`en-US.ts`, after the `bitbucket` group:

```ts
    onedrive: {
      tenantId: 'Directory (tenant) ID', tenantIdHint: 'The Microsoft Entra tenant ID or its domain, from the app registration overview.',
      clientId: 'Application (client) ID',
      clientSecret: 'Client secret', clientSecretHint: 'A client secret of the app registration. The app needs the Files.Read.All application permission with admin consent; it can read every drive in the tenant, so use a dedicated app registration.',
      user: 'OneDrive owner', userHint: 'The user principal name or object ID whose OneDrive is synced. Personal Microsoft accounts are not supported.',
    },
    sharepoint: {
      tenantId: 'Directory (tenant) ID', tenantIdHint: 'The Microsoft Entra tenant ID or its domain, from the app registration overview.',
      clientId: 'Application (client) ID',
      clientSecret: 'Client secret', clientSecretHint: 'A client secret of the app registration. Grant Sites.Selected and give the app read access to the site (least privilege), or grant Sites.Read.All.',
      siteUrl: 'Site URL', siteUrlHint: 'The SharePoint site to sync, such as https://contoso.sharepoint.com/sites/hr. A sub-site is its own data source.',
    },
```

In `connector`: `onedrive: 'Microsoft OneDrive (work or school)', sharepoint: 'Microsoft SharePoint'`. In `connectorDesc`: `onedrive: 'Sync folders and files from a user\'s OneDrive for work or school', sharepoint: 'Sync document libraries and folders from a SharePoint site'`.

- [ ] **Step 5: Icons and provenance**

`frontend/src/assets/img/datasource-onedrive.svg` (original cloud tile):

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><rect width="24" height="24" rx="5.3" fill="#1F6FEB"/><path d="M8.2 16.5h8.1a2.9 2.9 0 0 0 .5-5.76A4.2 4.2 0 0 0 8.9 9.2a3.4 3.4 0 0 0-.7 7.3z" fill="#FFFFFF"/></svg>
```

`frontend/src/assets/img/datasource-sharepoint.svg` (original site tile):

```svg
<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><rect width="24" height="24" rx="5.3" fill="#0B6A5B"/><circle cx="10" cy="10" r="4.2" fill="#FFFFFF"/><circle cx="15.2" cy="13.6" r="3.2" fill="#FFFFFF" fill-opacity="0.85"/><circle cx="9.6" cy="16.4" r="2.2" fill="#FFFFFF" fill-opacity="0.7"/></svg>
```

`ICONS.md`: add two rows and one sentence after the table:

```markdown
| `datasource-onedrive.svg` | AI KB cloud tile | Drawn for AI KB (2026-10-23); no Microsoft asset | Microsoft's product icons need an express licence, so this mark is original; the product name beside it is a truthful compatibility statement. |
| `datasource-sharepoint.svg` | AI KB site tile | Drawn for AI KB (2026-10-23); no Microsoft asset | Same as above. |
```

`datasourceIcons.ts`: import both files and add `onedrive: onedriveIcon, sharepoint: sharepointIcon` after `seafile`.

- [ ] **Step 6: Run the frontend checks**

Run: `npm --prefix frontend test && npm --prefix frontend run type-check 2>&1 | tail -15`
Expected: all PASS, including `localeKeyAudit.test.ts` and `channelLabel.test.ts`. Then `GOTOOLCHAIN=go1.26.10 go test ./internal/datasource/syncreason/` (the `en-US.ts` edit re-triggers it).

- [ ] **Step 7: Browser check on the dev stack**

Run `make dev-start && make dev-app && make dev-frontend` (memory `project-dev-stack-browser-check` lists the port and model prerequisites), open a knowledge base's data source settings, pick SharePoint, fill the four fields with the loopback fake values described in that memory, and confirm: the four fields render with hints, "Test connection" posts `validate-credentials` with `site_url` inside `credentials`, and the saved source shows `site_url` under settings. Record what was checked in the commit body.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue frontend/src/views/knowledge/settings/DataSourceEditorDialog.test.ts frontend/src/views/knowledge/settings/datasourceIcons.ts frontend/src/assets/img/datasource-onedrive.svg frontend/src/assets/img/datasource-sharepoint.svg frontend/src/assets/img/ICONS.md frontend/src/i18n/locales/en-US.ts
git commit -m "feat(ui): OneDrive and SharePoint connector forms with table-driven public settings"
```

---

### Task 9: Documentation, changelog, deviations table and the live check

**Files:**

- Modify: `website-docs/03-features/10-datasource.md` (table rows after line 42, allow-list sentence at 46, reconcile list at 56, two sections after the Seafile section, the model sentences at 160-166, the diagram label at 335, registration snippet at 469, the note at 473)
- Modify: `website-docs/01-getting-started/01-introduction.md:50,110`, `website-docs/01-getting-started/04-configuration.md:144`, `website-docs/04-api/02-api-infra.md:309,313`, `website-docs/05-clients/01-frontend.md:194`, `website-docs/06-development/03-extension-points.md:503,506`, `README.md:51,233`
- Modify: `CHANGELOG.md` (under `## [Unreleased]`, a `### Changes` heading with two `- **NEW**:` lines)
- Modify: `.claude/skills/upstream-review/SKILL.md:126` and a new row after line 133
- Modify: `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md` only if it names the connector list (check with `grep -n -i bitbucket`)

- [ ] **Step 1: Feature docs**

Table rows after Bitbucket:

```markdown
| OneDrive | `onedrive` | Folders of one user's OneDrive for work or school |
| SharePoint | `sharepoint` | Document libraries and folders of one SharePoint Online site |
```

Line 46: "unset or empty offers Notion, Confluence, GitHub, Bitbucket, OneDrive and SharePoint only". Line 56: add "OneDrive and SharePoint" to the reconcile list. Lines 160-166: add "OneDrive and SharePoint" wherever GitHub, Bitbucket, GitLab and Seafile are listed as streaming connectors that filter before download and restart on gained models. Line 335: `alt "StreamingConnector (Feishu/Lark, GitLab, GitHub, Bitbucket, Confluence, Seafile, OneDrive, SharePoint)"`. Line 469: two registration lines. Line 473: remove OneDrive from the not-implemented list.

New section after Seafile:

````markdown
#### OneDrive and SharePoint (`connector/msgraph/`)

Both types use one Microsoft Entra app registration with application permissions and the client-credentials grant; no user signs in, and personal Microsoft accounts (OneDrive at onedrive.live.com) are not supported. Register an app in the Microsoft Entra admin center, create a client secret, grant the permission below with admin consent, and enter the directory (tenant) ID, the application (client) ID and the secret in `credentials.tenant_id`, `credentials.client_id` and `credentials.client_secret`.

- **OneDrive**: the app needs `Files.Read.All` (application). That permission reads every drive in the tenant, so use a dedicated app registration. `settings.user` names the drive's owner (user principal name or object ID). The picker shows the drive, then its folders.
- **SharePoint**: the app needs `Sites.Selected` (application) plus a `read` grant on the site (an admin posts the grant to `/sites/{id}/permissions`), or `Sites.Read.All`. `settings.site_url` is the site (`https://contoso.sharepoint.com/sites/hr`); a sub-site is its own data source. The picker shows the site's document libraries, then their folders. Which of the two permissions the site lookup accepts in practice is recorded here after the live check.

Select whole libraries or folders; single files are not selectable. A sync walks each selected root with the Graph `delta` query: the first run enumerates everything, later runs ask only for changes since the stored position, and the position advances after every page is processed, so an interrupted run resumes at the first unprocessed page. Each known file keeps a fingerprint (content hash, size, folder and name), so an item Microsoft reports for a sharing or metadata change is not re-imported, a rename or move is re-imported under its new path, and a folder deletion removes the knowledge items under it. A file whose download failed is retried at the start of the next run. When Microsoft reports that the stored position expired (HTTP 410), the connector re-enumerates the root and removes the items the new listing no longer contains. Items are identified by the Graph item ID, so files keep their identity when renamed or moved; the knowledge path is `<library or drive name>/<folders>/<file>`. Shared folders from other drives, OneNote notebooks and files Microsoft flags as malware are not imported; files over `MAX_FILE_SIZE_MB` are reported as failed items. Throttling (`429` or `503`) is waited out once when `Retry-After` is at most 30 minutes, otherwise the run ends and resumes on the next sync. A folder rename or move does not rename the knowledge items under it (Microsoft reports no descendants for it); they take their new path when they next change. A data source in `full` sync mode starts every scheduled run without its cursor, as the other streaming connectors do, so it downloads and re-imports every file each run; use incremental mode and leave reconciliation to the delta tombstones. The cursor grows with the number of files (about 200 bytes each, including file and folder names, and it is readable through the data source API); split a selection of more than about 10,000 files across data sources. Only the global Microsoft cloud is supported.

```json
{"credentials":{"tenant_id":"contoso.onmicrosoft.com","client_id":"<app id>","client_secret":"<secret>"},"settings":{"site_url":"https://contoso.sharepoint.com/sites/hr"},"resource_ids":["b!<drive id>","b!<drive id>:<folder item id>"]}
```
````

- [ ] **Step 2: The other pages**

`01-introduction.md:50` and `:110`, `05-clients/01-frontend.md:194`, `README.md:51,233`: add "OneDrive / SharePoint" to each connector list. `04-configuration.md:144`: default `notion,confluence,github,bitbucket,onedrive,sharepoint` and add `onedrive`, `sharepoint` to the type list. `04-api/02-api-infra.md:309`: "16 connector types are registered: ..., bitbucket, seafile, onedrive and sharepoint"; line 313: the unset default names the six. `06-development/03-extension-points.md:503`: two table rows (`onedrive` and `sharepoint`, both `internal/datasource/connector/msgraph/`); line 506: drop `onedrive` from the unimplemented list.

Run: `git grep -n -i 'onedrive' -- '*.md' | grep -i 'not implemented\|metadata only\|cannot sync'`
Expected: no output.

- [ ] **Step 3: Changelog and deviations table**

`CHANGELOG.md` under `## [Unreleased]`:

```markdown
### Changes

- **NEW**: **OneDrive and SharePoint data sources**: sync folders from a user's OneDrive for work or school, or document libraries and folders from a SharePoint Online site, with a Microsoft Entra app registration (client credentials). Incremental sync uses the Graph delta query, so only changed files are fetched; renames and moves keep their identity; folder deletions remove their files; a failed download is retried on the next run; an expired delta position re-enumerates and removes what vanished. Offered by default next to Notion, Confluence, GitHub and Bitbucket: `onedrive` and `sharepoint` join the `AIKB_DATASOURCE_TYPES` default, so a deployment that sets the variable must list them to keep them. Personal Microsoft accounts are not supported.
- **CHANGED**: The data source editor mirrors a connector's public settings (Confluence edition, URL and username; OneDrive user; SharePoint site URL) through one table instead of Confluence-only code.
```

`.claude/skills/upstream-review/SKILL.md:126`: the default list becomes `notion,confluence,github,bitbucket,onedrive,sharepoint`. New row after the Bitbucket row:

```markdown
| no OneDrive or SharePoint data source (upstream has an `onedrive` catalog stub only) | `connector/msgraph/` (one package, two types, app-only client credentials, per-root Graph delta streams with a fingerprint map) | ours; an upstream OneDrive implementation would be reviewed against this design, not merged |
```

- [ ] **Step 4: Drift review and gates**

Run the `docs-drift-reviewer` agent on `git diff main...HEAD`, fix what it reports, then:

```bash
scripts/validate.sh --base main
```

Expected: every area passes, including english-only, emdash and version.

- [ ] **Step 5: Commit**

```bash
git add website-docs README.md CHANGELOG.md .claude/skills/upstream-review/SKILL.md internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md
git commit -m "docs(datasource): document the OneDrive and SharePoint connectors"
```

- [ ] **Step 6: Open the PR**

```bash
git push -u origin feat/microsoft-graph-datasources
gh pr create -R jrmatherly/ai-kb --title "feat(datasource): OneDrive and SharePoint connectors over Microsoft Graph" --body-file - <<'EOF'
Adds `onedrive` and `sharepoint` data source connectors (one Go package `connector/msgraph`), app-only client credentials, folder tree picker, delta-based incremental sync with checkpoints, resume, tombstones, folder fan-out, failed-download retry and 410 resync. Both types join the default allow-list.

Spec: docs/superpowers/specs/completed/2026-10-23-microsoft-graph-connectors-design.md
Plan: docs/superpowers/plans/completed/2026-10-23-microsoft-graph-connectors.md

Live check against a Microsoft 365 tenant is pending (see the last task of the plan).

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
```

- [ ] **Step 7: Live check (needs Jason's tenant)**

With an Entra app registration Jason creates (one with `Sites.Selected` granted `read` on a test site, one with `Files.Read.All`), on the dev stack: create a SharePoint source for the test site, select one library and one folder, sync; upload, edit, rename and delete a file in SharePoint, delete a folder, resync; deselect the folder, resync; repeat the create-and-sync with the OneDrive source. Record in `10-datasource.md`: which permission `GET /sites/{host}:/{path}` accepted, whether `GET /drives/{id}/items/{id}/delta` answered for the selected folder and whether it reports a file moved out of the folder as `deleted`, the observed `Retry-After` behaviour if any, whether `quickXorHash` was present on every file item, and whether the downloaded length of a Word or Excel file in a library with metadata columns equals the listed `size` (if not, the exact-size check in `download` becomes a shorter-only check in its own commit). Note the results on the PR as well. If the folder-scoped delta is refused, the fallback is to walk the drive root's delta and filter by ancestry through the `folders` map; that is a change to `walkRoot` and `applyPage` only, and gets its own commit.
