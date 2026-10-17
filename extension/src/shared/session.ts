import type { Capture, CaptureDraft, Intent } from './types';

// Per-tab scratch state lives in storage.session, so it dies with the browser; recent captures persist.
async function read<T>(key: string): Promise<T | null> {
  const stored = await chrome.storage.session.get(key);
  return (stored[key] as T | undefined) ?? null;
}

async function write(key: string, value: unknown): Promise<void> {
  if (value === null) await chrome.storage.session.remove(key);
  else await chrome.storage.session.set({ [key]: value });
}

export const readDraft = (tabId: number) => read<CaptureDraft>(`draft:${tabId}`);
export const writeDraft = (tabId: number, draft: CaptureDraft | null) => write(`draft:${tabId}`, draft);
export const readIntent = (tabId: number) => read<Intent>(`intent:${tabId}`);
export const writeIntent = (tabId: number, intent: Intent | null) => write(`intent:${tabId}`, intent);
export const readComposer = (tabId: number) => read<string>(`composer:${tabId}`);
export const writeComposer = (tabId: number, text: string | null) => write(`composer:${tabId}`, text);

export function onSessionChange(callback: (key: string) => void): () => void {
  const listener = (changes: Record<string, chrome.storage.StorageChange>, area: string) => {
    if (area !== 'session') return;
    for (const key of Object.keys(changes)) callback(key);
  };
  chrome.storage.onChanged.addListener(listener);
  return () => chrome.storage.onChanged.removeListener(listener);
}

export async function readRecent(): Promise<Capture[]> {
  const stored = await chrome.storage.local.get('recent');
  return (stored.recent as Capture[] | undefined) ?? [];
}

export const writeRecent = (list: Capture[]) => chrome.storage.local.set({ recent: list });
