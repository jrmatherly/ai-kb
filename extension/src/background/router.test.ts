import { test } from 'node:test';
import assert from 'node:assert/strict';
import { askFromGesture, clipFromGesture, type Deps, handle, isUnreachableUrl, noteFromGesture, quickClipIntent } from './router';
import type { Connection } from '../shared/types';

const bearer: Connection = { serverUrl: 'https://kb.example.com', auth: { kind: 'bearer', token: 'a1', refreshToken: 'r1' }, tenantId: 7, lastKbId: 'kb1', quickClip: true };
const sender = { id: 'ext', url: 'chrome-extension://ext/options.html' };

function deps(overrides: Partial<Deps> = {}) {
  let stored: Connection | null = bearer;
  const calls: string[] = [];
  const d: Deps & { calls: string[]; stored: () => Connection | null } = {
    load: async () => stored,
    save: async (c) => { stored = c; },
    refresh: async () => { calls.push('refresh'); return 'refreshed'; },
    tabInfo: async () => ({ url: 'https://e.com/p', title: 'P' }),
    inject: async (tabId, mode) => { calls.push(`inject ${tabId} ${mode}`); return { draft: { kind: mode, title: 'P', url: 'https://e.com/p', markdown: 'x', meta: {} } }; },
    commands: async () => [{ name: 'quick-ask', description: 'Ask about this page', shortcut: 'Alt+Shift+A' }],
    openPanel: (tabId) => { calls.push(`open ${tabId}`); return Promise.resolve(); },
    writeIntent: async (tabId, intent) => { calls.push(`intent ${tabId} ${intent}`); },
    readIntent: async () => null,
    writeClip: async (tabId, intent, draft) => { calls.push(`clip ${tabId} ${intent} ${draft.title}`); },
    writeComposer: async (tabId, text) => { calls.push(`composer ${tabId} ${text}`); },
    writeClipError: async (tabId, error) => { calls.push(`error ${tabId} ${JSON.stringify(error)}`); },
    pageSelection: async () => 'picked',
    extensionId: 'ext',
    extensionOrigin: 'chrome-extension://ext/',
    calls,
    stored: () => stored,
    ...overrides,
  };
  return d;
}

test('rejects senders that are not this extension', async () => {
  await assert.rejects(handle({ type: 'GET_HEADERS' }, { id: 'other', url: 'chrome-extension://ext/x.html' }, deps()), /forbidden sender/);
  await assert.rejects(handle({ type: 'GET_HEADERS' }, { id: 'ext', url: 'https://example.com/' }, deps()), /forbidden sender/);
});

test('GET_HEADERS reports the connection and its headers', async () => {
  assert.deepEqual(await handle({ type: 'GET_HEADERS' }, sender, deps()), {
    serverUrl: 'https://kb.example.com', headers: { Authorization: 'Bearer a1', 'X-Tenant-ID': '7' }, authKind: 'bearer',
  });
  const d = deps(); await d.save({ ...bearer, auth: { kind: 'signed_out' }, tenantId: null });
  assert.deepEqual(await handle({ type: 'GET_HEADERS' }, sender, d), { serverUrl: 'https://kb.example.com', headers: {}, authKind: 'signed_out' });
  const empty = deps(); await empty.save(null);
  assert.equal(await handle({ type: 'GET_HEADERS' }, sender, empty), null);
});

test('REFRESH forwards to the refresher', async () => {
  const d = deps();
  assert.equal(await handle({ type: 'REFRESH' }, sender, d), 'refreshed');
  assert.deepEqual(d.calls, ['refresh']);
});

test('SET_CONNECTION replaces, PATCH_CONNECTION merges, CLEAR_CONNECTION signs out or forgets', async () => {
  const d = deps();
  const next: Connection = { ...bearer, serverUrl: 'https://other.example.com', auth: { kind: 'api_key', key: 'sk' } };
  await handle({ type: 'SET_CONNECTION', connection: next }, sender, d);
  assert.deepEqual(d.stored(), next);
  await handle({ type: 'PATCH_CONNECTION', patch: { lastKbId: 'kb9', quickClip: false } }, sender, d);
  assert.deepEqual(d.stored(), { ...next, lastKbId: 'kb9', quickClip: false });
  await handle({ type: 'CLEAR_CONNECTION', forgetServer: false }, sender, d);
  assert.deepEqual(d.stored(), { ...next, lastKbId: 'kb9', quickClip: false, auth: { kind: 'signed_out' } });
  await handle({ type: 'CLEAR_CONNECTION', forgetServer: true }, sender, d);
  assert.equal(d.stored(), null);
});

test('TAB_ACCESS reports the url and title only under a grant', async () => {
  assert.deepEqual(await handle({ type: 'TAB_ACCESS', tabId: 1 }, sender, deps()), { url: 'https://e.com/p', title: 'P' });
  assert.deepEqual(await handle({ type: 'TAB_ACCESS', tabId: 1 }, sender, deps({ tabInfo: async () => ({}) })), { access: false });
});

test('EXTRACT injects on a reachable page and returns the draft', async () => {
  const d = deps();
  const reply = await handle({ type: 'EXTRACT', tabId: 3, mode: 'article' }, sender, d);
  assert.deepEqual(reply, { draft: { kind: 'article', title: 'P', url: 'https://e.com/p', markdown: 'x', meta: {} } });
  assert.deepEqual(d.calls, ['inject 3 article']);
});

test('EXTRACT classifies unreachable pages without injecting', async () => {
  const d = deps({ tabInfo: async () => ({ url: 'chrome://extensions', title: 'Extensions' }) });
  assert.deepEqual(await handle({ type: 'EXTRACT', tabId: 3, mode: 'article' }, sender, d), { error: 'unreachable' });
  assert.deepEqual(d.calls, []);
});

test('isUnreachableUrl', () => {
  for (const url of ['chrome://extensions', 'chrome-extension://abc/x.html', 'edge://settings', 'about:blank', 'file:///tmp/x.html', 'view-source:https://e.com', 'data:text/html,x', 'blob:https://e.com/uuid', 'devtools://devtools/x', 'chrome-untrusted://x', 'javascript:void(0)', 'https://chromewebstore.google.com/detail/x', 'https://e.com/doc.pdf', 'https://e.com/doc.PDF?x=1']) {
    assert.ok(isUnreachableUrl(url), url);
  }
  for (const url of ['https://e.com/p', 'http://kb.lan/', 'https://e.com/pdf-guide']) assert.ok(!isUnreachableUrl(url), url);
});

test('EXTRACT with an unknown url tries the injection and maps a refusal to no_access', async () => {
  const d = deps({ tabInfo: async () => ({}), inject: async () => { throw new Error('Cannot access contents of the page. Extension manifest must request permission to access the respective host.'); } });
  assert.deepEqual(await handle({ type: 'EXTRACT', tabId: 3, mode: 'select' }, sender, d), { error: 'no_access' });
  const gallery = deps({ inject: async () => { throw new Error('The extensions gallery cannot be scripted.'); } });
  assert.deepEqual(await handle({ type: 'EXTRACT', tabId: 3, mode: 'select' }, sender, gallery), { error: 'no_access' });
});

test('EXTRACT reports any other injection failure with its message', async () => {
  const d = deps({ inject: async () => { throw new Error('boom in the extractor'); } });
  assert.deepEqual(await handle({ type: 'EXTRACT', tabId: 3, mode: 'article' }, sender, d), { error: 'failed', message: 'boom in the extractor' });
});

test('EXTRACT passes a cancelled picker through', async () => {
  const d = deps({ inject: async () => ({ draft: null, reason: 'cancelled' }) });
  assert.deepEqual(await handle({ type: 'EXTRACT', tabId: 3, mode: 'select' }, sender, d), { draft: null, reason: 'cancelled' });
});

test('quick clip without lastKbId writes the editor intent', () => {
  assert.equal(quickClipIntent({ ...bearer, quickClip: true, lastKbId: null }, 'article'), 'clip:article');
  assert.equal(quickClipIntent({ ...bearer, quickClip: true, lastKbId: 'kb1' }, 'article'), 'quick:article');
  assert.equal(quickClipIntent({ ...bearer, quickClip: false, lastKbId: 'kb1' }, 'select'), 'clip:select');
  assert.equal(quickClipIntent(null, 'select'), 'clip:select');
});

test('GET_COMMANDS returns the bindings', async () => {
  assert.deepEqual(await handle({ type: 'GET_COMMANDS' }, sender, deps()), [{ name: 'quick-ask', description: 'Ask about this page', shortcut: 'Alt+Shift+A' }]);
});

test('clipFromGesture opens the panel first, writes a quick intent early, then the draft together with its intent', async () => {
  const d = deps();
  await clipFromGesture(3, 'article', d);
  assert.deepEqual(d.calls, ['open 3', 'inject 3 article', 'intent 3 quick:article', 'clip 3 quick:article P']);
});

test('clipFromGesture writes an editor clip intent only with its draft', async () => {
  const d = deps({ load: async () => ({ ...bearer, quickClip: false }) });
  await clipFromGesture(3, 'article', d);
  assert.deepEqual(d.calls, ['open 3', 'inject 3 article', 'clip 3 clip:article P']);
});

test('clipFromGesture on an unreachable page opens the panel and reports the page without an intent', async () => {
  const d = deps({ tabInfo: async () => ({ url: 'chrome://newtab', title: 'New tab' }) });
  await clipFromGesture(3, 'article', d);
  assert.deepEqual(d.calls, ['open 3', 'error 3 {"error":"unreachable","url":"chrome://newtab"}']);
});

test('clipFromGesture clears its intent and reports the failure when the injection fails', async () => {
  const failing = deps({ inject: async () => { throw new Error('Cannot access contents of the page.'); }, readIntent: async () => 'quick:article' });
  await clipFromGesture(3, 'article', failing);
  assert.deepEqual(failing.calls, ['open 3', 'intent 3 quick:article', 'intent 3 null', 'error 3 {"error":"no_access","url":"https://e.com/p"}']);
  const editor = deps({ load: async () => ({ ...bearer, quickClip: false }), inject: async () => { throw new Error('boom'); } });
  await clipFromGesture(3, 'article', editor);
  assert.deepEqual(editor.calls, ['open 3', 'error 3 {"error":"failed","message":"boom","url":"https://e.com/p"}'], 'an editor intent was never written, so nothing is cleared');
});

test('clipFromGesture clears its intent silently when the picker is cancelled', async () => {
  const cancelled = deps({ inject: async () => ({ draft: null, reason: 'cancelled' }), readIntent: async () => 'quick:select' });
  await clipFromGesture(3, 'select', cancelled);
  assert.deepEqual(cancelled.calls, ['open 3', 'intent 3 quick:select', 'intent 3 null']);
});

test('askFromGesture opens the panel, writes the ask intent and the selection', async () => {
  const d = deps();
  await askFromGesture(3, d, 'from menu');
  assert.deepEqual(d.calls, ['open 3', 'intent 3 ask', 'composer 3 from menu']);
  const fromPage = deps();
  await askFromGesture(3, fromPage);
  assert.deepEqual(fromPage.calls, ['open 3', 'intent 3 ask', 'composer 3 picked']);
  const unreadable = deps({ pageSelection: async () => { throw new Error('Cannot access'); } });
  await askFromGesture(3, unreadable);
  assert.deepEqual(unreadable.calls, ['open 3', 'intent 3 ask']);
});

test('noteFromGesture opens the panel and writes the note intent', async () => {
  const d = deps();
  await noteFromGesture(3, d);
  assert.deepEqual(d.calls, ['open 3', 'intent 3 note']);
});
