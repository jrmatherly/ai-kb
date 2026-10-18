<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { errorMessage } from '../shared/api';
import { CLIP_HINT } from '../shared/capture';
import { send, workerApi, type HeadersReply } from '../shared/messages';
import { readRecent, resyncRecent, writeComposer, writeIntent } from '../shared/session';
import type { Capture, Intent, Me } from '../shared/types';
import CaptureList from '../ui/CaptureList.vue';
import ConnectionNotice from '../ui/ConnectionNotice.vue';
import Toast from '../ui/Toast.vue';

const SHOWN = 5;

const headers = ref<HeadersReply | undefined>();
const me = ref<Me | null>(null);
const recent = ref<Capture[]>([]);
const question = ref('');
const error = ref('');
// Read once at mount, so no handler below awaits before chrome.sidePanel.open and loses the gesture.
const tabId = ref<number | undefined>();

const openSettings = () => void chrome.runtime.openOptionsPage();

const status = computed(() => {
  const h = headers.value;
  if (!h || h.authKind === 'signed_out') return '';
  if (!me.value) return `Connecting to ${h.serverUrl}`;
  return `Connected to ${h.serverUrl} as ${me.value.user.email}${me.value.tenant ? `, ${me.value.tenant.name}` : ''}`;
});

function openPanel() {
  if (tabId.value === undefined) return;
  chrome.sidePanel.open({ tabId: tabId.value }).catch((e: unknown) => (error.value = errorMessage(e)));
}

// The writes are started, not awaited: a started write completes after the popup closes, a later one may never run.
function start(intent: Intent) {
  if (tabId.value === undefined) return;
  void writeIntent(tabId.value, intent);
  openPanel();
}

function ask() {
  const text = question.value.trim();
  if (tabId.value === undefined || !text) return;
  void writeComposer(tabId.value, text);
  void writeIntent(tabId.value, 'ask');
  openPanel();
}

onMounted(async () => {
  try {
    const [tabs, reply, stored] = await Promise.all([
      chrome.tabs.query({ active: true, currentWindow: true }),
      send<HeadersReply>({ type: 'GET_HEADERS' }),
      readRecent(),
    ]);
    tabId.value = tabs[0]?.id;
    headers.value = reply;
    recent.value = stored.slice(0, SHOWN);
    if (!reply || reply.authKind === 'signed_out') return;
    const api = workerApi();
    await Promise.all([api.me().then((user) => (me.value = user)), resyncRecent(api).then((list) => (recent.value = list.slice(0, SHOWN)))]);
  } catch (e) {
    error.value = errorMessage(e);
  }
});
</script>

<template>
  <main class="popup">
    <ConnectionNotice :headers="headers" />
    <p v-if="status" class="muted status">{{ status }}</p>

    <form class="ask" @submit.prevent="ask">
      <input v-model="question" aria-label="Question" placeholder="Ask about this page" />
      <button type="submit" class="primary" :disabled="tabId === undefined || !question.trim()">Ask</button>
    </form>

    <div class="buttons">
      <button type="button" :disabled="tabId === undefined" @click="start('clip:article')">Smart clip</button>
      <button type="button" :disabled="tabId === undefined" @click="start('clip:select')">Select clip</button>
      <button type="button" :disabled="tabId === undefined" @click="start('note')">Quick note</button>
    </div>
    <p class="hint">{{ CLIP_HINT }}</p>
    <button type="button" :disabled="tabId === undefined" @click="openPanel">Open side panel</button>

    <CaptureList v-if="headers && recent.length" :captures="recent" :server-url="headers.serverUrl" />

    <a href="#" class="settings" @click.prevent="openSettings">Settings</a>
    <Toast v-if="error" kind="error" :message="error" />
  </main>
</template>

<style src="../ui/tokens.css"></style>

<style scoped>
.popup {
  width: 320px;
  box-sizing: border-box;
  display: grid;
  gap: var(--aikb-space-2);
  padding: var(--aikb-space-3);
}
.status {
  font-size: 12px;
  overflow-wrap: anywhere;
}
.settings {
  color: var(--aikb-brand);
}
.ask {
  display: flex;
  gap: var(--aikb-space-2);
}
.ask input {
  flex: 1;
  min-width: 0;
}
.buttons {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--aikb-space-1);
}
.buttons button {
  padding-inline: var(--aikb-space-1);
}
.settings {
  justify-self: start;
}
</style>
