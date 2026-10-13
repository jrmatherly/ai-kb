<template>
  <div class="tenant-info">
    <div class="section-header">
      <h2>{{ $t('tenant.title') }}</h2>
      <p class="section-description">{{ $t('tenant.sectionDescription') }}</p>
    </div>

    <!-- Loading state -->
    <div v-if="loading" class="loading-inline">
      <t-loading size="small" />
      <span>{{ $t('tenant.loadingInfo') }}</span>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="error-inline">
      <t-alert theme="error" :message="error">
        <template #operation>
          <t-button size="small" @click="loadInfo">{{ $t('tenant.retry') }}</t-button>
        </template>
      </t-alert>
    </div>

    <!-- Content: info list + danger zone, kept apart so the setting-row bottom border does not mix with a dashed line -->
    <div v-else class="tenant-info-body">
      <div class="settings-group">
        <!-- Tenant ID -->
        <div class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.details.idLabel') }}</label>
            <p class="desc">{{ $t('tenant.details.idDescription') }}</p>
          </div>
          <div class="setting-control">
            <span class="info-value">{{ tenantInfo?.id || '-' }}</span>
          </div>
        </div>

        <!-- Tenant name -->
        <div class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.details.nameLabel') }}</label>
            <p class="desc">{{ $t('tenant.details.nameDescription') }}</p>
          </div>
          <div class="setting-control">
            <!-- Read-only: name + edit button (only the owner sees it).
               In-place editing instead of a dialog, consistent with the other rows. -->
            <template v-if="!editing">
              <span class="info-value">{{ tenantInfo?.name || '-' }}</span>
              <t-button v-if="canEditTenant" theme="default" variant="text" shape="square" size="small"
                class="edit-btn" :title="$t('tenant.details.editName')" :aria-label="$t('tenant.details.editName')"
                @click="startEditName">
                <template #icon>
                  <t-icon name="edit" />
                </template>
              </t-button>
            </template>
            <!-- Editing: input + save/cancel. Enter saves, Esc cancels. -->
            <div v-else class="inline-edit">
              <t-input v-model="editName" :placeholder="$t('tenant.details.editNamePlaceholder')" :maxlength="64"
                :disabled="saving" autofocus class="inline-edit-input" @enter="saveTenantName"
                @keydown="onEditKeydown" />
              <t-button theme="primary" size="small" :loading="saving" :disabled="!canSubmit" @click="saveTenantName">
                {{ $t('tenant.details.editNameConfirm') }}
              </t-button>
              <t-button theme="default" variant="outline" size="small" :disabled="saving" @click="cancelEditName">
                {{ $t('tenant.details.editNameCancel') }}
              </t-button>
            </div>
          </div>
        </div>

        <!-- Tenant description -->
        <div class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.details.descriptionLabel') }}</label>
            <p class="desc">{{ $t('tenant.details.descriptionDescription') }}</p>
          </div>
          <div class="setting-control">
            <!-- Read-only: description (placeholder when empty) + edit button (only the owner sees it).
               Same in-place editing as the name. -->
            <template v-if="!editingDescription">
              <span class="info-value description-value" :class="{ 'is-empty': !tenantInfo?.description }">
                {{ tenantInfo?.description || $t('tenant.details.descriptionEmptyPlaceholder') }}
              </span>
              <t-button v-if="canEditTenant" theme="default" variant="text" shape="square" size="small"
                class="edit-btn" :title="$t('tenant.details.editDescription')"
                :aria-label="$t('tenant.details.editDescription')" @click="startEditDescription">
                <template #icon>
                  <t-icon name="edit" />
                </template>
              </t-button>
            </template>
            <!-- Editing: textarea + save/cancel. Esc cancels, Ctrl/Cmd+Enter saves;
               plain Enter inserts a newline, so it does not submit. -->
            <div v-else class="inline-edit inline-edit-description">
              <t-textarea v-model="editDescription"
                :placeholder="$t('tenant.details.editDescriptionPlaceholder')" :maxlength="512"
                :autosize="{ minRows: 2, maxRows: 6 }" :disabled="savingDescription" autofocus
                class="inline-edit-textarea" @keydown="onEditDescriptionKeydown" />
              <div class="inline-edit-actions">
                <t-button theme="primary" size="small" :loading="savingDescription"
                  :disabled="!canSubmitDescription" @click="saveTenantDescription">
                  {{ $t('tenant.details.editNameConfirm') }}
                </t-button>
                <t-button theme="default" variant="outline" size="small" :disabled="savingDescription"
                  @click="cancelEditDescription">
                  {{ $t('tenant.details.editNameCancel') }}
                </t-button>
              </div>
            </div>
          </div>
        </div>

        <!-- Tenant business -->
        <div v-if="tenantInfo?.business" class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.details.businessLabel') }}</label>
            <p class="desc">{{ $t('tenant.details.businessDescription') }}</p>
          </div>
          <div class="setting-control">
            <span class="info-value">{{ tenantInfo.business }}</span>
          </div>
        </div>

        <!-- Tenant status -->
        <div class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.details.statusLabel') }}</label>
            <p class="desc">{{ $t('tenant.details.statusDescription') }}</p>
          </div>
          <div class="setting-control">
            <t-tag :theme="getStatusTheme(tenantInfo?.status)" variant="light" size="small">
              {{ getStatusText(tenantInfo?.status) }}
            </t-tag>
          </div>
        </div>

        <!-- Tenant creation time -->
        <div class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.details.createdAtLabel') }}</label>
            <p class="desc">{{ $t('tenant.details.createdAtDescription') }}</p>
          </div>
          <div class="setting-control">
            <span class="info-value">{{ formatDate(tenantInfo?.created_at) }}</span>
          </div>
        </div>

        <!-- Storage quota -->
        <div v-if="tenantInfo?.storage_quota !== undefined" class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.storage.quotaLabel') }}</label>
            <p class="desc">{{ $t('tenant.storage.quotaDescription') }}</p>
          </div>
          <div class="setting-control">
            <span class="info-value">{{ formatBytes(tenantInfo.storage_quota) }}</span>
          </div>
        </div>

        <!-- Used storage -->
        <div v-if="tenantInfo?.storage_quota !== undefined" class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.storage.usedLabel') }}</label>
            <p class="desc">{{ $t('tenant.storage.usedDescription') }}</p>
          </div>
          <div class="setting-control">
            <span class="info-value">{{ formatBytes(tenantInfo.storage_used || 0) }}</span>
          </div>
        </div>

        <!-- Storage usage -->
        <div v-if="tenantInfo?.storage_quota !== undefined" class="setting-row">
          <div class="setting-info">
            <label>{{ $t('tenant.storage.usageLabel') }}</label>
            <p class="desc">{{ $t('tenant.storage.usageDescription') }}</p>
          </div>
          <div class="setting-control">
            <div class="usage-control">
              <span class="usage-text">{{ getUsagePercentage() }}%</span>
              <!-- t-progress: theme is the shape (line/plump/circle); color comes from status -->
              <t-progress :percentage="getUsagePercentage()" :show-info="false" size="small"
                :status="getUsagePercentage() > 80 ? 'warning' : 'success'" style="flex: 1;" />
            </div>
          </div>
        </div>

      </div>

      <aside v-if="showLeaveDangerZone" class="leave-space-panel" :aria-label="$t('tenant.leaveDangerZone.title')">
        <div class="leave-space-panel-inner">
          <div class="leave-space-panel-text">
            <div class="leave-space-panel-title">{{ $t('tenant.leaveDangerZone.title') }}</div>
            <p class="leave-space-panel-desc">{{ $t('tenant.leaveDangerZone.desc') }}</p>
          </div>
          <div class="leave-space-panel-action">
            <t-button theme="danger" variant="outline" size="medium" @click="confirmLeaveTenant">
              {{ $t('tenant.leaveDangerZone.button') }}
            </t-button>
          </div>
        </div>
      </aside>

      <aside v-if="showDeleteDangerZone" class="leave-space-panel delete-space-panel"
        :aria-label="$t('tenant.deleteDangerZone.title')">
        <div class="leave-space-panel-inner">
          <div class="leave-space-panel-text">
            <div class="leave-space-panel-title">{{ $t('tenant.deleteDangerZone.title') }}</div>
            <p class="leave-space-panel-desc">{{ $t('tenant.deleteDangerZone.desc') }}</p>
          </div>
          <div class="leave-space-panel-action">
            <t-button theme="danger" size="medium" @click="confirmDeleteTenant">
              {{ $t('tenant.deleteDangerZone.button') }}
            </t-button>
          </div>
        </div>
      </aside>

    </div>

    <t-dialog v-model:visible="deleteTenantVisible" :header="$t('tenant.deleteDangerZone.confirmTitle')"
      :confirm-btn="{
        content: $t('tenant.deleteDangerZone.confirm'),
        theme: 'danger',
        disabled: deleteConfirmName.trim() !== (tenantInfo?.name || ''),
        loading: deletingTenant,
      }" :cancel-btn="$t('common.cancel')" :close-on-overlay-click="!deletingTenant"
      :close-btn="!deletingTenant" @confirm="deleteCurrentTenant">
      <div class="delete-tenant-confirm">
        <p class="delete-tenant-confirm-body">
          {{ $t('tenant.deleteDangerZone.confirmBody', { name: tenantInfo?.name || '' }) }}
        </p>
        <p class="delete-tenant-confirm-hint">
          {{ $t('tenant.deleteDangerZone.confirmHint', { name: tenantInfo?.name || '' }) }}
        </p>
        <t-input v-model="deleteConfirmName" :placeholder="tenantInfo?.name || ''" :disabled="deletingTenant"
          clearable />
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { DialogPlugin, MessagePlugin } from 'tdesign-vue-next'
import { getCurrentUser, type TenantInfo } from '@/api/auth'
import { deleteTenant as deleteTenantApi, updateTenant as updateTenantApi } from '@/api/tenant'
import {
  leaveTenant,
  fetchAllTenantMembers,
  type TenantMember,
  type TenantRole,
} from '@/api/tenant/members'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { useRoleLabel, useHomeTenant } from '@/composables/useRoleLabel'
import {
  navigateAfterTenantSwitch,
  persistLastActiveTenantPreference,
  stashTenantSwitchToast,
} from '@/utils/tenantSwitch'

const { t, locale } = useI18n()
const { formatRole } = useRoleLabel()
const { homeTenantId } = useHomeTenant()
const authStore = useAuthStore()

// Reactive state
const tenantInfo = ref<TenantInfo | null>(null)
const loading = ref(true)
const error = ref('')

// Only the owner can rename the workspace (matches the g.Owner() guard in router.go;
// the server enforces it, this only decides whether the UI shows the control).
const canEditTenant = computed(() => authStore.hasRole('owner'))

/** As in TenantMembers.vue: hide Leave for the last Owner, which the server rejects as last-owner. */
const activeTenantNumericId = computed(() => Number(authStore.currentTenantId ?? 0))

const leaveMembersSnap = ref<TenantMember[]>([])
const leaveGateReady = ref(false)
const leaveGateLoading = ref(false)

const currentTenantRole = computed<TenantRole | ''>(() => (authStore.currentTenantRole || '') as TenantRole | '')

const canLeaveSpace = computed(() => {
  const r = currentTenantRole.value
  if (!r || !tenantInfo.value?.id) return false
  if (r !== 'owner') return true
  return leaveMembersSnap.value.filter((m) => m.role === 'owner').length > 1
})

/** Shown once the main content has loaded, the `listMembers` rule is resolved and leaving is allowed. */
const showLeaveDangerZone = computed(() => {
  if (loading.value || error.value || !tenantInfo.value) return false
  if (!leaveGateReady.value || leaveGateLoading.value) return false
  if (!currentTenantRole.value) return false
  if (Number(tenantInfo.value.id) !== activeTenantNumericId.value) return false
  return canLeaveSpace.value
})

const showDeleteDangerZone = computed(() => {
  if (loading.value || error.value || !tenantInfo.value) return false
  if (Number(tenantInfo.value.id) !== activeTenantNumericId.value) return false
  return authStore.hasRole('owner')
})

async function evaluateLeaveGate(): Promise<void> {
  leaveGateReady.value = false
  leaveMembersSnap.value = []
  leaveGateLoading.value = false

  const infoId = tenantInfo.value?.id != null ? Number(tenantInfo.value.id) : 0
  if (!infoId || !activeTenantNumericId.value || infoId !== activeTenantNumericId.value) {
    leaveGateReady.value = true
    return
  }

  const role = currentTenantRole.value
  if (!role) {
    leaveGateReady.value = true
    return
  }
  if (role !== 'owner') {
    leaveGateReady.value = true
    return
  }

  leaveGateLoading.value = true
  try {
    leaveMembersSnap.value = await fetchAllTenantMembers(infoId)
  } finally {
    leaveGateLoading.value = false
    leaveGateReady.value = true
  }
}

function confirmLeaveTenant() {
  const tid = Number(tenantInfo.value?.id ?? 0)
  if (!tid) return

  const dlg = DialogPlugin.confirm({
    header: t('tenantMember.leave.confirmTitle'),
    body: t('tenantMember.leave.confirmBody'),
    confirmBtn: { content: t('tenantMember.leave.confirm'), theme: 'danger' },
    cancelBtn: t('common.cancel'),
    onConfirm: async () => {
      try {
        const resp = await leaveTenant(tid)
        if (resp.success) {
          MessagePlugin.success(t('tenantMember.leave.success'))
          authStore.logout()
          window.location.href = '/login'
        } else {
          MessagePlugin.error(resp.message || t('tenantMember.errors.generic'))
        }
      } catch (err: any) {
        const status = err?.status
        if (status === 409) {
          MessagePlugin.error(t('tenantMember.errors.lastOwner'))
        } else {
          MessagePlugin.error(err?.message || t('tenantMember.errors.generic'))
        }
      } finally {
        dlg.destroy()
      }
    },
    onClose: () => dlg.destroy(),
  })
}

function confirmDeleteTenant() {
  const tid = Number(tenantInfo.value?.id ?? 0)
  const tenantName = tenantInfo.value?.name || ''
  if (!tid || !tenantName) return
  deleteConfirmName.value = ''
  deleteTenantVisible.value = true
}

async function deleteCurrentTenant() {
  const tid = Number(tenantInfo.value?.id ?? 0)
  const tenantName = tenantInfo.value?.name || ''
  if (!tid || !tenantName) return
  if (deleteConfirmName.value.trim() !== tenantName) {
    MessagePlugin.warning(t('tenant.deleteDangerZone.nameMismatch'))
    return
  }
  try {
    deletingTenant.value = true
    const resp = await deleteTenantApi(tid)
    if (resp.success) {
      MessagePlugin.success(t('tenant.deleteDangerZone.success'))
      authStore.setMemberships(
        (authStore.memberships ?? []).filter((m) => m.tenant_id !== tid),
      )
      await authStore.refreshFromAuthMe()
      const next =
        authStore.memberships.find((m) => m.tenant_id === homeTenantId.value) ??
        authStore.memberships[0]
      if (next) {
        const switchingToHome =
          homeTenantId.value !== null && homeTenantId.value === next.tenant_id
        const name = next.tenant_name?.trim() || `#${next.tenant_id}`
        authStore.setSelectedTenant(next.tenant_id, name)
        stashTenantSwitchToast({
          name,
          role: formatRole(next.role) || undefined,
          roleEnum: next.role || undefined,
        })
        const persist = persistLastActiveTenantPreference(
          switchingToHome ? null : next.tenant_id,
        )
        await Promise.race([persist, new Promise((r) => setTimeout(r, 400))])
        navigateAfterTenantSwitch()
        return
      }
      authStore.logout()
      window.location.href = '/login'
    } else {
      MessagePlugin.error(resp.message || t('tenant.deleteDangerZone.failed'))
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('tenant.deleteDangerZone.failed'))
  } finally {
    deletingTenant.value = false
    deleteConfirmName.value = ''
    deleteTenantVisible.value = false
  }
}

watch(
  [() => tenantInfo.value?.id, () => authStore.currentTenantId, () => authStore.currentTenantRole],
  () => {
    if (!loading.value && tenantInfo.value && !error.value) {
      void evaluateLeaveGate()
    }
  },
)

// In-place workspace name editing: editing toggles the row between read-only and edit.
// No dialog: for a single field it only interrupts browsing the settings.
const editing = ref(false)
const editName = ref('')
const saving = ref(false)
const deleteConfirmName = ref('')
const deleteTenantVisible = ref(false)
const deletingTenant = ref(false)
const editNameTrimmed = computed(() => editName.value.trim())
// No duplicate-name check: the backend name field has no uniqueIndex or uniqueness validation,
// and the service rejects empty names only on create, so the frontend guards against empty.
const canSubmit = computed(
  () => !saving.value && !!editNameTrimmed.value && editNameTrimmed.value !== tenantInfo.value?.name,
)

const startEditName = () => {
  editName.value = tenantInfo.value?.name || ''
  editing.value = true
}

const cancelEditName = () => {
  if (saving.value) return
  editing.value = false
  editName.value = ''
}

// t-input does not bubble Esc, so handle it here (symmetric with Enter).
const onEditKeydown = (_value: any, ctx: { e: KeyboardEvent }) => {
  if (ctx?.e?.key === 'Escape') {
    cancelEditName()
  }
}

// In-place description editing: same editing / editValue / saving state as the name.
// The description is optional, so submitting only requires a change, not a non-empty value.
const editingDescription = ref(false)
const editDescription = ref('')
const savingDescription = ref(false)
const editDescriptionTrimmed = computed(() => editDescription.value.trim())
const canSubmitDescription = computed(
  () => !savingDescription.value && editDescriptionTrimmed.value !== (tenantInfo.value?.description || ''),
)

const startEditDescription = () => {
  editDescription.value = tenantInfo.value?.description || ''
  editingDescription.value = true
}

const cancelEditDescription = () => {
  if (savingDescription.value) return
  editingDescription.value = false
  editDescription.value = ''
}

// In the textarea Enter inserts a newline, Ctrl/Cmd+Enter submits, Esc cancels.
const onEditDescriptionKeydown = (_value: any, ctx: { e: KeyboardEvent }) => {
  const e = ctx?.e
  if (!e) return
  if (e.key === 'Escape') {
    cancelEditDescription()
    return
  }
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    void saveTenantDescription()
  }
}

const saveTenantDescription = async () => {
  if (!tenantInfo.value?.id) return
  const newDesc = editDescriptionTrimmed.value
  if (newDesc === (tenantInfo.value.description || '')) {
    editingDescription.value = false
    return
  }

  try {
    savingDescription.value = true
    const resp = await updateTenantApi(Number(tenantInfo.value.id), { description: newDesc })
    if (resp.success) {
      // Update locally right away instead of waiting for /auth/me. Unlike the name, the description
      // is not shown in the workspace switcher, so authStore.tenant / memberships need no sync.
      if (tenantInfo.value) {
        tenantInfo.value = { ...tenantInfo.value, description: newDesc }
      }
      MessagePlugin.success(t('tenant.details.editDescriptionSuccess'))
      editingDescription.value = false
    } else {
      MessagePlugin.error(resp.message || t('tenant.details.editDescriptionFailed'))
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('tenant.details.editDescriptionFailed'))
  } finally {
    savingDescription.value = false
  }
}

const saveTenantName = async () => {
  const newName = editNameTrimmed.value
  if (!newName) {
    MessagePlugin.warning(t('tenant.details.editNameRequired'))
    return
  }
  if (!tenantInfo.value?.id) return
  if (newName === tenantInfo.value.name) {
    editing.value = false
    return
  }

  try {
    saving.value = true
    const resp = await updateTenantApi(Number(tenantInfo.value.id), { name: newName })
    if (resp.success) {
      // Update locally right away instead of waiting for /auth/me; also refresh the cached tenant
      // in the auth state (when the active workspace is the home tenant, the switcher updates too).
      if (tenantInfo.value) {
        tenantInfo.value = { ...tenantInfo.value, name: newName }
      }
      if (authStore.tenant && String(authStore.tenant.id) === String(tenantInfo.value?.id)) {
        authStore.setTenant({ ...authStore.tenant, name: newName })
      }
      // The workspace switcher reads tenant_name from memberships; sync it so the old name does not linger.
      if (authStore.memberships?.length) {
        const next = authStore.memberships.map((m) =>
          String(m.tenant_id) === String(tenantInfo.value?.id)
            ? { ...m, tenant_name: newName }
            : m,
        )
        authStore.setMemberships(next)
      }
      MessagePlugin.success(t('tenant.details.editNameSuccess'))
      editing.value = false
    } else {
      MessagePlugin.error(resp.message || t('tenant.details.editNameFailed'))
    }
  } catch (err: any) {
    MessagePlugin.error(err?.message || t('tenant.details.editNameFailed'))
  } finally {
    saving.value = false
  }
}

// Methods
const loadInfo = async () => {
  try {
    loading.value = true
    error.value = ''

    const userResponse = await getCurrentUser()

    const data = userResponse?.data as { tenant?: TenantInfo } | undefined
    if ((userResponse as any).success && data?.tenant) {
      tenantInfo.value = data.tenant
    } else {
      error.value = userResponse.message || t('tenant.messages.fetchFailed')
    }
  } catch (err: any) {
    error.value = err?.message || t('tenant.messages.networkError')
  } finally {
    loading.value = false
  }
  // Evaluate after loading=false: otherwise the loading condition in showLeaveDangerZone hides Leave,
  // and in some environments the role hydrates slightly after /auth/me returns.
  if (tenantInfo.value && !error.value) {
    await evaluateLeaveGate()
  }
}

const getStatusText = (status: string | undefined) => {
  switch (status) {
    case 'active':
      return t('tenant.statusActive')
    case 'inactive':
      return t('tenant.statusInactive')
    case 'suspended':
      return t('tenant.statusSuspended')
    default:
      return t('tenant.statusUnknown')
  }
}

const getStatusTheme = (status: string | undefined) => {
  switch (status) {
    case 'active':
      return 'success'
    case 'inactive':
      return 'warning'
    case 'suspended':
      return 'danger'
    default:
      return 'default'
  }
}

const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return t('tenant.unknown')

  try {
    const date = new Date(dateStr)
    const formatter = new Intl.DateTimeFormat(locale.value || 'en-US', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    })
    return formatter.format(date)
  } catch {
    return t('tenant.formatError')
  }
}

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const getUsagePercentage = () => {
  if (!tenantInfo.value?.storage_quota || tenantInfo.value.storage_quota === 0) {
    return 0
  }

  const used = tenantInfo.value.storage_used || 0
  const percentage = (used / tenantInfo.value.storage_quota) * 100
  return Math.min(Math.round(percentage * 100) / 100, 100)
}

// Lifecycle
onMounted(() => {
  loadInfo()
})
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/settings-section.less';

.tenant-info {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.loading-inline {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 40px 0;
  justify-content: center;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-base);
}

.error-inline {
  padding: 20px 0;
}

.tenant-info-body {
  display: flex;
  flex-direction: column;
}

.settings-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  .setting-row();
}

.setting-info {
  /* Not flex:1: the label column stays within a sensible max-content range (labels plus
     their desc text) and takes no free space, so long content cannot squeeze it to one character per line.
     min-width is the floor for a slightly longer desc. */
  flex: 0 0 auto;
  width: max-content;
  min-width: 140px;
  max-width: 40%;
  padding-right: 24px;

  label {
    font-size: var(--app-text-lg);
    font-weight: 500;
    color: var(--td-text-color-primary);
    display: block;
    margin-bottom: 4px;
  }

  .desc {
    font-size: var(--app-text-md);
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.5;
  }
}

.setting-control {
  .setting-control();
  flex: 1 1 auto;
  min-width: 0;

  .info-value {
    font-size: var(--app-text-base);
    color: var(--td-text-color-primary);
    text-align: right;
    /* anywhere is more aggressive than break-word: even long strings without spaces ("WorkspaceDefault...")
       break, so one value cannot overflow the row. */
    overflow-wrap: anywhere;
    min-width: 0;
  }

  .edit-btn {
    flex-shrink: 0;
  }
  /* Conversely, the content column takes the free space and may shrink and wrap, so long strings do not overflow.
     No min-width:280px: short content does not need that wide a slot. */
}

.inline-edit {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  justify-content: flex-end;
}

.inline-edit-input {
  /* When editing inline the input must not fill the row, or the two buttons hit the edge;
     cap it and let t-input's own ellipsis handle the rest. */
  max-width: 220px;
  flex: 1;
}

/* In-place description editing: the textarea wraps and grows, the buttons go below, right-aligned,
   so they are not squeezed horizontally as on the name row. */
.inline-edit-description {
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
  width: 100%;
  max-width: 360px;
}

.inline-edit-textarea {
  width: 100%;
}

.inline-edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* Read-only description: wraps across lines; an empty one uses the placeholder color to invite editing. */
.description-value {
  white-space: pre-wrap;
  word-break: break-word;

  &.is-empty {
    color: var(--td-text-color-placeholder);
  }
}

.leave-space-panel {
  margin-top: 4px;
}

.delete-space-panel {
  margin-top: 12px;
}

.leave-space-panel-inner {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 16px 18px;
  border-radius: var(--app-radius-lg);
  border: 1px solid var(--td-component-stroke);
  background-color: var(--td-bg-color-secondarycontainer);
  box-sizing: border-box;
}

.leave-space-panel-text {
  flex: 1;
  min-width: 0;
  max-width: min(65%, 28rem);
  padding-right: 8px;
}

.leave-space-panel-title {
  font-size: var(--app-text-lg);
  font-weight: 500;
  color: var(--td-text-color-primary);
  line-height: 1.4;
  margin-bottom: 4px;
}

.leave-space-panel-desc {
  margin: 0;
  font-size: var(--app-text-md);
  line-height: 1.55;
  color: var(--td-text-color-secondary);
}

.leave-space-panel-action {
  flex-shrink: 0;
}

@media (max-width: 560px) {
  .leave-space-panel-inner {
    flex-direction: column;
    align-items: stretch;
  }

  .leave-space-panel-text {
    max-width: none;
    padding-right: 0;
  }

  .leave-space-panel-action {
    display: flex;
    justify-content: flex-end;
  }
}

.usage-control {
  //   width: 100%;
  //   display: flex;
  //   align-items: center;
  //   gap: 12px;

  .usage-text {
    font-size: var(--app-text-base);
    font-weight: 500;
    color: var(--td-text-color-primary);
    min-width: 50px;
    text-align: right;
  }
}

.delete-tenant-confirm-body {
  margin: 0 0 10px;
  color: var(--td-text-color-primary);
  line-height: 1.6;
}

.delete-tenant-confirm-hint {
  margin: 0 0 12px;
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}
</style>
