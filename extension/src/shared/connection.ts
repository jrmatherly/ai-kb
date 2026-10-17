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
