# What keeps upstream's spelling

The rename (`map.tsv`) landed in the docs and the code on 2026-10-12. `scripts/rebrand/check-repo.sh`
proves it over the paths `scope.txt` selects. These keep upstream's spelling on purpose:

- **WeKnoraCloud**, Tencent's hosted model and document service: the provider id `weknoracloud`, the
  `WeKnoraCloud*` names, `AIKB_WEKNORACLOUD_*` settings, and its API host `weknora.weixin.qq.com` (bare host
  and `/api` paths). Only `/docs` and website links on that host moved to `ai-kb-docs.matherly.net`.
- **ClawHub skill `@lyingbug/weknora`** (third party): its package name and the `WEKNORA_BASE_URL` / `WEKNORA_API_KEY`
  variables it reads, shielded per path by `keep.tsv`.
- **BrowserSkill**: `Tencent/BrowserSkill`, the `bsk` daemon, release artifacts `browser-skill-weknora-*.zip`.
- `WeKnora-Chrome-Extension`: a separately checked-out folder, named only in `.gitignore`, `.dockerignore`
  and `.air.toml`.
- `support@weknora.com` in `mcp-server/pyproject.toml` and `setup.py`: AI KB has no support address yet.
- Provenance: upstream issue and PR links (`Tencent/WeKnora#NNNN`, `/issues/`, `/pull/`, `/discussions/`),
  `.upstream-review/`, the upstream-review skill, `CLAUDE.md`.

Outside `scope.txt`, so the check does not cover them:

- History: `CHANGELOG.md`, `cli/CHANGELOG.md`, `mcp-server/CHANGELOG.md`, `website-docs/07-releases/`.
- Generated: proto stubs, lockfiles and the `docs/` swagger are regenerated, never rewritten (swagger: pinned swag v1.16.6, `make docs`).
- `third_party/` (vendored firecrawl anydoc; only its benchmark logs mention upstream), `docs/images/` (art).
