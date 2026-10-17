import { reactive } from 'vue';
import { ApiError, errorMessage, type ApiClient } from '../shared/api';
import { writableKnowledgeBases } from '../shared/capture';
import type { CommandsReply, Request } from '../shared/messages';
import { normalizeServerUrl } from '../shared/serverUrl';
import type { Connection, KnowledgeBase, Me, Membership, OidcConfig } from '../shared/types';
import { OidcError, type useOidc } from './useOidc';

export type Mode = 'api_key' | 'password' | 'oidc';
export type Status = 'idle' | 'checking' | 'connected' | 'error' | 'signed_out';

const INVALID_URL_TEXT = 'Enter a valid server URL';
const REJECTED_TEXT = 'The server rejected this key or sign-in';
const OIDC_TEXT: Record<string, string> = {
  cancelled: 'Sign-in was cancelled',
  not_allowed: 'This server does not allow extension sign-in yet',
};

type Candidate = { auth: Connection['auth']; tenantId: number | null; memberships: Membership[] };
type Probes = Awaited<ReturnType<ApiClient['probeCapabilities']>>;

type Deps = {
  api: ApiClient;
  send: <T>(request: Request) => Promise<T>;
  oidc: Pick<ReturnType<typeof useOidc>, 'signIn' | 'returnTo'>;
  loadConnection: () => Promise<Connection | null>;
  commands: () => Promise<CommandsReply>;
};

const blank = () => ({
  serverUrl: '',
  insecure: false,
  mode: 'api_key' as Mode,
  apiKey: '',
  email: '',
  password: '',
  tenantId: null as number | null,
  memberships: [] as Membership[],
  needsTenant: false,
  lastKbId: null as string | null,
  quickClip: false,
  kbs: [] as KnowledgeBase[],
  status: 'idle' as Status,
  error: '',
  errorCode: null as string | null,
  probes: null as Probes | null,
  oidc: null as OidcConfig | null,
  me: null as Me | null,
  commands: [] as CommandsReply,
});

const fromTokens = (r: { token: string; refresh_token: string; memberships?: Membership[] }, tenant?: { id: number }): Candidate => ({
  auth: { kind: 'bearer', token: r.token, refreshToken: r.refresh_token },
  tenantId: tenant?.id ?? null,
  memberships: r.memberships ?? [],
});

function errorText(error: unknown): string {
  if (error instanceof OidcError) return OIDC_TEXT[error.code] ?? error.description ?? `Sign-in failed (${error.code})`;
  return errorMessage(error);
}

export function useConnection(deps: Deps) {
  const { api, send, oidc } = deps;
  const state = reactive(blank());

  // v-model.number yields '' for an emptied field.
  const chosenTenant = () => (Number.isInteger(state.tenantId) ? state.tenantId : null);

  const signIns: Record<Mode, (serverUrl: string) => Promise<Candidate>> = {
    api_key: async () => ({ auth: { kind: 'api_key', key: state.apiKey.trim() }, tenantId: chosenTenant(), memberships: [] }),
    password: async (serverUrl) => {
      const r = await api.login(serverUrl, state.email.trim(), state.password);
      state.password = '';
      return fromTokens(r, r.active_tenant);
    },
    oidc: async (serverUrl) => {
      const r = await oidc.signIn(serverUrl);
      return fromTokens(r, r.tenant);
    },
  };

  // A sign-in waiting on the workspace choice is reused, so picking one does not run the IdP flow again.
  let pending: { key: string; candidate: Candidate } | null = null;

  function fail(error: unknown): void {
    state.status = 'error';
    state.error = errorText(error);
    state.errorCode = error instanceof OidcError ? error.code : null;
  }

  function applyServerUrl(input: string): string | null {
    const normalized = normalizeServerUrl(input);
    if (!normalized.ok) return null;
    state.serverUrl = normalized.url;
    state.insecure = normalized.warning === 'insecure';
    state.error = '';
    state.errorCode = null;
    return normalized.url;
  }

  async function loadAccount(kind: Connection['auth']['kind']): Promise<void> {
    // me() goes first so a 409 (platform key without a workspace) is the error connect() sees.
    state.me = await api.me();
    const [probes, own, shared] = await Promise.all([
      kind === 'api_key' ? api.probeCapabilities() : null,
      api.listKnowledgeBases(),
      api.listSharedKnowledgeBases(),
    ]);
    state.probes = probes;
    state.kbs = writableKnowledgeBases(own, shared);
    state.status = 'connected';
  }

  async function checkServer(serverUrl: string): Promise<void> {
    const url = applyServerUrl(serverUrl);
    if (!url) {
      state.oidc = null;
      state.error = INVALID_URL_TEXT;
      return;
    }
    // A server without the OIDC routes, or one that is unreachable, just hides the third tab.
    state.oidc = await api.oidcConfig(url).catch(() => null);
    if (state.mode === 'oidc' && !state.oidc?.enabled) state.mode = 'api_key';
  }

  async function connect(): Promise<void> {
    const previous = await deps.loadConnection();
    const serverUrl = applyServerUrl(state.serverUrl || previous?.serverUrl || '');
    if (!serverUrl) return fail(new Error(INVALID_URL_TEXT));
    state.status = 'checking';
    try {
      const key = `${state.mode} ${serverUrl} ${state.mode === 'password' ? state.email.trim() : state.apiKey.trim()}`;
      const candidate = pending?.key === key ? pending.candidate : await signIns[state.mode](serverUrl);
      const { memberships } = candidate;
      state.memberships = memberships;
      const chosen = memberships.some((m) => m.tenant_id === chosenTenant()) ? chosenTenant() : null;
      if (memberships.length > 1 && chosen === null) {
        pending = { key, candidate };
        state.needsTenant = true;
        state.status = 'idle';
        return;
      }
      pending = null;
      state.needsTenant = false;
      const tenantId = chosen ?? candidate.tenantId ?? (memberships.length === 1 ? memberships[0].tenant_id : null);
      const samePrefs = previous?.serverUrl === serverUrl;
      const connection: Connection = {
        serverUrl,
        auth: candidate.auth,
        tenantId,
        lastKbId: samePrefs ? previous.lastKbId : null,
        quickClip: samePrefs ? previous.quickClip : state.quickClip,
      };
      await send({ type: 'SET_CONNECTION', connection });
      state.tenantId = tenantId;
      state.lastKbId = connection.lastKbId;
      state.quickClip = connection.quickClip;
      try {
        await loadAccount(candidate.auth.kind);
      } catch (error) {
        // The other pages keep working on the old session while the new one is rejected.
        if (previous && previous.auth.kind !== 'signed_out') await send({ type: 'SET_CONNECTION', connection: previous });
        throw error;
      }
    } catch (error) {
      // A platform key answers 409 until a workspace is chosen.
      if (error instanceof ApiError && error.status === 409 && state.mode === 'api_key') {
        state.needsTenant = true;
        state.status = 'idle';
        return;
      }
      fail(error instanceof ApiError && error.status === 401 ? new Error(REJECTED_TEXT) : error);
    }
  }

  async function load(): Promise<void> {
    const [commands, connection] = await Promise.all([deps.commands(), deps.loadConnection()]);
    state.commands = commands;
    if (!connection) return;
    state.lastKbId = connection.lastKbId;
    state.quickClip = connection.quickClip;
    state.tenantId = connection.tenantId;
    if (connection.auth.kind === 'signed_out') {
      await checkServer(connection.serverUrl);
      state.status = 'signed_out';
      return;
    }
    state.status = 'checking';
    const signedOut = (error: unknown) => {
      if (error instanceof ApiError && error.status === 401) state.status = 'signed_out';
      else fail(error);
    };
    await Promise.all([checkServer(connection.serverUrl), loadAccount(connection.auth.kind).catch(signedOut)]);
  }

  async function setLastKb(id: string): Promise<void> {
    await send({ type: 'PATCH_CONNECTION', patch: { lastKbId: id } });
    state.lastKbId = id;
  }

  async function setQuickClip(on: boolean): Promise<void> {
    await send({ type: 'PATCH_CONNECTION', patch: { quickClip: on } });
    state.quickClip = on;
  }

  // Signing out keeps the server and the form's mode and email; forgetting the server keeps only the shortcuts.
  async function signOut(forgetServer: boolean): Promise<void> {
    await send({ type: 'CLEAR_CONNECTION', forgetServer });
    pending = null;
    const { commands, mode, email, serverUrl, insecure, oidc: oidcConfig, tenantId, lastKbId, quickClip } = state;
    Object.assign(state, blank(), { commands, mode, email });
    if (!forgetServer) Object.assign(state, { serverUrl, insecure, oidc: oidcConfig, tenantId, lastKbId, quickClip, status: 'signed_out' });
  }

  return { state, fail, checkServer, connect, load, setLastKb, setQuickClip, signOut };
}
