---
paths:
  - "frontend/**"
---

# Frontend

- Vue 3 + TypeScript + Vite, Less styles. Checks: `npm --prefix frontend test` (node test runner via tsx) and `npm --prefix frontend run type-check` (vue-tsc).
- UI text goes through vue-i18n keys in `frontend/src/i18n/locales/en-US.ts`, the only locale. A new key goes there with English text; `localeKeyAudit.test.ts` checks the critical trees resolve, and `internal/datasource/syncreason/parity_test.go` (Go) checks `datasource.syncError.*` against the backend's reason-code tables.
- TDesign components are registered per template by `unplugin-vue-components` (`TDesignResolver` in `frontend/vite.config.ts`) in both entries, never with `app.use(TDesign)`; a component used from a render function or by string name needs an explicit import from `tdesign-vue-next`. The plugin writes `src/components.d.ts`, which is committed so `vue-tsc` type-checks the templates; `scripts/verify_frontend_pr.sh` fails when a build changes it. TDesign is not in `manualChunks`: each entry carries only the components its templates use.
- Product name in new UI copy is "AI KB".
