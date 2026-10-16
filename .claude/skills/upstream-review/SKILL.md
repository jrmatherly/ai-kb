---
name: upstream-review
description: Review what changed in upstream Tencent/WeKnora (main) since our last review and decide what to adopt into this copied repo. Use for "sync from upstream", "what changed upstream", "review upstream commits", "did WeKnora update", "bring upstream commit X in", "check upstream for new features", or any periodic upstream check. Encodes the copy-not-fork reality, the watermark driver, the per-file divergence markers, the English-only rule and the adoption rules for this repo.
argument-hint: "[--stat | --urls | --show <sha> | --diverged | --advance <sha>]"
allowed-tools: Bash(${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh *) Bash(scripts/check-english-only.sh *) Bash(scripts/validate.sh *) Bash(scripts/rebrand/apply.sh *) Bash(scripts/rebrand/check.sh *) Bash(scripts/rebrand/check-repo.sh) Bash(git show *) Bash(git log *) Bash(git diff *) Bash(git rev-parse *) Read Grep Glob
---

# Upstream review

This repo is a copy of `Tencent/WeKnora` taken at upstream commit `97796c637` (2026-10-12, `v0.8.2-268-g97796c637`, "fix(helm): configure app rollouts and document shared storage (#4180)"). It shares no git history with upstream. At the copy the tree was byte-identical to that commit except `.github/` (removed; archived in `private/OLD-.github/`) and `.gitignore`. Run `--diverged` for the current footprint. So:

- `git merge` and `git merge-base` are useless. Every range is `<watermark>..upstream/main`.
- `git cherry-pick -x <sha>` works once `upstream/main` is fetched: the 3-way merge uses the commit's own parent as the base. A hunk applies cleanly when our copy of those lines equals the parent's (the pre-image), and conflicts otherwise. The merge is per hunk, so a file we changed elsewhere still takes a non-overlapping upstream hunk.
- The driver compares our HEAD with each commit's pre-image before you pick anything.

## Tooling

| Piece | Path |
| --- | --- |
| Driver | `${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh` |
| English-only check | `scripts/check-english-only.sh` |
| Em dash check | `scripts/check-emdash.sh` (upstream text carries em dashes; rewrite them in the pick) |
| Watermark (tracked) | `.upstream-review/watermark.json` (`seedSha` fixed, `lastReviewedSha` advances) |
| Decision logs (tracked) | `.upstream-review/reviews/<YYYY-MM-DD>-review.md` |

```bash
${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh            # fetch, list un-reviewed commits oldest first
${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh --stat     # + each touched file with a divergence marker
${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh --urls     # + github commit URL per commit
${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh --show <sha>
${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh --diverged # our footprint: files that differ from the seed
${CLAUDE_SKILL_DIR}/scripts/upstream-review.sh --advance <tip-sha>  # after the log is written; the listing prints the tip SHA
scripts/check-english-only.sh [-n | --summary] [pathspec...]   # tracked files; exit 1 while CJK text remains
scripts/check-english-only.sh --diff [<range>]                  # added lines only: staged by default, or <sha>..HEAD
```

The first run adds the `upstream` remote and seeds `upstream/main` from `private/OLD-.git` (the archived upstream clone, gitignored) so the network fetch is incremental. Without that directory it fetches the full history once. The driver needs a commit on our side: `HEAD:<path>` is what it compares.

Markers in `--stat`, our HEAD vs the commit's parent (pre-image) for each file it touches:

| | Meaning | Adoption path |
| --- | --- | --- |
| `=` | ours equals the pre-image | `git cherry-pick -x <sha>` applies cleanly |
| `<` | ours equals the seed, not the pre-image | an older un-reviewed upstream commit touched it first (pick in order), or the watermark sits below the seed and we already carry the post-image (ALREADY-HAVE) |
| `~` | ours differs from both | we changed it; resolve the 3-way or hand-port |
| `x` | we removed it | check "Deleted or moved paths" below |
| `+` | new upstream file | cherry-pick creates it |
| `bin` | binary | inspect by hand |

Merge commits are listed but not expanded: their parents are in the range. A conflict resolution that exists only in the merge is not shown.

## Method

1. Run `--stat`. Group commits by area, the conventional-commit scope upstream uses: `fix(agent):`, `feat(gallery):`, `fix(frontend):`, `docs:`, `fix(helm):`, `test(docreader):`.
2. For each non-merge commit, `--show <sha>` and read the diff against its own upstream parent.
3. Classify our state from the markers and the diff: ALREADY-HAVE, ABSENT, DIVERGED, PARTIAL.
4. Verdict: `ADOPT` (cherry-pick, all files `=`, `<` with its predecessors, or `+`), `ADAPT` (hand-port, some `~`), `SKIP`, `MONITOR`.
5. Write every verdict with file-level evidence to the dated decision log (template below).
6. Adopt in a batch on a branch: cherry-pick oldest first, run `scripts/validate.sh --base <sha-before-batch>` (below), which ends with the English-only gate on the range.
7. Then `--advance <tip-sha>` with the tip SHA from the listing you reviewed (a fresh fetch may have moved on) and commit `.upstream-review/` with the adopted commits. The user pushes; never push.

For a large window (more than about 20 commits), dispatch one reviewer subagent per area with the SHAs, the marker output, and steps 2 to 5. Synthesize into one log.

### Validation per area

`scripts/validate.sh --base <sha-before-batch>` runs, for each area the batch touched, the same commands as our CI in `.github/workflows/` (ported from upstream's): Go root module, `cli/`, `client/`, `frontend/`, `docreader/`, model catalog, git hooks, license bundle, `website-docs/` and the rest, then the English-only and em dash gates. `--list` names the areas without running them. A workflow change upstream that adds a command goes into the script and into our workflow.

## Rules for this repo

- Keep `-x` on every cherry-pick so the log line "cherry picked from commit" carries provenance. Upstream `#NNNN` issue and PR numbers autolink to the wrong repo on GitHub; rewrite them to `Tencent/WeKnora#NNNN` in the picked commit message (`git commit --amend`) and in the decision log.
- Upstream commits that touch `.github/` are never cherry-picked: ours is a rebranded, SHA-pinned port (GHCR, tag-only publishing) that diverges on every line. Read them for new or changed validation commands and port those by hand into `.github/workflows/` and `scripts/validate.sh`, keeping our conventions (`scripts/check-workflows.sh`). Record the decision in the log.
- **English only.** This copy carries no Chinese (or other CJK) text: comments, log and error messages, prompts, docs, UI copy, fixtures. The tree is CJK-free (`scripts/check-english-only.sh --summary` is empty), so any hit after a pick is new. Consequences for review:
  - Translated files show `~` for every upstream commit that touches them. That is expected. Cherry-pick anyway: hunks that do not overlap a translated line merge cleanly, and a conflict is confined to the lines both sides changed. Resolve by taking upstream's logic and our language.
  - A pick never lands CJK text. After a batch, `scripts/check-english-only.sh --diff <sha-before-batch>..HEAD` is the gate. Translate every hit before the batch lands, with the `translate-code` skill (its `translation-check` proof).
  - Upstream commits whose only content is Chinese prose (docs under `website-docs/`, `README_CN.md`, comment edits) are SKIP unless they document behavior we adopted; then write the English equivalent.
- Generated files are regenerated, never merged by hand. A conflict in one means the source changed; resolve the source, rerun the generator, `git add` the output:
  - `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` from `make docs` (swag over `cmd/server/main.go`; `docs/swagger_contract_test.go` checks it).
  - Model catalog output under `internal/models/catalog/` from `make model-catalog-generate` (`scripts/model-catalog/generate.py`; CI runs `--check`).
  - `docreader` protobuf stubs from `make -C docreader proto`.
- A watermark advance without a decision log is a lost review. Seed the watermark below any backlog you intend to review, never at today's tip.
- Commit subjects keep upstream's conventional-commit form, `type(scope): imperative subject`, so adopted commits read consistently in `git log`.

## Deleted or moved paths

An `x` marker means we removed the path, not always that the change is irrelevant.

- `.github/**`: replaced by our own CI, not removed; port command changes by hand (see above). Upstream's originals are archived in `private/OLD-.github/`.
- 36 paths renamed by `map.tsv` on 2026-10-12 (`cli/skills/weknora-*`, `packages/dsh-weknora`, `Formula/weknora-lite.rb`, `frontend/public/weknora-widget.js`, systemd units, `mcp-server/weknora_mcp_server.py`, `frontend/src/assets/img/weknora.png`): an upstream change to the old path is ported to the path the map gives it.
- `mcp-server/*.egg-info/` (2026-10-13): setuptools build metadata, regenerated from `mcp-server/README.md` and `pyproject.toml` by every build, so the tracked copy went stale and `validate.sh` dirtied the tree. Now gitignored; an upstream change there needs no port.
- Add a line here for every later removal, with where the content went or why it is gone.

## Version and changelog

- `CHANGELOG.md` stays as upstream left it, as history. New entries are ours; do not port upstream's entries. Record adopted SHAs in the decision log.
- Versioning is dated CalVer, `YYYY.M.D[.MICRO]` unpadded (decided 2026-10-12 as `~/dev/ai-workspace` uses; landed 2026-10-13 at `2026.10.13`, a second release that day is `2026.10.13.1`). Release tags are `vYYYY.M.D[.MICRO]`; `docker-image.yml` tags images with the tag and with the bare version through a `type=match` rule, since `type=semver` drops a four-part version. Every file that carries a version is a row in the `TOUCHPOINTS` table of `scripts/version.py`, the only place that list lives; a new one is a row there, never a hand edit. The product set (`VERSION`, `website-docs/VERSION`, the npm packages and lockfiles of `frontend/`, `website-docs/`, `website-docs/homepage/` and `miniprogram/`, `docreader` in `pyproject.toml` and `uv.lock`, `helm/Chart.yaml` `appVersion` v-prefixed as the default image tag, the desktop `productVersion` without MICRO, the README release badge) follows `VERSION`; the chart's own `version`, `packages/dsh-ai-kb` (npm) and `mcp-server` (PyPI) keep their own semver. `make check-version` fails on any drift (validate.sh runs it every time, `guards.yml` in CI). Bump with `make bump-version [VERSION=...]` (default today, then `.1`, `.2` past the current version and existing tags) or `make bump-lib LIB=dsh-ai-kb|mcp-server|helm-chart PART=patch|minor|major` (or `VERSION=x.y.z`): each rewrites only the version token, makes one signed `chore(release): ...` commit and prints the tag and push commands; tagging and pushing stay manual. Never `npm version` (it mangles four parts) or `npm install --package-lock-only` (it rewrites unrelated lock entries). Never take upstream's version: an upstream release bump is SKIP, and a conflict on a version line keeps ours.

## Dependencies

Since 2026-10-13 this copy runs its own upgrade policy (plan and holds in `private/findings/2026-10-13-dependency-upgrades.md`): `go.mod`, `go.sum`, every `package.json` and `package-lock.json`, `docreader/uv.lock`, `mcp-server/uv.lock` and the `cli/` and `client/` module files are ours, and every lockfile shows `~`. A dependency-only upstream commit is SKIP unless it fixes an advisory or a bug we have; then take the version, never the hunk: `go get`, `npm install` or `uv lock -P` the named package, and rerun the area's validate checks. Never merge an upstream lockfile hunk.

Cooldown: an ordinary upgrade takes only releases at least 7 days old. Dependabot's `cooldown.default-days: 7` covers version PRs, each npm project's `.npmrc` sets `min-release-age=7` (npm >= 11.10; `npm ci` still installs the lockfile as is) and each uv project's `[tool.uv] exclude-newer = "7 days"` covers `uv lock`. Go has no setting: pick a version at least 7 days old by hand. A release that fixes a published advisory (including a repository advisory not yet in the npm audit database) skips the cooldown when it needs no refactor: `npm install pkg@x --min-release-age=0`, `uv lock -P pkg --exclude-newer false`.

## Deviations decided

Upstream commits touching translated files show `~`: port a docs hunk in English with the `translate-docs` skill, a code or config hunk by taking the logic and keeping our English. A new deviation gets a row here; a deleted path also gets a line in "Deleted or moved paths".

| Upstream | Ours | Status | Notes |
| --- | --- | --- | --- |
| Chinese text in code, docs and config | English only | landed | `scripts/check-english-only.sh` gates it (pre-commit, `guards.yml`). |
| WeKnora names | AI KB names | landed 2026-10-12 | See "Name translations when porting". |
| upstream semver | dated CalVer | landed 2026-10-13 | See "Version and changelog". |
| `.github/` | our own CI | landed | See "Rules for this repo". |
| locale bundles `zh-CN`, `zh-TW`, `ja-JP`, `ko-KR`, `ru-RU` | removed; `en-US` only | landed | Also `BUILT_IN_DEFAULT`, `SUPPORTED_LOCALES`, the parity test's `LOCALE_BUNDLES`, `AIKB_LANGUAGE` / `DEFAULT_LOCALE` defaults, and the embed channel `default_locale` (`10fcf0e33`). The IM channel `locale` keeps all six tags (`internal/types/locale.go`): it is the language the model replies in, not a UI bundle. Upstream commits that only touch removed bundles: SKIP. Commits adding `en-US` keys: ADOPT. |
| `README_CN.md`, `README_JA.md`, `README_KO.md`, `client/README.md` (Chinese) | removed; `client/README_EN.md` becomes `client/README.md` | landed | Upstream commits touching them: SKIP. |

Backup rule for every deletion above: copy the file to `private/removed/<original path>` (gitignored) before `git rm`, so the migration can be audited against the originals. The archived clone keeps every version too: `git --git-dir=private/OLD-.git show <sha>:<path>`.

Gate placement: `scripts/git-hooks/pre-commit` runs `check-english-only.sh --diff` on added lines today (install with `scripts/install-git-hooks.sh`; `git cherry-pick` does not run pre-commit, hence step 6). The tree-wide scan is a CI gate (`guards.yml`, `scripts/check-english-only.sh` with no base).

## Upstream behavior we removed on purpose

A conflict in these spots keeps ours, even though the general rule is to take upstream's logic.

- Duplicate knowledge no longer rewrites the existing row's `created_at` (2026-10-14; upstream `d6c11feb7` did it to bump a re-upload to the top). Across data sources it falsified another source's creation time and reordered its documents. Pinned by `TestCreateKnowledgeDuplicateKeepsExistingCreatedAt` in `internal/application/service/knowledge_create_gitlab_test.go`.
- A built-in agent's empty chat or rerank model slot no longer refuses the send (2026-10-14; upstream rejects every silent model fallback, Tencent/WeKnora#1813 and the maintainer comment on Tencent/WeKnora#2934). It resolves at request time to the workspace's active `is_default` model of that type, else the active built-in `is_default` one, else refuses as before (Quick Answer's rerank slot: skips reranking). Messages, titles and IM state record that same model. Custom agents, shared runs and stale configured ids are unchanged, and nothing is written to agent rows. One backend rule, `types.AgentModelSlot` in `internal/types/agent_model_slot.go`, mirrored by `resolveBuiltinDefaultModelId` in `frontend/src/utils/modelDefaults.ts`. Pinned by `internal/types/agent_model_slot_test.go`, `TestResolveChatModelIDBuiltinAgentUsesDeclaredDefault` and `TestApplyAgentOverridesResolvesBuiltinRerankSlot` in `internal/application/service/session_model_requirement_test.go`, `TestMessageChatModelID` (`internal/handler/session`), `TestAgentChatModelID` (`internal/im`) and the built-in cases in `frontend/src/utils/agent-readiness.test.mjs`.
- Docs that only serve upgrading an existing upstream install are gone (2026-10-15; AI KB is greenfield with no earlier installs): the ParadeDB 0.22.x upgrade page (`website-docs/01-getting-started/06-paradedb-upgrade.md`, upstream `paradedb-upgrade.md`) and its links, and the sandbox page's cleanup note for pre-rename `weknora-skill/`/`weknora-fork/` snapshots. An upstream change to those pages is not ported. Backups in `private/removed/website-docs/`.
- Weaviate retriever (2026-10-15, `internal/application/repository/retriever/weaviate/`): object IDs are a UUIDv5 of `source_id`, not the chunk ID (upstream let each generated question overwrite its chunk's row); enable and tag updates look objects up by `chunk_id` and PATCH-merge them; reads and writes use the named vector `embedding`; keyword search runs BM25 on `content_bigram` (`searchutil.CJKBigramsAndUnigrams`, `lowercase` tokenization), never `gse`, which the official images cannot load (weaviate/weaviate#10097). Compose drops `ENABLE_TOKENIZER_GSE`. A conflict here keeps ours. Pinned by `TestLiveRoundTrip` (`AIKB_WEAVIATE_TEST_HOST`) and the unit tests beside it.
- Doris compose profile (2026-10-15, `docker-compose.yml`): FE and BE run in FQDN mode with a BE start that registers itself and creates `DORIS_DATABASE`; upstream's `FE_SERVERS`/`BE_ADDR` hostnames fail the images' IP-only entrypoints. Pinned by `TestLiveRoundTrip` in `retriever/doris` (`AIKB_DORIS_TEST_ADDR`, run on the compose network).

## Name translations when porting

Every name changed on 2026-10-12: product "AI KB", repo and Go modules `github.com/jrmatherly/ai-kb` (`.../cli`, `.../client`), images `ghcr.io/jrmatherly/ai-kb-*`, env prefix `AIKB_`, binary `ai-kb`, containers `ai-kb-*`, docs host `ai-kb-docs.matherly.net`, CLI `aikb` (`~/.config/aikb`, `.aikb/project.yaml`), Lite home `~/.aikb`, Helm chart `ai-kb`, MCP tools `aikb_*`, tables `aikb_*`, database `ai_kb`, headers `X-AIKB-*`, code identifiers `AIKB*`. The contract is `scripts/rebrand/map.tsv` (column 1 an upstream Perl regex, column 2 ours); `scripts/rebrand/unchanged.md` lists what keeps upstream's spelling (WeKnoraCloud, BrowserSkill, provenance, history).

So nearly every upstream commit shows `~` and is ADAPT. Cherry-pick, then run `scripts/rebrand/apply.sh` on every file the pick touched and `scripts/rebrand/check-repo.sh` (exit 1 names each leftover). An added import of `github.com/Tencent/WeKnora/...` is rewritten the same way; run `gofmt -w` after, since the new path sorts differently. A row is never edited to fit a port; add one only when a new surface is decided, and confirm with check.sh output on `website-docs/` that it changes nothing unintended.

## Decision log template

```markdown
# Upstream review - <YYYY-MM-DD>

Window: `<from>..<full tip sha>` on upstream/main, N commits (M merges). Driver output: `--stat`.

## Headline
One paragraph: what to adopt now, what to port, what to skip.

## Verdicts
| # | Commit | Area | Our state | Markers | Verdict | Evidence |
|---|--------|------|-----------|---------|---------|----------|

## Adopted
Cherry-picked SHAs in order, validation result per area, `check-english-only.sh --diff` result.
```

## Red flags

- `git merge upstream/main` or any merge-base based range. Use the watermark.
- Cherry-picking a `~` file without reading our side first.
- Review state anywhere except `.upstream-review/`.
- Advancing the watermark to a SHA other than the tip printed by the listing you classified.
- A batch that lands with `scripts/check-english-only.sh --diff <sha-before-batch>..HEAD` exiting 1.
