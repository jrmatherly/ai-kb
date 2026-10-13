<template>
  <div class="websearch-settings">
    <div class="section-header">
      <h2>{{ t('webSearchSettings.title') }}</h2>
      <p class="section-description">{{ t('webSearchSettings.description') }}</p>
    </div>

    <h3 class="list-section-title">{{ t('webSearchSettings.providersTitle') }}</h3>

    <!-- Provider cards keep their page-specific actions beside the provider details. -->
    <div v-if="providerEntities.length === 0 && !authStore.hasRole('admin')" class="empty-state">
      <t-empty :description="t('webSearchSettings.noProvidersDesc')" />
    </div>
    <div v-else class="provider-grid">
      <div
        v-for="entity in providerEntities"
        :key="entity.id"
        class="provider-card"
        :class="[`provider-card--${entity.provider}`, { 'provider-card--clickable': isProviderCardClickable() }]"
        :role="isProviderCardClickable() ? 'button' : undefined"
        :tabindex="isProviderCardClickable() ? 0 : undefined"
        @click="onProviderCardClick($event, entity)"
        @keydown.enter="onProviderCardClick($event, entity)"
      >
        <div
          class="provider-card__badge"
          :class="badgeClass(entity.provider)"
          :style="badgeStyle(entity.provider)"
          :aria-label="entity.provider"
        >
          <img
            v-if="resolveLogo(entity.provider)?.mode === 'color'"
            :src="resolveLogo(entity.provider)!.url"
            :alt="entity.provider"
            class="provider-card__badge-img"
          />
          <template v-else-if="!resolveLogo(entity.provider)">
            {{ providerInitial(entity.provider) }}
          </template>
        </div>
        <div class="provider-card__body">
          <div class="provider-card__header">
            <h3 class="provider-card__title" :title="entity.name">{{ entity.name }}</h3>
            <div
              v-if="getProviderOptions(entity).length > 0"
              class="provider-card__actions"
              @click.stop
            >
              <t-dropdown
                :options="getProviderOptions(entity)"
                placement="bottom-right"
                attach="body"
                trigger="click"
                @click="(data: any) => handleMenuAction({ value: data.value }, entity)"
              >
                <t-button variant="text" shape="square" size="small" class="provider-card__more">
                  <t-icon name="ellipsis" />
                </t-button>
              </t-dropdown>
            </div>
          </div>
          <div class="provider-card__subtitle">
            <span class="provider-card__type">{{ providerTypeLabel(entity.provider) }}</span>
            <template v-if="entity.description">
              <span class="provider-card__sep">·</span>
              <span class="provider-card__desc" :title="entity.description">{{ entity.description }}</span>
            </template>
          </div>
          <div v-if="entity.parameters?.proxy_url" class="provider-card__url" :title="entity.parameters.proxy_url">
            {{ entity.parameters.proxy_url }}
          </div>
        </div>
      </div>
      <button
        v-if="authStore.hasRole('admin')"
        type="button"
        class="provider-card provider-card--add"
        @click="openAddDialog"
      >
        <span class="provider-card--add__icon" aria-hidden="true">
          <add-icon />
        </span>
        <span class="provider-card--add__label">{{ t('webSearchSettings.addProvider') }}</span>
      </button>
    </div>

    <!-- Add/Edit Drawer, in the same style as the ModelEditorDialog / Parser / Storage drawers -->
    <SettingDrawer
      v-model:visible="showAddProviderDialog"
      :title="editingProvider ? t('webSearchSettings.editProvider') : t('webSearchSettings.addProvider')"
      :class="drawerClass"
      :confirm-loading="saving"
      @confirm="saveProvider"
    >
      <!--
        Header icon: the same logo/mono/fallback as the list's .provider-card__badge.
        - color logo (such as the Bing/Google color marks) → <img>, and the header container gets a white background + thin border
        - mono logo (mask-image) → ::before-style span, tinted with currentColor
        - fallback: providerId initial-letter monogram
      -->
      <template v-if="selectedProviderType" #headerIcon>
        <img
          v-if="drawerLogo?.mode === 'color'"
          :src="drawerLogo.url"
          :alt="selectedProviderType.id"
          class="header-icon__img"
        />
        <span
          v-else-if="drawerLogo?.mode === 'mono'"
          class="header-icon__mono"
          :style="drawerLogoStyle"
        />
        <span v-else class="header-icon__text">{{ providerInitial(selectedProviderType.id) }}</span>
      </template>

      <!--
        Subtitle: provider type name + official docs link (if any).
      -->
      <template v-if="selectedProviderType" #subtitle>
        <span>{{ selectedProviderType.name }}</span>
        <a
          v-if="selectedProviderType.docs_url"
          :href="selectedProviderType.docs_url"
          target="_blank"
          rel="noopener noreferrer"
          class="doc-link doc-link--inline"
        >
          {{ t('webSearchSettings.viewDocs') }}
          <t-icon name="link" class="link-icon" />
        </a>
      </template>

      <!--
        Test connection (footer-left, same as the other drawers). This is now the single entry point:
        the outer card menu no longer shows "Test connection", so every test starts here.

        Every provider shows the button (including "free" ones such as DuckDuckGo / SearXNG).
        Free only means no api_key, not that no test is needed: DuckDuckGo goes over the internet and may be blocked,
        and SearXNG is self-hosted, so base_url reachability needs checking. disabled is controlled by canTestConnection
        throughout, graying the button out when any required field is missing.
      -->
      <template v-if="selectedProviderType" #footer-left>
        <t-button
          variant="outline"
          :loading="testing"
          :disabled="!canTestConnection"
          @click="testConnection"
        >
          <template #icon>
            <t-icon
              v-if="!testing && lastTestOk === true"
              name="check-circle-filled"
              class="status-icon available"
            />
            <t-icon
              v-else-if="!testing && lastTestOk === false"
              name="close-circle-filled"
              class="status-icon unavailable"
            />
          </template>
          {{ testing ? t('webSearchSettings.testing') : t('webSearchSettings.testConnection') }}
        </t-button>
      </template>

      <t-form ref="formRef" :data="providerForm" label-align="top" class="provider-form">
        <!-- Section 1: basic info -->
        <section class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ t('webSearchSettings.basicSection', 'Basic') }}</h4>

          <!-- providerType picker: editable only when creating -->
          <div class="form-item">
            <label class="form-label required">{{ t('webSearchSettings.providerTypeLabel') }}</label>
            <t-select
              v-model="providerForm.provider"
              :disabled="!!editingProvider"
              @change="onProviderTypeChange"
            >
              <!--
                Just provider name in each option. We used to append a "Free"
                t-tag for providers that don't take an api_key, but the
                "Free" category did not help users decide (DuckDuckGo / SearXNG also
                need a working network or self-hosted instance) and took up visual space.
              -->
              <t-option v-for="pt in providerTypes" :key="pt.id" :value="pt.id" :label="pt.name" />
            </t-select>
          </div>

          <div class="form-item">
            <label class="form-label">{{ t('webSearchSettings.providerNameLabel') }}</label>
            <t-input
              v-model="providerForm.name"
              :placeholder="selectedProviderType?.name || t('webSearchSettings.providerNamePlaceholder')"
            />
          </div>

          <div class="form-item">
            <label class="form-label">{{ t('webSearchSettings.providerDescLabel') }}</label>
            <t-input
              v-model="providerForm.description"
              :placeholder="t('webSearchSettings.providerDescPlaceholder')"
            />
          </div>
        </section>

        <!-- Section 2: connection config (base url / api key / engine id), rendered only when some field needs it -->
        <section
          v-if="selectedProviderType?.requires_api_key || selectedProviderType?.supports_optional_api_key || selectedProviderType?.requires_engine_id || selectedProviderType?.requires_base_url || selectedProviderType?.config_fields?.length"
          class="setting-drawer__section"
        >
          <h4 class="setting-drawer__section-title">{{ t('webSearchSettings.credentialsSection', 'Connection') }}</h4>

          <div v-if="selectedProviderType?.requires_base_url" class="form-item">
            <label class="form-label required">{{ t('webSearchSettings.baseUrlLabel') }}</label>
            <t-input
              v-model="providerForm.parameters.base_url"
              :placeholder="t('webSearchSettings.baseUrlPlaceholder')"
            />
          </div>

          <!--
            In Edit mode credentials are managed by CredentialResource (separate /credentials
            subresource calls), not coupled to this form's submit; Create mode uses a plain
            password input + lock prefix-icon, matching ModelEditorDialog.
          -->
          <div v-if="selectedProviderType?.requires_api_key || selectedProviderType?.supports_optional_api_key" class="form-item">
            <label class="form-label" :class="{ required: selectedProviderType?.requires_api_key }">
              {{ selectedProviderType?.supports_optional_api_key && !selectedProviderType?.requires_api_key
                ? t('webSearchSettings.apiKeyOptionalLabel', 'API Key (optional)')
                : t('webSearchSettings.apiKeyLabel') }}
            </label>
            <CredentialResource
              v-if="editingProvider?.id"
              :api="credentialApi"
              :fields="credentialFields"
              :meta="credentialMeta"
            />
            <t-input
              v-else
              v-model="providerForm.parameters.api_key"
              type="password"
              :placeholder="apiKeyPlaceholder"
            >
              <template #prefix-icon><t-icon name="lock-on" /></template>
            </t-input>
          </div>

          <div v-if="selectedProviderType?.requires_engine_id" class="form-item">
            <label class="form-label required">{{ t('webSearchSettings.engineIdLabel') }}</label>
            <t-input
              v-model="providerForm.parameters.engine_id"
              :placeholder="t('webSearchSettings.engineIdLabel')"
            />
          </div>

          <div
            v-for="field in selectedProviderType?.config_fields || []"
            :key="field.key"
            class="form-item"
          >
            <label class="form-label" :class="{ required: field.required }">
              {{ configFieldText(field.label_key, field.label) }}
            </label>
            <t-select
              v-if="field.type === 'select'"
              v-model="providerForm.parameters.extra_config[field.key]"
            >
              <t-option
                v-for="option in field.options || []"
                :key="option.value"
                :value="option.value"
                :label="configFieldText(option.label_key, option.label)"
              />
            </t-select>
            <p v-if="field.description" class="form-desc">
              {{ configFieldText(field.description_key, field.description) }}
            </p>
          </div>
        </section>

        <!-- Section 3: options (proxy / default) -->
        <section
          v-if="selectedProviderType?.supports_proxy || selectedProviderType"
          class="setting-drawer__section"
        >
          <h4 class="setting-drawer__section-title">{{ t('webSearchSettings.optionsSection', 'Options') }}</h4>

          <div v-if="selectedProviderType?.supports_proxy" class="form-item">
            <label class="form-label">{{ t('webSearchSettings.proxyUrlLabel') }}</label>
            <t-input
              v-model="providerForm.parameters.proxy_url"
              :placeholder="t('webSearchSettings.proxyUrlPlaceholder')"
            />
            <p class="form-desc">{{ t('webSearchSettings.proxyUrlHelp') }}</p>
          </div>

          <div class="form-item">
            <label class="form-label">{{ t('webSearchSettings.setAsDefault') }}</label>
            <div class="vision-toggle">
              <t-switch v-model="providerForm.is_default" />
              <span class="form-desc form-desc--inline">{{ t('webSearchSettings.setAsDefaultDesc') }}</span>
            </div>
          </div>
        </section>
      </t-form>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { AddIcon } from 'tdesign-icons-vue-next'
import {
  listWebSearchProviderTypes,
  createWebSearchProvider,
  updateWebSearchProvider,
  deleteWebSearchProvider as deleteWebSearchProviderAPI,
  testWebSearchProvider,
  putWebSearchProviderCredentials,
  deleteWebSearchProviderCredentialField,
  type WebSearchProviderEntity,
  type WebSearchProviderTypeInfo,
  type WebSearchCredentialField,
} from '@/api/web-search-provider'
import SettingDrawer from '@/components/settings/SettingDrawer.vue'
import CredentialResource, {
  type CredentialFieldDef,
  type CredentialResourceApi,
} from '@/components/credentials/CredentialResource.vue'
import { useConfirmDelete } from '@/components/settings/useConfirmDelete'
import { useAuthStore } from '@/stores/auth'
import { useChatResourcesStore } from '@/stores/chatResources'
import { providerLogo } from './providerLogos'

const { t } = useI18n()
const authStore = useAuthStore()
const chatResources = useChatResourcesStore()
const confirmDelete = useConfirmDelete()

// ===== State =====
const providerEntities = ref<WebSearchProviderEntity[]>([])
const providerTypes = ref<WebSearchProviderTypeInfo[]>([])
const showAddProviderDialog = ref(false)
const editingProvider = ref<WebSearchProviderEntity | null>(null)
const testing = ref(false)
const saving = ref(false)
const formRef = ref<any>()

// Tri-state hint icon next to the test button: null=neutral, true=just
// succeeded, false=just failed. Cleared whenever the user changes the
// underlying connection inputs (see watch() below, set up after providerForm
// is initialized so the watch's source function doesn't trip on TDZ).
const lastTestOk = ref<boolean | null>(null)

const providerForm = ref<{
  name: string
  provider: string
  description: string
  parameters: {
    api_key?: string
    engine_id?: string
    base_url?: string
    proxy_url?: string
    extra_config: Record<string, string>
  }
  is_default: boolean
}>({
  name: '',
  provider: 'duckduckgo',
  description: '',
  parameters: { extra_config: {} },
  is_default: false,
})

// Invalidate the cached test result whenever the user edits a connection
// field. Set up after providerForm is declared so the watch's source
// function — which dereferences providerForm.value on first run — doesn't
// hit a TDZ ReferenceError. proxy_url is excluded because the upstream
// call doesn't actually use it for credential validation.
watch(
  () => [
    providerForm.value.provider,
    providerForm.value.parameters?.api_key,
    providerForm.value.parameters?.engine_id,
    providerForm.value.parameters?.base_url,
    JSON.stringify(providerForm.value.parameters?.extra_config || {}),
  ],
  () => { lastTestOk.value = null },
)

// ===== Computed =====
const selectedProviderType = computed(() => {
  return providerTypes.value.find(pt => pt.id === providerForm.value.provider)
})

// Create-mode placeholder (edit mode replaces the input with
// <CredentialResource>, which has its own placeholder).
const apiKeyPlaceholder = computed(() => t('webSearchSettings.apiKeyPlaceholder'))

const credentialFields = computed<CredentialFieldDef<WebSearchCredentialField>[]>(() => [
  { key: 'api_key', label: t('webSearchSettings.apiKeyLabel') as string },
])

const credentialApi = computed<CredentialResourceApi<WebSearchCredentialField>>(() => {
  const id = editingProvider.value?.id ?? ''
  return {
    save: async (patch) => {
      const meta = await putWebSearchProviderCredentials(id, patch)
      return meta.fields
    },
    remove: async (field) => {
      await deleteWebSearchProviderCredentialField(id, field)
    },
  }
})

// Initial configured? from the main provider response (embedded server-side
// in dto.WebSearchProviderResponse.Credentials).
const credentialMeta = computed(() => editingProvider.value?.credentials ?? {
  api_key: { configured: false },
})

// Per-provider class on the drawer — the non-scoped CSS block at the
// bottom uses .websearch-drawer--{id} to color the header-icon container
// to match the matching list-card badge.
const drawerClass = computed(() => {
  const id = providerForm.value.provider
  return id
    ? `websearch-drawer websearch-drawer--${id}`
    : 'websearch-drawer'
})

// Reuses providerLogo() so the drawer header icon matches whatever the
// list card showed for the same provider id.
const drawerLogo = computed(() => {
  const id = providerForm.value.provider
  return id ? providerLogo('websearch', id) : null
})

const drawerLogoStyle = computed((): Record<string, string> => {
  const logo = drawerLogo.value
  if (!logo || logo.mode !== 'mono') return {}
  return { '--logo-url': `url("${logo.url}")` }
})

// Whether "Test connection" can fire. New-mode requires the user to have
// typed an api_key (and engine_id / base_url where applicable); edit-mode
// can fire with no fresh api_key because the backend will fall back to
// the stored credential. Free providers don't show the button at all.
const canTestConnection = computed(() => {
  const pt = selectedProviderType.value
  if (!pt) return false
  if (editingProvider.value) return true
  if (pt.requires_api_key && !providerForm.value.parameters.api_key) return false
  if (pt.requires_engine_id && !providerForm.value.parameters.engine_id) return false
  if (pt.requires_base_url && !providerForm.value.parameters.base_url) return false
  if (pt.config_fields?.some(field => field.required && !providerForm.value.parameters.extra_config?.[field.key])) return false
  return true
})

// Card initial-letter badge. Reuses the providerType info table so multi-byte abbreviations go through one place too.
const providerInitial = (providerId: string) => {
  const label = providerTypes.value.find(p => p.id === providerId)?.name || providerId
  return (label.trim().charAt(0) || '?').toUpperCase()
}

// See the comment of the same name in VectorStoreSettings: returns --logo-url for ::before to render with a mask.
const resolveLogo = (providerId: string) => providerLogo('websearch', providerId)

const badgeClass = (providerId: string) => {
  const m = resolveLogo(providerId)?.mode
  return {
    'provider-card__badge--logo': !!m,
    'provider-card__badge--color': m === 'color',
    'provider-card__badge--mono': m === 'mono',
  }
}

const badgeStyle = (providerId: string): Record<string, string> => {
  const logo = resolveLogo(providerId)
  return logo?.mode === 'mono' ? { '--logo-url': `url("${logo.url}")` } : {}
}

const providerTypeLabel = (providerId: string) => {
  return providerTypes.value.find(p => p.id === providerId)?.name || providerId
}

const configFieldText = (key: string | undefined, fallback: string) => {
  return key ? t(key, fallback) : fallback
}

const providerConfigDefaults = (providerId: string) => {
  const fields = providerTypes.value.find(p => p.id === providerId)?.config_fields || []
  return Object.fromEntries(
    fields
      .filter(field => field.default !== undefined)
      .map(field => [field.key, field.default as string]),
  )
}

// ===== Methods =====
const onProviderTypeChange = () => {
  providerForm.value.parameters = {
    extra_config: providerConfigDefaults(providerForm.value.provider),
  }
  lastTestOk.value = null
}

// Use chatResources' shared snapshot with a forced refresh: after a create, edit or delete the chat input bar reads the latest list,
// so it does not need to watch the settings modal closing to refetch.
const loadProviderEntities = async () => {
  try {
    await chatResources.ensureWebSearchProviders(true)
    providerEntities.value = chatResources.webSearchProviders
  } catch (error) {
    console.error('Failed to load provider entities:', error)
  }
}

const loadProviderTypes = async () => {
  try {
    providerTypes.value = await listWebSearchProviderTypes()
  } catch (error) {
    console.error('Failed to load provider types:', error)
  }
}

const openAddDialog = () => {
  editingProvider.value = null
  providerForm.value = {
    name: '',
    provider: providerTypes.value[0]?.id || 'duckduckgo',
    description: '',
    parameters: {
      extra_config: providerConfigDefaults(providerTypes.value[0]?.id || 'duckduckgo'),
    },
    is_default: providerEntities.value.length === 0
  }
  lastTestOk.value = null
  showAddProviderDialog.value = true
}

const editProvider = (entity: WebSearchProviderEntity) => {
  editingProvider.value = entity
  providerForm.value = {
    name: entity.name,
    provider: entity.provider,
    description: entity.description || '',
    parameters: {
      // Never pre-fill the api_key — even the redacted placeholder from the
      // server is ignored so that "non-empty means user typed it" holds.
      api_key: '',
      engine_id: entity.parameters?.engine_id || '',
      base_url: entity.parameters?.base_url || '',
      proxy_url: entity.parameters?.proxy_url || '',
      extra_config: {
        ...providerConfigDefaults(entity.provider),
        ...(entity.parameters?.extra_config || {}),
      },
    },
    is_default: entity.is_default || false,
  }
  lastTestOk.value = null
  showAddProviderDialog.value = true
}

const saveProvider = async () => {
  const validateResult = await formRef.value?.validate()
  if (validateResult !== true && validateResult !== undefined) {
    const firstError = typeof validateResult === 'object' ? Object.values(validateResult)[0] : ''
    MessagePlugin.warning(typeof firstError === 'string' ? firstError : 'Please check the form fields')
    return
  }

  saving.value = true
  try {
    // Build the parameters payload. api_key only flows in on initial
    // create — edit mode commits credentials through <CredentialResource>
    // (a dedicated PUT /credentials call) before this save runs.
    const paramsOut: WebSearchProviderEntity['parameters'] = {
      engine_id: providerForm.value.parameters.engine_id,
      base_url: providerForm.value.parameters.base_url,
      proxy_url: providerForm.value.parameters.proxy_url,
    }
    const extraConfig = Object.fromEntries(
      Object.entries(providerForm.value.parameters.extra_config || {})
        .filter(([, value]) => value !== ''),
    )
    if (Object.keys(extraConfig).length > 0) {
      paramsOut.extra_config = extraConfig
    }
    if (!editingProvider.value && providerForm.value.parameters.api_key) {
      paramsOut.api_key = providerForm.value.parameters.api_key
    }

    const data: Partial<WebSearchProviderEntity> = {
      name: providerForm.value.name.trim() || selectedProviderType.value?.name || providerForm.value.provider,
      provider: providerForm.value.provider as any,
      description: providerForm.value.description,
      parameters: paramsOut,
      is_default: providerForm.value.is_default,
    }

    if (editingProvider.value) {
      await updateWebSearchProvider(editingProvider.value.id!, data)
      MessagePlugin.success(t('webSearchSettings.toasts.providerUpdated'))
    } else {
      await createWebSearchProvider(data)
      MessagePlugin.success(t('webSearchSettings.toasts.providerCreated'))
    }
    showAddProviderDialog.value = false
    await loadProviderEntities()
  } catch (error: any) {
    MessagePlugin.error(error?.message || 'Failed to save provider')
  } finally {
    saving.value = false
  }
}

const deleteProvider = (entity: WebSearchProviderEntity) => {
  confirmDelete({
    body: t('webSearchSettings.deleteConfirm'),
    onConfirm: async () => {
      try {
        await deleteWebSearchProviderAPI(entity.id!)
        MessagePlugin.success(t('webSearchSettings.toasts.providerDeleted'))
        await loadProviderEntities()
      } catch (error: any) {
        MessagePlugin.error(error?.message || 'Failed to delete provider')
      }
    }
  })
}

const testConnection = async () => {
  testing.value = true
  try {
    const data = {
      provider: providerForm.value.provider,
      parameters: { ...providerForm.value.parameters },
    }

    let ok = false
    if (editingProvider.value && !data.parameters.api_key) {
      const res = await testWebSearchProvider(editingProvider.value.id!)
      ok = !!res.success
      if (res.success) {
        MessagePlugin.success(t('webSearchSettings.toasts.testSuccess'))
      } else {
        MessagePlugin.error(res.error || t('webSearchSettings.toasts.testFailed'))
      }
    } else {
      const res = await testWebSearchProvider(undefined, data)
      ok = !!res.success
      if (res.success) {
        MessagePlugin.success(t('webSearchSettings.toasts.testSuccess'))
      } else {
        MessagePlugin.error(res.error || t('webSearchSettings.toasts.testFailed'))
      }
    }
    lastTestOk.value = ok
  } catch (error: any) {
    MessagePlugin.error(error?.message || t('webSearchSettings.toasts.testFailed'))
    lastTestOk.value = false
  } finally {
    testing.value = false
  }
}

const isProviderCardClickable = () => authStore.hasRole('admin')

const onProviderCardClick = (event: Event, entity: WebSearchProviderEntity) => {
  if (!isProviderCardClickable()) return
  if (event.type === 'keydown') {
    const ke = event as KeyboardEvent
    if (ke.key !== 'Enter' && ke.key !== ' ') return
    ke.preventDefault()
  }
  const target = event.target as HTMLElement | null
  if (target?.closest('.provider-card__actions')) return
  editProvider(entity)
}

const getProviderOptions = (_entity: WebSearchProviderEntity) => {
  // Web search providers carry external API credentials; the backend
  // gates every mutation/test behind Admin+ (RegisterWebSearchProviderRoutes).
  // Hide the action menu entirely for non-Admins so they don't trip 403s.
  // Test connection moved to the edit drawer's footer and is no longer in the outer menu; a single entry avoids
  // user confusion ("why are there two test entries, do they give the same result?").
  if (!authStore.hasRole('admin')) {
    return []
  }
  return [
    { content: t('common.edit'), value: 'edit' },
    { content: t('common.delete'), value: 'delete', theme: 'error' as const }
  ]
}

const handleMenuAction = (data: { value: string }, entity: WebSearchProviderEntity) => {
  switch (data.value) {
    case 'edit':
      editProvider(entity)
      break
    case 'delete':
      deleteProvider(entity)
      break
  }
}

// ===== Init =====
onMounted(async () => {
  await Promise.all([loadProviderTypes(), loadProviderEntities()])
})
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/provider-card.less';

@import (reference) '@/components/css/settings-section.less';

.websearch-settings {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.list-section-title {
  font-size: var(--app-text-xl);
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 16px 0;
}

.provider-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;

  .provider-card--add {
    width: 100%;
    height: 100%;
  }
}

// The card visuals mirror ModelSettings' model-card (badge + title / subtitle / url in three parts).
// For now each copy keeps its own styles to avoid premature abstraction; if Mcp or a fourth consumer shows up,
// extract the shared pieces into a base under components/settings/.
.provider-card {
  .provider-card();

  &--clickable {
    .provider-card-interactive();


  }

  &--add {
    .provider-card-add();

    &:hover,
    &:focus-visible {
      box-shadow: none;
    }



    &__icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      border-radius: var(--app-radius-md);
      background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
      color: var(--td-brand-color);
      font-size: var(--app-text-2xl);
    }

    &__label {
      font-size: var(--app-text-md);
      font-weight: 500;
      line-height: 1.4;
    }
  }
}

.provider-card__actions {
  flex-shrink: 0;
}

.provider-card__badge {
  .provider-card-badge();
  .provider-card-badge-color(#0052d9);
}

// Real brand logo: white background + thin border, and the logo is tinted to currentColor with mask-image (keeping the brand color).
// The extra .provider-card level wins over the specific `.provider-card--<id> .provider-card__badge` rule.
.provider-card__badge-img {
  .provider-card-badge-img();
}

// Badge colors per search provider: no need to match the official logo, a low-saturation shade of the same hue is enough.
.provider-card--duckduckgo .provider-card__badge {
  .provider-card-badge-color(#de5833);
}
.provider-card--bing .provider-card__badge {
  .provider-card-badge-color(#0089ff);
}
.provider-card--google .provider-card__badge {
  .provider-card-badge-color(#4285f4);
}
.provider-card--tavily .provider-card__badge {
  .provider-card-badge-color(#6235bb);
}
.provider-card--baidu .provider-card__badge {
  .provider-card-badge-color(#2932e1);
}
.provider-card--searxng .provider-card__badge {
  .provider-card-badge-color(#215689);
}
.provider-card--ollama .provider-card__badge {
  .provider-card-badge-color(#464646);
}
.provider-card--keenable .provider-card__badge {
  .provider-card-badge-color(#149e82);
}
.provider-card--zhipu .provider-card__badge {
  .provider-card-badge-color(#2563eb);
}

.provider-card__body {
  .provider-card-body();
}

.provider-card__header {
  .provider-card-header();
}

.provider-card__title {
  .provider-card-title();
}

.provider-card__more {
  .provider-card-more();
}

.provider-card:hover .provider-card__more,
.provider-card:focus-within .provider-card__more,
.provider-card__actions:focus-within .provider-card__more {
  opacity: 1;
}

.provider-card__subtitle {
  .provider-card-subtitle();
}

.provider-card__type {
  font-weight: 500;
}

.provider-card__sep {
  color: var(--td-text-color-placeholder);
}

.provider-card__desc {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.provider-card__url {
  font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  font-size: var(--app-text-xs);
  line-height: 1.4;
  color: var(--td-text-color-placeholder);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}

.empty-state {
  padding: 64px 0;
  text-align: center;

  :deep(.t-empty__description) {
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
    margin-bottom: 16px;
  }
}

.provider-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

// ---- Drawer content, same conventions as ModelEditorDialog ----
.form-item {
  margin-bottom: 0;
}

.form-label {
  display: block;
  margin-bottom: 6px;
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-text-color-primary);
  line-height: 1.4;

  &.required::before {
    content: '*';
    color: var(--td-error-color);
    margin-right: 4px;
    font-weight: 500;
    line-height: 1;
  }
}

.form-desc {
  margin: 4px 0 0 0;
  font-size: var(--app-text-sm);
  line-height: 1.5;
  color: var(--td-text-color-placeholder);

  &--inline {
    margin: 0;
  }
}

:deep(.t-input),
:deep(.t-select),
:deep(.t-textarea),
:deep(.t-input-number) {
  width: 100%;
  font-size: var(--app-text-md);
}

// Hide t-form's default form-item container; we use custom .form-item / .form-label.
:deep(.t-form) .t-form-item {
  display: none;
}

.vision-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
}

// ---- Status icon for the footer-left test button (same as ModelEditorDialog/MCP) ----
.status-icon {
  font-size: var(--app-text-xl);
  flex-shrink: 0;

  &.available {
    color: var(--td-brand-color);
  }

  &.unavailable {
    color: var(--td-error-color);
  }
}

// ---- Header icon badge ----
.header-icon__img {
  width: 24px;
  height: 24px;
  object-fit: contain;
  display: block;
}

.header-icon__mono {
  display: inline-block;
  width: 22px;
  height: 22px;
  background-color: currentColor;
  -webkit-mask-image: var(--logo-url);
  -webkit-mask-position: center;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-size: contain;
  mask-image: var(--logo-url);
  mask-position: center;
  mask-repeat: no-repeat;
  mask-size: contain;
}

.header-icon__text {
  font-size: var(--app-text-lg);
  font-weight: 600;
  letter-spacing: 0.02em;
}

.doc-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: var(--app-text-md);
  font-weight: 500;
  color: var(--td-brand-color);
  text-decoration: none;
  transition: color var(--app-motion-fast) ease;

  &:hover {
    color: var(--td-brand-color-active);
  }

  .link-icon {
    font-size: var(--app-text-base);
  }

  &--inline {
    margin-left: 6px;
    font-size: var(--app-text-sm);
    font-weight: 500;
    vertical-align: baseline;

    .link-icon {
      font-size: var(--app-text-sm);
    }
  }
}
</style>

<!--
  Non-scoped block: per-provider header-icon coloring + color-logo
  background tweak. Same pattern as Storage/Parser drawers — these rules
  must be global so they always reach the t-drawer panel even if its
  scoped data-attribute is dropped in some builds. Each rule mirrors the
  matching .provider-card--{id} .provider-card__badge from the scoped
  block above so list-card → drawer hand-off stays visually continuous.
-->
<style lang="less">
// For a color logo give the header-icon container a white background + 1px border, so a light brand-colored background
// does not sit under the color icon and hurt contrast.
.websearch-drawer .setting-drawer__header-icon:has(.header-icon__img) {
  background: var(--td-bg-color-container);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.websearch-drawer--duckduckgo .setting-drawer__header-icon {
  background: rgba(222, 88, 51, 0.12);
  color: #DE5833;
}
.websearch-drawer--bing .setting-drawer__header-icon {
  background: rgba(0, 137, 255, 0.12);
  color: #0089FF;
}
.websearch-drawer--google .setting-drawer__header-icon {
  background: rgba(66, 133, 244, 0.12);
  color: #4285F4;
}
.websearch-drawer--tavily .setting-drawer__header-icon {
  background: rgba(98, 53, 187, 0.12);
  color: #6235BB;
}
.websearch-drawer--baidu .setting-drawer__header-icon {
  background: rgba(41, 50, 225, 0.12);
  color: #2932E1;
}
.websearch-drawer--searxng .setting-drawer__header-icon {
  background: rgba(33, 86, 137, 0.12);
  color: #215689;
}
.websearch-drawer--ollama .setting-drawer__header-icon {
  background: rgba(70, 70, 70, 0.12);
  color: #464646;
}
.websearch-drawer--keenable .setting-drawer__header-icon {
  background: rgba(20, 158, 130, 0.12);
  color: #149E82;
}
.websearch-drawer--zhipu .setting-drawer__header-icon {
  background: rgba(37, 99, 235, 0.12);
  color: #2563EB;
}
</style>
