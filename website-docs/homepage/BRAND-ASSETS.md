# Brand asset sources

Logos identify supported integrations; their names and marks belong to their respective owners.

Paths below written as `public/brand/…`, `public/brands/…` and `public/product/…` live under `public/docs/_home/`, so the site serves them from `/docs/_home/` next to the rest of the documentation.

- Product logo (**pending replacement**): `public/brand/ai-kb-original.png` is still the upstream project's README logo (`docs/images/logo.png`), copied unchanged. It shows the upstream name, not AI KB, and is replaced when an AI KB logo exists. CSS hides only the empty outer canvas and blends the white background into the page; dark mode uses an inverted presentation with hue rotation.
- Browser icon (**pending replacement**): copied unchanged from the product's `frontend/public/favicon.ico` into the documentation (`/docs/favicon.ico`), which the homepage also uses. It is still the upstream icon.
- Feishu, GitLab, Tencent IMA, Notion, Yuque, RSS, WeCom, Slack, Telegram: copied unchanged from the product frontend asset library.
- OpenAI, DeepSeek, Qwen, Hunyuan, Gemini, Ollama, MCP: static SVGs from [Lobe Icons](https://github.com/lobehub/lobe-icons), `@lobehub/icons-static-svg@1.95.0`. Upstream MIT license included with assets.
- Confluence, DingTalk: copied unchanged from the product frontend asset library (`frontend/src/assets/img/datasource-confluence.svg`, `frontend/src/assets/img/im/dingtalk.svg`).
- Claude (Anthropic), Zhipu, Kimi (Moonshot), Volcengine, MiniMax: copied unchanged from `internal/models/providers/assets/`, which carries the same Lobe Icons static SVGs; covered by the included Lobe Icons MIT license.
- BrowserSkill: `frontend/src/assets/browserskill/logo.png` from the product frontend (the extension's own icon, [Tencent/BrowserSkill](https://github.com/Tencent/BrowserSkill)), copied unchanged to `public/brands/browserskill.png` with its MIT license as `public/brands/BROWSERSKILL-LICENSE`.
- Chrome: [official Chrome website asset](https://www.google.com/chrome/static/images/chrome-logo-m100.svg).
- File formats, REST API, CLI: generic interface icons, not brand logos.
- LiteLLM: [official documentation favicon](https://docs.litellm.ai/img/favicon.ico).
- WeChat Dialog Open Platform: [official platform icon](https://res.wx.qq.com/mmspraiweb_node/dist/static/logo/logo180.png), referenced by the platform login page and saved unchanged as `public/brands/wechat-dialog.png`. No longer referenced by the homepage.
- Tencent Cloud: [official website icon](https://cloudcache.tencent-cloud.com/qcloud/favicon.ico), referenced by the Lighthouse product page and saved unchanged as `public/brands/tencent-cloud.ico`. No longer referenced by the homepage: both hosted options deploy the upstream product, not AI KB.

## Product screenshots

Every screenshot below was captured from the upstream build and shows the upstream logo and a Chinese UI. All are **pending replacement** with English AI KB captures under the same file names.

- Skill catalog and sandbox terminal: copied unchanged from `../public/screenshots/skill-catalog.png` and `sandbox-panel-terminal.png` to `public/product/skill-catalog.png` and `sandbox-terminal.png`, for the skills & sandbox gallery.
- Sandbox desktop (`public/product/sandbox-desktop.png`): the chat's sandbox panel on the Desktop tab, showing the XFCE desktop of a desktop template. It shares the Desktop and terminal card with the terminal screenshot, switched inside the card.
- v0.8.2 gallery: `mcp-server-endpoint.png`, `chat-steer-queue.png` and `browser-connection.png` copied unchanged from `../public/screenshots/` to `public/product/`. The local browser card switches between two views of the same capability: Run a task (below) and Connect the extension (the browser connection page).
- Browser task (`public/product/local-browser-task.png`): the default view of the local browser card, a smart-reasoning chat driving the connected Chrome through BrowserSkill (GitHub release lookup, then filling the httpbin.org sample order form without submitting), with the in-chat task preview and pause / end controls.

- Wiki browser: copied unchanged from `../public/screenshots/wiki-browser.png` to `public/product/wiki-browser.png`. Shows the actual Wiki directory, linked page and source references using sample company policies.
- Wiki graph and revision history: copied unchanged from `../public/screenshots/wiki-graph.png` and `wiki-revision-history.png` to matching files in `public/product/`, for the horizontal Wiki gallery.
