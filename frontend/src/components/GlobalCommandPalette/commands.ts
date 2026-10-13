import type { Router } from 'vue-router'
import type { Composer } from 'vue-i18n'
import { openNewUserGuide } from '@/config/contextualGuides'

/**
 * A single command that can be searched and invoked from the palette.
 * Kept intentionally minimal - commands are "just do something now", not
 * entities with detail pages.
 */
export interface CmdkCommand {
  id: string
  /** Localized display label. */
  label: string
  /** TDesign icon name rendered on the left. */
  icon: string
  /** Extra tokens used purely for fuzzy matching (aliases, synonyms). */
  keywords?: string[]
  /** Executed on primary action. Should close the palette itself if needed. */
  run: () => void
}

export interface CommandContext {
  router: Router
  t: Composer['t']
  /** Closes the palette; typically wires to commandPaletteStore.closePalette. */
  close: () => void
}

/**
 * Build the flat command list. Commands are intentionally static - dynamic
 * entities (KBs / agents / sessions) live in their own result groups.
 */
export function buildCommands(ctx: CommandContext): CmdkCommand[] {
  const { router, t, close } = ctx
  return [
    {
      id: 'new-chat',
      label: t('commandPalette.quick.newChat'),
      icon: 'chat-add',
      // Chinese search aliases match queries typed in Chinese.
      keywords: ['new', 'chat', 'conversation', '\u65b0\u5efa', '\u5bf9\u8bdd', 'создать'],
      run: () => {
        close()
        router.push('/platform/creatChat')
      },
    },
    {
      id: 'open-kb-list',
      label: t('commandPalette.quick.knowledgeBases'),
      icon: 'folder',
      keywords: ['kb', 'knowledge', 'base', '\u77e5\u8bc6\u5e93', '\u6587\u6863'],
      run: () => {
        close()
        router.push('/platform/knowledge-bases')
      },
    },
    {
      id: 'open-agents',
      label: t('commandPalette.quick.agents'),
      icon: 'user-circle',
      keywords: ['agent', 'bot', '\u667a\u80fd\u4f53', '\u52a9\u624b'],
      run: () => {
        close()
        router.push('/platform/agents')
      },
    },
    {
      id: 'open-organizations',
      label: t('commandPalette.quick.organizations'),
      icon: 'usergroup',
      keywords: ['org', 'organization', 'team', 'space', '\u7ec4\u7ec7', '\u5171\u4eab'],
      run: () => {
        close()
        router.push('/platform/organizations')
      },
    },
    {
      id: 'open-settings',
      label: t('commandPalette.quick.settings'),
      icon: 'setting',
      keywords: ['settings', 'preferences', 'config', '\u8bbe\u7f6e', '\u914d\u7f6e'],
      run: () => {
        close()
        router.push('/platform/settings')
      },
    },
    {
      id: 'open-product-tour',
      label: t('commandPalette.quick.productTour'),
      icon: 'help-circle',
      keywords: ['guide', 'tour', 'onboarding', 'help', '\u5f15\u5bfc', '\u65b0\u624b', '\u6559\u7a0b'],
      run: () => {
        close()
        openNewUserGuide()
      },
    },
  ]
}

/**
 * Filter commands by a free-text query. Matches on label OR any keyword,
 * case-insensitive. Empty query returns the full list unchanged so it can
 * double as a "default" empty-state listing.
 */
export function filterCommands(commands: CmdkCommand[], query: string): CmdkCommand[] {
  const q = (query || '').trim().toLowerCase()
  if (!q) return commands
  return commands.filter((cmd) => {
    if (cmd.label.toLowerCase().includes(q)) return true
    if (cmd.keywords) {
      for (const kw of cmd.keywords) {
        if (kw.toLowerCase().includes(q)) return true
      }
    }
    return false
  })
}
