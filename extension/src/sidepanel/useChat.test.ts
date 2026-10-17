import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, type ApiClient, type StreamHandlers } from '../shared/api';
import type { Agent, Auth, Message, StreamEvent } from '../shared/types';
import { useChat } from './useChat';

const quick: Agent = { id: 'builtin-quick-answer', name: 'Quick', config: { agent_mode: 'quick-answer', kb_selection_mode: 'all' } };
const smart: Agent = { id: 'a2', name: 'Smart', config: { agent_mode: 'smart-reasoning', kb_selection_mode: 'selected', knowledge_bases: ['kb9'] } };
const silent: Agent = { id: 'a3', name: 'Silent', config: { agent_mode: 'smart-reasoning', kb_selection_mode: 'none' } };

const history: Message[] = [
  { id: 'm1', role: 'user', content: 'Q', created_at: '2026-10-17T10:00:00Z' },
  { id: 'm2', role: 'assistant', content: 'A', created_at: '2026-10-17T10:00:01Z' },
];
const older: Message[] = [{ id: 'm0', role: 'assistant', content: 'Earlier', created_at: '2026-10-17T09:00:00Z' }];

type Call = [string, ...unknown[]];

function fake(opts: { events?: StreamEvent[]; loadMessages?: (calls: Call[]) => Message[] } = {}) {
  const calls: Call[] = [];
  const api = {
    createSession: async () => { calls.push(['createSession']); return 's1'; },
    chatStream: async (kind: string, sessionId: string, body: Record<string, unknown>, handlers: StreamHandlers, signal: AbortSignal) => {
      calls.push(['chatStream', kind, sessionId, body, signal instanceof AbortSignal]);
      for (const e of opts.events ?? []) handlers.onEvent(e);
    },
    stopSession: async (...args: unknown[]) => { calls.push(['stopSession', ...args]); },
    loadMessages: async (sessionId: string, o: { beforeTime?: string; resourceUrls: string }) => {
      calls.push(['loadMessages', sessionId, o]);
      return (opts.loadMessages ?? (() => history))(calls);
    },
    clearSession: async (id: string) => { calls.push(['clearSession', id]); },
    suggestedQuestions: async (id: string) => { calls.push(['suggestedQuestions', id]); return ['What is new?']; },
    listAgents: async () => { calls.push(['listAgents']); return [quick, smart]; },
    listKnowledgeBases: async () => [{ id: 'kb1', name: 'Docs' }],
    lastTarget: { serverUrl: 'https://kb.example.com', headers: {} },
    listSharedKnowledgeBases: async () => [{ knowledge_base: { id: 'kb1', name: 'Docs' }, permission: 'viewer' }, { knowledge_base: { id: 'kb2', name: 'Shared' }, permission: 'viewer' }, { knowledge_base: null, permission: 'viewer' }],
  } as unknown as ApiClient;
  return { api, calls };
}

function chat(api: ApiClient, authKind: Auth['kind'] = 'api_key') {
  return useChat({ api, authKind });
}

const named = (calls: Call[], name: string) => calls.filter((c) => c[0] === name);

test('ask on the quick-answer agent creates the session once and streams knowledge-chat with agent_enabled false', async () => {
  const { api, calls } = fake({ events: [{ response_type: 'answer', content: 'Hi' }, { response_type: 'complete', done: true }] });
  const c = chat(api);
  c.state.agents = [quick, smart];
  c.state.agentId = quick.id;
  c.state.kbIds = ['kb1', 'kb2'];
  await c.ask('first');
  await c.ask('second');
  assert.equal(named(calls, 'createSession').length, 1);
  assert.deepEqual(named(calls, 'chatStream')[0], [
    'chatStream', 'knowledge-chat', 's1',
    { query: 'first', channel: 'browser_extension', knowledge_base_ids: ['kb1', 'kb2'], agent_id: 'builtin-quick-answer', agent_enabled: false },
    true,
  ]);
  assert.deepEqual(c.state.messages.map((m) => [m.role, m.markdown, m.done]), [['user', 'first', true], ['assistant', 'Hi', true], ['user', 'second', true], ['assistant', 'Hi', true]]);
  assert.equal(c.state.streaming, false);
});

test('ask on a regular agent streams agent-chat with agent_enabled true', async () => {
  const { api, calls } = fake();
  const c = chat(api);
  c.state.agents = [quick, smart];
  c.state.agentId = smart.id;
  c.state.kbIds = ['kb9'];
  await c.ask('q');
  const [, kind, , body] = named(calls, 'chatStream')[0];
  assert.equal(kind, 'agent-chat');
  assert.deepEqual(body, { query: 'q', channel: 'browser_extension', knowledge_base_ids: ['kb9'], agent_id: 'a2', agent_enabled: true });
});

test('ask records the session title and routes a stream failure through applyError', async () => {
  const { api } = fake();
  (api as unknown as { chatStream: unknown }).chatStream = async (_k: string, _s: string, _b: unknown, handlers: StreamHandlers) => {
    handlers.onEvent({ response_type: 'session_title', content: 'Pricing' });
    handlers.onEvent({ response_type: 'answer', content: 'Hi' });
    handlers.onError('network', 'Failed to fetch');
  };
  const c = chat(api);
  c.state.agents = [quick];
  c.state.agentId = quick.id;
  await c.ask('q');
  assert.equal(c.state.sessionTitle, 'Pricing');
  assert.deepEqual([c.state.messages[1].markdown, c.state.messages[1].error], ['Hi', 'Cannot reach https://kb.example.com']);
});

test('stop after agent_query aborts and asks the client for a keepalive stop', async () => {
  let signal: AbortSignal | undefined;
  const { api, calls } = fake();
  let release!: () => void;
  (api as unknown as { chatStream: unknown }).chatStream = (_k: string, _s: string, _b: unknown, handlers: StreamHandlers, s: AbortSignal) => {
    signal = s;
    handlers.onEvent({ response_type: 'agent_query', assistant_message_id: 'am1', data: { assistant_message_id: 'am1' } });
    return new Promise<void>((resolve) => { release = resolve; });
  };
  const c = chat(api, 'bearer');
  c.state.agents = [smart];
  c.state.agentId = smart.id;
  const asking = c.ask('q');
  await Promise.resolve();
  assert.equal(c.state.streaming, true);
  await c.stop({ keepalive: true });
  assert.equal(signal?.aborted, true);
  assert.deepEqual(named(calls, 'stopSession'), [
    ['stopSession', 's1', 'am1', { keepalive: true }],
  ]);
  release();
  await asking;
  assert.equal(c.state.streaming, false);
});

test('openSession with an API key loads with handle resource urls', async () => {
  const { api, calls } = fake();
  const c = chat(api, 'api_key');
  assert.equal(c.state.historyMode, 'handle');
  await c.openSession('s7');
  assert.deepEqual(named(calls, 'loadMessages'), [['loadMessages', 's7', { resourceUrls: 'handle' }]]);
  assert.equal(c.state.sessionId, 's7');
  assert.deepEqual(c.state.messages.map((m) => [m.id, m.role, m.markdown]), [['m1', 'user', 'Q'], ['m2', 'assistant', 'A']]);
});

test('openSession with a bearer token retries once with handle after a 403 on public', async () => {
  const { api, calls } = fake({
    loadMessages: (cs) => {
      if (named(cs, 'loadMessages').length === 1) throw new ApiError(403, 'forbidden');
      return history;
    },
  });
  const c = chat(api, 'bearer');
  assert.equal(c.state.historyMode, 'public');
  await c.openSession('s7');
  assert.deepEqual(named(calls, 'loadMessages'), [
    ['loadMessages', 's7', { resourceUrls: 'public' }],
    ['loadMessages', 's7', { resourceUrls: 'handle' }],
  ]);
  assert.equal(c.state.historyMode, 'handle');
  assert.equal(c.state.messages.length, 2);
});

test('a 403 in handle mode is not retried', async () => {
  const { api, calls } = fake({ loadMessages: () => { throw new ApiError(403, 'forbidden'); } });
  const c = chat(api, 'api_key');
  await assert.rejects(c.openSession('s7'), { status: 403 });
  assert.equal(named(calls, 'loadMessages').length, 1);
});

test('loadOlder pages back from the oldest loaded message and prepends', async () => {
  const { api, calls } = fake({ loadMessages: (cs) => (named(cs, 'loadMessages').length === 1 ? history : older) });
  const c = chat(api);
  await c.openSession('s7');
  await c.loadOlder();
  assert.deepEqual(named(calls, 'loadMessages')[1], ['loadMessages', 's7', { beforeTime: '2026-10-17T10:00:00Z', resourceUrls: 'handle' }]);
  assert.deepEqual(c.state.messages.map((m) => m.id), ['m0', 'm1', 'm2']);
});

test('loadOlder during a stream keeps the streamed answer on the visible message', async () => {
  const { api } = fake({ loadMessages: (cs) => (named(cs, 'loadMessages').length === 1 ? history : older) });
  let handlers!: StreamHandlers;
  let release!: () => void;
  (api as unknown as { chatStream: unknown }).chatStream = (_k: string, _s: string, _b: unknown, h: StreamHandlers) => {
    handlers = h;
    return new Promise<void>((resolve) => { release = resolve; });
  };
  const c = chat(api);
  await c.openSession('s7');
  const asking = c.ask('q');
  await new Promise((r) => setTimeout(r, 0));
  await c.loadOlder();
  handlers.onEvent({ response_type: 'answer', content: 'Streamed' });
  handlers.onEvent({ response_type: 'complete', done: true });
  release();
  await asking;
  assert.deepEqual(c.state.messages.map((m) => [m.role, m.markdown, m.done]), [
    ['assistant', 'Earlier', true], ['user', 'Q', true], ['assistant', 'A', true], ['user', 'q', true], ['assistant', 'Streamed', true],
  ]);
});

test('agent, kbChoice and setKbIds follow the selected agent', async () => {
  const { api } = fake();
  const c = chat(api);
  c.state.agents = [quick, smart, silent];
  c.state.kbs = [{ id: 'kb1', name: 'Docs' }, { id: 'kb9', name: 'Lab' }];
  await c.selectAgent('a2');
  assert.equal(c.agent.value?.id, 'a2');
  assert.deepEqual(c.kbChoice.value, { show: true, locked: true, kbs: [{ id: 'kb9', name: 'Lab' }] });
  await c.selectAgent('builtin-quick-answer');
  c.setKbIds(['kb1']);
  assert.deepEqual([c.state.kbIds, c.kbChoice.value.locked, c.kbChoice.value.kbs.length], [['kb1'], false, 2]);
  await c.selectAgent('a3');
  assert.equal(c.kbChoice.value.show, false);
});

test('selectAgent sets the KBs per kb_selection_mode and loads suggestions', async () => {
  const { api } = fake();
  const c = chat(api);
  c.state.agents = [quick, smart, silent];
  c.state.kbIds = ['kb1'];
  await c.selectAgent('a2');
  assert.deepEqual([c.state.agentId, c.state.kbIds, c.state.suggestions], ['a2', ['kb9'], ['What is new?']]);
  await c.selectAgent('a3');
  assert.deepEqual(c.state.kbIds, []);
  c.state.kbIds = ['kb1'];
  await c.selectAgent('builtin-quick-answer');
  assert.deepEqual(c.state.kbIds, ['kb1']);
});

test('clear empties the server session then starts a new conversation', async () => {
  const { api, calls } = fake();
  const c = chat(api);
  c.state.agents = [quick];
  c.state.agentId = quick.id;
  await c.ask('q');
  await c.clear();
  assert.deepEqual(named(calls, 'clearSession'), [['clearSession', 's1']]);
  assert.deepEqual([c.state.sessionId, c.state.sessionTitle, c.state.assistantMessageId, c.state.messages], [null, '', null, []]);
  await c.ask('again');
  assert.equal(named(calls, 'createSession').length, 2);
});

test('load fills agents and readable knowledge bases once each and selects the first agent', async () => {
  const { api, calls } = fake();
  const c = chat(api);
  await c.load();
  assert.deepEqual(c.state.agents.map((a) => a.id), ['builtin-quick-answer', 'a2']);
  assert.deepEqual(c.state.kbs.map((kb) => kb.id), ['kb1', 'kb2']);
  assert.equal(c.state.agentId, 'builtin-quick-answer');
  assert.deepEqual(c.state.suggestions, ['What is new?']);
  assert.equal(named(calls, 'listAgents').length, 1);
});

test('ask is ignored while a stream is running', async () => {
  let release: () => void = () => undefined;
  const { api, calls } = fake();
  (api as unknown as { chatStream: unknown }).chatStream = async (kind: string) => { calls.push(['chatStream', kind]); await new Promise<void>((r) => { release = r; }); };
  const c = chat(api);
  c.state.agents = [quick];
  c.state.agentId = quick.id;
  const first = c.ask('one');
  await new Promise((r) => setTimeout(r, 0));
  await c.ask('two');
  release();
  await first;
  assert.equal(named(calls, 'chatStream').length, 1);
  assert.equal(c.state.messages.length, 2);
});

test('stop settles the open assistant message locally', async () => {
  const { api } = fake({ events: [{ response_type: 'agent_query', assistant_message_id: 'am1' }, { response_type: 'answer', content: 'part' }] });
  const c = chat(api);
  c.state.agents = [quick];
  c.state.agentId = quick.id;
  await c.ask('q');
  assert.equal(c.state.messages[1].done, false);
  await c.stop();
  assert.deepEqual([c.state.messages[1].done, c.state.messages[1].markdown, c.state.messages[1].error], [true, 'part', undefined]);
});
