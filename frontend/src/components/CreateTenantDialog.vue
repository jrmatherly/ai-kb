<template>
  <!-- Self-service create-workspace dialog. Any signed-in user can call POST /api/v1/tenants
       (the backend router dropped the g.CrossTenant() guard), and the handler makes the current
       user the Owner of the new space with EnsureOwner. -->
  <t-dialog :visible="visible" width="480px" :on-confirm="handleSubmit" :on-close="handleClose"
    :confirm-btn="{ content: $t('tenant.create.submit'), loading: submitting, theme: 'primary' }"
    :cancel-btn="{ content: $t('tenant.create.cancel') }" :close-on-overlay-click="!submitting"
    :close-on-esc-keydown="!submitting" @update:visible="onVisibleUpdate">
    <template #header>
      <span class="create-tenant-dialog-header">
        <t-icon name="system-sum" size="20px" class="create-tenant-dialog-header-icon" aria-hidden="true" />
        <span class="create-tenant-dialog-header-title">{{ $t('tenant.create.dialogTitle') }}</span>
      </span>
    </template>

    <p class="create-tenant-tip">{{ $t('tenant.create.dialogSubtitle') }}</p>

    <t-form ref="formRef" :data="form" :rules="formRules" label-align="top" class="create-tenant-form" @submit.prevent>
      <t-form-item :label="$t('tenant.create.nameLabel')" name="name">
        <t-input v-model="form.name" :placeholder="$t('tenant.create.namePlaceholder')" :maxlength="128" autofocus
          @enter="handleSubmit" />
      </t-form-item>
      <t-form-item :label="$t('tenant.create.descriptionLabel')" name="description">
        <t-textarea v-model="form.description" :placeholder="$t('tenant.create.descriptionPlaceholder')"
          :maxlength="512" :autosize="{ minRows: 3, maxRows: 5 }" />
      </t-form-item>
    </t-form>
  </t-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin, type FormInstanceFunctions, type FormRule } from 'tdesign-vue-next'
import { createTenant, type TenantInfo } from '@/api/tenant'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  // After creation, the parent decides how to navigate (switch to the new space, refresh the local list and so on).
  (e: 'created', tenant: TenantInfo): void
}>()

const { t } = useI18n()

const formRef = ref<FormInstanceFunctions | null>(null)
const submitting = ref(false)

const form = reactive({
  name: '',
  description: '',
})

// Trim-aware required check: t-input's required does not trim, so all spaces would
// pass; check non-empty after trim by hand. :maxlength hard-limits the length while typing,
// so no rule repeats it here (that would show two messages).
const formRules: Record<string, FormRule[]> = {
  name: [
    {
      validator: (val: string) => (val ?? '').trim().length > 0,
      message: t('tenant.create.nameRequired'),
      trigger: 'blur',
    },
  ],
}

watch(
  () => props.visible,
  (open) => {
    if (open) {
      form.name = ''
      form.description = ''
      requestAnimationFrame(() => formRef.value?.clearValidate?.())
    }
  },
)

const onVisibleUpdate = (next: boolean) => {
  if (!next && submitting.value) return
  emit('update:visible', next)
}

const handleClose = () => {
  if (submitting.value) return
  emit('update:visible', false)
}

const handleSubmit = async () => {
  if (submitting.value) return
  const validateResult = await formRef.value?.validate?.()
  if (validateResult !== true) return

  submitting.value = true
  try {
    const response = await createTenant({
      name: form.name.trim(),
      description: form.description.trim() || undefined,
    })
    if (!response.success || !response.data) {
      MessagePlugin.error(response.message || t('tenant.create.failed'))
      return
    }
    MessagePlugin.success(t('tenant.create.success'))
    emit('created', response.data)
    emit('update:visible', false)
  } catch (error: any) {
    console.error('Failed to create tenant:', error)
    MessagePlugin.error(error?.message || t('tenant.create.failed'))
  } finally {
    submitting.value = false
  }
}
</script>

<style lang="less" scoped>
.create-tenant-dialog-header {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.create-tenant-dialog-header-icon {
  flex-shrink: 0;
  color: var(--td-brand-color);
}

.create-tenant-dialog-header-title {
  font: inherit;
}

.create-tenant-tip {
  margin: 0 0 16px;
  font-size: var(--app-text-md);
  line-height: 1.55;
  color: var(--td-text-color-secondary);
}

.create-tenant-form {
  :deep(.t-form__item):last-child {
    margin-bottom: 0;
  }
}
</style>
