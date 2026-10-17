<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue';
import { MAX_CONTENT_CHARS } from '../shared/capture';
import { renderMarkdown } from '../shared/markdown';
import { kbUrl } from '../shared/serverUrl';
import type { Capture } from '../shared/types';
import KbPicker from '../ui/KbPicker.vue';
import Toast from '../ui/Toast.vue';
import type { useCapture } from './useCapture';

const props = defineProps<{ capture: ReturnType<typeof useCapture>; serverUrl: string }>();

const PARSE_TOAST: Record<Capture['status'], string> = {
  pending: 'Saved, parsing',
  completed: 'Saved',
  failed: 'Parsing failed',
  cancelled: 'Parsing was cancelled',
  timeout: 'Parsing is taking longer than two minutes',
};
const LIMIT = MAX_CONTENT_CHARS.toLocaleString('en-US');

const state = props.capture.state;
const editor = computed(() => (state.editor.phase === 'idle' ? null : state.editor));
const editing = computed(() => (state.editor.phase === 'editing' ? state.editor : null));
const preview = ref(false);
const picker = ref<HTMLElement | null>(null);

const over = computed(() => props.capture.charCount.value > MAX_CONTENT_CHARS);
const limit = computed(() => editing.value?.limit ?? null);
const saveError = computed(() => editing.value?.saveError ?? null);
const cleanedTitle = computed(() => (limit.value?.reason === 'bad_title' && limit.value.cleanedTitle !== editor.value?.draft.title ? limit.value.cleanedTitle : undefined));
const duplicate = computed(() => editing.value?.duplicate);
const duplicateKb = computed(() => state.kbs.find((kb) => kb.id === duplicate.value?.kbId)?.name ?? 'a knowledge base');
// Read only while the preview is shown, so the textarea never renders Markdown on a keystroke.
const previewHtml = computed(() => (editor.value ? renderMarkdown(editor.value.draft.markdown) : ''));
const toast = computed(() => {
  if (state.editor.phase !== 'saved') return null;
  const last = state.editor.lastStatus ?? { status: 'pending' as const };
  if (last.status !== 'failed') return { kind: 'info' as const, message: PARSE_TOAST[last.status] };
  return { kind: 'error' as const, message: `${PARSE_TOAST.failed}: ${last.message ?? 'unknown error'}` };
});

watch(saveError, (error) => {
  if (error?.field === 'kb') void nextTick(() => picker.value?.querySelector('select')?.focus());
});

const save = (asNew: boolean) => void props.capture.save(asNew);
</script>

<template>
  <div v-if="editor" class="editor">
    <Toast v-if="toast && editor.phase === 'saved' && editor.quick" :kind="toast.kind" :message="toast.message">
      <button type="button" @click="capture.editCapture(editor.existing)">Edit</button>
      <button type="button" @click="capture.cancel()">Done</button>
    </Toast>

    <fieldset v-else :disabled="editor.phase === 'saving'">
      <Toast v-if="saveError" kind="error" :message="saveError.text" />
      <input v-model="editor.draft.title" aria-label="Title" placeholder="Title" />
      <button v-if="cleanedTitle !== undefined" type="button" class="offer" @click="editor.draft.title = cleanedTitle">Use cleaned title</button>
      <p v-if="editor.draft.url" class="muted url">{{ editor.draft.url }}</p>
      <div ref="picker">
        <KbPicker
          aria-label="Knowledge base"
          :kbs="state.kbs"
          :disabled="capture.lockedKbId.value !== null"
          :model-value="capture.lockedKbId.value ?? state.kbId ?? ''"
          @update:model-value="state.kbId = $event || null"
        />
      </div>

      <div class="body-head">
        <button type="button" :class="{ active: preview }" :aria-pressed="preview" @click="preview = !preview">Preview</button>
        <span class="count" :class="{ over }">{{ capture.charCount.value.toLocaleString('en-US') }} / {{ LIMIT }}</span>
      </div>
      <div v-if="preview" class="preview markdown" v-html="previewHtml"></div>
      <textarea v-else v-model="editor.draft.markdown" rows="14" aria-label="Markdown"></textarea>
      <p class="muted note">HTML forms, scripts and embedded frames are removed by the server</p>
      <p v-if="limit?.reason === 'too_short'" class="error">There is too little content to save</p>

      <p v-if="duplicate" class="notice">
        Already saved to {{ duplicateKb }} on {{ new Date(duplicate.savedAt).toLocaleDateString() }}
        <a :href="kbUrl(serverUrl, duplicate.kbId)" target="_blank" rel="noopener">Open</a>
      </p>

      <div class="actions">
        <button type="button" @click="capture.cancel()">Cancel</button>
        <button v-if="over && editing" type="button" class="primary" @click="capture.trim()">Trim to the limit</button>
        <template v-else-if="duplicate">
          <button type="button" @click="save(false)">Update existing</button>
          <button type="button" class="primary" :disabled="!state.kbId" @click="save(true)">Save as new</button>
        </template>
        <button v-else type="button" class="primary" :disabled="!state.kbId && !editor.existing" @click="save(false)">Save</button>
      </div>

      <Toast v-if="toast" :kind="toast.kind" :message="toast.message" />
    </fieldset>
  </div>
</template>

<style scoped>
.editor,
fieldset {
  display: grid;
  gap: var(--aikb-space-2);
}
fieldset {
  margin: 0;
  padding: 0;
  border: none;
  min-width: 0;
}
.offer {
  justify-self: start;
}
.url {
  font-size: 12px;
  overflow-wrap: anywhere;
}
.body-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.body-head button.active {
  border-color: var(--aikb-brand);
  color: var(--aikb-brand);
}
.count {
  font-size: 12px;
  color: var(--aikb-text-secondary);
}
.count.over,
.error {
  color: var(--aikb-error);
}
.error {
  margin: 0;
}
textarea {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.preview {
  max-height: 60vh;
  overflow-y: auto;
  padding: var(--aikb-space-2);
  border: 1px solid var(--aikb-border);
  border-radius: var(--aikb-radius);
  background: var(--aikb-bg);
  overflow-wrap: anywhere;
}
.note {
  font-size: 12px;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--aikb-space-2);
}
</style>
