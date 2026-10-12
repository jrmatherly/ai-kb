# frontend/ (Vue 3 + Vite + TS)

- Own npm project (`frontend/package.json`, name `knowledage-base`, version tracks root `VERSION`). Run scripts with `npm --prefix frontend <script>`.
- Scripts: `dev` (vite), `build` (vite build), `build-with-types` (type-check + build in parallel), `type-check` (`vue-tsc --build`), `test` (`tsx --test`, Node `node:test`), `check-i18n`, `scan-i18n-gaps`, `regenerate-i18n-locales`.
- i18n: `src/i18n/index.ts` registers bundles from `src/i18n/locales/{zh-CN,zh-TW,en-US,ru-RU,ko-KR,ja-JP}.ts`; `resolveDefaultLocale.ts` has `SUPPORTED_LOCALES` and `BUILT_IN_DEFAULT = 'zh-CN'`; env `DEFAULT_LOCALE` / `VITE_DEFAULT_LOCALE`. `localeKeyAudit.test.ts` enforces key parity against `en-US` and compiles every message. Removing a locale means editing all three places and the audit's `LOCALE_BUNDLES`.
- Views under `src/views/<area>/`, components under `src/components/`, embed build (`embed.html`) and nginx confs beside `Dockerfile`. `mcpProxy.test.ts` at the frontend root.
- CI (archived): `npm ci`, `npm test`, `npm run type-check`, `npm run build`, docker build + `python3 scripts/test_embed_nginx.py weknora-ui:ci`.
