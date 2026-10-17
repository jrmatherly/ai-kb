<script setup lang="ts" generic="T extends string | string[]">
import { computed } from 'vue';
import type { KnowledgeBase } from '../shared/types';

const props = defineProps<{ kbs: KnowledgeBase[]; modelValue: T; multiple?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: T] }>();

const model = computed({ get: () => props.modelValue, set: (v: T) => emit('update:modelValue', v) });
</script>

<template>
  <select v-model="model" class="kb-picker" :multiple="multiple">
    <option v-if="!multiple" value="" disabled>Choose a knowledge base</option>
    <option v-for="kb in kbs" :key="kb.id" :value="kb.id">{{ kb.name }}</option>
  </select>
</template>

<style scoped>
.kb-picker {
  width: 100%;
}
</style>
