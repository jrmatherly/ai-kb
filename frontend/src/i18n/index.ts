import { createI18n } from 'vue-i18n'
import enUS from './locales/en-US.ts'
import { BUILT_IN_DEFAULT, resolveDefaultLocale } from './resolveDefaultLocale.ts'

// A saved choice wins over the deployment default; a removed locale left in storage falls back to en-US.
const savedLocale = resolveDefaultLocale(
  localStorage.getItem('locale') || window.__RUNTIME_CONFIG__?.DEFAULT_LOCALE,
  import.meta.env.VITE_DEFAULT_LOCALE,
)

const i18n = createI18n({
  legacy: false,
  locale: savedLocale,
  fallbackLocale: BUILT_IN_DEFAULT,
  globalInjection: true,
  // Some translations intentionally embed `<strong>` markup (e.g. agent step summaries).
  // We render them via v-html with our own sanitization, so silence vue-i18n's HTML warning
  // to avoid flooding the console and slowing renders during history loads.
  warnHtmlMessage: false,
  messages: { 'en-US': enUS },
})

export default i18n
