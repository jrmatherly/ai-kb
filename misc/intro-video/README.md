# AI KB intro video

The source for the product intro video (about 1:52, 1920×1080 at 30 fps, English on-screen text, no audio). The whole video is one HTML page, `index.html`, which draws any moment from its timestamp. `render.mjs` steps through the timeline with headless Chromium and pipes the frames into ffmpeg.

## Render

Requires Node 24+, ffmpeg and a Chromium build for Playwright.

```bash
cd misc/intro-video
npm install
npx playwright-core install chromium   # skip if CHROME_PATH points at a Chromium build
npm run render                          # writes ai-kb-intro.mp4, takes a few minutes
```

To check a few moments without rendering the whole video:

```bash
npm run stills -- 9.5 22 57 81          # PNGs in stills/
```

To preview in a browser, open `index.html?t=22` for one frame, or `index.html?play` to play from the start (`?play=40` starts at 40 s). Open it from the repository checkout, because the brand icons are loaded from `website-docs/homepage/public/docs/_home/brands/`.

## Publish

`README.md` and the docs homepage (`website-docs/homepage/app/interactive.tsx`) embed the same `https://github.com/user-attachments/assets/<id>` URL; the homepage audit fails if they differ. Upload by posting an issue with the file attached:

```bash
gh issue create --title "Asset: AI KB intro video (YYYY-MM-DD)" --body "Hosts the intro video embedded in README.md and the docs homepage. Do not delete." --attach misc/intro-video/ai-kb-intro.mp4
```

Take the URL from the issue body, check it without credentials with a ranged GET (`curl -sL -r 0-0 -o /dev/null -w '%{http_code} %{content_type}\n' <url>` prints `206 video/mp4`; a HEAD request gets `403` from the storage redirect), then close and lock the issue. Never delete it: the asset belongs to the issue. A file dragged into a comment box that is never submitted stays private to the uploader and returns 404 to everyone else.

The current 12.9 MB asset uploaded and serves on this repository. If GitHub rejects an upload for size, re-encode to fit under 10 MB:

```bash
ffmpeg -i ai-kb-intro.mp4 -c:v libx264 -preset slow -crf 24 -pix_fmt yuv420p -movflags +faststart ai-kb-intro-lite.mp4
```

## Structure

| Scene | Time (s) | Content |
| --- | --- | --- |
| `open` | 0–6.6 | Scattered sources and files: "Your team's knowledge lives in a hundred places." |
| `logo` | 6.2–12.2 | Logo and tagline |
| `pillars` | 11.8–17 | RAG / Agent / Wiki cards |
| `rag` | 16.6–31 | Chat with retrieval steps, streamed answer, citation popover |
| `agent` | 30.6–44.4 | Memory, knowledge search, MCP tool call, SLA table |
| `skill` | 44–60.4 | Toolbox skills, then sandbox terminal and the generated .docx |
| `browser` | 60–74.6 | BrowserSkill driving Chrome: release lookup and form fill |
| `wiki` | 74.2–89 | Documents become a wiki tree and page, then a knowledge graph |
| `eco` | 88.6–100.4 | Data sources, channels and model providers |
| `outro` | 100–112 | Docker Compose quick start, logo and links |

Scene boundaries are in the `S` object near the bottom of `index.html`. Each scene reads its local time and positions its elements from it, so to retime a scene, edit its offsets inside `render()`. The total length is `DUR`.

The product UI is rebuilt in HTML rather than screenshotted, so the text stays sharp when the camera zooms in. UI labels follow `frontend/src/i18n/locales/en-US.ts`. The conversation content (Nova-KB-POC, the Q3 ticket numbers, the `ticketing.query_issues` MCP tool, the humanoid-robot report) is sample data.

## When to update

- **Version:** the release shown in the browser scene and typed into the form; `make bump-version` rewrites it (`scripts/version.py` rows).
- **Counts:** the model provider count in the ecosystem scene ("30 providers"); check it against the model catalog.
- **Lists:** the data sources and channels, when integrations are added or removed.

`assets/logo-light.png` (black lettering, for paper backgrounds) and `assets/logo-dark.png` (white lettering, for navy backgrounds) come from `node scripts/brand/generate.mjs` at the repository root.
