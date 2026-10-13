<template>
  <div class="vectorstore-settings">
    <div class="section-header">
      <h2>{{ t('vectorStoreSettings.title') }}</h2>
      <p class="section-description">{{ t('vectorStoreSettings.description') }}</p>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-container">
      <t-loading size="small" />
    </div>

    <template v-else>
      <div class="settings-group">
        <h3 class="list-section-title">{{ t('vectorStoreSettings.storesTitle') }}</h3>

        <!-- Same shape as the other settings lists: engine badge on the left + title + env pill + subtitle + test action.
             The env source is read-only (engine_type / connection_config are written by .env), so there is no more menu;
             the user source keeps the three-dot menu's edit / delete entries; the test result appears as a colored bar at the bottom of the card. -->
        <div v-if="stores.length === 0 && !authStore.hasRole('admin')" class="empty-stores">
          <t-empty :description="t('vectorStoreSettings.emptyDesc')" />
        </div>
        <div v-else class="store-grid">
          <div
            v-for="store in [...envStores, ...userStores]"
            :key="store.id"
            class="store-card"
            :class="[
              `store-card--${store.engine_type}`,
              {
                'store-card--env': store.source === 'env',
                'store-card--clickable': isStoreCardClickable(store),
              },
            ]"
            :role="isStoreCardClickable(store) ? 'button' : undefined"
            :tabindex="isStoreCardClickable(store) ? 0 : undefined"
            @click="onStoreCardClick($event, store)"
            @keydown.enter="onStoreCardClick($event, store)"
          >
            <div class="store-card__main">
              <div
                class="store-card__badge"
                :class="badgeClass(store.engine_type)"
                :style="badgeStyle(store.engine_type)"
                :aria-label="store.engine_type"
              >
                <img
                  v-if="resolveLogo(store.engine_type)?.mode === 'color'"
                  :src="resolveLogo(store.engine_type)!.url"
                  :alt="store.engine_type"
                  class="store-card__badge-img"
                />
                <template v-else-if="!resolveLogo(store.engine_type)">{{ engineInitial(store.engine_type) }}</template>
              </div>
              <div class="store-card__body">
                <div class="store-card__header">
                  <h3 class="store-card__title" :title="store.name">{{ store.name }}</h3>
                  <span v-if="store.source === 'env'" class="store-card__pill">
                    {{ t('vectorStoreSettings.envTag') }}
                  </span>
                  <!--
                    Test connection moved to the edit drawer's footer; the outer menu no longer has a "Test" entry.
                    The env source (written by .env) needs no dropdown either: there is no action to run.
                  -->
                  <div
                    v-if="authStore.hasRole('admin') && storeActionsFor(store).length > 0"
                    class="store-card__actions"
                    @click.stop
                  >
                    <t-dropdown
                      :options="storeActionsFor(store)"
                      placement="bottom-right"
                      attach="body"
                      trigger="click"
                      @click="(action: any) => handleAction(action, store)"
                    >
                      <t-button variant="text" shape="square" size="small" class="store-card__more">
                        <t-icon name="ellipsis" />
                      </t-button>
                    </t-dropdown>
                  </div>
                </div>
                <div class="store-card__subtitle">
                  <span class="store-card__type">{{ store.engine_type }}</span>
                  <template v-if="getStoreEndpoint(store)">
                    <span class="store-card__sep">·</span>
                    <span class="store-card__endpoint" :title="getStoreEndpoint(store)">{{ getStoreEndpoint(store) }}</span>
                  </template>
                </div>
              </div>
            </div>
          </div>
          <button
            v-if="authStore.hasRole('admin')"
            type="button"
            class="store-card store-card--add"
            @click="openAddDialog"
          >
            <span class="store-card--add__icon" aria-hidden="true">
              <add-icon />
            </span>
            <span class="store-card--add__label">{{ t('vectorStoreSettings.addStore') }}</span>
          </button>
        </div>
      </div>
    </template>

    <!-- Add/Edit Drawer, same as ModelEditorDialog/Storage/Parser/WebSearch -->
    <SettingDrawer
      v-model:visible="showDialog"
      :title="editingStore ? t('vectorStoreSettings.editStore') : t('vectorStoreSettings.addStore')"
      :class="drawerClass"
      :confirm-loading="saving"
      @confirm="onDrawerConfirm"
      @cancel="showDialog = false"
    >
      <!--
        Header icon: the same logo/mono/fallback as the list's .store-card__badge.
        Per-engine colors are injected by .vectorstore-drawer--{engine} in the non-scoped block.
      -->
      <template v-if="form.engine_type" #headerIcon>
        <img
          v-if="drawerLogo?.mode === 'color'"
          :src="drawerLogo.url"
          :alt="form.engine_type"
          class="header-icon__img"
        />
        <span
          v-else-if="drawerLogo?.mode === 'mono'"
          class="header-icon__mono"
          :style="drawerLogoStyle"
        />
        <span v-else class="header-icon__text">{{ engineInitial(form.engine_type) }}</span>
      </template>

      <!-- Subtitle: engine display_name -->
      <template v-if="selectedType" #subtitle>
        <span>{{ selectedType.display_name || form.engine_type }}</span>
      </template>

      <!--
        Test connection (footer-left). Create mode: validates the current form's connection info live;
        edit mode: uses the stored connection config (the connection config cannot change in edit mode; the engine is immutable).
        The button is always shown, with disabled controlled by canTestConnection.
      -->
      <template #footer-left>
        <t-button
          variant="outline"
          :loading="testing"
          :disabled="!canTestConnection"
          @click="onDrawerTest"
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
          {{ testing ? t('vectorStoreSettings.testing') : t('vectorStoreSettings.testConnection') }}
        </t-button>
      </template>

      <t-form ref="formRef" :data="form" :rules="formRules" label-align="top" class="store-form">
        <!--
          Edit mode notice: engine_type / connection_config / index_config
          cannot change after creation; only name is editable. Uses an inline-alert rather than a large banner,
          visually matching the notices in the other drawers.
        -->
        <section v-if="editingStore" class="setting-drawer__section">
          <h4 class="setting-drawer__section-title">{{ t('vectorStoreSettings.basicSection', 'Basic') }}</h4>

          <div class="inline-alert inline-alert--info">
            <t-icon name="info-circle-filled" class="inline-alert__icon" />
            <span class="inline-alert__text">{{ t('vectorStoreSettings.immutableNotice') }}</span>
          </div>

          <div class="form-item">
            <label class="form-label required">{{ t('vectorStoreSettings.nameLabel') }}</label>
            <t-input v-model="form.name" :placeholder="t('vectorStoreSettings.namePlaceholder')" />
          </div>

          <!-- Read-only fields shown as an inline list (lightweight readonly rows) -->
          <div class="readonly-fields">
            <div class="readonly-row">
              <span class="readonly-label">{{ t('vectorStoreSettings.engineTypeLabel') }}</span>
              <span class="readonly-value">{{ selectedType?.display_name || editingStore.engine_type }}</span>
            </div>
            <template v-if="selectedType">
              <template v-for="field in selectedType.connection_fields" :key="field.name">
                <div v-if="field.sensitive || form.connection_config[field.name]" class="readonly-row">
                  <span class="readonly-label">{{ fieldLabel(field.name) }}</span>
                  <span class="readonly-value">
                    {{ field.sensitive ? '********' : form.connection_config[field.name] }}
                  </span>
                </div>
              </template>
            </template>
            <template v-if="selectedType?.index_fields?.length">
              <template v-for="field in selectedType.index_fields" :key="field.name">
                <div v-if="form.index_config[field.name]" class="readonly-row">
                  <span class="readonly-label">{{ fieldLabel(field.name) }}</span>
                  <span class="readonly-value">{{ form.index_config[field.name] }}</span>
                </div>
              </template>
            </template>
          </div>
        </section>

        <!-- Create mode: basic info + connection config + advanced index, three sections -->
        <template v-else>
          <!-- Section 1: basic info: engine type + name -->
          <section class="setting-drawer__section">
            <h4 class="setting-drawer__section-title">{{ t('vectorStoreSettings.basicSection', 'Basic') }}</h4>

            <div class="form-item">
              <label class="form-label required">{{ t('vectorStoreSettings.engineTypeLabel') }}</label>
              <t-select v-model="form.engine_type" @change="onEngineTypeChange">
                <t-option
                  v-for="st in storeTypes"
                  :key="st.type"
                  :value="st.type"
                  :label="st.display_name"
                />
              </t-select>
            </div>

            <div class="form-item">
              <label class="form-label required">{{ t('vectorStoreSettings.nameLabel') }}</label>
              <t-input v-model="form.name" :placeholder="t('vectorStoreSettings.namePlaceholder')" />
            </div>
          </section>

          <!-- Section 2: connection config (the engine type decides the fields) -->
          <section v-if="selectedType" class="setting-drawer__section">
            <h4 class="setting-drawer__section-title">{{ t('vectorStoreSettings.connectionInfo') }}</h4>

            <div
              v-for="field in selectedType.connection_fields"
              :key="field.name"
              class="form-item"
            >
              <label
                class="form-label"
                :class="{ required: field.required }"
              >{{ fieldLabel(field.name) }}</label>

              <!-- boolean field: switch + inline description / TLS warning -->
              <template v-if="field.type === 'boolean'">
                <div class="vision-toggle">
                  <t-switch v-model="form.connection_config[field.name]" />
                </div>
                <p
                  v-if="field.name === 'insecure_skip_verify' && form.connection_config[field.name]"
                  class="form-desc form-desc--warn"
                >
                  {{ t('vectorStoreSettings.insecureSkipVerifyWarning') }}
                </p>
              </template>

              <!-- Sensitive fields (password / api key and so on): lock prefix + password -->
              <t-input
                v-else-if="field.type === 'string' && field.sensitive"
                v-model="form.connection_config[field.name]"
                type="password"
                placeholder="********"
              >
                <template #prefix-icon><t-icon name="lock-on" /></template>
              </t-input>

              <!-- Number field: t-input + type=number, same as the MCP advanced config; no unit hint -->
              <t-input
                v-else-if="field.type === 'number'"
                v-model="connectionNumberTextProxy[field.name].value"
                type="number"
                :placeholder="field.default != null ? String(field.default) : ' '"
                class="number-input"
              />

              <!-- Plain string -->
              <t-input
                v-else
                v-model="form.connection_config[field.name]"
                :placeholder="field.default?.toString() || ''"
              />
            </div>
          </section>

          <!-- Section 3: advanced index (shown only when selectedType has index_fields) -->
          <section v-if="selectedType?.index_fields?.length" class="setting-drawer__section">
            <h4 class="setting-drawer__section-title">{{ t('vectorStoreSettings.advancedIndexConfig') }}</h4>

            <!-- Collapse/expand toggle: keeps the earlier optional display behavior, with lighter styling -->
            <button
              type="button"
              class="advanced-toggle"
              @click="showAdvanced = !showAdvanced"
            >
              <t-icon :name="showAdvanced ? 'chevron-down' : 'chevron-right'" />
              <span>{{ showAdvanced ? t('common.collapse', 'Collapse') : t('common.expand', 'Expand') }}</span>
            </button>

            <template v-if="showAdvanced">
              <div
                v-for="field in selectedType.index_fields"
                :key="field.name"
                class="form-item"
              >
                <label class="form-label">{{ fieldLabel(field.name) }}</label>

                <!-- Enum → dropdown -->
                <t-select
                  v-if="field.enum && field.enum.length"
                  v-model="form.index_config[field.name]"
                  :placeholder="field.default?.toString() || ''"
                >
                  <t-option v-for="opt in field.enum" :key="opt" :value="opt" :label="opt" />
                </t-select>

                <!-- Number → number input -->
                <t-input
                  v-else-if="field.type === 'number'"
                  v-model="indexNumberTextProxy[field.name].value"
                  type="number"
                  :placeholder="field.default?.toString()"
                  :min="field.min ?? 1"
                  :max="field.max ?? (isReplicaField(field.name) ? 10 : 64)"
                  class="number-input"
                />

                <!-- String -->
                <t-input
                  v-else
                  v-model="form.index_config[field.name]"
                  :placeholder="field.default?.toString() || ''"
                  :maxlength="128"
                />
              </div>
            </template>
          </section>
        </template>
      </t-form>
    </SettingDrawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, type WritableComputedRef } from 'vue'
import { MessagePlugin, DialogPlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { AddIcon } from 'tdesign-icons-vue-next'
import {
  listVectorStores,
  listVectorStoreTypes,
  createVectorStore,
  updateVectorStore,
  deleteVectorStore as deleteVectorStoreAPI,
  testVectorStoreRaw,
  type VectorStoreEntity,
  type VectorStoreTypeInfo,
} from '@/api/vector-store'
import { useAuthStore } from '@/stores/auth'
import { providerLogo } from './providerLogos'
import SettingDrawer from '@/components/settings/SettingDrawer.vue'

const { t } = useI18n()
const authStore = useAuthStore()

// ===== State =====
const stores = ref<VectorStoreEntity[]>([])
const storeTypes = ref<VectorStoreTypeInfo[]>([])
const loading = ref(false)
const showDialog = ref(false)
const editingStore = ref<VectorStoreEntity | null>(null)
const testing = ref(false)
const saving = ref(false)
const showAdvanced = ref(false)
const formRef = ref<any>()

const form = ref<{
  name: string
  engine_type: string
  connection_config: Record<string, any>
  index_config: Record<string, any>
}>({
  name: '',
  engine_type: '',
  connection_config: {},
  index_config: {},
})

// Tri-state hint icon next to the test button: null=neutral, true=just
// succeeded, false=just failed. Cleared when the user changes any
// connection-relevant field so a stale ✓/✗ doesn't follow a config the
// user is still editing.
const lastTestOk = ref<boolean | null>(null)

watch(
  () => [form.value.engine_type, form.value.connection_config],
  () => { lastTestOk.value = null },
  { deep: true },
)

// ===== Computed =====
const envStores = computed(() => stores.value.filter(s => s.source === 'env'))
const userStores = computed(() => stores.value.filter(s => s.source === 'user'))
const selectedType = computed(() => storeTypes.value.find(st => st.type === form.value.engine_type))

// Drawer header logo, from the same source as the list's .store-card__badge (providerLogo()), so the
// list card → drawer hand-off looks continuous.
const drawerLogo = computed(() => {
  if (!form.value.engine_type) return null
  return providerLogo('vectorstore', form.value.engine_type)
})

const drawerLogoStyle = computed((): Record<string, string> => {
  const logo = drawerLogo.value
  if (!logo || logo.mode !== 'mono') return {}
  return { '--logo-url': `url("${logo.url}")` }
})

// per-engine class on drawer for non-scoped header-icon coloring rules.
const drawerClass = computed(() => {
  return form.value.engine_type
    ? `vectorstore-drawer vectorstore-drawer--${form.value.engine_type}`
    : 'vectorstore-drawer'
})

// Whether test connection is clickable. Create mode: every required connection field must be filled;
// edit mode: the engine cannot change and the connection config is read-only, so testing is disabled (create a new entry instead of testing in the drawer).
const canTestConnection = computed(() => {
  if (editingStore.value) return false
  const st = selectedType.value
  if (!st) return false
  for (const f of st.connection_fields) {
    if (!f.required) continue
    const v = form.value.connection_config[f.name]
    if (v == null || v === '' || (typeof v === 'string' && v.trim() === '')) return false
  }
  return true
})

// Per-store dropdown options. The env source is written by .env, and the UI allows no edit / delete;
// test connection moved to the edit drawer's footer, and the outer menu no longer shows a "Test" item. The env source has no
// edit/delete entries → no dropdown needs to be shown at all.
const storeActionsFor = (store: VectorStoreEntity) => {
  if (store.source === 'env') return []
  return [
    { content: t('common.edit'), value: 'edit' },
    { content: t('common.delete'), value: 'delete', theme: 'error' as const },
  ]
}

const formRules = computed(() => {
  const rules: Record<string, any[]> = {
    name: [{ required: true, message: t('vectorStoreSettings.validation.nameRequired') }],
  }
  if (!editingStore.value) {
    rules.engine_type = [{ required: true, message: t('vectorStoreSettings.validation.engineTypeRequired') }]
    if (selectedType.value) {
      for (const field of selectedType.value.connection_fields) {
        if (field.required) {
          rules[`connection_config.${field.name}`] = [
            { required: true, message: t('vectorStoreSettings.validation.fieldRequired', { field: fieldLabel(field.name) }) },
          ]
        }
      }
      // Index name/collection string fields: pattern validation (optional - empty is allowed)
      for (const field of (selectedType.value.index_fields || [])) {
        if (field.type === 'string') {
          rules[`index_config.${field.name}`] = [
            {
              validator: (val: string) => !val || indexNamePattern.test(val),
              message: t('vectorStoreSettings.validation.indexNamePattern'),
              trigger: 'blur',
            },
          ]
        }
      }
    }
  }
  return rules
})

// Index/collection name pattern: must start with letter, alphanumeric + _ + - only, max 128
const indexNamePattern = /^[a-zA-Z][a-zA-Z0-9_-]{0,127}$/

// ===== Methods =====
const fieldLabel = (name: string): string => {
  const key = `vectorStoreSettings.fields.${name}`
  const translated = t(key)
  // If i18n key not found, vue-i18n returns the key itself - fall back to field name
  return translated === key ? name : translated
}

// Distinguish replica fields (max 10) from shard fields (max 64) for input bounds
const replicaFieldNames = ['number_of_replicas', 'replication_factor', 'replica_number']
const isReplicaField = (name: string): boolean => replicaFieldNames.includes(name)

const getStoreEndpoint = (store: VectorStoreEntity): string => {
  const cc = store.connection_config || {}
  return cc.addr || cc.host || ''
}

// Card badge initial. engine_type is always ASCII, so charAt directly.
const engineInitial = (engineType: string): string => {
  return (engineType || '?').charAt(0).toUpperCase()
}

// When the engine has a logo asset, pass the SVG URL through to CSS (::before renders it with mask-image)
// and switch the card background back to neutral white; with no logo return an empty object, keeping each engine's
// brand-color monogram style. Color mode needs no mask tint, so the url is not reported.
const resolveLogo = (engineType: string) => providerLogo('vectorstore', engineType)

const badgeClass = (engineType: string) => {
  const m = resolveLogo(engineType)?.mode
  return {
    'store-card__badge--logo': !!m,
    'store-card__badge--color': m === 'color',
    'store-card__badge--mono': m === 'mono',
  }
}

const badgeStyle = (engineType: string): Record<string, string> => {
  const logo = resolveLogo(engineType)
  return logo?.mode === 'mono' ? { '--logo-url': `url("${logo.url}")` } : {}
}

const onEngineTypeChange = () => {
  form.value.connection_config = {}
  form.value.index_config = {}
  showAdvanced.value = false
  // Drop cached number-text proxies so a switch to a different engine
  // doesn't keep stale entries pointing at the old field set.
  for (const k of Object.keys(connectionNumberText)) delete connectionNumberText[k]
  for (const k of Object.keys(indexNumberText)) delete indexNumberText[k]
}

// ---- Number-input text proxies (lazy per field name) ----
// A type=number input coerces an empty string to 0 / NaN through v-model, causing
// the annoying "user clears it → 0 comes back" interaction. We wrap it in a WritableComputedRef:
// on read the number is shown as a string; on write an empty string → delete the field (so the placeholder shows),
// non-empty → convert to int. The proxy is created per field name on demand and cached, avoiding repeated computeds.
const connectionNumberText: Record<string, WritableComputedRef<string>> = {}
const indexNumberText: Record<string, WritableComputedRef<string>> = {}

function ensureNumberProxy(
  bag: Record<string, WritableComputedRef<string>>,
  store: Record<string, any>,
  key: string,
): WritableComputedRef<string> {
  if (bag[key]) return bag[key]
  bag[key] = computed<string>({
    get: () => {
      const v = store[key]
      return v == null || v === '' ? '' : String(v)
    },
    set: (raw: string) => {
      const s = String(raw ?? '').trim()
      if (!s) {
        delete store[key]
        return
      }
      const n = Number(s)
      store[key] = Number.isFinite(n) ? n : s
    },
  })
  return bag[key]
}

// Vue templates can't call ensureNumberProxy on every render without the
// keys multiplying - wrap in a Proxy so `connectionNumberText[name].value`
// from the template lazily creates the proxy on first read.
const connectionNumberTextProxy = new Proxy(connectionNumberText, {
  get: (target, name: string) => ensureNumberProxy(target, form.value.connection_config, name),
})
const indexNumberTextProxy = new Proxy(indexNumberText, {
  get: (target, name: string) => ensureNumberProxy(target, form.value.index_config, name),
})

const loadStores = async () => {
  try {
    const response = await listVectorStores()
    if (response.data && Array.isArray(response.data)) {
      stores.value = response.data
    }
  } catch (error) {
    console.error('Failed to load vector stores:', error)
  }
}

const loadStoreTypes = async () => {
  try {
    storeTypes.value = await listVectorStoreTypes()
  } catch (error) {
    console.error('Failed to load vector store types:', error)
  }
}

const openAddDialog = () => {
  editingStore.value = null
  showAdvanced.value = false
  form.value = {
    name: '',
    engine_type: storeTypes.value[0]?.type || '',
    connection_config: {},
    index_config: {},
  }
  lastTestOk.value = null
  showDialog.value = true
}

// The env source is injected by .env, consistent with the list menu: not clickable for editing
const isStoreCardClickable = (store: VectorStoreEntity) =>
  authStore.hasRole('admin') && store.source !== 'env'

const onStoreCardClick = (event: Event, store: VectorStoreEntity) => {
  if (!isStoreCardClickable(store)) return
  if (event.type === 'keydown') {
    const ke = event as KeyboardEvent
    if (ke.key !== 'Enter' && ke.key !== ' ') return
    ke.preventDefault()
  }
  const target = event.target as HTMLElement | null
  if (target?.closest('.store-card__actions')) return
  editStore(store)
}

const editStore = (store: VectorStoreEntity) => {
  if (store.source === 'env') {
    return
  }
  editingStore.value = store
  showAdvanced.value = false
  form.value = {
    name: store.name,
    engine_type: store.engine_type,
    connection_config: { ...store.connection_config },
    index_config: { ...store.index_config },
  }
  lastTestOk.value = null
  showDialog.value = true
}

// Triggered by the SettingDrawer "Save" button: validate manually, then write to the backend.
// Edit mode can change only name; create mode submits the full connection / index config.
const onDrawerConfirm = async () => {
  const result = await formRef.value?.validate()
  if (result !== true && result !== undefined) {
    // Show the first error
    const firstError =
      typeof result === 'object'
        ? Object.values(result).map((errs: any) => Array.isArray(errs) ? errs[0]?.message : '').find(Boolean)
        : ''
    MessagePlugin.warning(firstError || (t('vectorStoreSettings.toasts.errorGeneric') as string))
    return
  }

  saving.value = true
  try {
    if (editingStore.value) {
      await updateVectorStore(editingStore.value.id!, { name: form.value.name.trim() })
      MessagePlugin.success(t('vectorStoreSettings.toasts.storeUpdated'))
    } else {
      const data: Partial<VectorStoreEntity> = {
        name: form.value.name.trim(),
        engine_type: form.value.engine_type,
        connection_config: { ...form.value.connection_config },
        index_config: showAdvanced.value ? { ...form.value.index_config } : {},
      }
      await createVectorStore(data)
      MessagePlugin.success(t('vectorStoreSettings.toasts.storeCreated'))
    }
    showDialog.value = false
    await loadStores()
  } catch (error: any) {
    const msg = error?.message || t('vectorStoreSettings.toasts.errorGeneric')
    if (msg.toLowerCase().includes('already exists') || msg.toLowerCase().includes('duplicate')) {
      MessagePlugin.error(t('vectorStoreSettings.toasts.duplicateName'))
    } else {
      MessagePlugin.error(msg)
    }
  } finally {
    saving.value = false
  }
}

const handleAction = (action: { value: string }, store: VectorStoreEntity) => {
  // test moved to the drawer; the outer menu no longer handles the 'test' value.
  if (action.value === 'edit') {
    editStore(store)
  } else if (action.value === 'delete') {
    confirmDelete(store)
  }
}

const confirmDelete = (store: VectorStoreEntity) => {
  const dialog = DialogPlugin.confirm({
    header: t('vectorStoreSettings.deleteConfirm'),
    confirmBtn: { content: t('common.delete'), theme: 'danger' },
    cancelBtn: t('common.cancel'),
    theme: 'danger',
    onConfirm: async () => {
      try {
        await deleteVectorStoreAPI(store.id!)
        MessagePlugin.success(t('vectorStoreSettings.toasts.storeDeleted'))
        await loadStores()
      } catch (error: any) {
        MessagePlugin.error(error?.message || t('vectorStoreSettings.toasts.errorGeneric'))
      }
      dialog.destroy()
    },
  })
}

// Test connection (triggered inside the drawer). In create mode it uses the current form data and calls
// the /test/raw endpoint. In edit mode the button is disabled, so only the create path is handled here.
const onDrawerTest = async () => {
  if (editingStore.value) return
  testing.value = true
  try {
    const data = {
      engine_type: form.value.engine_type,
      connection_config: { ...form.value.connection_config },
    }
    const res = await testVectorStoreRaw(data)
    lastTestOk.value = !!res.success
    if (res.success) {
      MessagePlugin.success(t('vectorStoreSettings.toasts.testSuccess'))
    } else {
      MessagePlugin.error(res.error || t('vectorStoreSettings.toasts.testFailed'))
    }
  } catch (error: any) {
    lastTestOk.value = false
    MessagePlugin.error(error?.message || t('vectorStoreSettings.toasts.testFailed'))
  } finally {
    testing.value = false
  }
}

// ===== Init =====
onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([loadStoreTypes(), loadStores()])
  } finally {
    loading.value = false
  }
})
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/provider-card.less';

@import (reference) '@/components/css/settings-section.less';

.vectorstore-settings {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.loading-container {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}

.settings-group {
  display: flex;
  flex-direction: column;
}

.list-section-title {
  font-size: var(--app-text-xl);
  font-weight: 600;
  color: var(--td-text-color-primary);
  margin: 0 0 16px 0;
}

.store-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;

  .store-card--add {
    width: 100%;
    height: 100%;
  }
}

// Same shape as Parser / Storage / Model and others: badge + three parts. The env source uses the secondaryContainer
// background to hint read-only; the test button is text mode, so it does not stand out in the title row.
.store-card {
  .provider-card();
  flex-direction: column;

  &--env {
    background: var(--td-bg-color-secondarycontainer);
  }

  &--clickable {
    .provider-card-interactive();


  }

  &--env:not(.store-card--clickable):hover {
    border-color: var(--td-component-stroke);
    box-shadow: none;
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

.store-card__actions {
  flex-shrink: 0;
}

.store-card__main {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}

.store-card__badge {
  .provider-card-badge();
  .provider-card-badge-color(#0052d9);
}

// Rendering a real brand logo: keep each engine class's color as the brand color,
// switch the background to neutral white + a thin border, and tint the single-color SVG to currentColor with ::before mask-image.
// The selector stacks an extra .store-card to win over the more specific
// `.store-card--<engine> .store-card__badge` brand background rule.
.store-card__badge-img {
  .provider-card-badge-img();
}

// Colors per vector engine (covering 11 common backends; unlisted ones fall back to the default blue)
.store-card--qdrant .store-card__badge {
  .provider-card-badge-color(#e12626);
}
.store-card--milvus .store-card__badge {
  .provider-card-badge-color(#0089ff);
}
.store-card--weaviate .store-card__badge {
  .provider-card-badge-color(#07a050);
}
.store-card--elasticsearch .store-card__badge,
.store-card--elasticfaiss .store-card__badge {
  .provider-card-badge-color(#d97706);
}
.store-card--postgres .store-card__badge {
  .provider-card-badge-color(#0052d9);
}
.store-card--opensearch .store-card__badge {
  .provider-card-badge-color(#6235bb);
}
.store-card--infinity .store-card__badge {
  .provider-card-badge-color(#6235bb);
}
.store-card--tencent_vectordb .store-card__badge {
  .provider-card-badge-color(#0052d9);
}
.store-card--doris .store-card__badge {
  .provider-card-badge-color(#e55a00);
}
.store-card--sqlite .store-card__badge {
  .provider-card-badge-color(#464646);
}

.store-card__body {
  .provider-card-body();
}

.store-card__header {
  .provider-card-header();
}

.store-card__title {
  .provider-card-title();
}

.store-card__pill {
  flex-shrink: 0;
  padding: 1px 6px;
  font-size: var(--app-text-xs);
  font-weight: 500;
  line-height: 16px;
  border-radius: 3px;
  color: var(--td-warning-color-7);
  background: var(--td-warning-color-1);
}

.store-card__more {
  .provider-card-more();
}

.store-card:hover .store-card__more,
.store-card:focus-within .store-card__more,
.store-card__actions:focus-within .store-card__more {
  opacity: 1;
}

.store-card__subtitle {
  .provider-card-subtitle();
}

.store-card__type {
  font-weight: 500;
}

.store-card__sep {
  color: var(--td-text-color-placeholder);
}

.store-card__endpoint {
  font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  font-size: var(--app-text-xs);
  color: var(--td-text-color-placeholder);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}


.empty-stores {
  padding: 64px 0;
  text-align: center;

  :deep(.t-empty__description) {
    font-size: var(--app-text-base);
    color: var(--td-text-color-placeholder);
    margin-bottom: 16px;
  }
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

  &--inline { margin: 0; }

  // "Danger confirmations" such as the TLS warning use red text
  &--warn { color: var(--td-error-color); }
}

:deep(.t-input),
:deep(.t-select),
:deep(.t-textarea) {
  width: 100%;
  font-size: var(--app-text-md);
}

// Hide t-form's default form-item container; use custom .form-item / .form-label
:deep(.t-form) .t-form-item {
  display: none;
}

.vision-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
}

// ---- inline alert (replaces the earlier large .immutable-notice banner) ----
.inline-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--app-text-md);
  line-height: 1.5;
  color: var(--td-text-color-secondary);
  flex-wrap: wrap;

  white-space: pre-line;

  &__icon {
    font-size: var(--app-text-lg);
    flex-shrink: 0;
    color: var(--td-text-color-placeholder);
  }

  &__text {
    flex: 1 1 auto;
    min-width: 0;
  }

  &--info {
    color: var(--td-text-color-primary);

    .inline-alert__icon { color: var(--td-brand-color); }
  }
}

// ---- Edit mode read-only field list (keeps the original look without the outer frame, right below the alert) ----
.readonly-fields {
  padding: 10px 12px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-md);
}

.readonly-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 4px 0;
  font-size: var(--app-text-sm);
  line-height: 1.4;
  border-bottom: 1px solid var(--td-component-stroke);

  &:last-child { border-bottom: none; }
}

.readonly-label {
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-xs);
  white-space: nowrap;
  min-width: 80px;
}

.readonly-value {
  color: var(--td-text-color-primary);
  font-size: var(--app-text-sm);
  font-family: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  word-break: break-all;
}

// ---- Advanced index expand/collapse button ----
.advanced-toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 0;
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  background: transparent;
  border: none;
  font-family: inherit;
  cursor: pointer;
  user-select: none;
  align-self: flex-start;

  &:hover { color: var(--td-brand-color); }

  .t-icon { font-size: var(--app-text-base); }
}

// ---- Number input: remove the native spinner (same as the MCP advanced config) ----
.number-input {
  :deep(input::-webkit-outer-spin-button),
  :deep(input::-webkit-inner-spin-button) {
    -webkit-appearance: none;
    appearance: none;
    margin: 0;
  }

  :deep(input[type="number"]) {
    -moz-appearance: textfield;
    appearance: textfield;
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

// ---- Status icon for the footer-left test button ----
.status-icon {
  font-size: var(--app-text-xl);
  flex-shrink: 0;

  &.available { color: var(--td-brand-color); }
  &.unavailable { color: var(--td-error-color); }
}
</style>

<!--
  Non-scoped block: per-engine header-icon coloring + color-logo background
  tweak. Same pattern as Storage/Parser/WebSearch drawers - these rules
  must be global so they reach the t-drawer panel even if its scoped
  data-attribute is dropped in some builds. Each rule mirrors the matching
  .store-card--{engine} .store-card__badge from the scoped block above so
  list-card → drawer hand-off stays visually continuous.
-->
<style lang="less">
// For a color logo give the header-icon container a white background + 1px border
.vectorstore-drawer .setting-drawer__header-icon:has(.header-icon__img) {
  background: var(--td-bg-color-container);
  box-shadow: inset 0 0 0 1px var(--td-component-stroke);
}

.vectorstore-drawer--qdrant .setting-drawer__header-icon {
  background: rgba(225, 38, 38, 0.12);
  color: #E12626;
}
.vectorstore-drawer--milvus .setting-drawer__header-icon {
  background: rgba(0, 137, 255, 0.12);
  color: #0089FF;
}
.vectorstore-drawer--weaviate .setting-drawer__header-icon {
  background: color-mix(in srgb, var(--td-brand-color) 12%, transparent);
  color: #07A050;
}
.vectorstore-drawer--elasticsearch .setting-drawer__header-icon,
.vectorstore-drawer--elasticfaiss .setting-drawer__header-icon {
  background: rgba(255, 153, 0, 0.12);
  color: #D97706;
}
.vectorstore-drawer--postgres .setting-drawer__header-icon {
  background: rgba(0, 82, 217, 0.1);
  color: #0052D9;
}
.vectorstore-drawer--opensearch .setting-drawer__header-icon {
  background: rgba(98, 53, 187, 0.12);
  color: #6235BB;
}
.vectorstore-drawer--infinity .setting-drawer__header-icon {
  background: rgba(98, 53, 187, 0.12);
  color: #6235BB;
}
.vectorstore-drawer--tencent_vectordb .setting-drawer__header-icon {
  background: rgba(0, 82, 217, 0.1);
  color: #0052D9;
}
.vectorstore-drawer--doris .setting-drawer__header-icon {
  background: rgba(255, 90, 0, 0.12);
  color: #E55A00;
}
.vectorstore-drawer--sqlite .setting-drawer__header-icon {
  background: rgba(70, 70, 70, 0.1);
  color: #464646;
}
</style>
