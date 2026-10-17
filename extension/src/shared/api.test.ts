import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { fetchEventSource } from '@microsoft/fetch-event-source';
import { ApiClient, ApiError, type StreamHandlers } from './api';
import type { StreamEvent } from './types';

type Call = { method: string; url: string; headers: Record<string, string>; body?: string; keepalive?: boolean };

type Scripted = { status: number; body?: unknown };

function fakeFetch(script: Scripted[]) {
  const calls: Call[] = [];
  const fetchFn = (async (input: string | URL | Request, init?: RequestInit) => {
    const step = script.shift();
    if (!step) throw new Error('unscripted fetch');
    calls.push({
      method: init?.method ?? 'GET',
      url: String(input),
      headers: (init?.headers as Record<string, string>) ?? {},
      body: init?.body == null ? undefined : String(init.body),
      keepalive: init?.keepalive,
    });
    return new Response(step.body === undefined ? null : JSON.stringify(step.body), { status: step.status, headers: { 'content-type': 'application/json' } });
  }) as typeof fetch;
  return { fetchFn, calls };
}

// Each call hands out the next access token, so a replay after a refresh is visible in the headers.
function rotatingBearer() {
  let n = 0;
  return async () => ({ serverUrl: 'https://kb.example.com', headers: { Authorization: `Bearer a${++n}` }, authKind: 'bearer' as const });
}

const apiKeyHeaders = async () => ({ serverUrl: 'https://kb.example.com', headers: { 'X-API-Key': 'sk-1', 'X-Tenant-ID': '7' }, authKind: 'api_key' as const });

function client(script: Scripted[], overrides: Partial<ConstructorParameters<typeof ApiClient>[0]> = {}) {
  const { fetchFn, calls } = fakeFetch(script);
  const api = new ApiClient({ getHeaders: apiKeyHeaders, refresh: async () => 'not_bearer', fetchFn, ...overrides });
  return { api, calls };
}

test('createManual posts a published browser_extension entry with the auth headers', async () => {
  const { api, calls } = client([{ status: 200, body: { success: true, data: { id: 'k1', title: 'T', parse_status: 'pending' } } }]);
  const knowledge = await api.createManual('kb1', { title: 'T', content: '# x' });
  assert.equal(knowledge.id, 'k1');
  assert.equal(calls[0].method, 'POST');
  assert.equal(calls[0].url, 'https://kb.example.com/api/v1/knowledge-bases/kb1/knowledge/manual');
  assert.equal(calls[0].body, '{"title":"T","content":"# x","status":"publish","channel":"browser_extension"}');
  assert.equal(calls[0].headers['X-API-Key'], 'sk-1');
  assert.equal(calls[0].headers['X-Tenant-ID'], '7');
});

test('login reads the top-level response and sends no auth headers', async () => {
  const { api, calls } = client([{ status: 200, body: { success: true, token: 'a1', refresh_token: 'r1', active_tenant: { id: 7 }, memberships: [] } }]);
  const login = await api.login('https://kb.example.com', 'me@example.com', 'pw');
  assert.equal(login.token, 'a1');
  assert.equal(calls[0].url, 'https://kb.example.com/api/v1/auth/login');
  assert.equal(calls[0].body, '{"email":"me@example.com","password":"pw"}');
  assert.equal(calls[0].headers['X-API-Key'], undefined);
});

test('login does not unwrap an envelope, so a wrapped fixture yields no token', async () => {
  const { api } = client([{ status: 200, body: { success: true, data: { token: 'a1' } } }]);
  assert.equal((await api.login('https://kb.example.com', 'me@example.com', 'pw')).token, undefined);
});

test('oidcConfig is unauthenticated and top-level', async () => {
  const { api, calls } = client([{ status: 200, body: { success: true, enabled: true, provider_display_name: 'Dex' } }]);
  const config = await api.oidcConfig('https://kb.example.com');
  assert.deepEqual(config, { success: true, enabled: true, provider_display_name: 'Dex' });
  assert.equal(calls[0].url, 'https://kb.example.com/api/v1/auth/oidc/config');
  assert.equal(calls[0].headers.Authorization, undefined);
});

test('loadMessages sends the resource_urls mode and before_time', async () => {
  const { api, calls } = client([{ status: 200, body: { success: true, data: [] } }, { status: 200, body: { success: true, data: [] } }]);
  await api.loadMessages('s1', { resourceUrls: 'handle' });
  await api.loadMessages('s1', { resourceUrls: 'public', beforeTime: '2026-10-17T00:00:00Z' });
  assert.equal(calls[0].url, 'https://kb.example.com/api/v1/messages/s1/load?limit=20&resource_urls=handle');
  assert.equal(calls[1].url, 'https://kb.example.com/api/v1/messages/s1/load?limit=20&resource_urls=public&before_time=2026-10-17T00%3A00%3A00Z');
});

test('the other endpoints build their literal URLs and bodies', async () => {
  const { api, calls } = client([
    { status: 200, body: { success: true, data: { user: { id: 'u1', email: 'e' }, tenant: { id: 7, name: 'T' }, memberships: [] } } },
    { status: 200, body: { success: true, data: [{ id: 'kb1', name: 'KB' }] } },
    { status: 200, body: { success: true, data: [{ knowledge_base: { id: 'kb2', name: 'S' }, permission: 'editor' }] } },
    { status: 200, body: { success: true, data: [{ id: 'ag1', name: 'A', config: { agent_mode: 'x', kb_selection_mode: 'all' } }] } },
    { status: 200, body: { success: true, data: { questions: [{ question: 'Q1' }, { question: 'Q2' }] } } },
    { status: 200, body: { success: true, data: { id: 's9' } } },
    { status: 200, body: { success: true, data: [{ id: 's1', title: 't', updated_at: 'u' }] } },
    { status: 200, body: { success: true } },
    { status: 200, body: { success: true } },
    { status: 200, body: { success: true, data: { id: 'k1', title: 'T', parse_status: 'completed' } } },
    { status: 200, body: { success: true, data: [{ id: 'k1', title: 'T', parse_status: 'completed' }] } },
  ]);
  assert.equal((await api.me()).tenant?.id, 7);
  assert.equal((await api.listKnowledgeBases())[0].id, 'kb1');
  assert.equal((await api.listSharedKnowledgeBases())[0].permission, 'editor');
  assert.equal((await api.listAgents())[0].id, 'ag1');
  assert.deepEqual(await api.suggestedQuestions('ag1'), ['Q1', 'Q2']);
  assert.equal(await api.createSession(), 's9');
  assert.equal((await api.listSessions())[0].id, 's1');
  await api.clearSession('s1');
  await api.updateManual('k1', { title: 'T2', content: 'c' });
  assert.equal((await api.getKnowledge('k1')).parse_status, 'completed');
  assert.equal((await api.getKnowledgeBatch(['k1', 'k2']))[0].id, 'k1');
  assert.deepEqual(calls.map((c) => `${c.method} ${c.url}`), [
    'GET https://kb.example.com/api/v1/auth/me',
    'GET https://kb.example.com/api/v1/knowledge-bases',
    'GET https://kb.example.com/api/v1/shared-knowledge-bases',
    'GET https://kb.example.com/api/v1/agents',
    'GET https://kb.example.com/api/v1/agents/ag1/suggested-questions?limit=6',
    'POST https://kb.example.com/api/v1/sessions',
    'GET https://kb.example.com/api/v1/sessions?page=1&page_size=20',
    'DELETE https://kb.example.com/api/v1/sessions/s1/messages',
    'PUT https://kb.example.com/api/v1/knowledge/manual/k1',
    'GET https://kb.example.com/api/v1/knowledge/k1',
    'GET https://kb.example.com/api/v1/knowledge/batch?ids=k1&ids=k2',
  ]);
  assert.equal(calls[5].body, '{}');
  assert.equal(calls[8].body, '{"title":"T2","content":"c","status":"publish","channel":"browser_extension"}');
});

test('listSharedKnowledgeBases returns [] on 403', async () => {
  const { api } = client([{ status: 403, body: { success: false, error: { message: 'no' } } }]);
  assert.deepEqual(await api.listSharedKnowledgeBases(), []);
});

test('a 401 refreshes once and replays once with the new token', async () => {
  let refreshes = 0;
  const { api, calls } = client(
    [{ status: 401, body: { success: false, error: { message: 'expired' } } }, { status: 200, body: { success: true, data: [{ id: 's1', title: 't', updated_at: 'u' }] } }],
    { getHeaders: rotatingBearer(), refresh: async () => { refreshes++; return 'refreshed'; } },
  );
  assert.equal((await api.listSessions())[0].id, 's1');
  assert.equal(refreshes, 1);
  assert.equal(calls.length, 2);
  assert.equal(calls[0].headers.Authorization, 'Bearer a1');
  assert.equal(calls[1].headers.Authorization, 'Bearer a2');
});

test('a 401 whose refresh signs out throws signed_out after one request', async () => {
  const { api, calls } = client([{ status: 401, body: { success: false } }], { refresh: async () => 'signed_out' });
  await assert.rejects(api.listSessions(), (e: unknown) => e instanceof ApiError && e.status === 401 && e.message === 'signed_out');
  assert.equal(calls.length, 1);
});

test('a non-2xx throws an ApiError with the server message', async () => {
  const { api } = client([{ status: 500, body: { success: false, error: { message: 'boom' } } }]);
  await assert.rejects(api.listSessions(), (e: unknown) => e instanceof ApiError && e.status === 500 && e.message === 'boom');
});

test('stopSession with a direct target uses it, never calls getHeaders, and passes keepalive', async () => {
  const { fetchFn, calls } = fakeFetch([{ status: 200, body: { success: true } }]);
  const api = new ApiClient({ getHeaders: async () => { throw new Error('must not be called'); }, refresh: async () => 'not_bearer', fetchFn });
  await api.stopSession('s1', 'am1', { keepalive: true, direct: { serverUrl: 'https://kb.example.com', headers: { 'X-API-Key': 'sk-1' } } });
  assert.equal(calls[0].url, 'https://kb.example.com/api/v1/sessions/s1/stop');
  assert.equal(calls[0].body, '{"message_id":"am1"}');
  assert.equal(calls[0].keepalive, true);
  assert.equal(calls[0].headers['X-API-Key'], 'sk-1');
});

test('probeCapabilities maps a 403 to false', async () => {
  const { api } = client([{ status: 200, body: { success: true, data: [] } }, { status: 403, body: { success: false } }]);
  assert.deepEqual(await api.probeCapabilities(), { retrieve: true, chat: false });
});

test('fetchFile returns null on 403 and 404', async () => {
  const { api, calls } = client([{ status: 403 }, { status: 404 }]);
  assert.equal(await api.fetchFile('minio://b/k.png'), null);
  assert.equal(await api.fetchFile('minio://b/k.png'), null);
  assert.equal(calls[0].url, 'https://kb.example.com/files?file_path=minio%3A%2F%2Fb%2Fk.png');
  assert.equal(calls[0].headers['X-API-Key'], 'sk-1');
});

type StreamStep =
  | { kind: 'open'; status: number; contentType?: string; body?: string; events?: string[] }
  | { kind: 'network'; message: string };

// Mirrors the library: a throwing onopen or a transport failure reaches onerror, and a rethrow rejects.
function fakeEventSource(script: StreamStep[]) {
  const opens: Array<{ url: string; headers: Record<string, string>; body: string }> = [];
  const fn = async (input: RequestInfo, init: Parameters<typeof fetchEventSource>[1]) => {
    opens.push({ url: String(input), headers: init.headers ?? {}, body: String(init.body) });
    const step = script.shift();
    if (!step) throw new Error('unscripted open');
    try {
      if (step.kind === 'network') throw new TypeError(step.message);
      const response = new Response(step.body ?? '', { status: step.status, headers: { 'content-type': step.contentType ?? 'text/event-stream' } });
      await init.onopen?.(response);
      for (const data of step.events ?? []) init.onmessage?.({ data, event: '', id: '', retry: undefined });
      init.onclose?.();
    } catch (e) {
      init.onerror?.(e);
      throw new Error('fake would retry: onerror did not rethrow');
    }
  };
  return { fn: fn as unknown as typeof fetchEventSource, opens };
}

function streamClient(script: StreamStep[], refresh: () => Promise<'refreshed' | 'signed_out' | 'not_bearer'>) {
  const { fn, opens } = fakeEventSource(script);
  const api = new ApiClient({ getHeaders: rotatingBearer(), refresh, fetchFn: fakeFetch([]).fetchFn, fetchEventSourceFn: fn });
  const events: StreamEvent[] = [];
  const errors: Array<[string, string | undefined]> = [];
  const handlers: StreamHandlers = { onEvent: (e) => events.push(e), onError: (kind, message) => errors.push([kind, message]) };
  return { api, opens, events, errors, handlers };
}

const complete = JSON.stringify({ response_type: 'complete', done: true });

test('chatStream: a 401 on open refreshes once and reopens with the new token', async () => {
  let refreshes = 0;
  const { api, opens, events, errors, handlers } = streamClient(
    [{ kind: 'open', status: 401, contentType: 'application/json', body: '{"success":false}' }, { kind: 'open', status: 200, events: [JSON.stringify({ response_type: 'answer', content: 'hi' }), complete] }],
    async () => { refreshes++; return 'refreshed'; },
  );
  await api.chatStream('knowledge-chat', 's1', { query: 'q' }, handlers, new AbortController().signal);
  assert.equal(refreshes, 1);
  assert.equal(opens.length, 2);
  assert.equal(opens[0].url, 'https://kb.example.com/api/v1/knowledge-chat/s1');
  assert.equal(opens[1].headers.Authorization, 'Bearer a2');
  assert.equal(events.length, 2);
  assert.deepEqual(errors, []);
});

test('chatStream: 401 twice reports unauthorized after one refresh', async () => {
  let refreshes = 0;
  const { api, opens, errors, handlers } = streamClient(
    [{ kind: 'open', status: 401, contentType: 'application/json', body: '{}' }, { kind: 'open', status: 401, contentType: 'application/json', body: '{}' }],
    async () => { refreshes++; return 'refreshed'; },
  );
  await api.chatStream('agent-chat', 's1', {}, handlers, new AbortController().signal);
  assert.equal(refreshes, 1);
  assert.equal(opens.length, 2);
  assert.deepEqual(errors, [['unauthorized', undefined]]);
});

test('chatStream: a network error reports network once and never retries', async () => {
  const { api, opens, errors, handlers } = streamClient([{ kind: 'network', message: 'Failed to fetch' }], async () => 'not_bearer');
  await api.chatStream('knowledge-chat', 's1', {}, handlers, new AbortController().signal);
  assert.equal(opens.length, 1);
  assert.deepEqual(errors, [['network', 'Failed to fetch']]);
});

test('chatStream: a close without complete reports incomplete', async () => {
  const { api, errors, events, handlers } = streamClient([{ kind: 'open', status: 200, events: [JSON.stringify({ response_type: 'answer', content: 'a' }), 'not json'] }], async () => 'not_bearer');
  await api.chatStream('knowledge-chat', 's1', {}, handlers, new AbortController().signal);
  assert.equal(events.length, 1);
  assert.deepEqual(errors, [['incomplete', undefined]]);
});

test('chatStream: a 500 on open reports the server message', async () => {
  const { api, errors, handlers } = streamClient([{ kind: 'open', status: 500, contentType: 'application/json', body: '{"success":false,"message":"boom"}' }], async () => 'not_bearer');
  await api.chatStream('knowledge-chat', 's1', {}, handlers, new AbortController().signal);
  assert.deepEqual(errors, [['server', 'boom']]);
});

test('an empty 2xx body resolves to undefined data instead of throwing', async () => {
  const { api } = client([{ status: 204 }]);
  await api.clearSession('s1');
});

test('getKnowledgeBatch with no ids makes no request', async () => {
  const { api, calls } = client([]);
  assert.deepEqual(await api.getKnowledgeBatch([]), []);
  assert.equal(calls.length, 0);
});

test('fetchFile refreshes once on 401 and returns the blob', async () => {
  const { api, calls } = client([{ status: 401 }, { status: 200, body: 'bytes' }], { getHeaders: rotatingBearer(), refresh: async () => 'refreshed' });
  const blob = await api.fetchFile('minio://b/k.png');
  assert.equal(await blob?.text(), '"bytes"');
  assert.equal(calls.length, 2);
  assert.equal(calls[1].headers.Authorization, 'Bearer a2');
});

test('fetchFile rejects a body over 10 MB', async () => {
  const big = 'x'.repeat(10 * 1024 * 1024 + 1);
  const { api } = client([{ status: 200, body: big }]);
  await assert.rejects(api.fetchFile('minio://b/k.png'), (e: unknown) => e instanceof ApiError && e.status === 413);
});

test('stopSession without a direct target goes through request with keepalive', async () => {
  const { api, calls } = client([{ status: 200, body: { success: true } }]);
  await api.stopSession('s1', 'am1', { keepalive: true });
  assert.equal(calls[0].url, 'https://kb.example.com/api/v1/sessions/s1/stop');
  assert.equal(calls[0].keepalive, true);
  assert.equal(calls[0].headers['X-API-Key'], 'sk-1');
});

test('probeCapabilities runs both probes concurrently', async () => {
  const { api, calls } = client([{ status: 200, body: { success: true, data: [] } }, { status: 200, body: { success: true, data: [] } }]);
  await api.probeCapabilities();
  assert.deepEqual(calls.map((c) => c.url), ['https://kb.example.com/api/v1/knowledge-bases', 'https://kb.example.com/api/v1/sessions?page=1&page_size=1']);
});

test('chatStream: an abort during the stream is not reported as incomplete', async () => {
  const controller = new AbortController();
  const { api, errors, handlers } = streamClient([{ kind: 'open', status: 200, events: [JSON.stringify({ response_type: 'answer', content: 'a' })] }], async () => 'not_bearer');
  await api.chatStream('knowledge-chat', 's1', {}, { ...handlers, onEvent: () => controller.abort() }, controller.signal);
  assert.deepEqual(errors, []);
});
