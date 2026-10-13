import { compile, type CompileError } from '@intlify/core-base'

import enUS from './locales/en-US.ts'

export const LOCALE_BUNDLES = {
  'en-US': enUS,
} as const

export type LocaleName = keyof typeof LOCALE_BUNDLES

/** Keys the app builds at runtime, so a missing one only shows up as a raw key in the UI. */
export const CRITICAL_LOCALE_KEYS = [
  'newUserGuide.steps.welcome.title',
  'contextualGuide.kbList.steps.create.title',
  'tenantMember.audit.action.rbac.member_added',
  'system.globalSettings.audit.action.system.setting_changed',
  'knowledgeEditor.activity.targets.knowledge_base',
  'modelSettings.builtinModels.descriptionAdmin',
  'model.editor.description.chat',
  'model.editor.description.asr',
  'inviteRegister.emailPlaceholder',
  'knowledgeList.sections.tenantOthers',
  'agent.sections.tenantOthers',
] as const

/** Flatten a locale object into dot-path keys without recursive flatMap allocations. */
export function collectLocaleKeys(root: unknown): Set<string> {
  const keys = new Set<string>()
  const stack: Array<{ value: unknown; path: string }> = [{ value: root, path: '' }]

  while (stack.length > 0) {
    const current = stack.pop()!
    const { value, path } = current

    if (typeof value === 'string') {
      if (path) keys.add(path)
      continue
    }

    if (Array.isArray(value)) {
      for (let index = value.length - 1; index >= 0; index -= 1) {
        stack.push({ value: value[index], path: `${path}[${index}]` })
      }
      continue
    }

    if (!value || typeof value !== 'object') continue

    for (const [key, child] of Object.entries(value as Record<string, unknown>)) {
      const nextPath = path ? `${path}.${key}` : key
      if (typeof child === 'string') keys.add(nextPath)
      else stack.push({ value: child, path: nextPath })
    }
  }

  return keys
}

export type LocaleMessageEntry = {
  path: string
  value: string
}

export type LocaleMessageCompileError = {
  path: string
  value: string
  message: string
}

const LOCALE_MESSAGE_COMPILE_OPTIONS = {
  warnHtmlMessage: false,
} as const

/** Flatten a locale object into dot-path message entries. */
export function collectLocaleMessages(root: unknown, path = ''): LocaleMessageEntry[] {
  if (typeof root === 'string') {
    return path ? [{ path, value: root }] : []
  }

  if (Array.isArray(root)) {
    return root.flatMap((item, index) => collectLocaleMessages(item, `${path}[${index}]`))
  }

  if (!root || typeof root !== 'object') return []

  return Object.entries(root as Record<string, unknown>).flatMap(([key, value]) => {
    const nextPath = path ? `${path}.${key}` : key
    return collectLocaleMessages(value, nextPath)
  })
}

/** Compile a locale message with the same vue-i18n rules used at runtime. */
export function compileLocaleMessage(
  value: string,
  context: { key: string; locale: string } = { key: 'audit', locale: 'en-US' },
): void {
  compile(value, {
    ...LOCALE_MESSAGE_COMPILE_OPTIONS,
    key: context.key,
    locale: context.locale,
    onError: (error: CompileError) => {
      throw error
    },
  })
}

export function findLocaleMessageCompileErrors(
  root: unknown,
  localeName = 'locale',
): LocaleMessageCompileError[] {
  const failures: LocaleMessageCompileError[] = []

  for (const { path, value } of collectLocaleMessages(root)) {
    try {
      compileLocaleMessage(value, { key: path, locale: localeName })
    } catch (error) {
      failures.push({
        path: `${localeName}:${path}`,
        value,
        message: error instanceof Error ? error.message : String(error),
      })
    }
  }

  failures.sort((left, right) => left.path.localeCompare(right.path))
  return failures
}

export function findAllLocaleMessageCompileErrors(
  bundles: Record<string, unknown>,
): LocaleMessageCompileError[] {
  return Object.entries(bundles).flatMap(([localeName, bundle]) =>
    findLocaleMessageCompileErrors(bundle, localeName),
  )
}

function getLocaleValueAtPathParts(current: unknown, parts: string[]): unknown {
  if (parts.length === 0) return current
  if (current == null || typeof current !== 'object' || Array.isArray(current)) return undefined

  const object = current as Record<string, unknown>
  // Audit action bags hold flat dotted keys (`kb.created`), so try the longest key first.
  for (let length = parts.length; length >= 1; length -= 1) {
    const key = parts.slice(0, length).join('.')
    if (!Object.prototype.hasOwnProperty.call(object, key)) continue

    const next = object[key]
    if (length === parts.length) return next
    return getLocaleValueAtPathParts(next, parts.slice(length))
  }

  return undefined
}

export function getLocaleValueAtPath(root: unknown, path: string): unknown {
  return getLocaleValueAtPathParts(root, path.split('.'))
}
