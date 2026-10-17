import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ApiError, type ApiClient } from '../shared/api';
import type { Request } from '../shared/messages';
import type { Connection, KnowledgeBase, Me, OidcResult, SharedKnowledgeBase } from '../shared/types';
import { useConnection } from './useConnection';
import { OidcError } from './useOidc';

const me: Me = { user: { id: 'u1', email: 'me@example.com' }, tenant: { id: 7, name: 'Team' }, memberships: [{ tenant_id: 7, tenant_name: 'Team' }] };
const kbs: KnowledgeBase[] = [{ id: 'kb1', name: 'Docs', type: 'document' }, { id: 'faq', name: 'FAQ', type: 'faq' }];
const shared: SharedKnowledgeBase[] = [{ knowledge_base: { id: 'kb2', name: 'Shared', type: 'document' }, permission: 'editor' }];
const stored: Connection = { serverUrl: 'https://kb.example.com', auth: { kind: 'api_key', key: 'sk-1' }, tenantId: null, lastKbId: 'kb1', quickClip: false };

type Fake = { api: ApiClient; sent: Request[]; apiCalls: string[]; stored: Connection | null };

function fake(overrides: Partial<Record<string, unknown>> = {}, initial: Connection | null = stored): Fake {
  const sent: Request[] = [];
  const apiCalls: string[] = [];
  const f: Fake = { sent, apiCalls, stored: initial, api: null as unknown as ApiClient };
  f.api = {
    me: async () => { apiCalls.push('me'); if (overrides.me instanceof Error) throw overrides.me; return (overrides.me as Me) ?? me; },
    login: async () => { apiCalls.push('login'); return overrides.login ?? { success: true, token: 'a1', refresh_token: 'r1', active_tenant: { id: 7 }, memberships: me.memberships }; },
    oidcConfig: async () => { apiCalls.push('oidcConfig'); return overrides.oidcConfig ?? { success: true, enabled: true, provider_display_name: 'Dex' }; },
    probeCapabilities: async () => { apiCalls.push('probe'); return overrides.probes ?? { retrieve: true, chat: false }; },
    listKnowledgeBases: async () => { apiCalls.push('kbs'); return kbs; },
    listSharedKnowledgeBases: async () => { apiCalls.push('shared'); return shared; },
  } as unknown as ApiClient;
  return f;
}

function harness(f: Fake, signIn: (serverUrl: string) => Promise<OidcResult> = async () => { throw new Error('no oidc'); }) {
  const send = async <T>(request: Request): Promise<T> => {
    f.sent.push(request);
    if (request.type === 'GET_HEADERS') return (f.stored ? { serverUrl: f.stored.serverUrl, headers: {}, authKind: f.stored.auth.kind } : null) as T;
    if (request.type === 'SET_CONNECTION') { f.stored = request.connection; return null as T; }
    if (request.type === 'PATCH_CONNECTION') { f.stored = f.stored ? { ...f.stored, ...request.patch } : null; return f.stored as T; }
    if (request.type === 'CLEAR_CONNECTION') { f.stored = request.forgetServer || !f.stored ? null : { ...f.stored, auth: { kind: 'signed_out' } }; return null as T; }
    return null as T;
  };
  const commands = async () => [{ name: 'quick-ask', description: 'Ask about this page', shortcut: 'Alt+Shift+A' }];
  return useConnection({ api: f.api, send, oidc: { signIn, returnTo: () => 'https://abc.chromiumapp.org/oidc' }, loadConnection: async () => f.stored, commands });
}

const setSent = (f: Fake) => f.sent.filter((r): r is Extract<Request, { type: 'SET_CONNECTION' }> => r.type === 'SET_CONNECTION');

test('connect with an API key normalizes the server, stores it, then probes', async () => {
  const f = fake();
  const c = harness(f);
  c.state.serverUrl = 'https://kb.example.com/';
  c.state.mode = 'api_key';
  c.state.apiKey = 'sk-1';
  await c.connect();
  const [set] = setSent(f);
  assert.deepEqual(set.connection, { serverUrl: 'https://kb.example.com', auth: { kind: 'api_key', key: 'sk-1' }, tenantId: null, lastKbId: 'kb1', quickClip: false });
  assert.deepEqual(f.apiCalls, ['me', 'probe', 'kbs', 'shared']);
  assert.equal(c.state.status, 'connected');
  assert.equal(c.state.probes?.chat, false);
  assert.deepEqual(c.state.kbs.map((kb) => kb.id), ['kb1', 'kb2']);
  assert.equal(c.state.me?.tenant?.name, 'Team');
});

test('a 409 on me means a platform key that needs a workspace', async () => {
  const f = fake({ me: new ApiError(409, 'tenant required') });
  const c = harness(f);
  c.state.mode = 'api_key';
  c.state.apiKey = 'sk-1';
  await c.connect();
  assert.equal(c.state.needsTenant, true);
  assert.equal(c.state.status, 'idle');
});

test('connect with a password keeps two memberships pending and stores one', async () => {
  const two = { success: true, token: 'a1', refresh_token: 'r1', active_tenant: { id: 7 }, memberships: [{ tenant_id: 7, tenant_name: 'A' }, { tenant_id: 8, tenant_name: 'B' }] };
  const f = fake({ login: two });
  const c = harness(f);
  c.state.mode = 'password';
  c.state.email = 'me@example.com';
  c.state.password = 'pw';
  await c.connect();
  assert.equal(c.state.needsTenant, true);
  assert.equal(setSent(f).length, 0);
  c.state.tenantId = 8;
  await c.connect();
  assert.equal(setSent(f)[0].connection.tenantId, 8);
  assert.deepEqual(setSent(f)[0].connection.auth, { kind: 'bearer', token: 'a1', refreshToken: 'r1' });

  const one = fake();
  const single = harness(one);
  single.state.mode = 'password';
  single.state.email = 'me@example.com';
  single.state.password = 'pw';
  await single.connect();
  assert.equal(setSent(one)[0].connection.tenantId, 7);
  assert.equal(single.state.status, 'connected');
});

test('connect with OIDC treats the result like a login and surfaces the server refusal', async () => {
  const f = fake();
  const c = harness(f, async () => ({ success: true, token: 'a1', refresh_token: 'r1', tenant: { id: 7 }, memberships: me.memberships }));
  c.state.mode = 'oidc';
  await c.connect();
  assert.deepEqual(setSent(f)[0].connection.auth, { kind: 'bearer', token: 'a1', refreshToken: 'r1' });
  assert.equal(setSent(f)[0].connection.tenantId, 7);
  assert.ok(f.apiCalls.includes('me'));

  const two = harness(fake(), async () => ({ success: true, token: 'a1', refresh_token: 'r1', memberships: [{ tenant_id: 7, tenant_name: 'A' }, { tenant_id: 8, tenant_name: 'B' }] }));
  two.state.mode = 'oidc';
  await two.connect();
  assert.equal(two.state.needsTenant, true);

  const refused = harness(fake(), async () => { throw new OidcError('not_allowed'); });
  refused.state.mode = 'oidc';
  await refused.connect();
  assert.equal(refused.state.status, 'error');
  assert.match(refused.state.error, /does not allow/);

  const cancelled = harness(fake(), async () => { throw new OidcError('cancelled'); });
  cancelled.state.mode = 'oidc';
  await cancelled.connect();
  assert.equal(cancelled.state.error, 'Sign-in was cancelled');
});

test('checkServer records whether the server offers OIDC', async () => {
  const off = harness(fake({ oidcConfig: { success: true, enabled: false } }));
  await off.checkServer('https://kb.example.com');
  assert.equal(off.state.oidc?.enabled, false);
  const on = harness(fake());
  await on.checkServer('kb.example.com');
  assert.equal(on.state.oidc?.provider_display_name, 'Dex');
  assert.equal(on.state.serverUrl, 'https://kb.example.com');
  const insecure = harness(fake());
  await insecure.checkServer('http://kb.lan');
  assert.equal(insecure.state.insecure, true);
});

test('preferences travel as patches, never as a whole connection', async () => {
  const f = fake();
  const c = harness(f);
  await c.setLastKb('kb2');
  await c.setQuickClip(true);
  assert.deepEqual(f.sent.filter((r) => r.type === 'PATCH_CONNECTION'), [
    { type: 'PATCH_CONNECTION', patch: { lastKbId: 'kb2' } },
    { type: 'PATCH_CONNECTION', patch: { quickClip: true } },
  ]);
  assert.equal(setSent(f).length, 0);
  assert.equal(c.state.lastKbId, 'kb2');
  assert.equal(c.state.quickClip, true);
});

test('a 401 on me ends in error, and signOut only clears the connection', async () => {
  const f = fake({ me: new ApiError(401, 'signed_out') });
  const c = harness(f);
  c.state.mode = 'api_key';
  c.state.apiKey = 'sk-1';
  await c.connect();
  assert.equal(c.state.status, 'error');
  await c.signOut(false);
  assert.deepEqual(f.sent.at(-1), { type: 'CLEAR_CONNECTION', forgetServer: false });
  assert.equal(c.state.status, 'signed_out');
  await c.signOut(true);
  assert.equal(f.stored, null);
  assert.equal(c.state.serverUrl, '');
});

test('load reads the stored connection and a signed-out session', async () => {
  const f = fake({}, { ...stored, auth: { kind: 'signed_out' }, quickClip: true });
  const c = harness(f);
  await c.load();
  assert.equal(c.state.status, 'signed_out');
  assert.equal(c.state.serverUrl, 'https://kb.example.com');
  assert.equal(c.state.quickClip, true);
  assert.equal(c.state.lastKbId, 'kb1');
  assert.deepEqual(c.state.commands, [{ name: 'quick-ask', description: 'Ask about this page', shortcut: 'Alt+Shift+A' }]);
  const live = harness(fake());
  await live.load();
  assert.equal(live.state.status, 'connected');
  assert.ok(live.state.kbs.length > 0);
});

test('a rejected key leaves the previous connection in place', async () => {
  const f = fake({ me: new ApiError(401, 'signed_out') });
  const c = harness(f);
  c.state.mode = 'api_key';
  c.state.apiKey = 'sk-wrong';
  await c.connect();
  assert.equal(c.state.status, 'error');
  assert.equal(c.state.error, 'The server rejected this key or sign-in');
  assert.deepEqual(f.stored?.auth, { kind: 'api_key', key: 'sk-1' });
  assert.deepEqual(setSent(f).map((r) => (r.connection.auth as { key: string }).key), ['sk-wrong', 'sk-1']);
});

test('a stored session the worker could not refresh loads as signed out', async () => {
  const f = fake({ me: new ApiError(401, 'signed_out') }, { ...stored, auth: { kind: 'bearer', token: 'a1', refreshToken: 'r1' } });
  const c = harness(f);
  await c.load();
  assert.equal(c.state.status, 'signed_out');
  assert.equal(c.state.error, '');
});

test('a server that stops offering OIDC drops the OIDC tab selection', async () => {
  const c = harness(fake({ oidcConfig: { success: true, enabled: false } }));
  c.state.mode = 'oidc';
  await c.checkServer('https://kb.example.com');
  assert.equal(c.state.mode, 'api_key');
});
