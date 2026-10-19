# What keeps upstream's spelling

The rename (`map.tsv`) landed in the docs and the code on 2026-10-12. `scripts/rebrand/check-repo.sh`
proves it over the paths `scope.txt` selects. These keep upstream's spelling on purpose:

- **WeKnoraCloud**, Tencent's hosted model and document service: the provider id `weknoracloud`, the
  `WeKnoraCloud*` names, `AIKB_WEKNORACLOUD_*` settings, and its API host `weknora.weixin.qq.com` (bare host
  and `/api` paths). Only `/docs` and website links on that host moved to `ai-kb.matherly.net` (`/docs` on the app host).
- **BrowserSkill**: `Tencent/BrowserSkill`, the `bsk` daemon, its upstream build output
  `browser-skillextension-*-chrome.zip`. `scripts/build_browserskill.sh` renames that to `browser-skill-ai-kb-*.zip`,
  the name the server serves.
- `WeKnora-Chrome-Extension`: a separately checked-out folder, named only in `.gitignore`, `.dockerignore`
  and `.air.toml`.
- `LICENSE`: Tencent's WeKnora notice is upstream's text, verbatim, below this project's own notice; `keep.tsv`
  shields the whole file.
- The upstream credit: `[Tencent/WeKnora](https://github.com/Tencent/WeKnora)` in the README and the homepage
  footer and the docs footer's "Based on Tencent/WeKnora" links, shielded by `keep.tsv`.
- Provenance: upstream issue and PR links (`Tencent/WeKnora#NNNN`, `/issues/`, `/pull/`, `/discussions/`).
- The git remote: Tencent/WeKnora is fetched as the remote `weknora`, so refs like `weknora/main` are ours and
  `keep.tsv` shields them.

Outside `scope.txt`, so the check does not cover them:

- History: `CHANGELOG.md`, `cli/CHANGELOG.md`, `mcp-server/CHANGELOG.md`.
- Generated: proto stubs and lockfiles. (The `docs/` swagger is in scope; its only upstream spellings are the WeKnoraCloud names above, and it is regenerated with `make docs`, pinned swag v1.16.6, never rewritten.)
- Agent and upstream-review state: `.upstream-review/`, `.claude/`, `.serena/`, `CLAUDE.md` and `AGENTS.md`, and the rebrand contract files themselves (`map.tsv`, `unchanged.md`, `scope.txt`, `keep.tsv`).
- `third_party/` (vendored firecrawl anydoc; only its benchmark logs mention upstream), `docs/images/` (art).
