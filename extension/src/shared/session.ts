import type { ApiClient } from './api';
import type { ExtractError } from './messages';
import { resyncCaptures } from './capture';
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
// The page the error describes; the panel drops it once the tab shows another.
export type ClipError = ExtractError & { url: string | null };
export const readClipError = (tabId: number) => read<ClipError>(`clipError:${tabId}`);
export const writeClipError = (tabId: number, error: ClipError) => write(`clipError:${tabId}`, error);
export const TAB_KEYS = ['draft', 'intent', 'clipError'] as const;
export type TabKey = (typeof TAB_KEYS)[number];
export const clearTabKeys = (tabId: number, names: TabKey[]) => chrome.storage.session.remove(names.map((name) => `${name}:${tabId}`));

// Removals are skipped: they are a page clearing what it already took.
export function onSessionChange(callback: (key: string) => void): () => void {
  const listener = (changes: Record<string, chrome.storage.StorageChange>, area: string) => {
    if (area !== 'session') return;
    for (const [key, change] of Object.entries(changes)) if (change.newValue !== undefined) callback(key);
  };
  chrome.storage.onChanged.addListener(listener);
  return () => chrome.storage.onChanged.removeListener(listener);
}

// One storage write, so a panel never sees the draft without the intent it was clipped under.
export const writeClip = (tabId: number, intent: Intent, draft: CaptureDraft) =>
  chrome.storage.session.set({ [`intent:${tabId}`]: intent, [`draft:${tabId}`]: draft });

// One key per capture, so two pages saving at once never overwrite each other's list.
const RECENT_LIMIT = 50;
const CAPTURE = 'capture:';

async function allCaptures(): Promise<Capture[]> {
  const stored = await chrome.storage.local.get(null);
  return Object.entries(stored)
    .filter(([key]) => key.startsWith(CAPTURE))
    .map(([, capture]) => capture as Capture)
    .sort((a, b) => b.savedAt.localeCompare(a.savedAt));
}

export const readRecent = async (): Promise<Capture[]> => (await allCaptures()).slice(0, RECENT_LIMIT);

export async function updateCaptures(list: Capture[]): Promise<void> {
  if (list.length > 0) await chrome.storage.local.set(Object.fromEntries(list.map((c) => [CAPTURE + c.knowledgeId, c])));
}

export async function rememberCapture(capture: Capture): Promise<void> {
  await updateCaptures([capture]);
  const stale = (await allCaptures()).slice(RECENT_LIMIT).map((c) => CAPTURE + c.knowledgeId);
  if (stale.length > 0) await chrome.storage.local.remove(stale);
}

// resyncCaptures returns unchanged captures as the same objects, so only the changed ones are written back.
export async function resyncRecent(api: ApiClient, resync = resyncCaptures): Promise<Capture[]> {
  const before = await readRecent();
  const synced = await resync(api, before);
  await updateCaptures(synced.filter((c, i) => c !== before[i]));
  return synced;
}
