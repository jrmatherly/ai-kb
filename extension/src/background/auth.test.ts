import { test } from 'node:test';
import assert from 'node:assert/strict';
import type { Connection } from '../shared/types';
import { authHeaders, createRefresher, patchConnection } from './auth';

const bearer: Connection = {
  serverUrl: 'https://kb.example.com',
  auth: { kind: 'bearer', token: 'a1', refreshToken: 'r1' },
  tenantId: 7,
  lastKbId: 'kb1',
  quickClip: true,
};

test('authHeaders for an API key', () => {
  assert.deepEqual(authHeaders({ ...bearer, auth: { kind: 'api_key', key: 'sk-1' }, tenantId: null }), { 'X-API-Key': 'sk-1' });
});

test('authHeaders for a bearer connection with a tenant', () => {
  assert.deepEqual(authHeaders(bearer), { Authorization: 'Bearer a1', 'X-Tenant-ID': '7' });
});

test('authHeaders when signed out', () => {
  assert.deepEqual(authHeaders({ ...bearer, auth: { kind: 'signed_out' }, tenantId: null }), {});
});

type Call = { url: string; body: string };

function fakeFetch(status: number, body: unknown, calls: Call[], gate?: Promise<void>) {
  return (async (url: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(url), body: String(init?.body) });
    if (gate) await gate;
    return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
  }) as typeof fetch;
}

function store(initial: Connection | null) {
  let current = initial;
  return {
    load: async () => current,
    save: async (c: Connection | null) => { current = c; },
    get: () => current,
  };
}

test('refreshes the bearer tokens and saves them', async () => {
  const calls: Call[] = [];
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(200, { success: true, access_token: 'a2', refresh_token: 'r2' }, calls) });
  assert.equal(await refresh(), 'refreshed');
  assert.deepEqual(calls, [{ url: 'https://kb.example.com/api/v1/auth/refresh', body: '{"refreshToken":"r1"}' }]);
  assert.deepEqual(s.get(), { ...bearer, auth: { kind: 'bearer', token: 'a2', refreshToken: 'r2' } });
});

test('concurrent callers share one refresh', async () => {
  const calls: Call[] = [];
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(200, { success: true, access_token: 'a2', refresh_token: 'r2' }, calls) });
  assert.deepEqual(await Promise.all([refresh(), refresh()]), ['refreshed', 'refreshed']);
  assert.equal(calls.length, 1);
});

test('a 401 from the refresh signs the connection out and keeps the rest', async () => {
  const calls: Call[] = [];
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(401, { success: false }, calls) });
  assert.equal(await refresh(), 'signed_out');
  assert.deepEqual(s.get(), { serverUrl: 'https://kb.example.com', auth: { kind: 'signed_out' }, tenantId: 7, lastKbId: 'kb1', quickClip: true });
});

test('an API key connection is not refreshable and makes no request', async () => {
  const calls: Call[] = [];
  const s = store({ ...bearer, auth: { kind: 'api_key', key: 'sk-1' } });
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(200, {}, calls) });
  assert.equal(await refresh(), 'not_bearer');
  assert.equal(calls.length, 0);
});

test('a failed refresh rejects and releases the in-flight slot', async () => {
  const calls: Call[] = [];
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(500, { success: false }, calls) });
  await assert.rejects(refresh());
  await assert.rejects(refresh());
  assert.equal(calls.length, 2);
});

test('a 200 without tokens rejects and saves nothing', async () => {
  const calls: Call[] = [];
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(200, { success: false }, calls) });
  await assert.rejects(refresh());
  assert.deepEqual(s.get(), bearer);
});

test('a refresh patches only auth on the connection saved meanwhile', async () => {
  const calls: Call[] = [];
  let release!: () => void;
  const gate = new Promise<void>((r) => { release = r; });
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(200, { success: true, access_token: 'a2', refresh_token: 'r2' }, calls, gate) });
  const pending = refresh();
  await new Promise((r) => setTimeout(r, 5));
  await s.save({ ...bearer, lastKbId: 'kb9', quickClip: false });
  release();
  assert.equal(await pending, 'refreshed');
  assert.deepEqual(s.get(), { ...bearer, lastKbId: 'kb9', quickClip: false, auth: { kind: 'bearer', token: 'a2', refreshToken: 'r2' } });
});

test('a refresh started from an older token does not overwrite a newer sign-in', async () => {
  const calls: Call[] = [];
  let release!: () => void;
  const gate = new Promise<void>((r) => { release = r; });
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(200, { success: true, access_token: 'a2', refresh_token: 'r2' }, calls, gate) });
  const pending = refresh();
  await Promise.resolve();
  const fresh: Connection = { ...bearer, serverUrl: 'https://other.example.com', auth: { kind: 'bearer', token: 'b1', refreshToken: 's1' } };
  await s.save(fresh);
  release();
  assert.equal(await pending, 'refreshed');
  assert.deepEqual(s.get(), fresh);
});

test('patchConnection merges into the stored connection and leaves auth alone', async () => {
  const s = store(bearer);
  assert.deepEqual(await patchConnection({ lastKbId: 'kb2', quickClip: false }, s), { ...bearer, lastKbId: 'kb2', quickClip: false });
  assert.deepEqual(s.get(), { ...bearer, lastKbId: 'kb2', quickClip: false });
  assert.equal(await patchConnection({ lastKbId: 'kb3' }, store(null)), null);
});

test('a patch that read the store before a refresh finished does not write the old tokens back', async () => {
  const calls: Call[] = [];
  let releaseFetch!: () => void;
  const fetchGate = new Promise<void>((r) => { releaseFetch = r; });
  const s = store(bearer);
  const refresh = createRefresher({ ...s, fetchFn: fakeFetch(200, { success: true, access_token: 'a2', refresh_token: 'r2' }, calls, fetchGate) });
  const pending = refresh();
  await Promise.resolve();
  // A slow read that captured the pre-refresh connection, as chrome.storage.local.get can.
  let releaseLoad!: () => void;
  const loadGate = new Promise<void>((r) => { releaseLoad = r; });
  const slow = { load: async () => { const seen = s.get(); await loadGate; return seen; }, save: s.save };
  const patched = patchConnection({ lastKbId: 'kb9' }, slow);
  releaseFetch();
  await new Promise((r) => setTimeout(r, 5));
  releaseLoad();
  await Promise.all([pending, patched]);
  assert.deepEqual(s.get(), { ...bearer, lastKbId: 'kb9', auth: { kind: 'bearer', token: 'a2', refreshToken: 'r2' } });
});
