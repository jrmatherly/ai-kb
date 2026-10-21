<template>
  <SettingsModalShell :visible="visible" :title="$t('general.settings')" overlay-class="all-settings-overlay"
    @close="modalShell.requestClose">
    <template #nav>
      <template v-for="group in navGroups" :key="group.key">
        <div class="nav-group-title">{{ group.label }}</div>
        <template v-for="item in group.items" :key="item.key">
          <div :class="['nav-item', {
            'active': currentSection === item.key,
            'has-submenu': item.children && item.children.length > 0,
            'expanded': expandedMenus.includes(item.key)
          }]" @click="handleNavClick(item)">
            <!-- Web search uses a custom SVG icon -->
            <svg v-if="item.key === 'websearch'" width="17" height="17" viewBox="0 0 18 18" fill="none"
              xmlns="http://www.w3.org/2000/svg" class="nav-icon">
              <circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.2" fill="none" />
              <path d="M 9 2 A 3.5 7 0 0 0 9 16" stroke="currentColor" stroke-width="1.2" fill="none" />
              <path d="M 9 2 A 3.5 7 0 0 1 9 16" stroke="currentColor" stroke-width="1.2" fill="none" />
              <line x1="2.94" y1="5.5" x2="15.06" y2="5.5" stroke="currentColor" stroke-width="1.2"
                stroke-linecap="round" />
              <line x1="2.94" y1="12.5" x2="15.06" y2="12.5" stroke="currentColor" stroke-width="1.2"
                stroke-linecap="round" />
            </svg>
            <!-- AI KB Cloud uses a custom W icon -->
            <svg v-else-if="item.key === 'weknoracloud'" width="17" height="17" viewBox="0 0 18 18"
              fill="none" xmlns="http://www.w3.org/2000/svg" class="nav-icon">
              <rect x="1.5" y="1.5" width="15" height="15" rx="3.5" stroke="currentColor" stroke-width="1.2"
                fill="none" />
              <path d="M4.5 5.5L6.5 12.5L9 7.5L11.5 12.5L13.5 5.5" stroke="currentColor" stroke-width="1.3"
                stroke-linecap="round" stroke-linejoin="round" fill="none" />
            </svg>
            <!-- Sandbox: an isolated run window, so it does not share a server with Ollama / system settings -->
            <svg v-else-if="item.key === 'sandbox'" width="17" height="17" viewBox="0 0 18 18" fill="none"
              xmlns="http://www.w3.org/2000/svg" class="nav-icon">
              <rect x="2.5" y="3" width="13" height="12" rx="2" stroke="currentColor" stroke-width="1.2"
                fill="none" />
              <path d="M2.5 6.5h13" stroke="currentColor" stroke-width="1.2" />
              <path d="M5.5 10h4M5.5 12.5h2.5" stroke="currentColor" stroke-width="1.2"
                stroke-linecap="round" />
            </svg>
            <span v-else-if="item.emoji" class="nav-icon nav-icon-emoji">{{ item.emoji }}</span>
            <t-icon v-else :name="item.icon" class="nav-icon" />
            <span class="nav-label">{{ item.label }}</span>
            <t-icon v-if="item.children && item.children.length > 0"
              :name="expandedMenus.includes(item.key) ? 'chevron-down' : 'chevron-right'"
              class="expand-icon" />
          </div>

          <!-- Submenu -->
          <Transition name="submenu">
            <div v-if="item.children && expandedMenus.includes(item.key)" class="submenu">
              <div v-for="(child, childIndex) in item.children" :key="childIndex"
                :class="['submenu-item', { 'active': currentSubSection === child.key }]"
                @click.stop="handleSubMenuClick(item.key, child.key)">
                <span class="submenu-label">{{ child.label }}</span>
              </div>
            </div>
          </Transition>
        </template>
      </template>
    </template>
    <div class="content-wrapper" :class="{
      'content-wrapper--wide': currentSection === 'members',
      'content-wrapper--resource-list': ['models', 'mymemory', 'platform-api-keys', 'model-catalog', 'members', 'sandbox'].includes(currentSection),
      'content-wrapper--full': SYSTEM_ADMIN_SECTIONS.has(currentSection) || isIntegrationSection(currentSection),
    }">
      <!-- The role may not access the current section (arrived by deep link / role downgraded after a cross-workspace switch); this renders before any specific section.
           Normal navigation goes through the navItems filter and never lands here, but the watch(navItems) fallback fires at the moment
           the role is downgraded; this block is the fallback that keeps old URLs working. -->
      <div v-if="!canSeeSection(currentSection)" class="section role-denied">
        <div class="role-denied-icon">
          <t-icon name="lock-on" size="48px" />
        </div>
        <div class="role-denied-title">{{ $t('settings.roleDenied.title') }}</div>
        <div class="role-denied-desc">{{ $t('settings.roleDenied.desc') }}</div>
      </div>
      <template v-else>
        <!-- General settings -->
        <div v-if="currentSection === 'general'" class="section">
          <GeneralSettings />
        </div>

        <!-- Ollama settings -->
        <div v-if="currentSection === 'ollama'" class="section">
          <OllamaSettings />
        </div>

        <!-- AI KB Cloud -->
        <div v-if="currentSection === 'weknoracloud'" class="section">
          <WeKnoraCloudSettings />
        </div>

        <!-- Model config -->
        <div v-if="currentSection === 'models'" class="section">
          <ModelSettings />
        </div>

        <!-- Web search config -->
        <div v-if="currentSection === 'websearch'" class="section">
          <WebSearchSettings />
        </div>

        <!-- Message management -->
        <div v-if="currentSection === 'chathistory'" class="section">
          <ChatHistorySettings />
        </div>

        <!-- Long-term memory (workspace-level switch) -->
        <div v-if="currentSection === 'memory'" class="section">
          <MemoryWorkspaceSettings />
        </div>

        <!-- My memory (personal memory management) -->
        <div v-if="currentSection === 'mymemory'" class="section">
          <MemorySettings />
        </div>

        <!-- Sandbox secrets (the member's own skill / sandbox secrets) -->
        <div v-if="currentSection === 'envvars'" class="section">
          <EnvVarSettings />
        </div>

        <!-- Vector database engine -->
        <div v-if="currentSection === 'vectorstore'" class="section">
          <VectorStoreSettings />
        </div>

        <!-- Parser engine -->
        <div v-if="currentSection === 'parser'" class="section">
          <ParserEngineSettings />
        </div>

        <!-- Storage engine -->
        <div v-if="currentSection === 'storage'" class="section">
          <StorageBackendSettings />
        </div>

        <!-- Sandbox -->
        <div v-if="currentSection === 'sandbox'" class="section">
          <SandboxSettings />
        </div>

        <!-- System info -->
        <div v-if="currentSection === 'system'" class="section">
          <SystemInfo />
        </div>

        <!-- Global runtime settings visible to system administrators -->
        <div v-if="currentSection === 'system-global'" class="section">
          <SystemSettings />
        </div>

        <div v-if="currentSection === 'model-catalog'" class="section">
          <ModelCatalog />
        </div>

        <!-- Task queue status visible to system administrators -->
        <div v-if="currentSection === 'runtime-queues'" class="section">
          <RuntimeQueues />
        </div>

        <div v-if="currentSection === 'platform-api-keys'" class="section">
          <PlatformAPIKeys />
        </div>

        <div v-if="currentSection === 'system-audit-log'" class="section">
          <SystemAuditLog />
        </div>

        <!-- User info (basic account info: ID / username / email / registration time).
           A user's basic info should not be tied to owner permission. -->
        <div v-if="currentSection === 'userprofile'" class="section">
          <UserProfile />
        </div>

        <!-- Workspace info -->
        <div v-if="currentSection === 'tenant'" class="section">
          <TenantInfo />
        </div>

        <!-- Member management (#1303 PR 3) -->
        <div v-if="currentSection === 'members'" class="section">
          <TenantMembers />
        </div>

        <!-- Publishing integrations -->
        <div v-if="isIntegrationSection(currentSection)" class="section">
          <IntegrationSettingsSection :tab="integrationTabFromSection(currentSection)" />
        </div>
      </template>
    </div>
  </SettingsModalShell>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { LocationQueryRaw } from 'vue-router'
import { useUIStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'
import { useModalShell } from '@/composables/useModalShell'
import SettingsModalShell from '@/components/SettingsModalShell.vue'
import SystemInfo from './SystemInfo.vue'
import TenantInfo from './TenantInfo.vue'
import UserProfile from './UserProfile.vue'
import GeneralSettings from './GeneralSettings.vue'
import ModelSettings from './ModelSettings.vue'
import ModelCatalog from '../system/ModelCatalog.vue'
import OllamaSettings from './OllamaSettings.vue'
import WebSearchSettings from './WebSearchSettings.vue'
import ChatHistorySettings from './ChatHistorySettings.vue'
import MemorySettings from './MemorySettings.vue'
import EnvVarSettings from './EnvVarSettings.vue'
import MemoryWorkspaceSettings from './MemoryWorkspaceSettings.vue'
import VectorStoreSettings from './VectorStoreSettings.vue'
import ParserEngineSettings from './ParserEngineSettings.vue'
import StorageBackendSettings from './StorageBackendSettings.vue'
import SandboxSettings from './SandboxSettings.vue'
import WeKnoraCloudSettings from './WeKnoraCloudSettings.vue'
import TenantMembers from './TenantMembers.vue'
import SystemSettings from '@/views/system/SystemSettings.vue'
import RuntimeQueues from '@/views/system/RuntimeQueues.vue'
import PlatformAPIKeys from '@/views/system/PlatformAPIKeys.vue'
import SystemAuditLog from '@/views/system/SystemAuditLog.vue'
import IntegrationSettingsSection from '@/views/integrations/IntegrationSettingsSection.vue'
import {
  INTEGRATION_PREVIEW_ITEMS,
  INTEGRATION_TAB_MIN_ROLE,
} from '@/config/integrations'
import {
  SETTINGS_SECTION_MIN_ROLE,
  SYSTEM_ADMIN_SETTINGS_SECTIONS,
} from '@/config/settingsAccess'
import { isToolboxSection, toolboxLocation } from '@/config/toolbox'
import { hostSkillsOnly } from '@/utils/skillTarget'
import {
  buildSettingsRouteQuery,
  integrationSectionKey,
  integrationTabFromSection,
  isIntegrationSection,
  normalizeSettingsSection as normalizeSettingsSectionFromQuery,
  settingsQueryUnchanged,
} from '@/config/settingsRoute'

const route = useRoute()
const router = useRouter()
const uiStore = useUIStore()
const authStore = useAuthStore()
const deploymentCapabilities = useDeploymentCapabilitiesStore()
const { t, te } = useI18n()
const hostSkills = computed(() => hostSkillsOnly(
  deploymentCapabilities.isSupported('settings.sandbox.remote'),
  deploymentCapabilities.isSupported('settings.sandbox.host'),
))
function envNavLabel(): string {
  const hostKey = 'envVarSettings.host.title'
  if (hostSkills.value && te(hostKey)) return t(hostKey)
  return t('envVarSettings.title')
}

const currentSection = ref<string>('general')
const currentSubSection = ref<string>('')
const expandedMenus = ref<string[]>([])

type NavItem = {
  key: string
  icon: string
  label: string
  emoji?: string
  children?: Array<{ key: string; label: string }>
}

type NavGroup = {
  key: string
  label: string
  items: NavItem[]
}

// The minimum visible role for the settings second-level nav comes from settingsAccess.ts and
// aligns with the guard matrix in internal/router/router.go.
// The baseline is "the lowest role required by at least one meaningful write on the page": infrastructure
// config (models write, ollama download, websearch write, parser/storage/vector/mcp
// CRUD, sandbox connections, skills install, chat-history config) all goes to admin; read-only pages (general / system info /
// tenant-info / members roster) stay visible to viewer; the most sensitive reset api
// key is owner-only. Before changing this table, recheck the matching route groups in router.go.
//
// Notes:
// - The only switch on the chathistory page, "Enable message index", PUT /tenants/kv/chat-history-config,
//   goes through g.Admin() on the backend. Showing viewer/contributor the entry, letting them flip the switch and then
//   failing with 403 on save is a poor experience, so the entry itself belongs to admin.
// - The models list is readable by viewer, and the "+ Add model / Edit / Delete" buttons on the page are
//   gated separately with hasRole('admin') in ModelSettings.vue, so keeping the entry for
//   viewer is reasonable (contributors can browse the model list too).
const SYSTEM_ADMIN_SECTIONS = SYSTEM_ADMIN_SETTINGS_SECTIONS

const normalizeSettingsSection = (section: string) => {
  return normalizeSettingsSectionFromQuery(section, route.query.tab as string | undefined)
}

const syncSettingsRoute = (sectionKey: string) => {
  if (route.path !== '/platform/settings') return
  const query = buildSettingsRouteQuery(sectionKey, route.query)
  if (settingsQueryUnchanged(route.query, query)) return
  void router.replace({
    path: '/platform/settings',
    query: query as LocationQueryRaw,
  })
}

const isSectionSupported = deploymentCapabilities.isSettingsSectionSupported

const canSeeSection = (key: string): boolean => {
  if (isIntegrationSection(key)) {
    const min = INTEGRATION_TAB_MIN_ROLE[integrationTabFromSection(key)]
    if (!min) return true
    if (authStore.canAccessAllTenants) return true
    return authStore.hasRole(min)
  }
  if (SYSTEM_ADMIN_SECTIONS.has(key)) {
    return authStore.isSystemAdmin
  }
  const min = SETTINGS_SECTION_MIN_ROLE[key] ?? 'viewer'
  // canAccessAllTenants (superuser) must bypass just like the router layer, otherwise cross-tenant
  // admins cannot see the entries they are allowed to operate (see canManage in TenantMembers.vue).
  if (authStore.canAccessAllTenants) return true
  return authStore.hasRole(min)
}

const navItems = computed(() => {
  // Always go through the SETTINGS_SECTION_MIN_ROLE table, so ad-hoc isAdmin/isOwner checks do not scatter.
  // The server still enforces g.Viewer/Admin/Owner on every route; this only decides whether the UI
  // shows the entry. When changing entry rules, update settingsAccess.ts and the matching backend routes too.
  const integrationItems: NavItem[] = INTEGRATION_PREVIEW_ITEMS.map((item) => ({
    key: integrationSectionKey(item.key),
    icon: item.icon.type === 'icon' ? item.icon.name : 'integration',
    emoji: item.icon.type === 'emoji' ? item.icon.value : undefined,
    label: t(`integrations.tabs.${item.key}`),
  }))
  const all: NavItem[] = [
    { key: 'general', icon: 'setting', label: t('general.title') },
    { key: 'ollama', icon: 'server', label: 'Ollama' },
    { key: 'weknoracloud', icon: '', label: 'AI KB Cloud' },
    { key: 'models', icon: 'control-platform', label: t('settings.modelManagement') },
    { key: 'websearch', icon: 'search', label: t('settings.webSearchConfig') },
    { key: 'chathistory', icon: 'chat', label: t('chatHistorySettings.title') },
    { key: 'memory', icon: 'bulletpoint', label: t('memoryWorkspaceSettings.title') },
    { key: 'vectorstore', icon: 'data-base', label: t('settings.vectorStoreEngine') },
    { key: 'parser', icon: 'file-search', label: t('settings.parserEngine') },
    { key: 'storage', icon: 'cloud', label: t('settings.storageEngine') },
    { key: 'sandbox', icon: 'code', label: t('settings.sandbox.title') },
    { key: 'system', icon: 'info-circle', label: t('settings.versionInfo') },
    { key: 'system-global', icon: 'server', label: t('settings.system') },
    { key: 'model-catalog', icon: 'control-platform', label: t('modelCatalog.title') },
    { key: 'runtime-queues', icon: 'queue', label: t('settings.taskQueue') },
    { key: 'platform-api-keys', icon: 'secured', label: t('platformApiKeys.title') },
    { key: 'system-audit-log', icon: 'history', label: t('system.globalSettings.audit.tabLabel') },
    { key: 'userprofile', icon: 'user', label: t('userProfile.title') },
    { key: 'mymemory', icon: 'bookmark', label: t('memorySettings.title') },
    { key: 'envvars', icon: 'key', label: envNavLabel() },
    { key: 'tenant', icon: 'user-circle', label: t('settings.tenantInfo') },
    { key: 'members', icon: 'usergroup', label: t('tenantMember.title') },
    ...integrationItems,
  ]
  // An empty currentTenantRole means "membership not loaded yet". Rather than rendering the whole set of
  // viewer entries and having them vanish when the role arrives, hold off rendering, which is more stable and matches the old members
  // entry strategy.
  if (!authStore.currentTenantRole && !authStore.canAccessAllTenants) {
    return [] as NavItem[]
  }
  return all.filter((it) => canSeeSection(it.key) && isSectionSupported(it.key))
})

const navGroups = computed<NavGroup[]>(() => {
  const itemMap = new Map(navItems.value.map((item) => [item.key, item]))
  const pickItems = (keys: string[]) => keys.map((key) => itemMap.get(key)).filter(Boolean) as NavItem[]
  // Groups: Account → Workspace → Models → Publishing integrations → Data and extensions → System administration → Platform
  // Key changes: personal preferences (general) and user info move into "Account";
  // the in-workspace feature switch (chathistory) moves from "Platform" to "Workspace";
  // retrieval engines and external integrations merge into "Data and extensions", avoiding two narrow groups of 2-3 items.
  return [
    {
      key: 'account',
      label: t('settings.navGroups.account'),
      items: pickItems(['general', 'userprofile', 'mymemory', 'envvars']),
    },
    {
      key: 'workspace',
      label: t('settings.navGroups.workspace'),
      items: pickItems(['tenant', 'members', 'chathistory', 'memory']),
    },
    {
      key: 'models_runtime',
      label: t('settings.navGroups.modelsRuntime'),
      items: pickItems(['models', 'ollama', 'weknoracloud']),
    },
    {
      key: 'integrations',
      label: t('integrations.title'),
      items: pickItems(INTEGRATION_PREVIEW_ITEMS.map((item) => integrationSectionKey(item.key))),
    },
    {
      key: 'data_extensions',
      label: t('settings.navGroups.dataExtensions'),
      items: pickItems([
        'vectorstore',
        'parser',
        'storage',
        'sandbox',
        'websearch',
      ]),
    },
    {
      key: 'system_administration',
      label: t('settings.navGroups.systemAdministration'),
      items: pickItems(['system-global', 'model-catalog', 'runtime-queues', 'platform-api-keys', 'system-audit-log']),
    },
    {
      key: 'platform',
      label: t('settings.navGroups.platform'),
      items: pickItems(['system']),
    },
  ].filter((group) => group.items.length > 0)
})

// Nav item click handling
const handleNavClick = (item: any) => {
  if (item.children && item.children.length > 0) {
    // Has a submenu: toggle its expanded state
    const index = expandedMenus.value.indexOf(item.key)
    if (index > -1) {
      expandedMenus.value.splice(index, 1)
    } else {
      expandedMenus.value.push(item.key)
    }
    currentSubSection.value = item.children[0].key
  } else {
    currentSubSection.value = ''
  }

  // Switch to the matching page and sync the URL to ?section=<navKey> (integration-claw included).
  // Otherwise, arriving from another section leaves the query unchanged and the route watcher pulls the content back.
  currentSection.value = item.key
  syncSettingsRoute(item.key)
}

// Submenu click handling
const handleSubMenuClick = (parentKey: string, childKey: string) => {
  currentSection.value = parentKey
  currentSubSection.value = childKey

  // Scroll to the matching model type area
  setTimeout(() => {
    const element = document.querySelector(`[data-model-type="${childKey}"]`)
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, 100)
}

// Controls modal visibility
const visible = computed(() => {
  return route.path === '/platform/settings' || uiStore.showSettingsModal
})

// Close the modal
const handleClose = () => {
  // Blur before unmount so TDesign textarea autosize won't run on a detached node.
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur()
  }
  uiStore.closeSettings()
  // If the current route is the settings page, go back
  if (route.path === '/platform/settings') {
    const sec = route.query.section
    if (sec === 'model-catalog' || sec === 'system-global' || sec === 'runtime-queues' || sec === 'platform-api-keys' || sec === 'system-audit-log') {
      router.push('/platform/knowledge-bases')
    } else {
      router.back()
    }
  }
}

// Existing in-product shortcuts can still call openSettings; moved tools
// always navigate to the toolbox, with the requested sandbox selection intact.
const redirectToToolbox = (section: string, subSection?: string | null) => {
  if (!isToolboxSection(section)) return false
  uiStore.closeSettings()
  void router.push(toolboxLocation(section, subSection || undefined))
  return true
}

// Watch the initial navigation settings
watch(() => uiStore.settingsInitialSection, (section) => {
  if (section && visible.value) {
    const normalizedSection = normalizeSettingsSection(section)
    if (redirectToToolbox(normalizedSection, uiStore.settingsInitialSubSection)) return
    if (deploymentCapabilities.loaded && !isSectionSupported(normalizedSection)) {
      MessagePlugin.warning(t('settings.capabilityUnavailable'))
      currentSection.value = navItems.value[0]?.key || 'general'
      currentSubSection.value = ''
      return
    }
    currentSection.value = normalizedSection
    syncSettingsRoute(normalizedSection)
    const navItem = (navItems.value as any[]).find((item) => item.key === normalizedSection)
    if (navItem && navItem.children && navItem.children.length > 0) {
      if (!expandedMenus.value.includes(section)) {
        expandedMenus.value.push(section)
      }
      currentSubSection.value = uiStore.settingsInitialSubSection || navItem.children[0].key
      if (uiStore.settingsInitialSubSection) {
        setTimeout(() => {
          const element = document.querySelector(`[data-model-type="${uiStore.settingsInitialSubSection}"]`)
          if (element) {
            element.scrollIntoView({ behavior: 'smooth', block: 'start' })
          }
        }, 300)
      }
    } else {
      currentSubSection.value = ''
    }
  }
}, { immediate: true })

watch(
  () => [visible.value, route.path, route.query.section, deploymentCapabilities.loaded] as const,
  ([isVisible, path, section, capabilitiesLoaded]) => {
    if (!isVisible || path !== '/platform/settings') return
    if (typeof section !== 'string') {
      syncSettingsRoute(currentSection.value || 'general')
      return
    }
    const normalizedSection = normalizeSettingsSectionFromQuery(
      section,
      typeof route.query.tab === 'string' ? route.query.tab : undefined,
    )
    if (capabilitiesLoaded && !isSectionSupported(normalizedSection)) {
      MessagePlugin.warning(t('settings.capabilityUnavailable'))
      const fallback = navItems.value[0]?.key || 'general'
      currentSection.value = fallback
      currentSubSection.value = ''
      syncSettingsRoute(fallback)
      return
    }
    currentSection.value = normalizedSection
    currentSubSection.value = ''
    syncSettingsRoute(normalizedSection)
  },
  { immediate: true },
)

// The role may change after switching workspaces, and admin-only panels that were visible may disappear.
// If currentSection lands on a key that is no longer shown, fall back to the first visible item.
watch(navItems, (items) => {
  if (!items.some((item) => item.key === currentSection.value)) {
    const fallback = items[0]?.key || 'general'
    currentSection.value = fallback
    currentSubSection.value = ''
    syncSettingsRoute(fallback)
  }
})

// Close on Esc / mask click (the same shell interaction as the other settings modals)
const modalShell = useModalShell({
  visible: () => visible.value,
  close: handleClose,
})

// Handle quick navigation events
const handleSettingsNav = (e: CustomEvent) => {
  const { section, subsection } = e.detail
  if (section) {
    const normalizedSection = normalizeSettingsSection(section)
    if (redirectToToolbox(normalizedSection, subsection)) return
    if (deploymentCapabilities.loaded && !isSectionSupported(normalizedSection)) {
      MessagePlugin.warning(t('settings.capabilityUnavailable'))
      currentSection.value = navItems.value[0]?.key || 'general'
      currentSubSection.value = ''
      return
    }
    currentSection.value = normalizedSection
    syncSettingsRoute(normalizedSection)
    // If there is a submenu, expand it automatically
    const navItem = (navItems.value as any[]).find((item: any) => item.key === normalizedSection)
    if (navItem && navItem.children && navItem.children.length > 0) {
      if (!expandedMenus.value.includes(section)) {
        expandedMenus.value.push(section)
      }
      // If there is a subsection, select the matching submenu item
      currentSubSection.value = subsection || navItem.children[0].key
    }
  }
}

onMounted(() => {
  window.addEventListener('settings-nav', handleSettingsNav as EventListener)
})

watch(currentSection, () => {
  if (document.activeElement instanceof HTMLElement) {
    document.activeElement.blur()
  }
})

onUnmounted(() => {
  window.removeEventListener('settings-nav', handleSettingsNav as EventListener)
})
</script>

<style lang="less">
.settings-modal-shell.all-settings-overlay > .settings-modal {
  max-width: 1280px;
  height: 900px;
}
</style>

<style lang="less" scoped>
/* Mask layer */
/* Modal container */
/* Close button */
/* Left nav: slightly more compact than the first version, with moderate font size and spacing */
.expand-icon {
  margin-left: 4px;
  font-size: var(--app-text-base);
  transition: transform var(--app-motion-base) ease;
}

/* Submenu */
.submenu {
  margin-left: 28px;
  margin-bottom: 3px;
  overflow: hidden;
}

.submenu-item {
  padding: 5px 12px;
  margin-bottom: 2px;
  border-radius: var(--app-radius-xs);
  cursor: pointer;
  color: var(--td-text-color-primary);
  font-size: var(--app-text-md);
  transition: all var(--app-motion-base) ease;
  user-select: none;

  &:hover {
    background-color: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }

  &.active {
    background-color: var(--td-bg-color-secondarycontainer);
    color: var(--td-brand-color);
    font-weight: 500;
  }
}

.submenu-label {
  display: block;
}

/* Submenu animation */
.submenu-enter-active,
.submenu-leave-active {
  transition: all var(--app-motion-base) ease;
}

.submenu-enter-from {
  opacity: 0;
  max-height: 0;
}

.submenu-enter-to {
  opacity: 1;
  max-height: 300px;
}

.submenu-leave-from {
  opacity: 1;
  max-height: 300px;
}

.submenu-leave-to {
  opacity: 0;
  max-height: 0;
}

/* Right content area */
.content-wrapper {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 40px 48px;

  /* Table pages use less padding, leaving room for multi-column content. */
  &--wide {
    padding: 32px 36px 40px;
  }

  &--full {
    padding: 30px 34px 40px;
  }

  &--resource-list {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    padding: 20px 28px 0;

    > .section {
      flex: 1;
      min-height: 0;
    }

    @media (max-width: 720px) {
      padding: 16px 16px 0;
    }
  }
}

.section {
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }

  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Modal animation */
/* Scrollbar styles */
.role-denied {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 64px 24px;
  gap: 12px;
  min-height: 240px;

  .role-denied-icon {
    color: var(--td-text-color-placeholder);
  }

  .role-denied-title {
    font-size: var(--app-text-xl);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .role-denied-desc {
    font-size: var(--app-text-md);
    color: var(--td-text-color-secondary);
    max-width: 360px;
    line-height: 1.6;
  }
}
</style>
