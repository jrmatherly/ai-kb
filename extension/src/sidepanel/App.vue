<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef, watch } from 'vue';
import { errorKind, errorMessage } from '../shared/api';
import { loadConnection, onIdentityChange } from '../shared/connection';
import { send, workerApi, type CommandsReply, type HeadersReply, type TabAccessReply } from '../shared/messages';
import { onSessionChange, readComposer, writeComposer } from '../shared/session';
import type { Capture } from '../shared/types';
import CaptureList from '../ui/CaptureList.vue';
import ConnectionNotice from '../ui/ConnectionNotice.vue';
import KbPicker from '../ui/KbPicker.vue';
import Toast from '../ui/Toast.vue';
import CaptureEditor from './CaptureEditor.vue';
import ChatView from './ChatView.vue';
import SessionList from './SessionList.vue';
import { useCapture } from './useCapture';
import { useChat } from './useChat';

type Tab = 'chat' | 'capture' | 'recent';
const TABS: Array<{ id: Tab; label: string }> = [
  { id: 'chat', label: 'Chat' },
  { id: 'capture', label: 'Capture' },
  { id: 'recent', label: 'Recent' },
];

const headers = ref<HeadersReply | undefined>();
const tab = ref<Tab>('chat');
const tabId = ref<number | null>(null);
const tabAccess = ref<TabAccessReply | null>(null);
const commands = ref<CommandsReply>([]);
const composer = ref('');
const history = ref(false);
const error = ref('');
const chat = shallowRef<ReturnType<typeof useChat> | null>(null);
const capture = shallowRef<ReturnType<typeof useCapture> | null>(null);

// Children report null when the action that failed is retried. Signed out takes the panel's place.
function report(e: unknown) {
  if (e === null) return void (error.value = '');
  const kind = errorKind(e);
  if (kind === 'unauthorized' && headers.value) return void (headers.value = { ...headers.value, authKind: 'signed_out' });
  error.value = kind === 'network' ? `Cannot reach ${headers.value?.serverUrl ?? 'the server'}` : errorMessage(e);
}

// Every part of the panel is built for one connection, so connecting, signing out or switching rebuilds it.
const stopIdentityWatch = onIdentityChange(() => location.reload());

const shortcut = computed(() => {
  const name = capture.value?.state.intent?.endsWith(':select') ? 'select-clip' : 'smart-clip';
  return commands.value.find((c) => c.name === name)?.shortcut || 'the shortcut';
});
const coldHint = computed(() => `Click the AI KB icon, press ${shortcut.value}, or use the right-click menu to clip this page`);
const pageState = computed(() => capture.value?.pageState.value ?? { kind: 'ok' as const });
// Nothing shows until the first TAB_ACCESS reply, so a page that can be clipped never flashes the cold hint.
const pageHint = computed(() => {
  const p = pageState.value;
  if (!tabAccess.value || p.kind === 'ok') return '';
  if (p.kind === 'failed') return `Could not load the content: ${p.message}`;
  return p.kind === 'cold' ? coldHint.value : 'This page cannot be clipped';
});

const clip = (mode: 'article' | 'select') => void capture.value?.startClip(mode).catch(report);

function editRecent(c: Capture) {
  tab.value = 'capture';
  void capture.value?.editCapture(c).catch(report);
}

// The popup's ask goes to Chat, where the composer is prefilled from composer:<tabId>. A clip intent that came
// with its draft is already open, and a quick one was handled by the composable.
watch(() => capture.value?.state.intentSeq, () => {
  const c = capture.value;
  const intent = c?.state.intent;
  if (!c || !intent) return;
  tab.value = intent === 'ask' ? 'chat' : 'capture';
  if (intent === 'note') c.newNote();
  else if (intent.startsWith('clip:') && !c.state.intentDraft && tabAccess.value && 'url' in tabAccess.value) clip(intent === 'clip:select' ? 'select' : 'article');
});
// A clip arriving under the same intent as the last one leaves the intent unchanged, so the editor opening is the signal too.
watch(() => capture.value?.state.editor.phase, (phase, old) => {
  if (phase !== 'idle' && old === 'idle') tab.value = 'capture';
});

async function takeComposer(id: number) {
  const text = await readComposer(id);
  if (text === null || tabId.value !== id) return;
  composer.value = text;
  await writeComposer(id, null);
}

async function refreshAccess(id: number) {
  const reply = await send<TabAccessReply>({ type: 'TAB_ACCESS', tabId: id });
  if (tabId.value === id) tabAccess.value = reply;
}
// The access reply comes first, so a clip intent from the popup finds the page it may clip.
watch(tabId, (id) => {
  if (id === null) return;
  void Promise.all([refreshAccess(id).then(() => capture.value?.loadFromSession()), takeComposer(id)]).catch(report);
});

let windowId: number | undefined;
const onActivated = (info: chrome.tabs.OnActivatedInfo) => {
  if (info.windowId === windowId) tabId.value = info.tabId;
};
// A navigation in the followed tab changes its URL and drops the activeTab grant.
const onUpdated = (id: number, info: chrome.tabs.OnUpdatedInfo) => {
  if (id === tabId.value && (info.status === 'complete' || info.url)) void refreshAccess(id).catch(report);
};
const stopSessionWatch = onSessionChange((key) => {
  if (tabId.value !== null && key === `composer:${tabId.value}`) void takeComposer(tabId.value).catch(report);
});
const onPageHide = () => {
  if (chat.value?.state.streaming) void chat.value.stop({ keepalive: true });
};

onMounted(async () => {
  try {
    headers.value = await send<HeadersReply>({ type: 'GET_HEADERS' });
    if (!headers.value || headers.value.authKind === 'signed_out') return;
    const api = workerApi();
    const c = (chat.value = useChat({ api, authKind: headers.value.authKind }));
    capture.value = useCapture({ api, send, tabId: () => tabId.value!, tabAccess: () => tabAccess.value ?? { access: false }, loadConnection, onError: report });
    void capture.value.resync().catch(report);
    addEventListener('pagehide', onPageHide);
    chrome.tabs.onActivated.addListener(onActivated);
    chrome.tabs.onUpdated.addListener(onUpdated);
    const fromUrl = Number(new URLSearchParams(location.search).get('tabId'));
    const initialTab = fromUrl ? Promise.resolve(fromUrl) : chrome.tabs.query({ active: true, lastFocusedWindow: true }).then((tabs) => tabs[0]?.id ?? null);
    await Promise.all([
      chrome.windows.getCurrent().then((w) => (windowId = w.id)),
      initialTab.then((id) => (tabId.value ??= id)),
      send<CommandsReply>({ type: 'GET_COMMANDS' }).then((reply) => (commands.value = reply)),
      c.load(),
    ]);
  } catch (e) {
    report(e);
  }
});

onUnmounted(() => {
  stopIdentityWatch();
  stopSessionWatch();
  capture.value?.stop();
  chrome.tabs.onActivated.removeListener(onActivated);
  chrome.tabs.onUpdated.removeListener(onUpdated);
  removeEventListener('pagehide', onPageHide);
});
</script>

<template>
  <main class="shell">
    <ConnectionNotice :headers="headers" />

    <template v-if="chat && capture && headers && headers.authKind !== 'signed_out'">
      <nav class="tabs" role="tablist">
        <button
          v-for="t in TABS"
          :key="t.id"
          type="button"
          role="tab"
          :aria-selected="tab === t.id"
          :class="{ active: tab === t.id }"
          @click="tab = t.id"
        >{{ t.label }}</button>
      </nav>

      <section v-if="tab === 'chat'" class="chat">
        <div class="controls">
          <select :value="chat.state.agentId ?? ''" aria-label="Agent" @change="chat.selectAgent(($event.target as HTMLSelectElement).value)">
            <option v-for="a in chat.state.agents" :key="a.id" :value="a.id">{{ a.name }}</option>
          </select>
          <button type="button" :class="{ active: history }" @click="history = !history">History</button>
        </div>
        <KbPicker
          v-if="chat.kbChoice.value.show"
          multiple
          aria-label="Knowledge bases"
          :disabled="chat.kbChoice.value.locked"
          :kbs="chat.kbChoice.value.kbs"
          :model-value="chat.state.kbIds"
          @update:model-value="chat.setKbIds"
        />
        <SessionList v-if="history" :chat="chat" @picked="history = false" @error="report" />
        <ChatView
          v-show="!history"
          v-model:draft="composer"
          :chat="chat"
          :server-url="headers.serverUrl"
          :tab-access="tabAccess"
          :cold-hint="coldHint"
          @error="report"
        />
      </section>
      <section v-else-if="tab === 'capture'" class="capture">
        <CaptureEditor v-if="capture.state.editor.phase !== 'idle'" :capture="capture" :server-url="headers.serverUrl" />
        <div v-else class="starts">
          <template v-if="pageState.kind === 'ok'">
            <button type="button" class="primary" @click="clip('article')">Clip this page</button>
            <button type="button" class="primary" @click="clip('select')">Clip selection</button>
          </template>
          <p v-else-if="pageHint" class="notice">{{ pageHint }}</p>
          <button type="button" class="primary" @click="capture.newNote()">New note</button>
        </div>
      </section>
      <div v-else class="recent">
        <p v-if="!capture.state.recent.length" class="muted">No captures yet</p>
        <CaptureList v-else v-slot="{ capture: c }" :captures="capture.state.recent" :server-url="headers.serverUrl">
          <button type="button" @click="editRecent(c)">Edit</button>
        </CaptureList>
      </div>
    </template>

    <Toast v-if="error" kind="error" :message="error" />
  </main>
</template>

<style src="../ui/tokens.css"></style>

<style scoped>
.shell {
  height: 100vh;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: var(--aikb-space-2);
  padding: var(--aikb-space-2);
}
.capture {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  align-content: start;
  gap: var(--aikb-space-2);
}
.starts {
  display: grid;
  gap: var(--aikb-space-2);
}
.recent {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
.chat {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: var(--aikb-space-2);
}
.controls {
  display: flex;
  gap: var(--aikb-space-2);
}
.controls select {
  flex: 1;
  min-width: 0;
}
.controls button.active {
  border-color: var(--aikb-brand);
  color: var(--aikb-brand);
}
</style>
