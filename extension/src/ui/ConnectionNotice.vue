<script setup lang="ts">
import type { HeadersReply } from '../shared/messages';

defineProps<{ headers: HeadersReply | undefined }>();
const openSettings = () => void chrome.runtime.openOptionsPage();
</script>

<template>
  <p v-if="headers === null" class="signed-out">
    Connect to a server in Settings
    <a href="#" @click.prevent="openSettings">Settings</a>
  </p>
  <p v-else-if="headers?.authKind === 'signed_out'" class="signed-out">
    Signed out elsewhere. Sign in again.
    <a href="#" @click.prevent="openSettings">Settings</a>
  </p>
</template>
