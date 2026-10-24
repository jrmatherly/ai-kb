<template>
  <!-- Global top-right "pending invitations" bell.
       - Same logic as the bell that used to live in UserMenu: renders only when pendingInvitationCount > 0,
         so an empty inbox takes no corner pixels.
       - Fixed position with a z-index far below t-drawer's default 1500, so right-side drawers on business pages (FAQ, KB debugging,
         tenant audit, SettingDrawer and so on) naturally cover the bell with no explicit hide logic.
       - Clicking the bell reuses the same MyInvitationsDialog, with unchanged behavior. -->
  <template v-if="pendingInvitationCount > 0">
    <t-badge :count="pendingInvitationCount" :max-count="99" :offset="[6, 4]"
      class="global-invitation-bell">
      <button type="button" class="global-invitation-bell__btn"
        :title="$t('tenantInvitation.inboxTooltip')" @click="openDialog">
        <t-icon name="notification" size="18px" />
      </button>
    </t-badge>
  </template>
  <MyInvitationsDialog v-model:visible="dialogVisible" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import MyInvitationsDialog from '@/components/MyInvitationsDialog.vue'

const authStore = useAuthStore()

const pendingInvitationCount = computed(() => authStore.pendingInvitationCount)

const dialogVisible = ref(false)
const openDialog = () => {
  dialogVisible.value = true
}
</script>

<style lang="less" scoped>
.global-invitation-bell {
  position: fixed;
  top: 12px;
  right: 16px;
  /* Far below the TDesign drawer's default z-index (1500), so right-side drawers on business pages cover the bell.
     Above ordinary page content (usually 0 to 10), so list cards do not cover it. */
  z-index: 100;
}

.global-invitation-bell__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-lg);
  /* Container background instead of transparent, so the bell stays legible over page backgrounds of any color. */
  background: var(--td-bg-color-container);
  color: var(--td-text-color-secondary);
  cursor: pointer;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.04);
  transition: background-color 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;

  &:hover {
    background-color: var(--td-bg-color-secondarycontainer);
    color: var(--td-brand-color);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
  }

  &:focus-visible {
    outline: 2px solid var(--td-brand-color-focus);
    outline-offset: 1px;
  }
}
</style>
