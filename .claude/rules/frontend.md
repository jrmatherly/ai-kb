---
paths:
  - "frontend/**"
---

# Frontend

- Vue 3 + TypeScript + Vite, Less styles. Checks: `npm --prefix frontend test` (node test runner via tsx) and `npm --prefix frontend run type-check` (vue-tsc).
- UI text goes through vue-i18n keys in `frontend/src/i18n/locales/`. `en-US.ts` is the only locale this copy keeps; the other bundles (`zh-CN`, `zh-TW`, `ja-JP`, `ko-KR`, `ru-RU`) are slated for removal but still exist. Until they are removed, `localeKeyAudit.test.ts` fails unless every bundle has the same keys, so a new key goes into all six bundles with the English value (the English-only gate rejects CJK in added lines).
- Product name in new UI copy is "AI KB".
