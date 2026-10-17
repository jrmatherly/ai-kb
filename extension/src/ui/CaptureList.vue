<script setup lang="ts">
import { kbUrl } from '../shared/serverUrl';
import type { Capture } from '../shared/types';

defineProps<{ captures: Capture[]; serverUrl: string }>();
defineSlots<{ default?: (props: { capture: Capture }) => unknown }>();

const STATUS: Record<Capture['status'], string> = {
  pending: 'Parsing',
  completed: 'Ready',
  failed: 'Parsing failed',
  cancelled: 'Cancelled',
  timeout: 'Still parsing',
};
</script>

<template>
  <ul class="captures">
    <li v-for="c in captures" :key="c.knowledgeId">
      <span class="truncate">{{ c.title || 'Untitled' }}</span>
      <span class="muted meta">
        <span class="status" :class="c.status">{{ STATUS[c.status] }}</span>, {{ new Date(c.savedAt).toLocaleDateString() }}
      </span>
      <span class="actions">
        <a :href="kbUrl(serverUrl, c.kbId)" target="_blank" rel="noopener">Open</a>
        <slot :capture="c" />
      </span>
    </li>
  </ul>
</template>

<style scoped>
.captures {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: var(--aikb-space-2);
}
li {
  display: grid;
  gap: var(--aikb-space-1);
  padding-bottom: var(--aikb-space-2);
  border-bottom: 1px solid var(--aikb-border);
}
.meta {
  font-size: 12px;
}
.status.failed {
  color: var(--aikb-error);
}
.actions {
  display: flex;
  align-items: center;
  gap: var(--aikb-space-2);
}
a {
  color: var(--aikb-brand);
}
</style>
