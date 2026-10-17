import type { RefreshOutcome } from '../shared/messages';
import { apiBase } from '../shared/serverUrl';
import type { Connection, RefreshResponse } from '../shared/types';

const KEY = 'connection';

export async function loadConnection(): Promise<Connection | null> {
  const stored = await chrome.storage.local.get(KEY);
  return (stored[KEY] as Connection | undefined) ?? null;
}

export async function saveConnection(connection: Connection | null): Promise<void> {
  if (connection) await chrome.storage.local.set({ [KEY]: connection });
  else await chrome.storage.local.remove(KEY);
}

type Store = { load: () => Promise<Connection | null>; save: (connection: Connection | null) => Promise<void> };
const localStore: Store = { load: loadConnection, save: saveConnection };

// Reads and writes of the connection run one at a time, so a slow read cannot be saved back over a
// refresh that finished meanwhile.
let queue: Promise<unknown> = Promise.resolve();
function serialized<T>(work: () => Promise<T>): Promise<T> {
  const run = queue.then(work, work);
  queue = run.catch(() => undefined);
  return run;
}

// Every partial write goes through here so a page's stale copy never overwrites rotated tokens.
export function patchConnection(patch: Partial<Omit<Connection, 'auth'>>, store: Store = localStore): Promise<Connection | null> {
  return serialized(async () => {
    const current = await store.load();
    if (!current) return null;
    const next = { ...current, ...patch };
    await store.save(next);
    return next;
  });
}

export function replaceConnection(connection: Connection, store: Store = localStore): Promise<void> {
  return serialized(() => store.save(connection));
}

// Signing out keeps the server and preferences unless the server is forgotten too.
export function signOut(forgetServer: boolean, store: Store = localStore): Promise<void> {
  return serialized(async () => {
    const current = await store.load();
    await store.save(forgetServer || !current ? null : { ...current, auth: { kind: 'signed_out' } });
  });
}

export function authHeaders(connection: Connection): Record<string, string> {
  const headers: Record<string, string> = {};
  const { auth } = connection;
  if (auth.kind === 'api_key') headers['X-API-Key'] = auth.key;
  else if (auth.kind === 'bearer') headers.Authorization = `Bearer ${auth.token}`;
  if (connection.tenantId != null) headers['X-Tenant-ID'] = String(connection.tenantId);
  return headers;
}

type RefresherDeps = Store & { fetchFn: typeof fetch };

// The refresh token rotates, so two pages hitting a 401 together must share one request.
export function createRefresher(deps: RefresherDeps): () => Promise<RefreshOutcome> {
  let inFlight: Promise<RefreshOutcome> | null = null;

  // Only auth is written back, and only while the stored session is still the one the refresh
  // started from: the options page may have changed preferences or signed in again meanwhile.
  function setAuth(startedFrom: string, auth: Connection['auth']): Promise<void> {
    return serialized(async () => {
      const current = await deps.load();
      if (current?.auth.kind === 'bearer' && current.auth.refreshToken === startedFrom) await deps.save({ ...current, auth });
    });
  }

  async function refresh(): Promise<RefreshOutcome> {
    const connection = await deps.load();
    if (!connection || connection.auth.kind !== 'bearer') return 'not_bearer';
    const startedFrom = connection.auth.refreshToken;
    const response = await deps.fetchFn(`${apiBase(connection.serverUrl)}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken: startedFrom }),
    });
    if (response.status === 401 || response.status === 403) {
      await setAuth(startedFrom, { kind: 'signed_out' });
      return 'signed_out';
    }
    if (!response.ok) throw new Error(`refresh failed with ${response.status}`);
    const body = (await response.json()) as Partial<RefreshResponse>;
    if (!body.success || !body.access_token || !body.refresh_token) throw new Error('refresh returned no tokens');
    await setAuth(startedFrom, { kind: 'bearer', token: body.access_token, refreshToken: body.refresh_token });
    return 'refreshed';
  }

  return () => {
    if (!inFlight) inFlight = refresh().finally(() => { inFlight = null; });
    return inFlight;
  };
}
