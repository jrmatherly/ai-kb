import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { createI18n } from 'vue-i18n'

import enUS from '../i18n/locales/en-US.ts'

test('knowledge upload hints interpolate the configured limit', () => {
  const size = 73
  const i18n = createI18n({ legacy: false, locale: 'en-US', messages: { 'en-US': enUS } })

  assert.match(i18n.global.t('knowledgeBase.pdfDocFormat', { size }), /73/, 'pdf/doc hint')
  assert.match(i18n.global.t('knowledgeBase.textMarkdownFormat', { size }), /73/, 'text/markdown hint')
})

test('empty state and drag overlay pass the runtime limit to both upload hints', () => {
  for (const name of ['empty-knowledge.vue', 'upload-mask.vue']) {
    const source = readFileSync(new URL(`./${name}`, import.meta.url), 'utf8')

    assert.match(source, /import \{ MAX_FILE_SIZE_MB \} from '@\/utils'/)
    assert.match(
      source,
      /\$t\('knowledgeBase\.pdfDocFormat', \{ size: MAX_FILE_SIZE_MB \}\)/,
      `${name}: pdf/doc hint`,
    )
    assert.match(
      source,
      /\$t\('knowledgeBase\.textMarkdownFormat', \{ size: MAX_FILE_SIZE_MB \}\)/,
      `${name}: text/markdown hint`,
    )
  }
})
