# Source-Identity Dedup Capability and Shared Channel Labels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adding a connector becomes one registry entry plus one label line: the duplicate-check scope comes from the connector registry instead of a hand-kept switch, the capability test is a truth table that demands a row per type, and every view that names an ingestion channel reads one shared table that a parity test holds to the backend's constants.

**Architecture:** A new capability `source_identity_dedup` on `ConnectorMetadataRegistry` replaces `usesSourceIdentityDuplicateCheck`; the knowledge create path asks `datasource.HasCapability`. The registry test becomes one table of the three behavior-changing capabilities per type. On the frontend, `frontend/src/utils/channelLabel.ts` holds the channel-to-i18n-key table, in source-filter order, plus the set of upload channels the filter labels on its own; the five views import it and lose their private copies.

**Tech Stack:** Go 1.26 (`GOTOOLCHAIN=go1.26.10` to match CI), testify where the surrounding file already uses it; Vue 3 + TypeScript, vue-i18n, node test runner via tsx (`npm --prefix frontend test`), `vue-tsc` (`npm --prefix frontend run type-check`).

**Spec:** none. The design is in this document (below). It was validated against the code on 2026-10-22 and reviewed by two independent readers (opus, sonnet) the same day; every finding they raised that survived re-verification is folded in.

## Design

What exists:

- `usesSourceIdentityDuplicateCheck` (`internal/application/service/knowledge_create.go:1155`) names GitHub, Bitbucket, GitLab, Confluence and Seafile. For those channels `CreateKnowledgeFromFile` scopes the file-hash duplicate check to `datasource_id` + `external_id` (`knowledge_create.go:103-109`). The repository only applies that scope when both values are set (`repository/knowledge.go:440`). Upstream shipped the switch with three names; the two git connectors each added theirs by hand.
- Four callers reach `CreateKnowledgeFromFile`. The sync path (`datasource_service.go:1455-1465`) passes `ds.Type`, or the item's `metadata["channel"]` when set, and is the only caller that supplies `datasource_id` and `external_id`. The HTTP handler (`handler/knowledge.go:416`) and the CLI (`cli/cmd/doc/upload.go`, default `api`) pass a free-form string, and the IM service maps platforms to wechat, wecom, feishu, dingtalk, slack or im (`im/service.go:3521`). A free-form `channel=github` on an upload reaches the scoped branch today and after this change alike, and scopes nothing because the upload has no source identity. The five connectors in the set each emit their own type as the channel (`github/connector.go:563`, `bitbucket/connector.go:513`, `gitlab/connector.go:421`, `confluence/connector.go:584`, `seafile/sync.go:445`), so a lookup by channel equals a lookup by type for them. The Lark wiki connector emits `feishu` (`feishu/wiki/connector.go:336`), which is why the capability comment must state that a connector declaring it emits its own type.
- `TestReconcileCapabilitiesMatchConnectors` (`internal/datasource/connector_test.go:15`) restates the registry as a `reconciles` map and derives `selection_tracks_source` as "reconciles, except Seafile, GitHub and Bitbucket". Its comment (`connector_test.go:10-14`) carries the one non-obvious why: GitHub and Bitbucket keep their selection in `settings.repositories`, not in the resource IDs the deselection count reads.
- Five Vue files each keep a channel-to-label table and disagree: `DocumentInfo.vue:105` and `DocumentCardView.vue:150` lack `notion`, `yuque`, `feishu_drive` and `lark_drive`, so those documents show "Unknown" in the card hover and the chat document panel; `doc-content.vue:981` (the document detail drawer) lacks `notion`, `yuque` and `rss`; `DocumentListView.vue:95` is an `if` chain that knows every channel but `rss`; `KnowledgeBase.vue:680` is the source filter and lacks `lark_drive` and `rss`. No table knows `rss` and `en-US.ts` has no `channelRss`, so RSS documents show "Unknown" or "Upload" everywhere. All five index a plain object, so a stored channel of `constructor` or `toString` would hand a function to `t()`. The backend filters `channel = ?` for any source value that is not `url` or `manual` (`repository/knowledge.go:161-166`), so new filter options need no backend change. `usermsg.vue:175` holds a chat-message channel table under `chat.*` keys, a different domain, and is left alone.
- The three map views hide the channel row when it is `web` (`DocumentCardView.vue:633`, `DocumentInfo.vue:48`, `doc-content.vue:1674`), so an uploaded document shows no channel there.
- `internal/application/service` already imports `internal/datasource` (`datasource_service.go:19`), and `internal/datasource` imports nothing under `internal/application`, so the service can read the registry.
- The only frontend reader of the metadata `capabilities` array is `DataSourceEditorDialog.vue:1318`, which tests for `full_sync_reconcile` alone, so a new capability string in the API payload changes nothing on screen. `client/`, `cli/`, `mcp-server/`, `extension/src` and the swagger files do not enumerate capability values.
- The comment at `datasource_service.go:1450-1454` says Feishu Drive sets the channel to `feishu`; it sets `feishu_drive` or `lark_drive` (`feishu/drive/connector.go:370-372`).

Decisions:

1. **One capability, not an inference.** `source_identity_dedup` is declared per type. It cannot be derived from `hierarchical` (Confluence has dedup but is flat) or from `full_sync_reconcile` (GitLab has dedup but no reconcile). Its comment states the invariant the lookup rests on: a connector that declares it emits its own type as `metadata["channel"]`, or no channel at all.
2. **The registry test is a truth table.** One row per registered type with the three behavior-changing capabilities; a type with no row fails, a row with no type fails. The old test's why-comment moves onto the rows it explains. The one real invariant, `selection_tracks_source` implies `full_sync_reconcile`, is its own test.
3. **One frontend table and one set.** `CHANNEL_LABEL_KEYS` maps every channel value to its key and is written in source-filter order; `UPLOAD_CHANNELS` is the set the filter labels on its own ("Upload", "API", "Browser Extension") and the list view sends to the type fallback. The filter's connector options are derived: the keys minus the set. Labels stay i18n keys, not registry `Name`s, because half the channels are not connectors. Lookups are own-property only.
4. **The list view keeps its type fallback.** For a channel outside `UPLOAD_CHANNELS` with a label it shows the sync icon and the label, then falls back to the knowledge type (url, manual, upload) as today. RSS documents gain their label and the sync icon there; nothing else changes in that view.
5. **Parity is tested across the language boundary**, as `syncreason/parity_test.go` already does from Go: the TypeScript test reads the `Channel*` constants in `internal/types/knowledge.go` and the `ConnectorType*` constants in `internal/types/datasource.go`, and fails when any value lacks a label, except the five registry types that are not channels (Lark, whose wiki emits `feishu`; Google Drive, OneDrive, the web crawler and IMAP, which have metadata entries and no connector). A new git connector that follows GitHub and emits its `ConnectorType*` constant is caught.

## Global Constraints

- English only; no CJK text; no em dashes in prose, comments or commit messages (`scripts/check-emdash.sh`).
- Commit subjects `type(scope): imperative subject`; `git mv` for renames (none here).
- Work on a branch from `main`; never commit to `main`.
- New UI text is an `en-US.ts` key; product name "AI KB".
- `scripts/validate.sh --base main` must pass before the PR; the pre-commit hook runs `golangci-lint --new-from-rev=HEAD` over the root module when Go files are staged.
- Comments state a non-obvious why only.
- A doc claim a reader can act on cites code that matches it; after the code change, grep tracked Markdown for the removed names and run the `docs-drift-reviewer` agent on the diff.

## Review Focus

1. An upload or IM document never carries a registry type as its channel except by a client sending one deliberately, and then it carries no source identity, so the duplicate check stays knowledge-base-wide. Pinned by the existing negative cases at `knowledge_create_gitlab_test.go:76-82` (channels `""` and `feishu`, and GitLab without an identity), which must pass unchanged.
2. A connector type that is not a channel (Lark emits `feishu`) must not be demanded a label, and a connector type that is one must. Pinned by the `TYPES_WITHOUT_A_CHANNEL` allowlist in Task 2's parity test.
3. A stored channel that is an object prototype property (`constructor`, `toString`) or empty must label as Unknown and must not reach `t()` as a function. Pinned in `channelLabel.test.ts`. The empty case is defensive: the column defaults to `web` and the views guard with `v-if`, but the label function is the one place every view goes through.
4. The source filter must still send `web` for the Upload option, which the backend maps to `channel = 'web'`; keeping the upload options out of the derived list must not reorder or drop them. Pinned by the `UPLOAD_CHANNELS` assertion in Task 2 and the manual check in Task 2 Step 9.
5. The existing scoped-dedup behavior tests (`TestCreateKnowledgeFromFileGitHubUsesSourceIdentity`, `...BitbucketUsesSourceIdentity`, `...ConfluencePreservesCopiedPages`, `...GitLabPreservesSourcePaths`, `TestProcessSync_SeafileSameContentDifferentPathsStayIndependent`) must pass unchanged; they are the proof that the capability lookup reproduces the switch.

---

### Task 1: Registry capability for source-identity dedup

**Files:**

- Modify: `internal/datasource/connector.go:184-199` (constants), registry entries for `ConnectorTypeConfluence`, `ConnectorTypeGitHub`, `ConnectorTypeBitbucket`, `ConnectorTypeGitLab`, `ConnectorTypeSeafile`
- Modify: `internal/datasource/connector_test.go:10-34`
- Modify: `internal/application/service/knowledge_create.go:103-109,1155-1163`
- Modify: `internal/application/service/datasource_service.go:1450-1454` (comment only)
- Modify: `internal/application/service/datasource_seafile_test.go:352`
- Modify: `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md:416`

**Interfaces:**

- Produces: `const CapabilitySourceIdentityDedup = "source_identity_dedup"` in package `datasource`, declared by the five types above. Readable through the existing `HasCapability(connectorType, capability string) bool`.

- [ ] **Step 1: Branch**

```bash
git checkout -b refactor/connector-capability-channel-labels main
```

- [ ] **Step 2: Replace the registry test with the truth table and the invariant**

Delete `TestReconcileCapabilitiesMatchConnectors` and its comment (`connector_test.go:10-34`). Add:

```go
// The truth table of the behavior-changing capabilities. A new connector type
// needs a row here. GitHub and Bitbucket reconcile but do not track their
// selection: they keep it in settings.repositories, not in the resource IDs
// the deselection count reads; Seafile stores the library root while the
// selection may name folders inside it.
func TestCapabilityTableMatchesRegistry(t *testing.T) {
	type row struct{ reconcile, tracksSource, sourceIdentityDedup bool }
	want := map[string]row{
		types.ConnectorTypeFeishu:      {reconcile: true, tracksSource: true},
		types.ConnectorTypeLark:        {reconcile: true, tracksSource: true},
		types.ConnectorTypeFeishuDrive: {reconcile: true, tracksSource: true},
		types.ConnectorTypeLarkDrive:   {reconcile: true, tracksSource: true},
		types.ConnectorTypeDingTalk:    {reconcile: true, tracksSource: true},
		types.ConnectorTypeConfluence:  {reconcile: true, tracksSource: true, sourceIdentityDedup: true},
		types.ConnectorTypeSeafile:     {reconcile: true, sourceIdentityDedup: true},
		types.ConnectorTypeGitHub:      {reconcile: true, sourceIdentityDedup: true},
		types.ConnectorTypeBitbucket:   {reconcile: true, sourceIdentityDedup: true},
		types.ConnectorTypeGitLab:      {sourceIdentityDedup: true},
		types.ConnectorTypeNotion:      {},
		types.ConnectorTypeYuque:       {},
		types.ConnectorTypeIMA:         {},
		types.ConnectorTypeGoogleDrive: {},
		types.ConnectorTypeOneDrive:    {},
		types.ConnectorTypeWebCrawler:  {},
		types.ConnectorTypeSlack:       {},
		types.ConnectorTypeIMAP:        {},
		types.ConnectorTypeRSS:         {},
	}
	for connectorType := range ConnectorMetadataRegistry {
		r, ok := want[connectorType]
		if !ok {
			t.Errorf("%q has no row in the capability table", connectorType)
			continue
		}
		got := row{
			reconcile:           HasCapability(connectorType, CapabilityFullSyncReconcile),
			tracksSource:        HasCapability(connectorType, CapabilitySelectionTracksSource),
			sourceIdentityDedup: HasCapability(connectorType, CapabilitySourceIdentityDedup),
		}
		if got != r {
			t.Errorf("%q capabilities = %+v, want %+v", connectorType, got, r)
		}
	}
	for connectorType := range want {
		if _, ok := ConnectorMetadataRegistry[connectorType]; !ok {
			t.Errorf("table row %q is not a registered connector type", connectorType)
		}
	}
}

func TestSelectionTracksSourceImpliesFullSyncReconcile(t *testing.T) {
	for connectorType := range ConnectorMetadataRegistry {
		if HasCapability(connectorType, CapabilitySelectionTracksSource) &&
			!HasCapability(connectorType, CapabilityFullSyncReconcile) {
			t.Errorf("%q tracks its selection but cannot reconcile a full sync", connectorType)
		}
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/datasource/ -run 'TestCapabilityTableMatchesRegistry|TestSelectionTracksSource' -v`
Expected: compile error, `undefined: CapabilitySourceIdentityDedup`.

- [ ] **Step 4: Declare the capability and add it to the five registry entries**

In `internal/datasource/connector.go`, after `CapabilitySelectionTracksSource`:

```go
// CapabilitySourceIdentityDedup marks a connector whose items may carry the
// same bytes at different paths (README templates in a repository, copied
// Confluence pages), so the create path scopes its duplicate check to
// datasource_id + external_id instead of the whole knowledge base. The create
// path looks it up by the item's channel, so a connector that declares it
// emits its own type as metadata["channel"], or no channel at all.
const CapabilitySourceIdentityDedup = "source_identity_dedup"
```

Append `CapabilitySourceIdentityDedup` to the `Capabilities` slice of `ConnectorTypeConfluence`, `ConnectorTypeGitHub`, `ConnectorTypeBitbucket`, `ConnectorTypeGitLab` and `ConnectorTypeSeafile`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/datasource/ -v -run 'Capability|SelectionTracks'`
Expected: PASS for both.

- [ ] **Step 6: Replace the switch with the registry lookup**

In `internal/application/service/knowledge_create.go`: import `"github.com/jrmatherly/ai-kb/internal/datasource"`; at line 106 replace `usesSourceIdentityDuplicateCheck(channel)` with `datasource.HasCapability(channel, datasource.CapabilitySourceIdentityDedup)`; delete `func usesSourceIdentityDuplicateCheck`. Keep the existing comment above the `if`.

In `internal/application/service/datasource_seafile_test.go:352` replace `usesSourceIdentityDuplicateCheck(created.channel)` with `datasource.HasCapability(created.channel, datasource.CapabilitySourceIdentityDedup)` (the file already imports `datasource`).

In `internal/application/service/datasource_service.go:1450-1454` correct the comment's example: Feishu Drive sets the channel to `feishu_drive` or `lark_drive` so Drive documents get their own label, distinct from the wiki's "Feishu".

- [ ] **Step 7: Run the behavior tests that prove the lookup reproduces the switch**

Run: `go test ./internal/application/service/ -run 'TestCreateKnowledgeFromFile|TestProcessSync_Seafile' -v`
Expected: PASS, including the five tests named in Review Focus 5 and the negative cases in `TestCreateKnowledgeFromFileGitLabPreservesSourcePaths`.

Run: `go build ./... && go vet ./internal/datasource/... ./internal/application/service/...`
Expected: no output.

- [ ] **Step 8: Document the capability in the connector guide**

In `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md:416` change "Two capability values change behavior." to "Three capability values change behavior." and, after the `selection_tracks_source` sentence, add: "`source_identity_dedup` (`CapabilitySourceIdentityDedup`) says two items of the connector may carry the same bytes at different paths, so the create path scopes its duplicate check to `datasource_id` + `external_id`; declare it for any connector that syncs files from a tree, and have such a connector emit its own type as `metadata["channel"]` (or none), since the create path looks the capability up by channel."

- [ ] **Step 9: Commit**

```bash
git add internal/datasource/connector.go internal/datasource/connector_test.go internal/application/service/knowledge_create.go internal/application/service/datasource_service.go internal/application/service/datasource_seafile_test.go internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md
git commit -m "refactor(datasource): declare source-identity dedup as a registry capability"
```

### Task 2: One channel-label table for the frontend

**Files:**

- Create: `frontend/src/utils/channelLabel.ts`
- Create: `frontend/src/utils/channelLabel.test.ts`
- Modify: `frontend/src/i18n/locales/en-US.ts:805` (add `channelRss`)
- Modify: `frontend/src/components/doc-content.vue:981-1006`
- Modify: `frontend/src/views/chat/components/tool-results/DocumentInfo.vue:105-125`
- Modify: `frontend/src/views/knowledge/components/DocumentCardView.vue:150-170`
- Modify: `frontend/src/views/knowledge/components/DocumentListView.vue:95-118`
- Modify: `frontend/src/views/knowledge/KnowledgeBase.vue:680-702`
- Modify: `internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md:422-426` (Step 9)
- Modify: `CHANGELOG.md` (`[Unreleased]`)

**Interfaces:**

- Produces, in `frontend/src/utils/channelLabel.ts`:
  - `export const UPLOAD_CHANNELS: ReadonlySet<string>` = `web`, `api`, `browser_extension`.
  - `export const CHANNEL_LABEL_KEYS: Record<string, string>`, every channel value mapped to its `knowledgeBase.channel*` key, written in this order (the source filter's order after its own upload entries): web, api, browser_extension, feishu, feishu_drive, lark_drive, notion, yuque, confluence, gitlab, github, bitbucket, seafile, ima, rss, wechat, wecom, dingtalk, slack, im.
  - `export function channelLabelKey(channel: string | undefined): string`, the key for an own property of the table, else `'knowledgeBase.channelUnknown'`. Uses `Object.hasOwn`.
  - `export function sourceFilterChannels(): string[]`, the table's keys minus `UPLOAD_CHANNELS`, in table order.

- [ ] **Step 1: Write the failing test**

`frontend/src/utils/channelLabel.test.ts`, node test runner, importing from `./channelLabel.ts`, importing `LOCALE_BUNDLES` and `getLocaleValueAtPath` from `../i18n/localeKeyAudit.ts`, and reading the two Go files through `fileURLToPath(new URL('../../../internal/types/knowledge.go', import.meta.url))` and the same for `datasource.go`:

```ts
// Registry types no document ever carries as its channel: the Lark wiki emits
// "feishu", and the other four have a metadata entry but no connector.
const TYPES_WITHOUT_A_CHANNEL = new Set(['lark', 'google_drive', 'onedrive', 'web_crawler', 'imap'])

const goConstants = (file: string, prefix: string) =>
  [...readFileSync(file, 'utf8').matchAll(new RegExp(`^\\t${prefix}\\w+\\s+=\\s+"([a-z_]+)"`, 'gm'))].map((m) => m[1])

test('every channel label key resolves in en-US', () => {
  for (const [channel, key] of Object.entries(CHANNEL_LABEL_KEYS)) {
    assert.equal(typeof getLocaleValueAtPath(LOCALE_BUNDLES['en-US'], key), 'string', `${channel}: ${key}`)
  }
})

test('every backend channel and connector type has a label', () => {
  const channels = goConstants(KNOWLEDGE_GO, 'Channel')
  const types = goConstants(DATASOURCE_GO, 'ConnectorType').filter((t) => !TYPES_WITHOUT_A_CHANNEL.has(t))
  assert.ok(channels.length >= 17 && types.length >= 14, `parsed ${channels.length} channels, ${types.length} types`)
  assert.deepEqual([...channels, ...types].filter((c) => !Object.hasOwn(CHANNEL_LABEL_KEYS, c)), [])
})

test('the source filter offers every labelled channel except the upload ones', () => {
  const offered = sourceFilterChannels()
  assert.deepEqual(offered.filter((c) => UPLOAD_CHANNELS.has(c)), [])
  assert.deepEqual(offered.slice(0, 3), ['feishu', 'feishu_drive', 'lark_drive'])
  assert.equal(offered.length, Object.keys(CHANNEL_LABEL_KEYS).length - UPLOAD_CHANNELS.size)
})

test('an unknown, empty or prototype channel is Unknown', () => {
  assert.equal(channelLabelKey('github'), 'knowledgeBase.channelGitHub')
  for (const bad of ['nope', '', 'constructor', 'toString', undefined]) {
    assert.equal(channelLabelKey(bad), 'knowledgeBase.channelUnknown', String(bad))
  }
})
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd frontend && npx tsx --test src/utils/channelLabel.test.ts`
Expected: FAIL, cannot find module `./channelLabel.ts`.

- [ ] **Step 3: Add the `channelRss` key and write the module**

In `en-US.ts`, after `channelSeafile: 'Seafile',` add `channelRss: 'RSS',`.

Write `frontend/src/utils/channelLabel.ts` with the four exports from the Interfaces block. One comment at the top: the table mirrors `types.Channel*` in `internal/types/knowledge.go` and the connector types that documents carry as a channel; `channelLabel.test.ts` fails when they drift.

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd frontend && npx tsx --test src/utils/channelLabel.test.ts`
Expected: PASS, 4 tests.

- [ ] **Step 5: Migrate the three map-based views**

In `doc-content.vue`, `DocumentInfo.vue` and `DocumentCardView.vue`: delete the `channelLabelMap` constant; import `channelLabelKey` from `@/utils/channelLabel`; the body of `getChannelLabel` becomes `t(channelLabelKey(channel))`. Keep the function name so the templates do not change.

- [ ] **Step 6: Migrate the list view and the source filter**

In `DocumentListView.vue`, import `UPLOAD_CHANNELS` and `channelLabelKey`, and replace the `if (ch === ...)` chain in `getSourceInfo` with:

```ts
const ch = item.channel;
if (ch && !UPLOAD_CHANNELS.has(ch)) {
  const key = channelLabelKey(ch);
  if (key !== 'knowledgeBase.channelUnknown') return { icon: 'cloud-download', label: t(key) };
}
```

followed by the unchanged `url`, `manual` and upload fallbacks.

In `KnowledgeBase.vue`, keep the first six `sourceOptions` entries (all sources, upload, url, manual, api, browser extension) and replace the fifteen connector entries with `...sourceFilterChannels().map((value) => ({ label: t(channelLabelKey(value)), value }))`.

- [ ] **Step 7: Type-check and run the frontend tests**

Run: `npm --prefix frontend run type-check && npm --prefix frontend test`
Expected: both exit 0.

- [ ] **Step 8: Update the guide and the changelog**

In `CONNECTOR_IMPLEMENTATION_GUIDE.md` Step 9 (line 422), add a third bullet: "Add the connector's channel to `CHANNEL_LABEL_KEYS` in `frontend/src/utils/channelLabel.ts`, in the position the source filter should list it, with a `knowledgeBase.channel*` key in `en-US.ts`; `channelLabel.test.ts` fails while a `types.Channel*` or `types.ConnectorType*` constant has no label."

In `CHANGELOG.md` under `[Unreleased]`, add a `### Bug Fixes` section above `### Changes` with:

- `- **FIXED**: Documents synced from Notion, Yuque, Feishu Drive and Lark Drive show their source in the card hover, the document detail drawer and the chat document panel instead of "Unknown"; RSS documents are labelled "RSS" everywhere; Lark Drive and RSS appear in the knowledge base source filter. Every view now reads one channel table, which a test holds to the backend's channel constants.`

- [ ] **Step 9: Check the surfaces in the browser**

Start the dev stack (`./scripts/dev.sh start --minio`, `SERVER_PORT=8081 ./scripts/dev.sh app`, `VITE_DEV_PROXY_TARGET=http://localhost:8081 ./scripts/dev.sh frontend`; run `lsof -nP -iTCP:8081 -sTCP:LISTEN` first). In a knowledge base with one document uploaded from the UI and one uploaded with `channel=api` (the upload form field, or `aikb doc upload --channel api`): the list view shows "Upload" for both; the card hover shows no channel line for the UI upload and "API" for the other; the source filter lists Lark Drive and RSS after Feishu Drive and Tencent IMA; selecting "Upload" still filters and the request carries `source=web`. Stop the stack.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/utils/channelLabel.ts frontend/src/utils/channelLabel.test.ts frontend/src/i18n/locales/en-US.ts frontend/src/components/doc-content.vue frontend/src/views/chat/components/tool-results/DocumentInfo.vue frontend/src/views/knowledge/components/DocumentCardView.vue frontend/src/views/knowledge/components/DocumentListView.vue frontend/src/views/knowledge/KnowledgeBase.vue internal/datasource/CONNECTOR_IMPLEMENTATION_GUIDE.md CHANGELOG.md
git commit -m "fix(ui): read every channel label from one table"
```

- [ ] **Step 11: Docs drift, repository gates and the PR**

Run: `git grep -n 'usesSourceIdentityDuplicateCheck\|channelLabelMap\|TestReconcileCapabilitiesMatchConnectors' -- '*.md' ':!docs/superpowers/plans/completed' ':!docs/superpowers/specs/completed'`
Expected: no hits outside this plan.

Run the `docs-drift-reviewer` agent on `git diff main` and fix what it finds in a `docs:` commit.

Run: `scripts/validate.sh --base main`
Expected: every area passes.

Open the PR against `main`. Title `refactor: registry capability for source-identity dedup and shared channel labels`. Body: the two commit subjects, the "What exists" bullets from the Design section as the motivation, the FIXED line, and `🤖 Generated with [Claude Code](https://claude.com/claude-code)`.
