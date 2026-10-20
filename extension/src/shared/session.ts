import type { ApiClient } from './api';
import type { ExtractError } from './messages';
import { resyncCaptures } from './capture';
import { loadConnection } from './connection';
import type { Capture, CaptureDraft, Connection, Intent } from './types';

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

export const captureScope = (c: Pick<Connection, 'serverUrl' | 'tenantId'>): string => JSON.stringify([c.serverUrl, c.tenantId]);

async function currentScope(): Promise<string | null> {
  const connection = await loadConnection();
  return connection ? captureScope(connection) : null;
}

async function storedCaptures(): Promise<Capture[]> {
  const stored = await chrome.storage.local.get(null);
  return Object.entries(stored)
    .filter(([key]) => key.startsWith(CAPTURE))
    .map(([, capture]) => capture as Capture);
}

// Only the current connection's captures, newest first, so a server switch never shows or resyncs another server's ids.
const inScope = (list: Capture[], scope: string): Capture[] =>
  list.filter((c) => c.scope === scope).sort((a, b) => b.savedAt.localeCompare(a.savedAt));

export async function readRecent(): Promise<Capture[]> {
  const scope = await currentScope();
  return scope === null ? [] : inScope(await storedCaptures(), scope).slice(0, RECENT_LIMIT);
}

// Keeps the scope each entry was stored under, so a poll or resync that finishes after a switch cannot re-home a capture.
export async function updateCaptures(list: Capture[]): Promise<void> {
  if (list.length === 0) return;
  const keys = list.map((c) => CAPTURE + c.knowledgeId);
  const stored = await chrome.storage.local.get(keys);
  const entries = list.map((c, i) => [keys[i], { ...c, scope: (stored[keys[i]] as Capture | undefined)?.scope ?? c.scope }]);
  await chrome.storage.local.set(Object.fromEntries(entries));
}

// The only place a scope is stamped. Captures saved before scopes existed are pruned here.
export async function rememberCapture(capture: Capture): Promise<void> {
  const scope = await currentScope();
  if (scope === null) return;
  await chrome.storage.local.set({ [CAPTURE + capture.knowledgeId]: { ...capture, scope } });
  const all = await storedCaptures();
  const stale = [...inScope(all, scope).slice(RECENT_LIMIT), ...all.filter((c) => c.scope === undefined)];
  if (stale.length > 0) await chrome.storage.local.remove(stale.map((c) => CAPTURE + c.knowledgeId));
}

// resyncCaptures returns unchanged captures as the same objects, so only the changed ones are written back.
// A switch during the resync means the statuses came from the wrong server, so they are dropped.
export async function resyncRecent(api: ApiClient, resync = resyncCaptures): Promise<Capture[]> {
  const scope = await currentScope();
  const before = await readRecent();
  const synced = await resync(api, before);
  if ((await currentScope()) !== scope) return before;
  await updateCaptures(synced.filter((c, i) => c !== before[i]));
  return synced;
}
