<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { loadConnection } from '../shared/connection';
import { send, workerApi, type CommandsReply } from '../shared/messages';
import { normalizeServerUrl } from '../shared/serverUrl';
import KbPicker from '../ui/KbPicker.vue';
import Toast from '../ui/Toast.vue';
import { useConnection, type Mode } from './useConnection';
import { useOidc } from './useOidc';

const DOCS_URL = 'https://ai-kb-docs.matherly.net/docs/05-clients/06-browser-extension';

const api = workerApi();
const oidc = useOidc({
  launch: (details) => chrome.identity.launchWebAuthFlow(details),
  redirectUrl: () => chrome.identity.getRedirectURL('oidc'),
});
const { state, fail, checkServer, connect, load, setLastKb, setQuickClip, signOut } = useConnection({
  api,
  send,
  oidc,
  loadConnection,
  commands: () => send<CommandsReply>({ type: 'GET_COMMANDS' }),
});
const returnTo = oidc.returnTo();

// Chrome ignores a manifest description for the toolbar action, so its label lives here.
const commandLabel = (c: { name: string; description: string }) => c.description || (c.name === '_execute_action' ? 'Open the popup' : c.name);

const tabs = computed(() => {
  const list: Array<{ mode: Mode; label: string }> = [
    { mode: 'api_key', label: 'API key' },
    { mode: 'password', label: 'Email and password' },
  ];
  if (state.oidc?.enabled) list.push({ mode: 'oidc', label: `Sign in with ${state.oidc.provider_display_name ?? 'SSO'}` });
  return list;
});

const hostOrigins = computed(() => {
  const normalized = normalizeServerUrl(state.serverUrl);
  return normalized.ok ? [`${new URL(normalized.url).origin}/*`] : null;
});

// The host prompt needs the click gesture, which the first await would spend.
function onConnect() {
  const origins = hostOrigins.value;
  if (!origins) return void connect();
  chrome.permissions.request({ origins }).then((granted) => (granted ? connect() : fail(new Error('Allow access to the server to connect'))), fail);
}

function copyReturnTo() {
  void navigator.clipboard.writeText(returnTo);
}

function onKbChange(value: string | string[]) {
  if (typeof value === 'string' && value) void setLastKb(value);
}

onMounted(() => void load());
</script>

<template>
  <main class="page">
    <h1>AI KB</h1>

    <p v-if="state.status === 'signed_out'" class="signed-out">Signed out elsewhere. Sign in again.</p>

    <section class="card">
      <label class="field">
        <span>Server URL</span>
        <input v-model="state.serverUrl" type="url" placeholder="https://kb.example.com" @change="checkServer(state.serverUrl)" />
      </label>
      <p v-if="state.insecure" class="warning">Your key, password or sign-in will travel unencrypted</p>

      <div class="tabs" role="tablist">
        <button
          v-for="tab in tabs"
          :key="tab.mode"
          type="button"
          role="tab"
          :aria-selected="state.mode === tab.mode"
          :class="{ active: state.mode === tab.mode }"
          @click="state.mode = tab.mode"
        >{{ tab.label }}</button>
      </div>

      <div v-if="state.mode === 'api_key'" class="panel">
        <label class="field">
          <span>API key</span>
          <input v-model="state.apiKey" type="password" autocomplete="off" />
        </label>
        <p class="muted">Conversations are shared with everyone who uses this key</p>
      </div>

      <div v-else-if="state.mode === 'password'" class="panel">
        <label class="field">
          <span>Email</span>
          <input v-model="state.email" type="email" autocomplete="username" />
        </label>
        <label class="field">
          <span>Password</span>
          <input v-model="state.password" type="password" autocomplete="current-password" />
        </label>
      </div>

      <div v-else class="panel">
        <label class="field">
          <span>Return URL</span>
          <span class="row">
            <input :value="returnTo" readonly />
            <button type="button" @click="copyReturnTo">Copy</button>
          </span>
        </label>
        <p class="muted">Your server must list this URL in OIDC_AUTH_EXTENSION_RETURN_URLS</p>
      </div>

      <label v-if="state.needsTenant" class="field">
        <span>Workspace</span>
        <select v-if="state.memberships.length" v-model="state.tenantId">
          <option :value="null" disabled>Choose a workspace</option>
          <option v-for="m in state.memberships" :key="m.tenant_id" :value="m.tenant_id">{{ m.tenant_name }}</option>
        </select>
        <input v-else v-model.number="state.tenantId" type="number" min="1" placeholder="Workspace ID" />
      </label>

      <div class="actions">
        <button type="button" class="primary" :disabled="state.status === 'checking'" @click="onConnect">Connect</button>
      </div>

      <Toast v-if="state.error" kind="error" :message="state.error">
        <a v-if="state.errorCode === 'not_allowed'" :href="DOCS_URL" target="_blank" rel="noopener">Set up extension sign-in</a>
      </Toast>
    </section>

    <section v-if="state.status === 'connected'" class="card">
      <h2>Connected</h2>
      <p>{{ state.me?.tenant?.name ?? 'No workspace' }} <span class="muted">{{ state.me?.user.email }}</span></p>
      <ul v-if="state.probes" class="probes">
        <li :class="state.probes.retrieve ? 'ok' : 'bad'">{{ state.probes.retrieve ? 'Can read knowledge bases' : 'Cannot read knowledge bases' }}</li>
        <li :class="state.probes.chat ? 'ok' : 'bad'">{{ state.probes.chat ? 'Can chat' : 'Cannot chat: create a key with the chat capability' }}</li>
      </ul>

      <label class="field">
        <span>Last used knowledge base</span>
        <KbPicker :kbs="state.kbs" :model-value="state.lastKbId ?? ''" @update:model-value="onKbChange" />
      </label>
      <label class="check">
        <input type="checkbox" :checked="state.quickClip" @change="setQuickClip(($event.target as HTMLInputElement).checked)" />
        Quick clip
      </label>
      <p class="muted">Context menu items and the clip shortcuts save straight to the last used knowledge base. The popup and the side panel always open the editor.</p>
    </section>

    <section class="card">
      <h2>Keyboard shortcuts</h2>
      <ul class="shortcuts">
        <li v-for="command in state.commands" :key="command.name">
          <span>{{ commandLabel(command) }}</span>
          <kbd v-if="command.shortcut">{{ command.shortcut }}</kbd>
          <span v-else class="muted">not set</span>
        </li>
      </ul>
    </section>

    <div class="actions">
      <button type="button" @click="signOut(false)">Sign out</button>
      <button type="button" @click="signOut(true)">Forget server</button>
    </div>
  </main>
</template>

<style src="../ui/tokens.css"></style>

<style scoped>
.page {
  max-width: 600px;
  margin: 0 auto;
  padding: var(--aikb-space-5) var(--aikb-space-4);
  display: grid;
  gap: var(--aikb-space-4);
}
h1 {
  margin: 0;
  font-size: 20px;
}
h2 {
  margin: 0 0 var(--aikb-space-2);
  font-size: 16px;
}
.card {
  display: grid;
  gap: var(--aikb-space-3);
  padding: var(--aikb-space-4);
  background: var(--aikb-bg);
  border: 1px solid var(--aikb-border);
  border-radius: var(--aikb-radius-medium);
}
.field {
  display: grid;
  gap: var(--aikb-space-1);
}
.field > span:first-child {
  color: var(--aikb-text-secondary);
}
.row {
  display: flex;
  gap: var(--aikb-space-2);
}
.row input {
  flex: 1;
}
.panel {
  display: grid;
  gap: var(--aikb-space-3);
}
.actions {
  display: flex;
  gap: var(--aikb-space-2);
}
.warning {
  margin: 0;
  color: var(--aikb-warning);
}
.toast a {
  margin-left: var(--aikb-space-2);
  color: var(--aikb-brand);
}
.probes,
.shortcuts {
  margin: 0;
  padding: 0;
  list-style: none;
  display: grid;
  gap: var(--aikb-space-1);
}
.ok {
  color: var(--aikb-brand);
}
.bad {
  color: var(--aikb-error);
}
.shortcuts li {
  display: flex;
  justify-content: space-between;
}
kbd {
  padding: 0 var(--aikb-space-1);
  border: 1px solid var(--aikb-border);
  border-radius: var(--aikb-radius);
  background: var(--aikb-bg-secondary);
  font: inherit;
}
.check {
  display: flex;
  align-items: center;
  gap: var(--aikb-space-2);
}
</style>
