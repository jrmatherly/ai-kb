# frontend/ (Vue 3 + Vite + TS)

- Own npm project (`frontend/package.json`, name `knowledage-base`, version tracks root `VERSION`). Run scripts with `npm --prefix frontend <script>`.
- Scripts: `dev` (vite), `build` (vite build), `build-with-types` (type-check + build in parallel), `type-check` (`vue-tsc --build`), `test` (`tsx --test`, Node `node:test`), `check-i18n`, `scan-i18n-gaps`, `regenerate-i18n-locales`.
- i18n: `src/i18n/index.ts` registers only `src/i18n/locales/en-US.ts`; `resolveDefaultLocale.ts` has `SUPPORTED_LOCALES = ['en-US']` and `BUILT_IN_DEFAULT = 'en-US'`; env `DEFAULT_LOCALE` / `VITE_DEFAULT_LOCALE` accept only `en-US`. `embed.ts` holds the embed widget's en-US messages.
- Views under `src/views/<area>/`, components under `src/components/`, embed build (`embed.html`) and nginx confs beside `Dockerfile`. `mcpProxy.test.ts` at the frontend root.
- CI (archived): `npm ci`, `npm test`, `npm run type-check`, `npm run build`, docker build + `python3 scripts/test_embed_nginx.py weknora-ui:ci`.
