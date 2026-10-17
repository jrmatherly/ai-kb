import { fetchEventSource } from '@microsoft/fetch-event-source';
import type { HeadersReply, RefreshOutcome } from './messages';
import { browserFetch } from './browserFetch';
import { apiBase } from './serverUrl';
import type {
  Agent, Knowledge, KnowledgeBase, LoginResponse, Me, Message, OidcConfig, Session, SharedKnowledgeBase, StreamEvent,
} from './types';

export class ApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

export type StreamErrorKind = 'unauthorized' | 'server' | 'network' | 'incomplete';
export type StreamHandlers = { onEvent(event: StreamEvent): void; onError(kind: StreamErrorKind, message?: string): void };
export type ManualBody = { title: string; content: string };
export type Target = { serverUrl: string; headers: Record<string, string> };

type Deps = {
  getHeaders: () => Promise<HeadersReply>;
  refresh: () => Promise<RefreshOutcome>;
  fetchFn?: typeof fetch;
  fetchEventSourceFn?: typeof fetchEventSource;
};

// `target` skips the worker's headers and the refresh: sign-in calls and the pagehide stop use it.
type RequestOptions = { envelope?: boolean; keepalive?: boolean; target?: Target };

const JSON_HEADERS = { 'Content-Type': 'application/json' };
const MAX_FILE_BYTES = 10 * 1024 * 1024;

function messageOf(body: unknown, fallback: string): string {
  if (body && typeof body === 'object') {
    const b = body as { error?: { message?: string }; message?: string };
    return b.error?.message ?? b.message ?? fallback;
  }
  return fallback;
}

async function parseBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

async function onForbidden<T>(promise: Promise<T>, fallback: T): Promise<T> {
  try {
    return await promise;
  } catch (e) {
    if (e instanceof ApiError && e.status === 403) return fallback;
    throw e;
  }
}

const published = (body: ManualBody) => ({ ...body, status: 'publish', channel: 'browser_extension' });

export class ApiClient {
  private readonly fetchFn: typeof fetch;
  private readonly fetchEventSourceFn: typeof fetchEventSource;

  constructor(private readonly deps: Deps) {
    this.fetchFn = deps.fetchFn ?? browserFetch;
    this.fetchEventSourceFn = deps.fetchEventSourceFn ?? fetchEventSource;
  }

  private async session(): Promise<Target> {
    const reply = await this.deps.getHeaders();
    if (!reply) throw new ApiError(401, 'signed_out');
    return reply;
  }

  // The one place the 401 rule lives: refresh once, replay once with fresh headers, never with the old token.
  private async withSession<T>(attempt: (target: Target) => Promise<T>): Promise<T> {
    try {
      return await attempt(await this.session());
    } catch (e) {
      if (!(e instanceof ApiError) || e.status !== 401) throw e;
      if ((await this.deps.refresh()) !== 'refreshed') throw new ApiError(401, 'signed_out');
      return attempt(await this.session());
    }
  }

  private async send(url: string, init: RequestInit): Promise<Response> {
    const response = await this.fetchFn(url, init);
    if (response.status === 401) throw new ApiError(401, messageOf(await parseBody(response), 'unauthorized'));
    return response;
  }

  private async request<T>(method: string, path: string, body?: unknown, opts: RequestOptions = {}): Promise<T> {
    const run = async (target: Target) => {
      const response = await this.send(`${apiBase(target.serverUrl)}${path}`, {
        method,
        headers: body === undefined ? target.headers : { ...target.headers, ...JSON_HEADERS },
        body: body === undefined ? undefined : JSON.stringify(body),
        keepalive: opts.keepalive,
      });
      const parsed = await parseBody(response);
      if (!response.ok) throw new ApiError(response.status, messageOf(parsed, `HTTP ${response.status}`));
      return parsed;
    };
    const parsed = opts.target ? await run(opts.target) : await this.withSession(run);
    return (opts.envelope ?? true) ? ((parsed as { data?: T } | null)?.data as T) : (parsed as T);
  }

  me(): Promise<Me> {
    return this.request('GET', '/auth/me');
  }

  login(serverUrl: string, email: string, password: string): Promise<LoginResponse> {
    return this.request('POST', '/auth/login', { email, password }, { envelope: false, target: { serverUrl, headers: {} } });
  }

  oidcConfig(serverUrl: string): Promise<OidcConfig> {
    return this.request('GET', '/auth/oidc/config', undefined, { envelope: false, target: { serverUrl, headers: {} } });
  }

  listKnowledgeBases(): Promise<KnowledgeBase[]> {
    return this.request('GET', '/knowledge-bases');
  }

  listSharedKnowledgeBases(): Promise<SharedKnowledgeBase[]> {
    return onForbidden(this.request('GET', '/shared-knowledge-bases'), []);
  }

  listAgents(): Promise<Agent[]> {
    return this.request('GET', '/agents');
  }

  async suggestedQuestions(agentId: string, limit = 6): Promise<string[]> {
    const data = await this.request<{ questions?: Array<{ question: string }> }>('GET', `/agents/${agentId}/suggested-questions?limit=${limit}`);
    return (data?.questions ?? []).map((q) => q.question);
  }

  async createSession(): Promise<string> {
    return (await this.request<{ id: string }>('POST', '/sessions', {})).id;
  }

  listSessions(page = 1, pageSize = 20): Promise<Session[]> {
    return this.request('GET', `/sessions?page=${page}&page_size=${pageSize}`);
  }

  loadMessages(sessionId: string, opts: { beforeTime?: string; resourceUrls: 'handle' | 'public' }): Promise<Message[]> {
    const before = opts.beforeTime ? `&before_time=${encodeURIComponent(opts.beforeTime)}` : '';
    return this.request('GET', `/messages/${sessionId}/load?limit=20&resource_urls=${opts.resourceUrls}${before}`);
  }

  async clearSession(sessionId: string): Promise<void> {
    await this.request('DELETE', `/sessions/${sessionId}/messages`);
  }

  async stopSession(sessionId: string, assistantMessageId: string, opts?: { keepalive?: boolean; direct?: Target }): Promise<void> {
    await this.request('POST', `/sessions/${sessionId}/stop`, { message_id: assistantMessageId }, { keepalive: opts?.keepalive, target: opts?.direct });
  }

  createManual(kbId: string, body: ManualBody): Promise<Knowledge> {
    return this.request('POST', `/knowledge-bases/${kbId}/knowledge/manual`, published(body));
  }

  async updateManual(id: string, body: ManualBody): Promise<void> {
    await this.request('PUT', `/knowledge/manual/${id}`, published(body));
  }

  getKnowledge(id: string): Promise<Knowledge> {
    return this.request('GET', `/knowledge/${id}`);
  }

  async getKnowledgeBatch(ids: string[]): Promise<Knowledge[]> {
    if (ids.length === 0) return [];
    return this.request('GET', `/knowledge/batch?${ids.map((id) => `ids=${encodeURIComponent(id)}`).join('&')}`);
  }

  fetchFile(ref: string): Promise<Blob | null> {
    return this.withSession(async (target) => {
      const response = await this.send(`${target.serverUrl}/files?file_path=${encodeURIComponent(ref)}`, { headers: target.headers });
      if (response.status === 403 || response.status === 404) return null;
      if (!response.ok) throw new ApiError(response.status, `HTTP ${response.status}`);
      if (Number(response.headers.get('content-length') ?? 0) > MAX_FILE_BYTES) throw new ApiError(413, 'file too large');
      const blob = await response.blob();
      if (blob.size > MAX_FILE_BYTES) throw new ApiError(413, 'file too large');
      return blob;
    });
  }

  async probeCapabilities(): Promise<{ retrieve: boolean; chat: boolean }> {
    const allowed = (path: string) => onForbidden(this.request('GET', path).then(() => true), false);
    const [retrieve, chat] = await Promise.all([allowed('/knowledge-bases'), allowed('/sessions?page=1&page_size=1')]);
    return { retrieve, chat };
  }

  async chatStream(
    kind: 'knowledge-chat' | 'agent-chat',
    sessionId: string,
    body: Record<string, unknown>,
    handlers: StreamHandlers,
    signal: AbortSignal,
  ): Promise<void> {
    let ended = false;
    const open = (target: Target) =>
      this.fetchEventSourceFn(`${apiBase(target.serverUrl)}/${kind}/${sessionId}`, {
        method: 'POST',
        headers: { ...target.headers, ...JSON_HEADERS, Accept: 'text/event-stream' },
        body: JSON.stringify(body),
        signal,
        openWhenHidden: true,
        onopen: async (response) => {
          if (response.ok && (response.headers.get('content-type') ?? '').includes('text/event-stream')) return;
          throw new ApiError(response.status, messageOf(await parseBody(response), `HTTP ${response.status}`));
        },
        onmessage: (message) => {
          let event: StreamEvent;
          try {
            event = JSON.parse(message.data) as StreamEvent;
          } catch {
            return;
          }
          if (event.response_type === 'complete' || event.response_type === 'error') ended = true;
          handlers.onEvent(event);
        },
        // The library retries unless onerror throws; every failure here is final.
        onerror: (error) => { throw error; },
      });

    try {
      await this.withSession(open);
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) handlers.onError('unauthorized');
      else if (error instanceof ApiError) handlers.onError('server', error.message);
      else handlers.onError('network', error instanceof Error ? error.message : String(error));
      return;
    }
    if (!ended && !signal.aborted) handlers.onError('incomplete');
  }
}
