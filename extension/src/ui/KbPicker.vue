<script setup lang="ts">
import type { KnowledgeBase } from '../shared/types';

const props = defineProps<{ kbs: KnowledgeBase[]; modelValue: string | string[]; multiple?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: string | string[]] }>();

function onChange(event: Event) {
  const select = event.target as HTMLSelectElement;
  emit('update:modelValue', props.multiple ? Array.from(select.selectedOptions, (o) => o.value) : select.value);
}
</script>

<template>
  <select class="kb-picker" :multiple="multiple" :value="multiple ? undefined : modelValue" @change="onChange">
    <option v-if="!multiple" value="" disabled>Choose a knowledge base</option>
    <option v-for="kb in kbs" :key="kb.id" :value="kb.id" :selected="multiple ? modelValue.includes(kb.id) : undefined">{{ kb.name }}</option>
  </select>
</template>

<style scoped>
.kb-picker {
  width: 100%;
}
</style>
