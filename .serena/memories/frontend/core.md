# frontend/ (Vue 3 + Vite + TS)

- Own npm project (`frontend/package.json`, name `knowledage-base`, version tracks root `VERSION`). Run scripts with `npm --prefix frontend <script>`.
- Scripts: `dev` (vite), `build` (vite build), `build-with-types` (type-check + build in parallel), `type-check` (`vue-tsc --build`), `test` (`tsx --test`, Node `node:test`), `check-i18n`.
- i18n: `src/i18n/index.ts` registers only `src/i18n/locales/en-US.ts`; `resolveDefaultLocale.ts` has `SUPPORTED_LOCALES = ['en-US']` and `BUILT_IN_DEFAULT = 'en-US'`; env `DEFAULT_LOCALE` / `VITE_DEFAULT_LOCALE` accept only `en-US`. `embed.ts` holds the embed widget's en-US messages.
- TDesign: `main.ts` loads `tdesign.css` only; components register per template through `unplugin-vue-components` + `TDesignResolver` (`vite.config.ts`, `dts: false` until the 92 template type mismatches are fixed) and bundle into one `vendor-tdesign` chunk (one file beat 65 per-component chunks on HTTP/1.1). `embed-main.ts` still does `app.use(TDesign)`. A render-function or string-name use needs an explicit import.
- Views under `src/views/<area>/`, components under `src/components/`, embed build (`embed.html`) and nginx confs beside `Dockerfile`. `mcpProxy.test.ts` at the frontend root.
- CI (`.github/workflows/frontend.yml`): `npm ci`, `npm test`, `npm run type-check`, `npm run build`, then `docker build -f frontend/Dockerfile -t ai-kb-ui:ci frontend/` + `python3 scripts/test_embed_nginx.py ai-kb-ui:ci`; locally `scripts/validate.sh` area `frontend`.
