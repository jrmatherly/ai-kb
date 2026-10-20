import { ApiClient } from './api';
import type { Auth, CaptureDraft, Connection } from './types';

export type HeadersReply = { serverUrl: string; headers: Record<string, string>; authKind: Auth['kind'] } | null;
export type RefreshOutcome = 'refreshed' | 'signed_out' | 'not_bearer';
export type ExtractMode = 'article' | 'select';
export type ExtractResult = { draft: CaptureDraft } | { draft: null; reason: 'cancelled' };
export type ExtractError = { error: 'unreachable' | 'no_access' } | { error: 'failed'; message: string };
export type ExtractReply = ExtractResult | ExtractError;
export type TabAccessReply = { url: string; title: string } | { access: false };
export type CommandsReply = Array<{ name: string; description: string; shortcut: string }>;

export type Request =
  | { type: 'GET_HEADERS' }
  | { type: 'REFRESH' }
  | { type: 'SET_CONNECTION'; connection: Connection }
  | { type: 'PATCH_CONNECTION'; patch: Partial<Omit<Connection, 'auth'>> }
  | { type: 'CLEAR_CONNECTION'; forgetServer: boolean }
  | { type: 'EXTRACT'; tabId: number; mode: ExtractMode }
  | { type: 'TAB_ACCESS'; tabId: number }
  | { type: 'GET_COMMANDS' };

// A worker replies within milliseconds; silence means Chrome ended the idle worker as the message arrived,
// which drops the reply (seen live on the Stop request). Sending again starts a fresh worker.
const SILENCE_MS = 2_000;
// EXTRACT injects and reads the page, which can legitimately take seconds; a second send would run it twice.
const NO_RESEND = new Set<Request['type']>(['EXTRACT']);

export async function send<T>(request: Request, silenceMs = SILENCE_MS): Promise<T> {
  const silence = Symbol('silence');
  const once = () => chrome.runtime.sendMessage(request) as Promise<T | undefined>;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const first = NO_RESEND.has(request.type)
    ? await once()
    : await Promise.race([once(), new Promise<typeof silence>((r) => { timer = setTimeout(() => r(silence), silenceMs); })]);
  clearTimeout(timer);
  const reply = first === silence ? await once() : first;
  // A missing listener resolves undefined, which would otherwise read as a sign-out.
  if (reply === undefined) throw new Error(`no reply to ${request.type}`);
  return reply;
}

// Pages get their headers from the worker, which alone holds and refreshes the connection.
export const workerApi = () => new ApiClient({ getHeaders: () => send<HeadersReply>({ type: 'GET_HEADERS' }), refresh: () => send<RefreshOutcome>({ type: 'REFRESH' }) });
