<template>
  <Transition name="sandbox-panel" :duration="{ enter: 240, leave: 300 }">
    <div v-if="panel?.visible.value" class="chat-sandbox-panel-clip">
      <aside
        class="chat-sandbox-panel"
        :class="{ 'is-shifted': shifted, 'is-resizing': resizing }"
        :style="{ width: `${panel?.width.value ?? 420}px`, '--references-shift': `${shiftWidth}px` }"
        role="complementary"
        :aria-label="t('chat.sandbox.panelTitle')"
      >
      <!-- Drag handle on the left edge: press and drag left or right to resize the panel. -->
      <PanelResizeHandle edge="left" :label="t('knowledgeStages.resizeDrawer')"
        :value="panel.width.value" :min="SANDBOX_PANEL_MIN_WIDTH" :max="SANDBOX_PANEL_MAX_WIDTH"
        @start="startResize" @resize="resizePanel" @end="resizing = false" />
      <div class="chat-sandbox-panel__tabs">
        <div class="chat-sandbox-panel__tablist" role="tablist">
          <button
            v-for="tab in tabs"
            :key="tab.id"
            type="button"
            class="chat-sandbox-panel__tab"
            :class="{ 'is-active': panel?.activeTab.value === tab.id }"
            role="tab"
            :aria-selected="panel?.activeTab.value === tab.id"
            @click="panel?.open(tab.id)"
          >
            <t-icon :name="tab.icon" size="16px" />
            <span>{{ tab.label }}</span>
            <span
              v-if="tab.id === 'artifacts' && artifacts.length"
              class="chat-sandbox-panel__tab-count"
              aria-hidden="true"
            >{{ artifacts.length }}</span>
          </button>
        </div>
        <button
          type="button"
          class="chat-sandbox-panel__close"
          :aria-label="t('common.close')"
          @click="panel?.close()"
        >
          <t-icon name="close" size="16px" />
        </button>
      </div>

      <div
        class="chat-sandbox-panel__body"
        :class="{ 'is-flush': panel?.activeTab.value === 'artifacts' }"
      >
        <ChatArtifactsPanel
          v-show="panel?.activeTab.value === 'artifacts'"
          class="chat-sandbox-panel__artifacts"
          :session-id="sessionId"
          :items="artifacts"
          :collecting="artifactsCollecting"
          :active="panel?.activeTab.value === 'artifacts'"
          @deleted="emit('artifactDeleted', $event)"
        />

        <!-- Terminal: mounted lazily on first activation; tab switches use v-show to keep the instance (the PTY survives). -->
        <SandboxTerminal
          v-if="terminalMounted"
          v-show="panel?.activeTab.value === 'terminal'"
          :key="sessionId"
          ref="terminalRef"
          :session-id="sessionId"
          :agent-id="agentId"
          :agent-source-tenant-id="agentSourceTenantId"
          class="chat-sandbox-panel__terminal"
        />
        <div v-else-if="panel?.activeTab.value === 'terminal'" class="chat-sandbox-panel__placeholder">
          <t-skeleton animation="gradient" :row-col="[{ width: '100%', height: '100%', type: 'rect' }]" />
        </div>

        <!-- Desktop: the same lazy mount + v-show keep-alive as the terminal. Reconnecting the desktop costs far more than the terminal
             (the 3 to 8 second lazy start runs again), so disconnecting on a tab switch is unacceptable. -->
        <SandboxDesktop
          v-if="desktopMounted && desktopTabVisible"
          v-show="panel?.activeTab.value === 'desktop'"
          :key="sessionId"
          ref="desktopRef"
          :session-id="sessionId"
          :agent-id="agentId"
          :agent-source-tenant-id="agentSourceTenantId"
          class="chat-sandbox-panel__desktop"
        />
        <div v-else-if="panel?.activeTab.value === 'desktop' && desktopTabVisible" class="chat-sandbox-panel__placeholder">
          <t-skeleton animation="gradient" :row-col="[{ width: '100%', height: '100%', type: 'rect' }]" />
        </div>
      </div>
    </aside>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  useChatSandboxPanel,
  SANDBOX_PANEL_MIN_WIDTH,
  SANDBOX_PANEL_MAX_WIDTH,
  type SandboxPanelTab,
} from '@/composables/useChatSandboxPanel'
import SandboxTerminal from '@/views/chat/components/SandboxTerminal.vue'
import SandboxDesktop from '@/views/chat/components/SandboxDesktop.vue'
import ChatArtifactsPanel from '@/views/chat/components/ChatArtifactsPanel.vue'
import PanelResizeHandle from '@/components/PanelResizeHandle.vue'
import { useChatResourcesStore } from '@/stores/chatResources'
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities'
import type { SessionArtifactItem } from '@/utils/sessionArtifacts'

const props = withDefaults(
  defineProps<{
    sessionId: string
    /** The agent selected in the current session (the first connection creates a sandbox from its config). */
    agentId?: string
    /** Source space of a shared agent; omitted means an agent owned by this space. */
    agentSourceTenantId?: string | number | null
    /** Shift the whole panel left while the references panel is open, so the two fixed panels do not overlap. */
    shifted?: boolean
    /** Current width of the references panel (wider while viewing the original), which is how far to shift left. */
    shiftWidth?: number
    artifacts?: SessionArtifactItem[]
    artifactsCollecting?: boolean
  }>(),
  {
    artifacts: () => [],
    artifactsCollecting: false,
    shiftWidth: 420,
  },
)

// The artifact list is owned by the chat view (a computed over the loaded
// history), so a delete inside the panel has to travel back up to it.
const emit = defineEmits<{ (e: 'artifactDeleted', payload: { messageId: string; index: number }): void }>()

const { t } = useI18n()
const panel = useChatSandboxPanel()
const chatResources = useChatResourcesStore()
const deploymentCapabilities = useDeploymentCapabilitiesStore()
const sandboxConfigsReady = ref(false)

// Hide the desktop tab for CLI / Docker configs. Shared agents whose
// sandbox row is not in this workspace still show the tab and let the
// backend return DESKTOP_UNSUPPORTED.
const desktopTabVisible = computed(() => {
  if (!deploymentCapabilities.isSupported('settings.sandbox.remote')) return false
  const agentId = props.agentId?.trim()
  if (!agentId) return false
  const agent = chatResources.agents.find((item) => item.id === agentId)
  if (!agent) {
    return chatResources.agents.length > 0
  }
  const configId = agent.config?.sandbox_config_id?.trim()
  if (!configId) return false
  if (!sandboxConfigsReady.value) return false
  const cfg = chatResources.sandboxConfigs.find((item) => item.id === configId)
  if (!cfg) return true
  return Boolean(cfg.config?.desktop_enabled)
})

const tabs = computed(() => {
  const list: Array<{ id: SandboxPanelTab; icon: string; label: string }> = [
    { id: 'artifacts', icon: 'folder', label: t('chat.sandbox.tabArtifacts') },
    { id: 'terminal', icon: 'terminal', label: t('chat.sandbox.tabTerminal') },
  ]
  if (desktopTabVisible.value) {
    list.push({ id: 'desktop', icon: 'desktop', label: t('chat.sandbox.tabDesktop') })
  }
  return list
})

// Terminal and desktop mount lazily (the first time their tab is selected) and are destroyed when the panel closes (v-if),
// matching how ChatReferencesDrawer opens and closes; a session switch rebuilds them through :key.
const terminalMounted = ref(false)
const terminalRef = ref<{ focus?: () => void } | null>(null)
const desktopMounted = ref(false)
const desktopRef = ref<{ start?: () => void } | null>(null)

watch(
  () => [panel?.visible.value, panel?.activeTab.value] as const,
  ([visible, tab]) => {
    if (!visible) {
      // Drop the lazy-mount flags so reopening on Files does not remount
      // either panel (which would connect and refresh TTL).
      terminalMounted.value = false
      desktopMounted.value = false
      return
    }
    if (tab === 'terminal') {
      terminalMounted.value = true
      void nextTick(() => terminalRef.value?.focus?.())
      return
    }
    if (tab === 'desktop' && desktopTabVisible.value) {
      // Same as the terminal tab: mount runs a lookup-only connect. A
      // running sandbox attaches; paused or missing stays on the overlay
      // until the user confirms. Opening the panel must never create or
      // resume a microVM as a side effect.
      desktopMounted.value = true
    }
  },
  { immediate: true },
)

watch(
  () => [panel?.visible.value, props.agentId] as const,
  ([visible]) => {
    if (!visible) return
    void chatResources.ensureSandboxConfigs().finally(() => {
      sandboxConfigsReady.value = true
    })
  },
  { immediate: true },
)

watch(desktopTabVisible, (show) => {
  if (show) return
  if (panel?.activeTab.value === 'desktop') {
    panel.activeTab.value = 'artifacts'
  }
  desktopMounted.value = false
})

watch(
  () => props.sessionId,
  () => {
    panel?.clearArtifactFocus()
  },
)

// --- Drag the left edge to resize -------------------------------------------
const resizing = ref(false)
let resizeStartWidth = 0
function startResize() {
  if (!panel) return
  resizeStartWidth = panel.width.value
  resizing.value = true
}
function resizePanel(delta: number) {
  panel?.setWidth(resizeStartWidth - delta)
}

</script>

<style scoped lang="less">
.chat-sandbox-panel-clip {
  position: fixed;
  inset: 0;
  z-index: 1201;
  overflow: hidden;
  pointer-events: none;
}

.chat-sandbox-panel {
  pointer-events: auto;
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: min(420px, 100vw);
  max-width: 100vw;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-container);
  border-left: 1px solid var(--td-component-stroke);
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.06);

  &.is-shifted {
    @media (min-width: 1400px) {
      right: var(--references-shift, 420px);
    }
  }

  // Turn off transitions and text selection while resizing, so it follows the pointer.
  &.is-resizing {
    transition: none;
    user-select: none;

    .chat-sandbox-panel__body,
    .chat-sandbox-panel__tabs {
      pointer-events: none;
    }
  }
}

.chat-sandbox-panel__close {
  border: 0;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
  width: 28px;
  height: 28px;
  border-radius: var(--app-radius-md);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: background var(--app-motion-fast) ease, color var(--app-motion-fast) ease;

  &:hover {
    background: color-mix(in srgb, var(--td-text-color-primary) 8%, var(--td-bg-color-secondarycontainer));
    color: var(--td-text-color-primary);
  }
}

.chat-sandbox-panel__tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  height: var(--app-chat-header-height);
  box-sizing: border-box;
  padding: 0 12px;
  border-bottom: 1px solid var(--td-component-stroke);
  flex-shrink: 0;
}

.chat-sandbox-panel__tablist {
  display: flex;
  gap: 4px;
  min-width: 0;
  flex: 1;
  overflow-x: auto;
}

.chat-sandbox-panel__tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 8px;
  line-height: 20px;
  border: 0;
  border-radius: var(--app-radius-sm);
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-md);
  cursor: pointer;
  white-space: nowrap;
  transition: background-color var(--app-motion-fast) ease, color var(--app-motion-fast) ease;

  &:hover {
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-container-hover);
  }

  &.is-active {
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-secondarycontainer);
    font-weight: 500;
  }

  &:focus-visible {
    outline: 2px solid var(--td-text-color-secondary);
    outline-offset: -2px;
  }
}

.chat-sandbox-panel__tab-count {
  min-width: 16px;
  height: 16px;
  padding: 0 5px;
  border-radius: var(--app-radius-md);
  background: color-mix(in srgb, var(--td-text-color-primary) 8%, transparent);
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-xs);
  line-height: 16px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.chat-sandbox-panel__body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 8px;

  &.is-flush {
    padding: 0;
  }
}

.chat-sandbox-panel__terminal,
.chat-sandbox-panel__desktop,
.chat-sandbox-panel__artifacts {
  flex: 1;
  min-height: 0;
  min-width: 0;
}

.chat-sandbox-panel__placeholder {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-md);

  p {
    margin: 0;
  }
}

.sandbox-panel-enter-active .chat-sandbox-panel {
  transition:
    transform 0.24s cubic-bezier(0.22, 0.61, 0.36, 1),
    opacity 0.24s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.sandbox-panel-leave-active .chat-sandbox-panel {
  transition:
    transform 0.3s cubic-bezier(0.22, 0.61, 0.36, 1),
    opacity 0.3s cubic-bezier(0.22, 0.61, 0.36, 1);
}

.sandbox-panel-enter-from .chat-sandbox-panel,
.sandbox-panel-leave-to .chat-sandbox-panel {
  transform: translateX(100%);
  opacity: 0.6;
}
</style>
