import type { Connection } from './types';

const KEY = 'connection';

export async function loadConnection(): Promise<Connection | null> {
  const stored = await chrome.storage.local.get(KEY);
  return (stored[KEY] as Connection | undefined) ?? null;
}

export async function saveConnection(connection: Connection | null): Promise<void> {
  if (connection) await chrome.storage.local.set({ [KEY]: connection });
  else await chrome.storage.local.remove(KEY);
}

// Storage change values are untyped; a removed key arrives as undefined.
const identity = (value: unknown) => {
  const c = value as Connection | undefined;
  return c ? JSON.stringify([c.serverUrl, c.auth?.kind, c.tenantId]) : '';
};

// Token refreshes and preference patches rewrite the connection too; only who is connected where is reported.
export function onIdentityChange(callback: () => void): () => void {
  const listener = (changes: Record<string, chrome.storage.StorageChange>, area: string) => {
    const change = changes[KEY];
    if (area === 'local' && change && identity(change.oldValue) !== identity(change.newValue)) callback();
  };
  chrome.storage.onChanged.addListener(listener);
  return () => chrome.storage.onChanged.removeListener(listener);
}
