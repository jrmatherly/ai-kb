import type { Auth, CaptureDraft, Connection } from './types';

export type HeadersReply = { serverUrl: string; headers: Record<string, string>; authKind: Auth['kind'] } | null;
export type RefreshOutcome = 'refreshed' | 'signed_out' | 'not_bearer';
export type ExtractMode = 'article' | 'select';
export type ExtractResult = { draft: CaptureDraft } | { draft: null; reason: 'cancelled' };
export type ExtractReply = ExtractResult | { error: 'unreachable' | 'no_access' } | { error: 'failed'; message: string };
export type TabAccessReply = { url: string; title: string } | { access: false };
export type CommandsReply = Array<{ name: string; shortcut: string }>;

export type Request =
  | { type: 'GET_HEADERS' }
  | { type: 'REFRESH' }
  | { type: 'SET_CONNECTION'; connection: Connection }
  | { type: 'PATCH_CONNECTION'; patch: Partial<Omit<Connection, 'auth'>> }
  | { type: 'CLEAR_CONNECTION'; forgetServer: boolean }
  | { type: 'EXTRACT'; tabId: number; mode: ExtractMode }
  | { type: 'TAB_ACCESS'; tabId: number }
  | { type: 'GET_COMMANDS' };

export async function send<T>(request: Request): Promise<T> {
  const reply = (await chrome.runtime.sendMessage(request)) as T | undefined;
  // A missing listener resolves undefined, which would otherwise read as a sign-out.
  if (reply === undefined) throw new Error(`no reply to ${request.type}`);
  return reply;
}
