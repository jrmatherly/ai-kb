<script setup lang="ts">
defineProps<{ message: string; kind?: 'info' | 'error' }>();
</script>

<template>
  <div class="toast" :class="kind ?? 'info'" :role="kind === 'error' ? 'alert' : 'status'">
    {{ message }}
    <slot />
  </div>
</template>

<style scoped>
.toast {
  padding: var(--aikb-space-2) var(--aikb-space-3);
  border-radius: var(--aikb-radius-medium);
  border: 1px solid var(--aikb-border);
  background: var(--aikb-bg-secondary);
}
.error {
  border-color: var(--aikb-error);
  background: var(--aikb-error-soft);
}
</style>
