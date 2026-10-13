import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  EMBED_MESSAGES,
  normalizeEmbedLocale,
  SUPPORTED_LOCALES,
} from './embed.ts'

const EXPECTED_CONVERSATION_TIME_KEYS = [
  'today',
  'yesterday',
  'thisYear',
  'otherYear',
] as const

const EXPECTED_REFERENCES_DRAWER_KEYS = [
  'referencesDrawerTitle',
  'referencesDrawerTitleWeb',
  'referencesDrawerTitleDocs',
  'referencesDrawerTitleTools',
  'referencesDrawerTitleMixed',
  'referencesDrawerWebSection',
  'referencesDrawerDocsSection',
  'referencesDrawerToolsSection',
  'referencesDrawerEmpty',
] as const

test('the embed ships en-US only', () => {
  assert.deepEqual([...SUPPORTED_LOCALES], ['en-US'])
  assert.deepEqual(Object.keys(EMBED_MESSAGES), ['en-US'])
})

test('every supported locale defines conversationTime and referencesDrawer in chat', () => {
  for (const locale of SUPPORTED_LOCALES) {
    const bundle = EMBED_MESSAGES[locale] as {
      chat?: Record<string, unknown>
      common?: Record<string, unknown>
    }
    assert.ok(bundle, `Locale bundle for ${locale} must exist`)
    assert.ok(bundle.chat, `Locale bundle for ${locale} must have chat section`)

    // conversationTime checks
    const conversationTime = bundle.chat.conversationTime as Record<string, string> | undefined
    assert.ok(
      conversationTime && typeof conversationTime === 'object',
      `Locale ${locale} is missing chat.conversationTime`,
    )
    for (const key of EXPECTED_CONVERSATION_TIME_KEYS) {
      assert.equal(
        typeof conversationTime[key],
        'string',
        `Locale ${locale} is missing chat.conversationTime.${key}`,
      )
      assert.ok(
        conversationTime[key].trim().length > 0,
        `chat.conversationTime.${key} in ${locale} should not be empty`,
      )
      assert.ok(
        conversationTime[key].includes('{time}'),
        `chat.conversationTime.${key} in ${locale} must include {time} placeholder`,
      )
    }

    // referencesDrawer checks
    const chatBag = bundle.chat as Record<string, unknown>
    for (const key of EXPECTED_REFERENCES_DRAWER_KEYS) {
      const val: unknown = chatBag[key]
      assert.equal(
        typeof val,
        'string',
        `Locale ${locale} is missing chat.${key}`,
      )
      assert.ok(
        (val as string).trim().length > 0,
        `chat.${key} in ${locale} should not be empty`,
      )
    }

    // common.close check
    assert.ok(bundle.common, `Locale bundle for ${locale} must have common section`)
    assert.equal(
      typeof bundle.common.close,
      'string',
      `Locale ${locale} is missing common.close`,
    )
    assert.ok(
      (bundle.common.close as string).trim().length > 0,
      `common.close in ${locale} should not be empty`,
    )
  }
})

test('en-US translations match expected strings for previously missing keys', () => {
  const enChat = EMBED_MESSAGES['en-US'].chat as Record<string, any>
  const enCommon = EMBED_MESSAGES['en-US'].common as Record<string, any>
  assert.equal(enChat.conversationTime.today, 'Today {time}')
  assert.equal(enChat.conversationTime.yesterday, 'Yesterday {time}')
  assert.equal(enChat.conversationTime.thisYear, '{month}/{day} {time}')
  assert.equal(enChat.conversationTime.otherYear, '{month}/{day}/{year} {time}')
  assert.equal(enChat.referencesDrawerTitleDocs, 'Document sources')
  assert.equal(enChat.referencesDrawerEmpty, 'No sources available')
  assert.equal(enCommon.close, 'Close')
})

test('normalizeEmbedLocale resolves every host tag to en-US', () => {
  for (const raw of ['en-US', 'en', 'EN-gb', 'zh-CN', 'ja-JP', 'unknown-locale', '   ']) {
    assert.equal(normalizeEmbedLocale(raw), 'en-US', raw)
  }
})
