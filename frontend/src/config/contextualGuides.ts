import type { SpotlightGuideStep } from '@/types/spotlightGuide'

export const GLOBAL_USER_GUIDE_KEY = 'aikb:new-user-guide-done:v1'
export const OPEN_NEW_USER_GUIDE_EVENT = 'aikb:open-new-user-guide'

export function openNewUserGuide() {
  window.dispatchEvent(new CustomEvent(OPEN_NEW_USER_GUIDE_EVENT))
}

export const KB_EDITOR_FOCUS_SECTION_EVENT = 'aikb:kb-editor-focus-section'
export const AGENT_EDITOR_FOCUS_SECTION_EVENT = 'aikb:agent-editor-focus-section'

export type ContextualGuideTourId =
  | 'kbList'
  | 'kbCreate'
  | 'kbDetail'
  | 'chat'
  | 'tenantModels'
  | 'agentList'
  | 'agentCreate'

export const focusKbEditorSection = (section: string) => {
  window.dispatchEvent(
    new CustomEvent(KB_EDITOR_FOCUS_SECTION_EVENT, { detail: { section } }),
  )
}

export const focusAgentEditorSection = (section: string) => {
  window.dispatchEvent(
    new CustomEvent(AGENT_EDITOR_FOCUS_SECTION_EVENT, { detail: { section } }),
  )
}

const focusKbEditorBasic = () => focusKbEditorSection('basic')

export interface ContextualGuideTourConfig {
  storageKey: string
  stepI18nPrefix: string
  steps: SpotlightGuideStep[]
  /** Delay before the first display (milliseconds) */
  openDelayMs: number
  /** Other guides marked completed together with this one */
  alsoCompleteTours?: ContextualGuideTourId[]
}

export const CONTEXTUAL_GUIDE_TOURS: Record<ContextualGuideTourId, ContextualGuideTourConfig> = {
  kbList: {
    storageKey: 'aikb:contextual-guide-kb-list:v2',
    stepI18nPrefix: 'contextualGuide.kbList.steps',
    openDelayMs: 500,
    steps: [
      {
        key: 'create',
        // With an empty list, highlight the centered main CTA first; otherwise fall back to the top-bar create button
        target: '.empty-state-btn[data-guide="kb-list-create"], [data-guide="kb-list-create"]',
        placement: 'bottom',
        interact: true,
      },
    ],
  },
  // Steps are assembled dynamically by KbCreateContextualGuide.vue for document knowledge bases and FAQ
  kbCreate: {
    storageKey: 'aikb:contextual-guide-kb-create:v3',
    stepI18nPrefix: 'contextualGuide.kbCreate.steps',
    openDelayMs: 450,
    alsoCompleteTours: ['kbList'],
    steps: [],
  },
  tenantModels: {
    storageKey: 'aikb:contextual-guide-tenant-models:v1',
    stepI18nPrefix: 'contextualGuide.tenantModels.steps',
    openDelayMs: 500,
    steps: [],
  },
  kbDetail: {
    storageKey: 'aikb:contextual-guide-kb-detail:v1',
    stepI18nPrefix: 'contextualGuide.kbDetail.steps',
    openDelayMs: 600,
    steps: [
      {
        key: 'intro',
      },
      {
        key: 'upload',
        target: '[data-guide="kb-detail-add-doc"]',
        placement: 'bottom',
      },
      { key: 'done' },
    ],
  },
  chat: {
    storageKey: 'aikb:contextual-guide-chat:v1',
    stepI18nPrefix: 'contextualGuide.chat.steps',
    openDelayMs: 800,
    steps: [
      {
        key: 'kb',
        target: '[data-guide="chat-kb-mention"]',
        placement: 'top',
        optional: true,
      },
      {
        key: 'input',
        target: '[data-guide="chat-input"]',
        placement: 'top',
      },
      {
        key: 'send',
        target: '[data-guide="chat-send"]',
        placement: 'top',
      },
      { key: 'done' },
    ],
  },
  agentList: {
    storageKey: 'aikb:contextual-guide-agent-list:v1',
    stepI18nPrefix: 'contextualGuide.agentList.steps',
    openDelayMs: 500,
    steps: [
      {
        key: 'create',
        target: '.empty-state-btn[data-guide="agent-list-create"], [data-guide="agent-list-create"]',
        placement: 'bottom',
        interact: true,
      },
    ],
  },
  agentCreate: {
    storageKey: 'aikb:contextual-guide-agent-create:v1',
    stepI18nPrefix: 'contextualGuide.agentCreate.steps',
    openDelayMs: 450,
    alsoCompleteTours: ['agentList'],
    steps: [],
  },
}

export function isContextualGuideDone(tourId: ContextualGuideTourId): boolean {
  return localStorage.getItem(CONTEXTUAL_GUIDE_TOURS[tourId].storageKey) === '1'
}

export function markContextualGuideDone(tourId: ContextualGuideTourId) {
  const config = CONTEXTUAL_GUIDE_TOURS[tourId]
  localStorage.setItem(config.storageKey, '1')
  config.alsoCompleteTours?.forEach((id) => {
    localStorage.setItem(CONTEXTUAL_GUIDE_TOURS[id].storageKey, '1')
  })
}

export function isGlobalUserGuideDone(): boolean {
  return localStorage.getItem(GLOBAL_USER_GUIDE_KEY) === '1'
}
