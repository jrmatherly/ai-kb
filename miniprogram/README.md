# AI KB Mini Program

This directory contains a WeChat Mini Program client for AI KB. It gives mobile users a lightweight entry point to:

- configure an AI KB API endpoint and workspace API key;
- list available knowledge bases;
- import a URL into a selected knowledge base;
- ask a selected knowledge base through AI KB knowledge chat.

## Getting started

1. Open `miniprogram/` in WeChat DevTools.
2. Copy `project.private.config.json.example` to `project.private.config.json` and set your real Mini Program AppID. The shared `project.config.json` ships WeChat's `touristappid`; the private file overrides it with yours.
3. Open the **Settings** tab and fill in:
   - API Base URL, for example `https://ai-kb.example.com`;
   - API Key created in AI KB under Settings → Publish & Integrations → API Integration (shown once, `sk-...`).
4. Open the **Knowledge** tab, refresh knowledge bases, and select the target knowledge base.
5. Import a URL or switch to **Chat** to ask questions.

## UI text

UI strings live in `utils/i18n.js`. The Mini Program ships in English only.

## Local development notes

- WeChat DevTools may block `localhost` requests when URL validation is enabled. For local testing, either disable domain validation in DevTools or expose AI KB through a HTTPS development domain.
- In production Mini Programs, add the AI KB API domain to the Mini Program request domain allowlist.
- The chat endpoint returns Server-Sent Events. The Mini Program client parses completed SSE text responses and displays accumulated `answer` chunks.

## Test

```bash
cd miniprogram
npm test
```
