import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { before, test } from 'node:test'
import { fileURLToPath } from 'node:url'

import { EMBED_MESSAGES } from './embed.ts'
import { REGISTERED_AUDIT_ACTION_ENTRIES, KB_ACTIVITY_DETAIL_VALUES, KB_ACTIVITY_I18N_ROOTS, KB_ACTIVITY_OUTCOMES } from './auditActionRegistry.ts'
import {
  CRITICAL_LOCALE_KEYS,
  LOCALE_BUNDLES,
  collectLocaleKeys,
  findAllLocaleMessageCompileErrors,
  getLocaleValueAtPath,
  type LocaleName,
} from './localeKeyAudit.ts'

let localeKeysByName: Record<LocaleName, Set<string>>

before(() => {
  localeKeysByName = Object.fromEntries(
    Object.entries(LOCALE_BUNDLES).map(([name, bundle]) => [name, collectLocaleKeys(bundle)]),
  ) as Record<LocaleName, Set<string>>
})

test('critical runtime i18n trees are present', () => {
  const en = localeKeysByName['en-US']
  const missing = CRITICAL_LOCALE_KEYS.filter((key) => !en.has(key))
  assert.deepEqual(missing, [], missing.join('\n'))
})

test('installer command progress messages resolve in the settings namespace', () => {
  for (const [locale, bundle] of Object.entries(LOCALE_BUNDLES)) {
    for (const name of ['installCommandRunning', 'installCommandWaiting']) {
      const key = `settings.sandbox.${name}`
      assert.equal(typeof getLocaleValueAtPath(bundle, key), 'string', `${locale}: missing ${key}`)
    }
  }
})

const PARSER_ENGINE_NAMES = [
  'builtin',
  'simple',
  'mineru',
  'mineru_cloud',
  'paddleocr_vl',
  'paddleocr_vl_cloud',
  'weknoracloud',
  'markitdown',
  'opendataloader',
] as const

/** Worker pools / queues use dynamic te() paths in RuntimeQueues.vue; keep in sync with internal/types/task.go. */
const RUNTIME_WORKER_POOL_NAMES = [
  'core',
  'postprocess',
  'enrichment',
  'maintenance',
  'shared',
  'wiki',
] as const

const RUNTIME_QUEUE_NAMES = [
  'default',
  'chat_attachment',
  'postprocess',
  'summary',
  'sync',
  'low',
  'multimodal',
  'graph',
  'question',
  'wiki',
] as const

const RUNTIME_TASK_TYPE_KEYS = [
  'documentProcess',
  'manualProcess',
  'temporaryDocumentProcess',
  'postProcess',
  'summary',
  'tableSummary',
  'question',
  'multimodal',
  'graph',
  'sync',
  'faqImport',
  'batchReparse',
  'batchDelete',
  'move',
  'indexDelete',
  'kbClone',
  'kbDelete',
  'wikiIngest',
  'wikiFinalize',
] as const

test('registered audit action labels exist as flat keys in every locale', () => {
  const failures: string[] = []

  for (const { root, actions } of REGISTERED_AUDIT_ACTION_ENTRIES) {
    for (const action of actions) {
      const dottedPath = `${root}.${action}`
      for (const [localeName, bundle] of Object.entries(LOCALE_BUNDLES) as Array<[LocaleName, unknown]>) {
        const bag = getLocaleValueAtPath(bundle, root)
        const label =
          bag !== null && typeof bag === 'object'
            ? (bag as Record<string, string>)[action]
            : undefined
        if (typeof label !== 'string' || !label) {
          failures.push(`${localeName}: missing flat ${dottedPath}`)
        }
      }
    }
  }

  assert.deepEqual(failures, [], failures.slice(0, 20).join('\n'))
})

test('KB activity outcomes and detail values exist in every locale', () => {
  const failures: string[] = []

  for (const outcome of KB_ACTIVITY_OUTCOMES) {
    const path = `${KB_ACTIVITY_I18N_ROOTS.outcomes}.${outcome}`
    for (const [localeName, bundle] of Object.entries(LOCALE_BUNDLES) as Array<[LocaleName, unknown]>) {
      const label = getLocaleValueAtPath(bundle, path)
      if (typeof label !== 'string' || !label) failures.push(`${localeName}: missing ${path}`)
    }
  }

  for (const value of KB_ACTIVITY_DETAIL_VALUES) {
    const path = `${KB_ACTIVITY_I18N_ROOTS.detailValues}.${value}`
    for (const [localeName, bundle] of Object.entries(LOCALE_BUNDLES) as Array<[LocaleName, unknown]>) {
      const label = getLocaleValueAtPath(bundle, path)
      if (typeof label !== 'string' || !label) failures.push(`${localeName}: missing ${path}`)
    }
  }

  assert.deepEqual(failures, [], failures.slice(0, 20).join('\n'))
})

test('runtime queue worker pool and queue labels exist in every locale', () => {
  const failures: string[] = []

  for (const pool of RUNTIME_WORKER_POOL_NAMES) {
    for (const [localeName, keys] of Object.entries(localeKeysByName) as Array<[LocaleName, Set<string>]>) {
      if (!keys.has(`system.globalSettings.runtime.pools.${pool}`)) {
        failures.push(`${localeName}: missing system.globalSettings.runtime.pools.${pool}`)
      }
      if (!keys.has(`system.globalSettings.runtime.poolDescriptions.${pool}`)) {
        failures.push(`${localeName}: missing system.globalSettings.runtime.poolDescriptions.${pool}`)
      }
    }
  }

  for (const queue of RUNTIME_QUEUE_NAMES) {
    for (const [localeName, keys] of Object.entries(localeKeysByName) as Array<[LocaleName, Set<string>]>) {
      if (!keys.has(`system.globalSettings.runtime.queueNames.${queue}`)) {
        failures.push(`${localeName}: missing system.globalSettings.runtime.queueNames.${queue}`)
      }
      if (!keys.has(`system.globalSettings.runtime.queueDescriptions.${queue}`)) {
        failures.push(`${localeName}: missing system.globalSettings.runtime.queueDescriptions.${queue}`)
      }
    }
  }

  for (const taskType of RUNTIME_TASK_TYPE_KEYS) {
    for (const [localeName, keys] of Object.entries(localeKeysByName) as Array<[LocaleName, Set<string>]>) {
      if (!keys.has(`system.globalSettings.runtime.tasks.taskTypes.${taskType}`)) {
        failures.push(`${localeName}: missing system.globalSettings.runtime.tasks.taskTypes.${taskType}`)
      }
    }
  }

  assert.deepEqual(failures, [], failures.slice(0, 20).join('\n'))
})

test('parser engine display keys exist in every locale', () => {
  const failures: string[] = []

  for (const engineName of PARSER_ENGINE_NAMES) {
    for (const [localeName, keys] of Object.entries(localeKeysByName) as Array<[LocaleName, Set<string>]>) {
      for (const suffix of ['name', 'desc'] as const) {
        const key = `kbSettings.parser.engines.${engineName}.${suffix}`
        if (!keys.has(key)) failures.push(`${localeName}: missing ${key}`)
      }
    }
  }

  assert.deepEqual(failures, [], failures.join('\n'))
})

test('locale messages compile with vue-i18n syntax rules', () => {
  const failures = findAllLocaleMessageCompileErrors(LOCALE_BUNDLES)
  const summary = failures.map(
    (failure) => `${failure.path}: ${failure.message}\n  ${failure.value}`,
  )
  assert.deepEqual(summary, [], summary.slice(0, 20).join('\n'))
})

function assertLocalized(keys: string[]) {
  for (const key of keys) {
    for (const [locale, bundle] of Object.entries(LOCALE_BUNDLES)) {
      assert.equal(typeof getLocaleValueAtPath(bundle, key), 'string', `${locale}: missing ${key}`)
    }
  }
}

// datasource.syncError.* keys are checked against the backend emitters by
// internal/datasource/syncreason/parity_test.go.
test('Seafile picker texts are localized', () => {
  const keys = [
    'connector.seafile', 'connectorDesc.seafile',
    'seafile.baseUrl', 'seafile.apiToken', 'seafile.apiTokenHint',
    'seafile.singleLibraryOnly', 'seafile.selectionRequired',
    'resourceType.library', 'noResourcesDesc_seafile',
    ...[1, 2, 3].map(step => `guideStep${step}_seafile`),
  ].map(key => `datasource.${key}`)
  keys.push('knowledgeBase.channelSeafile')
  assertLocalized(keys)
})

test('the GitHub and GitLab form label groups have the same members', () => {
  const en = LOCALE_BUNDLES["en-US"] as any
  assert.deepEqual(Object.keys(en.datasource.github).sort(), [...Object.keys(en.datasource.gitlab), "baseUrlHint"].sort())
})

test('the Bitbucket form label group matches GitLab plus its own credential keys', () => {
  const en = LOCALE_BUNDLES["en-US"] as any
  const gitlab = Object.keys(en.datasource.gitlab).filter(key => key !== 'baseUrl')
  assert.deepEqual(Object.keys(en.datasource.bitbucket).sort(), [...gitlab, 'username', 'usernameHint', 'accessTokenHint'].sort())
})

test('DingTalk configuration texts are localized', () => {
  const keys = [
    'connector.dingtalk', 'connectorDesc.dingtalk',
    'field.clientId', 'field.clientSecret', 'field.operatorId', 'field.operatorIdHint',
    'prereqBarText_dingtalk', 'prereqOpenConsole_dingtalk',
    ...[1, 2, 3].flatMap(step => [`prereqStep${step}Brief_dingtalk`, `prereqStep${step}Desc_dingtalk`]),
  ].map(key => `datasource.${key}`)
  assertLocalized(keys)
})

test('OneDrive and SharePoint setup guide texts are localized', () => {
  const keys = ['onedrive', 'sharepoint'].flatMap(type => [
    `prereqBarText_${type}`, `prereqOpenConsole_${type}`,
    ...[1, 2, 3].flatMap(step => [`prereqStep${step}Brief_${type}`, `prereqStep${step}Desc_${type}`]),
  ]).map(key => `datasource.${key}`)
  assertLocalized(keys)
})

const SRC_ROOT = fileURLToPath(new URL('..', import.meta.url))

function listSourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name)
    if (entry.isDirectory()) return entry.name === 'locales' ? [] : listSourceFiles(path)
    return /\.(vue|ts|tsx|js)$/.test(entry.name) && !entry.name.endsWith('.test.ts') ? [path] : []
  })
}

// The embed widget (embed-main.ts) mounts its own i18n instance with the bundle in embed.ts.
const EMBED_ONLY_FILE = /^(embed-main\.ts|views\/embed\/|components\/Embed|composables\/useEmbed)/

const T_CALL = String.raw`(?<![\w$.])(?:\$t|t|i18n\.t|i18n\.global\.t)\(\s*`
const KEY = String.raw`(['"])([A-Za-z][\w-]*(?:\.[\w-]+)+)`
// Static string literals as the first argument, alone or as both arms of a ternary; template-built keys are out of scope.
const LITERAL_T_CALLS = [
  new RegExp(`${T_CALL}${KEY}\\1\\s*[,)]`, 'g'),
  new RegExp(`${T_CALL}[^'"(),?]*\\?\\s*${KEY}\\1\\s*:\\s*(['"])([A-Za-z][\\w-]*(?:\\.[\\w-]+)+)\\3\\s*[,)]`, 'g'),
]

test('every literal key passed to t() exists in en-US', () => {
  const missing = new Set<string>()
  for (const file of listSourceFiles(SRC_ROOT)) {
    const path = relative(SRC_ROOT, file)
    const bundle = EMBED_ONLY_FILE.test(path) ? EMBED_MESSAGES['en-US'] : LOCALE_BUNDLES['en-US']
    const source = readFileSync(file, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '')
    for (const pattern of LITERAL_T_CALLS) {
      for (const match of source.matchAll(pattern)) {
        for (const key of [match[2], match[4]].filter(Boolean)) {
          if (typeof getLocaleValueAtPath(bundle, key) !== 'string') missing.add(`${key} (${path})`)
        }
      }
    }
  }
  const sorted = [...missing].sort()
  assert.deepEqual(sorted, [], sorted.join('\n'))
})
