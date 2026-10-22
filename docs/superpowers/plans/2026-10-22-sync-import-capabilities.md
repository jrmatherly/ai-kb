# Capability-Aware Sync Filtering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A data-source sync fetches only files the target knowledge base can ingest right now, skips the rest without deleting anything, and re-fetches skipped files once a model is configured.

**Architecture:** One value, `utils.ImportCapabilities{Images, Audio}`, is derived from the knowledge base's VLM and ASR configs, rides on the `DataSourceConfig` every connector receives (replacing `MultimodalEnabled`), and is the single filter through `(*DataSourceConfig).Importable(name)`. The service populates it wherever it builds a connector config, prechecks it before an update deletes existing knowledge, stamps it on every persisted cursor, and forces a nil-cursor full sync when a capability was gained since the last recorded run.

**Tech Stack:** Go 1.26, testify, the connectors' fake HTTP servers.

**Spec:** `docs/superpowers/specs/2026-10-22-sync-import-capabilities-and-mdx.md`, Part 1. Revised 2026-10-22 after a three-model plan review; the spec's decisions 1, 3, 5 and 6 are amended in the same commit (predicates instead of exported sets, the wizard's knowledge base id, a capability-only precheck on content items, a pointer cursor field).

## Global Constraints

- The service imports no connector package; connectors import `types` and `utils`, never the service.
- `types` already imports `utils`; `utils` must not import `types` (today it depends only on `internal/ipclass`).
- Every task's commit builds and passes the whole-module pre-commit lint, so a field is deleted only in the commit that migrates its last reader.
- `repofile.IsSupported` is deleted once every caller uses `Importable`.
- Image extensions are `jpg, jpeg, png, gif, webp, bmp, svg, tiff`; audio extensions are `mp3, wav, m4a, flac, ogg` (copied from `knowledge_util.go:441` and `:451`).
- English only; no em dashes; gofumpt, lines at most 120.
- Runs after the MDX plan, so `mdx` is in the shared set when Task 1's tests run.

## Review Focus

1. A file name with an upper-case extension (`Shot.PNG`) must be filtered the same as lower-case (Task 1, `TestIsImportableIgnoresCase`).
2. A stored cursor written before this change has no capability field and must not force a full sync on the first run after deploy (Task 3, `TestStartCursorForForcesFullSyncOnlyAgainstARecordedValue`).
3. An update of an existing knowledge item whose type is no longer importable must leave the existing item in place (Task 3, `TestIngestItemSkipsUnimportableBeforeDeleting`).
4. Losing a capability must not delete anything on an incremental run, on GitHub (Task 4, `TestFetchStreamLostCapabilityEmitsNoTombstones`, with a moved head) and on Seafile, whose reconcile runs every run (Task 4, `TestWalkDirKeepsCursorFilesHiddenByALostCapability`).
5. The create wizard's picker, which has no saved data source, must see the same capabilities as the sync (Task 3, `TestListResourcesForConfigLoadsTheKnowledgeBase`).

---

### Task 1: Capabilities and the importable predicate

**Files:**

- Modify: `internal/utils/import_filetypes.go`
- Modify: `internal/application/service/knowledge_util.go:439-457` (`IsImageType`, `IsAudioType` become one-line delegates; both gain case-insensitivity, which the create gate already applies before calling them)
- Create: `internal/utils/import_capabilities_test.go` (`package utils`, importing `github.com/stretchr/testify/require`, names unqualified)
- Test: `internal/application/service/knowledge_util_filetype_test.go` (one new test)

**Interfaces:**

- Produces, in `utils`: `type ImportCapabilities struct { Images bool; Audio bool }` with JSON tags `images` and `audio`; `func (c ImportCapabilities) Gained(prev ImportCapabilities) bool` (true when `c.Images && !prev.Images` or `c.Audio && !prev.Audio`); `func IsImageExtension(ext string) bool`; `func IsAudioExtension(ext string) bool` (both normalise with `NormalizeImportExtension`); `func MissingModel(ext string, caps ImportCapabilities) string` (returns `"vision"` for an image extension without `caps.Images`, `"speech"` for an audio extension without `caps.Audio`, else `""`); `func IsImportable(ext string, caps ImportCapabilities) bool` (`IsSupportedImportExtension(ext) && MissingModel(ext, caps) == ""`).

- [ ] **Step 1: Write the failing tests**

```go
package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsImportable(t *testing.T) {
	all := ImportCapabilities{Images: true, Audio: true}
	images := ImportCapabilities{Images: true}
	none := ImportCapabilities{}
	for _, tc := range []struct {
		ext  string
		caps ImportCapabilities
		want bool
	}{
		{"md", none, true}, {"mdx", none, true}, {"png", none, false}, {"png", images, true}, {"png", all, true},
		{"mp3", none, false}, {"mp3", images, false}, {"mp3", ImportCapabilities{Audio: true}, true},
		{"webp", all, false}, // image type outside the shared set stays unsupported
		{"go", all, false}, {"", all, false},
	} {
		require.Equal(t, tc.want, IsImportable(tc.ext, tc.caps), "%s %+v", tc.ext, tc.caps)
	}
}

func TestIsImportableIgnoresCase(t *testing.T) {
	require.False(t, IsImportable(".PNG", ImportCapabilities{}))
	require.True(t, IsImportable(".PNG", ImportCapabilities{Images: true}))
}

func TestMissingModel(t *testing.T) {
	require.Equal(t, "vision", MissingModel("png", ImportCapabilities{}))
	require.Equal(t, "speech", MissingModel(".WAV", ImportCapabilities{Images: true}))
	require.Equal(t, "", MissingModel("png", ImportCapabilities{Images: true}))
	require.Equal(t, "", MissingModel("md", ImportCapabilities{}))
	require.Equal(t, "", MissingModel("", ImportCapabilities{}))
}

func TestGained(t *testing.T) {
	require.True(t, ImportCapabilities{Images: true}.Gained(ImportCapabilities{}))
	require.False(t, ImportCapabilities{}.Gained(ImportCapabilities{Images: true}))
	require.False(t, ImportCapabilities{Images: true}.Gained(ImportCapabilities{Images: true}))
}
```

In `knowledge_util_filetype_test.go`:

```go
func TestImageAndAudioTypePredicatesIgnoreCase(t *testing.T) {
	if !IsImageType("PNG") || !IsAudioType(".Wav") || IsImageType("md") {
		t.Fatal("predicates must normalise case and a leading dot")
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/utils/ -run 'TestIsImportable|TestMissingModel|TestGained' && go test ./internal/application/service/ -run TestImageAndAudioTypePredicatesIgnoreCase`
Expected: FAIL, undefined `ImportCapabilities`; the service test fails on `"PNG"`.

- [ ] **Step 3: Implement the type, `Gained`, the two extension predicates, `MissingModel` and `IsImportable` in `import_filetypes.go`; make `IsImageType` and `IsAudioType` delegate**

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/utils/ ./internal/application/service/ -run 'TestIsImportable|TestMissingModel|TestGained|TestImageAndAudioTypePredicatesIgnoreCase|TestImportExtensionSetIsSharedAcrossPaths'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/utils/import_filetypes.go internal/utils/import_capabilities_test.go internal/application/service/knowledge_util.go internal/application/service/knowledge_util_filetype_test.go
git commit -m "feat(utils): model what a knowledge base can import"
```

### Task 2: The capability on the config, the cursor and the knowledge base

**Files:**

- Modify: `internal/types/datasource.go:232-238` (field swap), the `DataSourceConfig` methods region (new method), `:421-435` (cursor field)
- Modify: `internal/types/knowledgebase.go:902-907` (new method beside `IsMultimodalEnabled`, which stays)
- Modify: `internal/application/service/datasource_service.go:721` (the only service reader of the deleted field: `config.MultimodalEnabled = kb.IsMultimodalEnabled()` becomes `config.Imports = kb.ImportCapabilities()`; Task 3 reshapes this site again)
- Modify: `internal/datasource/connector/feishu/core/engine.go:156` (`config.MultimodalEnabled` becomes `config.Imports.Images`) and `internal/datasource/connector/feishu/drive/drive_blocks_test.go:20` (the only test that sets the config field). The `DocxFetchInput.MultimodalEnabled` field in `feishu/core/shared.go:315` and its wiki and drive parameters are a different field and keep their names.
- Test: `internal/types/datasource_test.go` (imports testify `assert` already; use it)

**Interfaces:**

- Consumes: `utils.ImportCapabilities`, `utils.IsImportable` from Task 1.
- Produces: `DataSourceConfig.Imports utils.ImportCapabilities` (`json:"-"`) replacing `MultimodalEnabled`; `func (c *DataSourceConfig) Importable(name string) bool` (`utils.IsImportable(path.Ext(name), c.Imports)`, false on a nil receiver); `SyncCursor.Imports *utils.ImportCapabilities` (`json:"imports,omitempty"`; nil means the cursor was recorded before this field existed); `func (kb *KnowledgeBase) ImportCapabilities() utils.ImportCapabilities` (`Images: kb.VLMConfig.IsEnabled()`, `Audio: kb.ASRConfig.IsASREnabled()`, zero on nil).

- [ ] **Step 1: Write the failing tests**

```go
func TestDataSourceConfigImportable(t *testing.T) {
	cfg := &DataSourceConfig{Imports: utils.ImportCapabilities{Audio: true}}
	assert.True(t, cfg.Importable("docs/a.md"))
	assert.False(t, cfg.Importable("docs/Shot.PNG"))
	assert.True(t, cfg.Importable("talk.mp3"))
	assert.False(t, cfg.Importable("LICENSE"), "a dotless name has no importable extension")
	assert.False(t, (*DataSourceConfig)(nil).Importable("a.md"))
}

func TestSyncCursorImportsRoundTripAndOmitWhenNil(t *testing.T) {
	raw, _ := json.Marshal(SyncCursor{})
	assert.NotContains(t, string(raw), "imports")
	c := SyncCursor{Imports: &utils.ImportCapabilities{Images: true}}
	raw, _ = json.Marshal(c)
	assert.Contains(t, string(raw), `"imports":{"images":true,"audio":false}`)
	var back SyncCursor
	assert.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, c.Imports, back.Imports)
}

func TestKnowledgeBaseImportCapabilities(t *testing.T) {
	kb := &KnowledgeBase{VLMConfig: VLMConfig{Enabled: true, ModelID: "m"}}
	assert.Equal(t, utils.ImportCapabilities{Images: true}, kb.ImportCapabilities())
	assert.Equal(t, utils.ImportCapabilities{}, (*KnowledgeBase)(nil).ImportCapabilities())
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/types/ -run 'TestDataSourceConfigImportable|TestSyncCursorImports|TestKnowledgeBaseImportCapabilities'`
Expected: FAIL, undefined fields and methods.

- [ ] **Step 3: Implement the types changes, delete `MultimodalEnabled`, and migrate its three readers (`datasource_service.go:721`, `feishu/core/engine.go:156`, `drive_blocks_test.go:20`)**

- [ ] **Step 4: Build and run the touched packages' tests**

Run: `go build ./... && go test ./internal/types/ ./internal/datasource/connector/feishu/... ./internal/application/service/ -run 'Importable|SyncCursorImports|ImportCapabilities|Feishu|Drive|Wiki'`
Expected: PASS; `grep -rn 'onfig.MultimodalEnabled' internal` prints nothing (`DocxFetchInput.MultimodalEnabled` still exists and is unrelated).

- [ ] **Step 5: Commit**

```bash
git add internal/types/datasource.go internal/types/datasource_test.go internal/types/knowledgebase.go internal/application/service/datasource_service.go internal/datasource/connector/feishu
git commit -m "feat(datasource): carry the knowledge base's import capabilities on the connector config"
```

### Task 3: The service populates, prechecks, stamps and compares

**Files:**

- Modify: `internal/application/service/datasource_service.go`: `:383,418,477,1303` (replace `ds.ParseConfig()` with `s.connectorConfig(ctx, ds)`), `:705-721` (keep `ds.ParseConfig()` there and the `config.Imports = kb.ImportCapabilities()` line from Task 2, since `kb` is already loaded at `:671`), `:433-443` (`ListResourcesForConfig` gains `kbID string`), `:814` and `:1142-1145` (pass `config.Imports` to `applyFetchedItem` and the stream handler), `:902-1013` (`applyFetchedItem` gains `imports`, the error switch at `:986` gains the `errUnimportable` case before the `embedded_image` case), `:1033-1042` (`streamSyncHandler` gains `imports utils.ImportCapabilities`), `:1132` (`startCursorFor`), `:1387-1436` (`ingestItem` gains `imports`, the precheck)
- Modify: `internal/application/service/datasource_deletions.go:94-102` (`runDeletions` gains `imports utils.ImportCapabilities`; `withPending` sets `out.Imports = &d.imports`) and the two `newRunDeletions` call sites (pass `config.Imports`)
- Modify: `internal/handler/datasource.go:367-372,387-393` (`listResourcesForConfigRequest` gains a `KnowledgeBaseID` string field with JSON tag `knowledge_base_id`, passed to the service)
- Modify: `frontend/src/api/datasource/index.ts:146-153` (`listResourcesForConfig` sends `knowledge_base_id`) and `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue:542` (passes the dialog's knowledge base id)
- Modify, existing test call sites of `applyFetchedItem` and `ingestItem` (17, each gains a `utils.ImportCapabilities{Images: true, Audio: true}` argument unless the test is about capabilities): `datasource_result_cap_test.go:24,47`, `datasource_service_test.go:552,574,588,703,800`, `datasource_sweep_wiring_test.go:123,176,210,255,279,302,336,347`, `knowledge_create_gitlab_test.go:56`; `TestApplyFetchedItem_EmbeddedImageIngestFailureCountsAsSkip` (`datasource_sweep_wiring_test.go:318-352`) passes `Images: true` so its control case still fails at create rather than being skipped by the precheck
- Create: `internal/application/service/datasource_import_capabilities_test.go`, using `sweepFakeKS` and `sweepFakeRepo` (`datasource_sweep_wiring_test.go:17-97`), `deletionLookupKnowledgeRepo` (`datasource_service_test.go:324`) and `newStreamHandler` (`datasource_stream_test.go:65`, a keyed literal)

**Interfaces:**

- Consumes: Task 2's `Imports`, `Importable`, `SyncCursor.Imports`, `kb.ImportCapabilities()`; Task 1's `MissingModel`, `Gained`.
- Produces: `func (s *DataSourceService) connectorConfig(ctx context.Context, ds *types.DataSource) (*types.DataSourceConfig, error)` (parse; load the knowledge base through `s.kbService.GetKnowledgeBaseByID(ctx, ds.KnowledgeBaseID)`; set `Imports`; when the knowledge base cannot be loaded, log a warning and return the parsed config with full capabilities `{Images: true, Audio: true}`, so a picker never hides files for want of a lookup); `ListResourcesForConfig(ctx, config, kbID, parentID)` sets `config.Imports` from the knowledge base when `kbID != ""` and to full capabilities otherwise; `applyFetchedItem(ctx, ds, imports, item, tagIDs, result)` and `ingestItem(ctx, ds, imports, item, tagIDs)`; `var errUnimportable = errors.New("file type needs a model the knowledge base lacks")`; the precheck in `ingestItem` runs only for items with content (`len(item.Content) > 0`, the URL route keeps auto-detecting its type) and only for a missing model: `if m := utils.MissingModel(path.Ext(item.FileName), imports); m != "" { logger.Infof(ctx, "skipping %q: a %s model is not configured for %s files", item.FileName, m, path.Ext(item.FileName)); return false, errUnimportable }`; `applyFetchedItem` counts `errUnimportable` as `Skipped`; `func startCursorFor(stored *types.SyncCursor, imports utils.ImportCapabilities) *types.SyncCursor` returns nil when `stored != nil && stored.Imports != nil && imports.Gained(*stored.Imports)` (logging `"import capabilities gained since the last run, starting a full sync"`) and `stored` otherwise; `processSyncStreaming` applies it to the result of `streamStartCursor`; the batch path (`:736`) is deliberately not compared, since every connector that filters by extension streams; every persisted cursor carries `Imports` because `withPending` stamps it.

- [ ] **Step 1: Write the failing tests**

```go
func TestIngestItemSkipsUnimportableBeforeDeleting(t *testing.T) {
	// sweepFakeRepo seeded so FindByDataSourceExternalID returns an existing row; a png item with
	// Content; imports {}: ingestItem returns errUnimportable and the repo recorded no delete.
}

func TestIngestItemPrecheckIgnoresURLOnlyItems(t *testing.T) {
	// a URL-only item with no FileName and imports {}: no errUnimportable; CreateKnowledgeFromURL is called.
}

func TestApplyFetchedItemCountsUnimportableAsSkipped(t *testing.T) {
	// result.Skipped == 1, Failed == 0, no SyncItemError recorded.
}

func TestStartCursorForForcesFullSyncOnlyAgainstARecordedValue(t *testing.T) {
	// nil stored: nil. stored.Imports nil, kb with VLM: stored returned unchanged.
	// stored.Imports {}, kb with VLM: nil. stored.Imports {Images:true}, kb without VLM: stored unchanged.
}

func TestWithPendingStampsImports(t *testing.T) {
	// runDeletions{imports: {Images:true}}: withPending(nil).Imports == &{Images:true}; and the
	// connector's cursor passed in gets the same stamp.
}

func TestListResourcesForConfigLoadsTheKnowledgeBase(t *testing.T) {
	// a stub connector records the config it receives; with kbID of a KB without VLM, Imports.Images is
	// false; with kbID "" it is true.
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/application/service/ -run 'TestIngestItem|TestApplyFetchedItemCountsUnimportable|TestStartCursorFor|TestWithPendingStamps|TestListResourcesForConfigLoads'`
Expected: FAIL to compile, undefined `errUnimportable`, `startCursorFor` and the new parameters.

- [ ] **Step 3: Implement `connectorConfig`, the call sites, the wizard id, the precheck, `startCursorFor`, the stamping, and update the 17 existing calls**

- [ ] **Step 4: Run the service and handler tests and the frontend check**

Run: `go test ./internal/application/service/ ./internal/handler/ && cd frontend && npx vue-tsc --noEmit 2>/dev/null || npm run type-check`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/service internal/handler/datasource.go frontend/src/api/datasource/index.ts frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue
git commit -m "feat(datasource): skip what the knowledge base cannot import and resync when it can"
```

### Task 4: Connectors filter through the config

**Files:**

- Modify: `internal/datasource/connector/github/connector.go:34-56` (`Connector` gains `imports utils.ImportCapabilities`, set from `ds.Imports` in `configured`; `func (c *Connector) importable(path string) bool`), `github/snapshot.go:41-66` (`snapshot.add` takes `importable func(string) bool`; `snapshot` and `walkSubtrees` pass `c.importable`)
- Modify: `internal/datasource/connector/gitlab/connector.go:18-39` (the same `imports` field, `configured` and `importable` method as GitHub), `:188,193,196,291,298,303,366` (`repofile.IsSupported(x)` becomes `c.importable(x)`; `walkFiles` is a method and uses `c.importable`)
- Modify: `internal/datasource/connector/seafile/connector.go:127-150` (`listDirectory` gains `importable func(string) bool`, passed from `ListResources` as `ds.Importable`), `seafile/sync.go:24-37` (`syncRun` gains `importable func(string) bool`, set in `runSync` from `ds.Importable`), `:194` and `:246` (use `r.importable(e.Name)`), `:221-250` (`walkDir`: a file that fails `importable` but is present in `r.cur.Files` is marked `r.seen[p] = true` without being fetched, so `reconcile` keeps it; the comment says a lost model must not delete what was imported under it)
- Delete: `repofile.IsSupported` in `internal/datasource/connector/repofile/repofile.go:66-71` and its tests in `repofile_test.go:33-38` and `gitlab/client_test.go:305-321`
- Test: `github/connector_test.go` (a helper `func withImports(cfg *types.DataSourceConfig, caps utils.ImportCapabilities) *types.DataSourceConfig`), `gitlab/connector_recursion_test.go` (its inline `&types.DataSourceConfig{...}` literals at `:72,150,216` gain `Imports: utils.ImportCapabilities{Images: true, Audio: true}`), `seafile/sync_test.go`, `seafile/connector_test.go`

**Interfaces:**

- Consumes: `(*types.DataSourceConfig).Importable` and `.Imports` from Task 2.

- [ ] **Step 1: Write the failing tests**

```go
// github/connector_test.go
func TestFetchStreamFetchesImagesOnlyWithVLM(t *testing.T) {
	// tree: docs/a.md, docs/shot.png. Without Images: summary has only +docs/a.md and server.log has no
	// /git/blobs/ request for shot.png's blob. With Images: both items, and the blob request is logged.
}

func TestFetchStreamLostCapabilityEmitsNoTombstones(t *testing.T) {
	// cursor at head c1 recorded while both files were importable; new head c2 changes docs/a.md only;
	// Imports{}: the run emits +docs/a.md and no -docs/shot.png.
}

// gitlab/connector_recursion_test.go
func TestNestedDirectoriesSkipAudioWithoutASR(t *testing.T) {}

// seafile/sync_test.go
func TestWalkDirSkipsImagesWithoutVLM(t *testing.T) {}
func TestWalkDirKeepsCursorFilesHiddenByALostCapability(t *testing.T) {
	// cursor lists /docs/shot.png; the listing still has it; Imports{}: no fetch, no deletion emitted,
	// and the returned cursor still lists it.
}

// seafile/connector_test.go
func TestListResourcesHidesImagesWithoutVLM(t *testing.T) {}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/datasource/connector/github/ ./internal/datasource/connector/gitlab/ ./internal/datasource/connector/seafile/ -run 'VLM|ASR|LostCapability'`
Expected: FAIL (images and audio are fetched regardless, or the Seafile reconcile tombstones the hidden file).

- [ ] **Step 3: Migrate the eleven call sites, thread `importable` into the GitHub snapshot, the GitLab connector and the Seafile run and listing, apply the Seafile keep rule, delete `repofile.IsSupported` and its tests**

- [ ] **Step 4: Run every connector package and lint**

Run: `go test ./internal/datasource/... && golangci-lint run --new-from-rev=HEAD~3 ./...`
Expected: PASS; `grep -rn "IsSupported(" internal/datasource` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/datasource/connector
git commit -m "feat(connectors): filter repository and Seafile files by the knowledge base's import capabilities"
```

### Task 5: Docs, changelog and live verification

**Files:**

- Modify: `website-docs/03-features/10-datasource.md` (after the deletion-detection sentence at `:147`: a paragraph on what a sync skips, the skipped count, the forced full sync on a gained model, that it re-imports every document of every streaming data source on that knowledge base once, and removal of now-unimportable items only by a full-mode sync with deletion sync on)
- Modify: `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md` (a new paragraph in the fetch guidance: filter candidate files through `ds.Importable`, never a private extension list)
- Modify: `docs/superpowers/specs/2026-10-22-bitbucket-connector-design.md:38` ("have a supported extension" becomes "pass `ds.Importable`") and `:12` (`IsSupported` dropped from the repofile list)
- Modify: `docs/superpowers/specs/2026-10-22-sync-import-capabilities-and-mdx.md` decisions 1, 3, 5 and 6 as this plan's header states
- Modify: `CHANGELOG.md` under `## [Unreleased]` `### Changes` (the heading exists once the MDX plan has landed)

- [ ] **Step 1: Write the docs, the spec amendments and the changelog entry**

Changelog line: `- **CHANGED**: Data-source syncs fetch only what the knowledge base can ingest. GitHub, GitLab and Seafile skip images when the knowledge base has no vision model and audio when it has no speech model, instead of downloading them and failing; the picker hides them too. A file skipped on ingest is counted as skipped and never replaces an existing item. Configuring a model later makes the next sync of each data source on that knowledge base a full one, so the skipped files are fetched.`

- [ ] **Step 2: Run validate**

Run: `scripts/validate.sh`
Expected: every listed area passed.

- [ ] **Step 3: Commit**

```bash
git add website-docs/03-features/10-datasource.md internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md docs/superpowers/specs CHANGELOG.md
git commit -m "docs(datasource): capability-aware sync filtering"
```

- [ ] **Step 4: Live check on the dev stack**

A GitHub data source over a directory holding `a.md` and `shot.png`, in a knowledge base without a vision model: sync once.
Expected: the sync log shows one created, one skipped, zero failed; the server log has no blob request for `shot.png`.
Open the create wizard's picker for a Seafile source on the same knowledge base.
Expected: image files are not listed; on a knowledge base with a vision model they are.
Configure a vision model, sync again.
Expected: the server log says a full sync starts because capabilities were gained; `shot.png` is created.
Remove the model, sync again (incremental).
Expected: nothing deleted. Run a full-mode sync with deletion sync on.
Expected: `shot.png` is removed and the log names it under the reconcile.
