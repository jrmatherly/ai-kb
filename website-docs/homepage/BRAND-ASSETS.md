# Brand asset sources

Logos identify supported integrations; their names and marks belong to their respective owners.

Paths below written as `public/brand/…`, `public/brands/…` and `public/product/…` live under `public/docs/_home/`, so the site serves them from `/docs/_home/` next to the rest of the documentation.

- Product logo: `public/brand/ai-kb-lockup-light.png` and `public/brand/ai-kb-lockup-dark.png` are the AI KB lockup (the Kay mark on its black tile beside "AI KB" in Helvetica Neue Bold) on a transparent 945 x 238 canvas, black lettering for light pages and white for dark ones. `.wk-logo` in `shared/brand.css` shows the one that matches `html.dark`, so the accent keeps its color in both themes. The product app does the same with `frontend/src/assets/img/aikb.png` and `aikb-dark.png`, keyed on `html[theme-mode]`.
- Browser icon: `/docs/favicon.ico` (16 and 32 px, the Kay mark on its tile), the same file as the product's `frontend/public/favicon.ico`. The homepage reuses it.
- Both are rendered by `node scripts/brand/generate.mjs` at the repository root, the single source for every AI KB logo: the marks, accent palette and the full list of outputs live in that script, and its SVG sources are written to `docs/images/brand/`. Rendering needs Google Chrome and ImageMagick. Change `DEFAULT` in the script and rerun it to switch the mark or accent everywhere, then rerun `docs/images/readme/src/generate.py`, which embeds the wordmark PNGs in the README hero.
- Feishu, GitLab, Tencent IMA, Notion, Yuque, RSS, WeCom, Slack, Telegram: copied unchanged from the product frontend asset library (`frontend/src/assets/img/datasource-*` and `frontend/src/assets/img/im/*`).
- OpenAI, DeepSeek, Qwen, Hunyuan, Gemini, Ollama, MCP: static SVGs from [Lobe Icons](https://github.com/lobehub/lobe-icons), `@lobehub/icons-static-svg@1.95.0`. Upstream MIT license included with assets.
- Confluence, DingTalk: copied unchanged from the product frontend asset library (`frontend/src/assets/img/datasource-confluence.svg`, `frontend/src/assets/img/im/dingtalk.svg`).
- Claude (Anthropic), Zhipu, Kimi (Moonshot), Volcengine, MiniMax: copied unchanged from `internal/models/providers/assets/`, which carries the same Lobe Icons static SVGs; covered by the included Lobe Icons MIT license.
- BrowserSkill: `frontend/src/assets/browserskill/logo.png` from the product frontend (the extension's own icon, [Tencent/BrowserSkill](https://github.com/Tencent/BrowserSkill)), copied unchanged to `public/brands/browserskill.png` with its MIT license as `public/brands/BROWSERSKILL-LICENSE`.
- File formats, REST API, CLI: generic interface icons, not brand logos.
- LiteLLM: [official documentation favicon](https://docs.litellm.ai/img/favicon.ico).

## Product screenshots

The upstream screenshots showed the upstream logo and a Chinese UI, so they were removed on 2026-10-13 (backups in the untracked `private/removed/`). A gallery entry in `app/page.tsx` renders its "Screenshot pending" placeholder, which names the expected file, until an English capture from the running AI KB app is saved under `public/product/` with that name (3840 x 2112 PNG, a 1920 x 1056 viewport at 2x).

Captured on 2026-10-13 from the development stack with the `website-docs/sample-data/` corpus: `mcp-server-endpoint.png`, `skill-catalog.png` and `agent-chat.png`, the video poster (a Quick Answer turn with its sources panel; the answer text quotes the sample product manual and FAQ).

Captured on 2026-10-14 the same way: `chat-steer-queue.png` (a Smart Reasoning run with queued messages), `wiki-browser.png`, `wiki-graph.png` and `wiki-revision-history.png` (a generated wiki), `skill-sandbox-chat.png` and `sandbox-terminal.png` (a Docker sandbox).

Still pending: `local-browser-task.png` and `browser-connection.png` (need a paired BrowserSkill browser) and `sandbox-desktop.png` (needs a Cube or E2B sandbox with a desktop template; Docker sandboxes have no desktop).
