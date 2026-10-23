import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { LOCALE_BUNDLES, getLocaleValueAtPath } from '../i18n/localeKeyAudit.ts'
import { CHANNEL_LABEL_KEYS, UPLOAD_CHANNELS, channelLabelKey, sourceFilterChannels } from './channelLabel.ts'

const KNOWLEDGE_GO = fileURLToPath(new URL('../../../internal/types/knowledge.go', import.meta.url))
const DATASOURCE_GO = fileURLToPath(new URL('../../../internal/types/datasource.go', import.meta.url))

// Registry types no document ever carries as its channel: the Lark wiki emits
// "feishu", and the other four have a metadata entry but no connector.
const TYPES_WITHOUT_A_CHANNEL = new Set(['lark', 'google_drive', 'onedrive', 'web_crawler', 'imap'])

const goConstants = (file: string, prefix: string) =>
  [...readFileSync(file, 'utf8').matchAll(new RegExp(`^\\t${prefix}\\w+\\s+=\\s+"([a-z_]+)"`, 'gm'))].map((m) => m[1])

test('every channel label key resolves in en-US', () => {
  for (const [channel, key] of Object.entries(CHANNEL_LABEL_KEYS)) {
    assert.equal(typeof getLocaleValueAtPath(LOCALE_BUNDLES['en-US'], key), 'string', `${channel}: ${key}`)
  }
})

test('every backend channel and connector type has a label', () => {
  const channels = goConstants(KNOWLEDGE_GO, 'Channel')
  const types = goConstants(DATASOURCE_GO, 'ConnectorType').filter((t) => !TYPES_WITHOUT_A_CHANNEL.has(t))
  assert.ok(channels.length >= 17 && types.length >= 14, `parsed ${channels.length} channels, ${types.length} types`)
  assert.deepEqual([...channels, ...types].filter((c) => !Object.hasOwn(CHANNEL_LABEL_KEYS, c)), [])
})

test('the source filter offers every labelled channel except the upload ones', () => {
  const offered = sourceFilterChannels()
  assert.deepEqual(offered.filter((c) => UPLOAD_CHANNELS.has(c)), [])
  assert.deepEqual(offered.slice(0, 3), ['feishu', 'feishu_drive', 'lark_drive'])
  assert.equal(offered.length, Object.keys(CHANNEL_LABEL_KEYS).length - UPLOAD_CHANNELS.size)
})

test('an unknown, empty or prototype channel is Unknown', () => {
  assert.equal(channelLabelKey('github'), 'knowledgeBase.channelGitHub')
  for (const bad of ['nope', '', 'constructor', 'toString', undefined]) {
    assert.equal(channelLabelKey(bad), 'knowledgeBase.channelUnknown', String(bad))
  }
})
