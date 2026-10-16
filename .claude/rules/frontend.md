---
paths:
  - "frontend/**"
---

# Frontend

- Vue 3 + TypeScript + Vite, Less styles. Checks: `npm --prefix frontend test` (node test runner via tsx) and `npm --prefix frontend run type-check` (vue-tsc).
- UI text goes through vue-i18n keys in `frontend/src/i18n/locales/en-US.ts`, the only locale. A new key goes there with English text; `localeKeyAudit.test.ts` checks the critical trees resolve, and `internal/datasource/syncreason/parity_test.go` (Go) checks `datasource.syncError.*` against the backend's reason-code tables.
- Product name in new UI copy is "AI KB".
