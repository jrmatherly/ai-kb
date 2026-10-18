<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, shallowRef, watch } from 'vue';
import { renderMarkdown } from '../shared/markdown';
import { inlineCitations, STATUS_LABEL, type UiMessage } from './chat';
import type { TabAccessReply } from '../shared/messages';
import { kbUrl, sessionUrl } from '../shared/serverUrl';
import type { useChat } from './useChat';

const props = defineProps<{
  chat: ReturnType<typeof useChat>;
  serverUrl: string;
  tabAccess: TabAccessReply | null;
  coldHint: string;
}>();
const draft = defineModel<string>('draft', { required: true });
const emit = defineEmits<{ error: [error: unknown] }>();

const state = props.chat.state;
const list = ref<HTMLElement | null>(null);
const fail = (e: unknown) => emit('error', e);

const sessionLink = computed(() => sessionUrl(props.serverUrl, state.sessionId ?? ''));

// The reducer returns a new message object on every event, so identity is the change signal.
const cache = new WeakMap<UiMessage, string>();
const rendered = shallowRef(new Map<string, string>());
let frame = 0;

function flush() {
  frame = 0;
  const el = list.value;
  const stick = !el || el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  const next = new Map<string, string>();
  for (const m of state.messages) {
    if (m.role !== 'assistant') continue;
    let html = cache.get(m);
    if (html === undefined) cache.set(m, (html = renderMarkdown(inlineCitations(m.markdown, m.references, `cite-${m.id}`))));
    next.set(m.id, html);
  }
  rendered.value = next;
  void nextTick(() => {
    resolveImages();
    if (stick && list.value) list.value.scrollTop = list.value.scrollHeight;
  });
}

// A streaming answer re-renders at most once per frame; a finished one renders at once.
watch(
  () => [...state.messages],
  () => {
    if (state.messages.at(-1)?.done === false) {
      frame ||= requestAnimationFrame(flush);
      return;
    }
    cancelAnimationFrame(frame);
    flush();
  },
  { immediate: true },
);

// One fetch per storage ref, shared by every re-render of the same answer.
const images = new Map<string, Promise<string | null>>();
function imageUrl(ref: string): Promise<string | null> {
  let url = images.get(ref);
  if (!url) {
    url = props.chat.api.fetchFile(ref).then((blob) => (blob ? URL.createObjectURL(blob) : null), () => null);
    images.set(ref, url);
  }
  return url;
}

function resolveImages() {
  for (const img of list.value?.querySelectorAll<HTMLImageElement>('img[data-storage-ref]') ?? []) {
    const ref = img.dataset.storageRef!;
    img.removeAttribute('data-storage-ref');
    void imageUrl(ref).then((url) => {
      if (url) img.src = url;
      else img.replaceWith(Object.assign(document.createElement('span'), { className: 'image-unavailable', textContent: 'image unavailable' }));
    });
  }
}

function onAnswerClick(event: MouseEvent) {
  const button = (event.target as Element).closest<HTMLButtonElement>('button.aikb-remote-image');
  if (!button?.dataset.src) return;
  button.replaceWith(Object.assign(document.createElement('img'), { src: button.dataset.src, alt: '' }));
}

function ask(query: string) {
  const q = query.trim();
  if (!q || state.streaming) return;
  emit('error', null);
  if (q === draft.value.trim()) draft.value = '';
  props.chat.ask(q).catch((e: unknown) => {
    if (!draft.value) draft.value = q;
    fail(e);
  });
}

function onEnter(event: KeyboardEvent) {
  if (event.isComposing) return;
  event.preventDefault();
  ask(draft.value);
}

function askAboutPage() {
  const page = props.tabAccess;
  if (page && 'url' in page) draft.value = `About ${page.title} (${page.url}): `;
}

const stop = () => void props.chat.stop().catch(fail);
const loadOlder = () => void props.chat.loadOlder().catch(fail);

onUnmounted(() => {
  cancelAnimationFrame(frame);
  for (const url of images.values()) void url.then((u) => u && URL.revokeObjectURL(u));
});
</script>

<template>
  <div class="view">
    <div ref="list" class="messages" @click="onAnswerClick">
      <button v-if="state.messages[0]?.createdAt" type="button" class="older" @click="loadOlder">Load older</button>

      <article v-for="m in state.messages" :key="m.id" class="message" :class="m.role">
        <p v-if="m.role === 'user'" class="text">{{ m.markdown }}</p>
        <template v-else>
          <template v-for="card in m.cards" :key="card.kind + card.id">
            <details v-if="card.kind === 'thinking'" class="card">
              <summary>Thinking <span class="status muted">{{ STATUS_LABEL[card.status] }}</span></summary>
              <p class="text">{{ card.text }}</p>
            </details>
            <div v-else-if="card.kind === 'tool'" class="card">
              {{ card.name }} <span class="status muted" :class="card.status">{{ STATUS_LABEL[card.status] }}</span>
            </div>
            <div v-else class="card approval">
              {{ card.text }}
              <a :href="sessionLink" target="_blank" rel="noopener">Open in the web UI</a>
            </div>
          </template>
          <div class="answer markdown" v-html="rendered.get(m.id) ?? ''"></div>
          <p v-if="!m.done && !m.markdown && !m.cards.length" class="muted">Thinking...</p>
          <p v-if="m.error" class="error">{{ m.error }}</p>
          <ol v-if="m.references.length" class="citations">
            <li v-for="(r, i) in m.references" :id="`cite-${m.id}-${i + 1}`" :key="r.id">
              <a v-if="r.knowledge_base_id" :href="kbUrl(serverUrl, r.knowledge_base_id)" target="_blank" rel="noopener">{{ r.knowledge_title }}</a>
              <span v-else>{{ r.knowledge_title }}</span>
            </li>
          </ol>
        </template>
      </article>

      <div v-if="!state.messages.length && state.suggestions.length" class="chips">
        <button v-for="s in state.suggestions" :key="s" type="button" class="chip" @click="ask(s)">{{ s }}</button>
      </div>
    </div>

    <div class="composer">
      <button v-if="tabAccess && 'url' in tabAccess" type="button" class="chip" @click="askAboutPage">Ask about this page</button>
      <p v-else-if="tabAccess" class="muted">{{ coldHint }}</p>
      <label class="field">
        <span>Question</span>
        <textarea v-model="draft" rows="3" placeholder="Ask a question" @keydown.enter.exact="onEnter"></textarea>
      </label>
      <p class="hint">Enter sends, Shift+Enter adds a line</p>
      <div class="actions">
        <button v-if="state.streaming" type="button" @click="stop">Stop</button>
        <button type="button" class="primary" :disabled="state.streaming || !draft.trim()" @click="ask(draft)">Send</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: var(--aikb-space-2);
}
.messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: var(--aikb-space-3);
}
.older {
  align-self: center;
}
.message {
  display: grid;
  gap: var(--aikb-space-2);
  overflow-wrap: anywhere;
}
.message.user {
  align-self: flex-end;
  padding: var(--aikb-space-2) var(--aikb-space-3);
  border-radius: var(--aikb-radius-medium);
  background: var(--aikb-bg-secondary);
}
.text {
  margin: 0;
  white-space: pre-wrap;
}
.card {
  padding: var(--aikb-space-1) var(--aikb-space-2);
  border: 1px solid var(--aikb-border);
  border-radius: var(--aikb-radius-medium);
  background: var(--aikb-bg);
}
.card summary {
  cursor: pointer;
}
.approval {
  border-color: var(--aikb-warning);
  background: var(--aikb-warning-soft);
}
.approval a,
.citations a {
  color: var(--aikb-brand);
}
.status.failed,
.error {
  margin: 0;
  color: var(--aikb-error);
}
.answer :deep(.image-unavailable) {
  color: var(--aikb-text-secondary);
}
.citations {
  margin: 0;
  padding-left: var(--aikb-space-5);
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: var(--aikb-space-1);
}
.chip {
  border-radius: var(--aikb-radius-medium);
  text-align: left;
}
.composer {
  display: grid;
  gap: var(--aikb-space-2);
}
.composer .chip {
  justify-self: start;
}
.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--aikb-space-2);
}
</style>
