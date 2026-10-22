# Capability-Aware Sync Filtering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A data-source sync fetches only files the target knowledge base can ingest right now, skips the rest without deleting anything, and re-fetches skipped files once a model is configured.

**Architecture:** One value, `utils.ImportCapabilities{Images, Audio}`, is derived from the knowledge base's VLM and ASR configs, rides on the `DataSourceConfig` every connector receives (replacing `MultimodalEnabled`), and is the single filter through `(*DataSourceConfig).Importable(name)`. The service populates it in one helper, prechecks it before an update deletes existing knowledge, and forces a nil-cursor full sync when a capability was gained since the last run (recorded on the sync cursor).

**Tech Stack:** Go 1.26, testify, the connectors' fake HTTP servers.

**Spec:** `docs/superpowers/specs/2026-10-22-sync-import-capabilities-and-mdx.md`, Part 1.

## Global Constraints

- The service imports no connector package; connectors import `types` and `utils`, never the service.
- `types` already imports `utils`; `utils` must not import `types`.
- `DataSourceConfig.MultimodalEnabled` is deleted in the same change that adds `Imports`; no compatibility alias.
- `repofile.IsSupported` is deleted once every caller uses `Importable`.
- Image extensions are `jpg, jpeg, png, gif, webp, bmp, svg, tiff`; audio extensions are `mp3, wav, m4a, flac, ogg` (copied from `knowledge_util.go:441` and `:451`).
- English only; no em dashes; gofumpt, lines at most 120; the pre-commit hook lints the whole root module.

## Review Focus

1. A file name with an upper-case extension (`Shot.PNG`) must be filtered the same as lower-case (Task 1, `TestIsImportableIgnoresCase`).
2. A stored cursor written before this change has no capability field and must not force a full sync on the first run after deploy (Task 3, `TestGainedCapabilityForcesFullSyncOnlyAgainstARecordedValue`).
3. An update of an existing knowledge item whose type is no longer importable must leave the existing item in place (Task 3, `TestIngestItemSkipsUnimportableBeforeDeleting`).
4. Losing a capability must not delete anything on an incremental run (Task 4, `TestFetchStreamLostCapabilityEmitsNoTombstones`).
5. The picker must hide a file the sync would skip, so a user cannot select what cannot import (Task 4, Seafile `TestListResourcesHidesImagesWithoutVLM`).

---

### Task 1: Capabilities and the importable predicate

**Files:**

- Modify: `internal/utils/import_filetypes.go`
- Modify: `internal/application/service/knowledge_util.go:439-457` (`IsImageType`, `IsAudioType` delegate)
- Test: `internal/utils/import_filetypes_test.go`

**Interfaces:**

- Produces: `type ImportCapabilities struct { Images bool; Audio bool }`; `func (c ImportCapabilities) Gained(prev ImportCapabilities) bool` (true when `c.Images && !prev.Images` or `c.Audio && !prev.Audio`); `func IsImageExtension(ext string) bool`; `func IsAudioExtension(ext string) bool` (both normalise with `NormalizeImportExtension`); `func IsImportable(ext string, caps ImportCapabilities) bool` (in the shared set, and not an image without `caps.Images`, and not audio without `caps.Audio`).

- [ ] **Step 1: Write the failing tests**

```go
func TestIsImportable(t *testing.T) {
	all := utils.ImportCapabilities{Images: true, Audio: true}
	none := utils.ImportCapabilities{}
	for _, tc := range []struct {
		ext  string
		caps utils.ImportCapabilities
		want bool
	}{
		{"md", none, true}, {"png", none, false}, {"png", all, true},
		{"mp3", none, false}, {"mp3", utils.ImportCapabilities{Audio: true}, true},
		{"webp", all, false}, // image type outside the shared set stays unsupported
		{"go", all, false}, {"", all, false},
	} {
		require.Equal(t, tc.want, utils.IsImportable(tc.ext, tc.caps), "%s %+v", tc.ext, tc.caps)
	}
}

func TestIsImportableIgnoresCase(t *testing.T) {
	require.False(t, utils.IsImportable(".PNG", utils.ImportCapabilities{}))
	require.True(t, utils.IsImportable(".PNG", utils.ImportCapabilities{Images: true}))
}

func TestGained(t *testing.T) {
	require.True(t, utils.ImportCapabilities{Images: true}.Gained(utils.ImportCapabilities{}))
	require.False(t, utils.ImportCapabilities{}.Gained(utils.ImportCapabilities{Images: true}))
	require.False(t, utils.ImportCapabilities{Images: true}.Gained(utils.ImportCapabilities{Images: true}))
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/utils/ -run 'TestIsImportable|TestGained'`
Expected: FAIL, undefined `ImportCapabilities`.

- [ ] **Step 3: Implement the type, `Gained`, the two extension predicates and `IsImportable` in `import_filetypes.go`; make `IsImageType` and `IsAudioType` in `knowledge_util.go` one-line delegates**

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/utils/ ./internal/application/service/ -run 'TestIsImportable|TestGained|TestIsImageType|TestIsAudioType|TestValidate'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/utils/import_filetypes.go internal/utils/import_filetypes_test.go internal/application/service/knowledge_util.go
git commit -m "feat(utils): model what a knowledge base can import"
```

### Task 2: The capability on the config, the cursor and the knowledge base

**Files:**

- Modify: `internal/types/datasource.go:232-238` (field swap), `:216` region (new method), `:421-435` (cursor field)
- Modify: `internal/types/knowledgebase.go:902-907` (new method beside `IsMultimodalEnabled`)
- Modify: `internal/datasource/connector/feishu/core/engine.go:156`, `feishu/core/shared.go:315,452`, `feishu/wiki/connector.go:322,350`, `feishu/drive/connector.go:359,393` and their tests (6 references)
- Test: `internal/types/datasource_test.go`

**Interfaces:**

- Consumes: `utils.ImportCapabilities`, `utils.IsImportable` from Task 1.
- Produces: `DataSourceConfig.Imports utils.ImportCapabilities` (`json:"-"`) replacing `MultimodalEnabled`; `func (c *DataSourceConfig) Importable(name string) bool` (`utils.IsImportable(path.Ext(name), c.Imports)`, false on a nil receiver); `SyncCursor.Imports *utils.ImportCapabilities` (`json:"imports,omitempty"`, nil means "recorded before this field existed"); `func (kb *KnowledgeBase) ImportCapabilities() utils.ImportCapabilities` (`Images: kb.VLMConfig.IsEnabled()`, `Audio: kb.ASRConfig.IsASREnabled()`, zero on nil). Feishu's `MultimodalEnabled` parameters and struct field are renamed to read `config.Imports.Images` at `engine.go:156` and keep their internal names otherwise.

- [ ] **Step 1: Write the failing tests**

```go
func TestDataSourceConfigImportable(t *testing.T) {
	cfg := &DataSourceConfig{Imports: utils.ImportCapabilities{Audio: true}}
	require.True(t, cfg.Importable("docs/a.md"))
	require.False(t, cfg.Importable("docs/Shot.PNG"))
	require.True(t, cfg.Importable("talk.mp3"))
	require.False(t, (*DataSourceConfig)(nil).Importable("a.md"))
}

func TestSyncCursorImportsRoundTripAndOmitWhenNil(t *testing.T) {
	raw, _ := json.Marshal(SyncCursor{})
	require.NotContains(t, string(raw), "imports")
	c := SyncCursor{Imports: &utils.ImportCapabilities{Images: true}}
	raw, _ = json.Marshal(c)
	var back SyncCursor
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, c.Imports, back.Imports)
}

func TestKnowledgeBaseImportCapabilities(t *testing.T) {
	kb := &KnowledgeBase{VLMConfig: VLMConfig{Enabled: true, ModelID: "m"}}
	require.Equal(t, utils.ImportCapabilities{Images: true}, kb.ImportCapabilities())
	require.Equal(t, utils.ImportCapabilities{}, (*KnowledgeBase)(nil).ImportCapabilities())
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/types/ -run 'TestDataSourceConfigImportable|TestSyncCursorImports|TestKnowledgeBaseImportCapabilities'`
Expected: FAIL, undefined fields and methods.

- [ ] **Step 3: Implement the three types changes and delete `MultimodalEnabled`; migrate the Feishu references so `go build ./...` is clean**

- [ ] **Step 4: Build and run the touched packages' tests**

Run: `go build ./... && go test ./internal/types/ ./internal/datasource/connector/feishu/...`
Expected: PASS; `grep -rn MultimodalEnabled internal` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/types/datasource.go internal/types/datasource_test.go internal/types/knowledgebase.go internal/datasource/connector/feishu
git commit -m "feat(datasource): carry the knowledge base's import capabilities on the connector config"
```

### Task 3: The service populates, prechecks and compares

**Files:**

- Modify: `internal/application/service/datasource_service.go:383,418,477,705,1303` (replace `ds.ParseConfig()` with `s.connectorConfig(ctx, ds)`), `:721` (delete the `MultimodalEnabled` line), `:814`, `:902-1013`, `:1055` and `:1387-1436` (`applyFetchedItem` and `ingestItem` gain `imports`, the precheck), `:1132-1140` (`processSyncStreaming` start cursor), `:736` and the cursor writes at `:763` and `:1070` (stamp `Imports`)
- Test: a new `internal/application/service/datasource_import_capabilities_test.go` using the stubs in `knowledge_create_gitlab_test.go` and `knowledgebase_delete_datasource_test.go`

**Interfaces:**

- Consumes: Task 2's `Imports`, `Importable`, `SyncCursor.Imports`, `kb.ImportCapabilities()`.
- Produces: `func (s *DataSourceService) connectorConfig(ctx context.Context, ds *types.DataSource) (*types.DataSourceConfig, error)` (parse, load the knowledge base through `s.kbService.GetKnowledgeBaseByID`, set `Imports`; a missing knowledge base returns the parse result with zero capabilities and a warning, so validation of an orphaned source still fails on its own terms); `applyFetchedItem` (`:902`, called from the batch loop at `:814` and the streaming handler at `:1055`) and `ingestItem` (`:1387`) each gain a parameter `imports utils.ImportCapabilities` after `ds`; the streaming handler struct built in `processSyncStreaming` stores `imports` from `config.Imports` and the batch loop passes `config.Imports`; `ingestItem` returns `(false, errUnimportable)` before the existing-knowledge lookup when `!utils.IsImportable(path.Ext(item.FileName), imports)`, where `var errUnimportable = errors.New("file type needs a model the knowledge base lacks")`, and `applyFetchedItem`'s error switch at `:984` counts it as `Skipped` with an `Infof` line naming the file and extension; `processSyncStreaming` replaces `startCursor` with nil when `startCursor != nil && startCursor.Imports != nil && config.Imports.Gained(*startCursor.Imports)`, logging `"import capabilities gained since the last run, starting a full sync"`; every cursor the service persists (`:763`, `:1070`) carries `Imports: &config.Imports`.

- [ ] **Step 1: Write the failing tests**

```go
func TestIngestItemSkipsUnimportableBeforeDeleting(t *testing.T) {
	// a png item, a config with Imports{} and an existing knowledge row for its external id:
	// ingestItem returns errUnimportable, the repo's delete count stays 0, the handler counts Skipped.
}

func TestGainedCapabilityForcesFullSyncOnlyAgainstARecordedValue(t *testing.T) {
	// stored cursor Imports nil, kb with VLM: startCursor stays non-nil.
	// stored cursor Imports{} (recorded), kb with VLM: startCursor becomes nil.
	// stored cursor Imports{Images:true}, kb without VLM: startCursor stays non-nil.
}

func TestPersistedCursorCarriesImports(t *testing.T) {
	// after a streaming checkpoint, ds.LastSyncCursor JSON contains "imports":{"images":true,"audio":false}.
}
```

Write them against `streamStartCursor`'s caller by extracting the comparison into `func startCursorFor(stored *types.SyncCursor, imports utils.ImportCapabilities) *types.SyncCursor` so the second test needs no stack.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/application/service/ -run 'TestIngestItemSkips|TestGainedCapability|TestPersistedCursorCarries'`
Expected: FAIL, undefined `errUnimportable` and `startCursorFor`.

- [ ] **Step 3: Implement `connectorConfig`, the five call sites, the precheck, `startCursorFor` and the cursor stamping**

- [ ] **Step 4: Run the service tests**

Run: `go test ./internal/application/service/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/service/datasource_service.go internal/application/service/datasource_import_capabilities_test.go
git commit -m "feat(datasource): skip what the knowledge base cannot import and resync when it can"
```

### Task 4: Connectors filter through the config

**Files:**

- Modify: `internal/datasource/connector/github/connector.go:34-56` (`Connector` gains `imports utils.ImportCapabilities`, set in `configured`), `github/snapshot.go:41-66` (`snapshot.add` takes `importable func(string) bool`; `snapshot` and `walkSubtrees` pass `c.importable`)
- Modify: `internal/datasource/connector/gitlab/connector.go:188,193,196,291,298,303,366` (`repofile.IsSupported(x)` becomes `ds.Importable(x)` or `c.importable(x)` where `ds` is out of scope; `walkFiles` gains an `importable func(string) bool` parameter)
- Modify: `internal/datasource/connector/seafile/connector.go:146`, `seafile/sync.go:194,246` (`utils.IsSupportedImportExtension(path.Ext(e.Name))` becomes `ds.Importable(e.Name)`; `syncRun` keeps the config it already receives)
- Delete: `repofile.IsSupported` in `internal/datasource/connector/repofile/repofile.go:66-71` and its tests in `repofile_test.go` and `gitlab/client_test.go:305-321`
- Test: `github/connector_test.go`, `gitlab/connector_recursion_test.go`, `seafile/sync_test.go`

**Interfaces:**

- Consumes: `(*types.DataSourceConfig).Importable` from Task 2. The GitHub and GitLab test helper `sourceConfig` gains an `imports utils.ImportCapabilities` field set by a new helper `withImports(cfg, caps)`.

- [ ] **Step 1: Write the failing tests**

```go
// github/connector_test.go
func TestFetchStreamFetchesImagesOnlyWithVLM(t *testing.T) {
	// tree: docs/a.md, docs/shot.png. Without Images: summary has only +docs/a.md and the fake's log
	// has no blob request for shot.png. With Images: both items, and the blob request is logged.
}

func TestFetchStreamLostCapabilityEmitsNoTombstones(t *testing.T) {
	// old cursor at head c1 recorded with both files; same head, Imports{}: the run emits nothing.
}

// gitlab/connector_recursion_test.go
func TestNestedDirectoriesSkipAudioWithoutASR(t *testing.T) {}

// seafile/sync_test.go
func TestListResourcesHidesImagesWithoutVLM(t *testing.T) {}
func TestWalkDirSkipsImagesWithoutVLM(t *testing.T) {}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/datasource/connector/github/ ./internal/datasource/connector/gitlab/ ./internal/datasource/connector/seafile/ -run 'VLM|ASR|LostCapability'`
Expected: FAIL (images and audio are fetched regardless).

- [ ] **Step 3: Migrate the eleven call sites, thread `importable` into the GitHub snapshot and the GitLab walk, delete `repofile.IsSupported` and its tests**

- [ ] **Step 4: Run every connector package and lint**

Run: `go test ./internal/datasource/... && golangci-lint run --new-from-rev=HEAD~4 ./...`
Expected: PASS; `grep -rn "IsSupported(" internal/datasource` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector
git commit -m "feat(connectors): filter repository and Seafile files by the knowledge base's import capabilities"
```

### Task 5: Docs, changelog and live verification

**Files:**

- Modify: `website-docs/03-features/10-datasource.md` (after the deletion-detection sentence at `:147`: a paragraph on what a sync skips, the skipped count, the forced full sync on a gained model, and removal of now-unimportable items by a full-mode sync with deletion sync on)
- Modify: `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md` (the file-filter guidance names `DataSourceConfig.Importable`)
- Modify: `docs/superpowers/specs/2026-10-22-bitbucket-connector-design.md` decision 1 ("have a supported extension" becomes "pass `ds.Importable`")
- Modify: `CHANGELOG.md` under `## [Unreleased]` `### Changes`

- [ ] **Step 1: Write the docs and the changelog entry**

Changelog line: `- **CHANGED**: Data-source syncs fetch only what the knowledge base can ingest. GitHub, GitLab and Seafile skip images when the knowledge base has no vision model and audio when it has no speech model, instead of downloading them and failing; the picker hides them too. A file skipped on ingest is counted as skipped and never replaces an existing item. Configuring a model later makes the next sync a full one, so the skipped files are fetched.`

- [ ] **Step 2: Run validate**

Run: `scripts/validate.sh`
Expected: every listed area passed.

- [ ] **Step 3: Commit**

```bash
git add website-docs/03-features/10-datasource.md internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md docs/superpowers/specs/2026-10-22-bitbucket-connector-design.md CHANGELOG.md
git commit -m "docs(datasource): capability-aware sync filtering"
```

- [ ] **Step 4: Live check on the dev stack**

A GitHub data source over a directory holding `a.md` and `shot.png`, in a knowledge base without a vision model: sync once.
Expected: the sync log shows one created, one skipped, zero failed; the server log has no blob request for `shot.png`.
Configure a vision model, sync again.
Expected: the server log says a full sync starts because capabilities were gained; `shot.png` is created.
Remove the model, sync again (incremental).
Expected: nothing deleted. Run a full-mode sync with deletion sync on.
Expected: `shot.png` is removed and the log names it under the reconcile.
