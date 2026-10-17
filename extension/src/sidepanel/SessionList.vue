<script setup lang="ts">
import { onMounted, ref } from 'vue';
import type { Session } from '../shared/types';
import type { useChat } from './useChat';

const props = defineProps<{ chat: ReturnType<typeof useChat> }>();
const emit = defineEmits<{ picked: []; error: [error: unknown] }>();

const sessions = ref<Session[] | null>(null);
const fail = (e: unknown) => emit('error', e);

async function open(id: string) {
  try {
    await props.chat.openSession(id);
    emit('picked');
  } catch (e) {
    fail(e);
  }
}

function startNew() {
  props.chat.newConversation();
  emit('picked');
}

onMounted(() => props.chat.api.listSessions().then((list) => (sessions.value = list), fail));
</script>

<template>
  <div class="sessions">
    <button type="button" class="primary" @click="startNew">New conversation</button>
    <p v-if="sessions?.length === 0" class="muted">No conversations yet</p>
    <ul v-else-if="sessions?.length">
      <li v-for="s in sessions" :key="s.id">
        <button type="button" :class="{ current: s.id === chat.state.sessionId }" @click="open(s.id)">
          <span class="truncate">{{ s.title || 'Untitled' }}</span>
          <span class="muted date">{{ new Date(s.updated_at).toLocaleString() }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.sessions {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  align-content: start;
  gap: var(--aikb-space-2);
}
ul {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: var(--aikb-space-1);
}
li button {
  width: 100%;
  display: grid;
  text-align: left;
}
li button.current {
  border-color: var(--aikb-brand);
}
.date {
  font-size: 12px;
}
</style>
