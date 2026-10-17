import type { CommandsReply, ExtractMode, ExtractReply, ExtractResult, HeadersReply, RefreshOutcome, Request, TabAccessReply } from '../shared/messages';
import type { ClipError } from '../shared/session';
import type { CaptureDraft, Connection, Intent } from '../shared/types';
import { authHeaders, patchConnection, replaceConnection, signOut } from './auth';

export type Deps = {
  load: () => Promise<Connection | null>;
  save: (connection: Connection | null) => Promise<void>;
  refresh: () => Promise<RefreshOutcome>;
  tabInfo: (tabId: number) => Promise<{ url?: string; title?: string }>;
  inject: (tabId: number, mode: ExtractMode) => Promise<ExtractResult>;
  commands: () => Promise<CommandsReply>;
  openPanel: (tabId: number) => Promise<void>;
  writeIntent: (tabId: number, intent: Intent | null) => Promise<void>;
  readIntent: (tabId: number) => Promise<Intent | null>;
  writeClip: (tabId: number, intent: Intent, draft: CaptureDraft) => Promise<void>;
  writeComposer: (tabId: number, text: string | null) => Promise<void>;
  writeClipError: (tabId: number, error: ClipError) => Promise<void>;
  pageSelection: (tabId: number) => Promise<string>;
  extensionId: string;
  extensionOrigin: string;
};

const UNREACHABLE_SCHEMES = new Set(['chrome:', 'chrome-extension:', 'chrome-untrusted:', 'chrome-search:', 'edge:', 'about:', 'file:', 'view-source:', 'devtools:', 'data:', 'blob:', 'javascript:']);

// Pages where executeScript is refused or useless, decided by URL rather than by the error text.
export function isUnreachableUrl(url: string): boolean {
  try {
    const parsed = new URL(url);
    return UNREACHABLE_SCHEMES.has(parsed.protocol) || parsed.hostname === 'chromewebstore.google.com' || parsed.pathname.toLowerCase().endsWith('.pdf');
  } catch {
    return true;
  }
}

export function quickClipIntent(connection: Connection | null, mode: ExtractMode): Intent {
  return connection?.quickClip && connection.lastKbId ? `quick:${mode}` : `clip:${mode}`;
}

const isAccessRefusal = (error: unknown) => error instanceof Error && /Cannot access|cannot be scripted/i.test(error.message);

async function extract(tabId: number, mode: ExtractMode, deps: Deps): Promise<ExtractReply> {
  const tab = await deps.tabInfo(tabId);
  if (tab.url && isUnreachableUrl(tab.url)) return { error: 'unreachable' };
  try {
    return await deps.inject(tabId, mode);
  } catch (error) {
    if (isAccessRefusal(error)) return { error: 'no_access' };
    console.error('extraction failed', error);
    return { error: 'failed', message: error instanceof Error ? error.message : String(error) };
  }
}

export async function handle(request: Request, sender: { id?: string; url?: string }, deps: Deps): Promise<unknown> {
  if (sender.id !== deps.extensionId || !sender.url?.startsWith(deps.extensionOrigin)) throw new Error('forbidden sender');
  switch (request.type) {
    case 'GET_HEADERS': {
      const connection = await deps.load();
      return (connection ? { serverUrl: connection.serverUrl, headers: authHeaders(connection), authKind: connection.auth.kind } : null) satisfies HeadersReply;
    }
    case 'REFRESH':
      return deps.refresh();
    case 'SET_CONNECTION':
      await replaceConnection(request.connection, deps);
      return null;
    case 'PATCH_CONNECTION':
      return patchConnection(request.patch, deps);
    case 'CLEAR_CONNECTION':
      await signOut(request.forgetServer, deps);
      return null;
    case 'TAB_ACCESS': {
      const tab = await deps.tabInfo(request.tabId);
      return (tab.url ? { url: tab.url, title: tab.title ?? '' } : { access: false }) satisfies TabAccessReply;
    }
    case 'EXTRACT':
      return extract(request.tabId, request.mode, deps);
    case 'GET_COMMANDS':
      return deps.commands();
  }
}

// The menu or command gesture is spent by the first await, so the panel opens before any other work.
async function fromGesture(tabId: number, deps: Deps, work: () => Promise<void>): Promise<void> {
  const opened = deps.openPanel(tabId).catch(() => undefined);
  await work();
  await opened;
}

// A draft arrives through storage; when nothing can arrive, the intent is cleared so the panel does not wait,
// and a page that could not be read is reported so the panel shows why.
// A clip intent is written only with its draft: on its own the panel would start a second extraction.
export function clipFromGesture(tabId: number, mode: ExtractMode, deps: Deps): Promise<void> {
  return fromGesture(tabId, deps, async () => {
    const tab = await deps.tabInfo(tabId);
    if (tab.url && isUnreachableUrl(tab.url)) return deps.writeClipError(tabId, { error: 'unreachable', url: tab.url });
    // The extraction is the slow part, so it runs while the intent is decided and written.
    const extracting = extract(tabId, mode, deps);
    const intent = quickClipIntent(await deps.load(), mode);
    const early = intent.startsWith('quick:');
    if (early) await deps.writeIntent(tabId, intent);
    const result = await extracting;
    if ('draft' in result && result.draft) await deps.writeClip(tabId, intent, result.draft);
    else if (early && (await deps.readIntent(tabId)) === intent) await deps.writeIntent(tabId, null);
    if ('error' in result) await deps.writeClipError(tabId, { ...result, url: tab.url ?? null });
  });
}

export function askFromGesture(tabId: number, deps: Deps, selection?: string): Promise<void> {
  return fromGesture(tabId, deps, async () => {
    await deps.writeIntent(tabId, 'ask');
    const text = selection ?? (await deps.pageSelection(tabId).catch(() => ''));
    if (text) await deps.writeComposer(tabId, text);
  });
}

export function noteFromGesture(tabId: number, deps: Deps): Promise<void> {
  return fromGesture(tabId, deps, () => deps.writeIntent(tabId, 'note'));
}
