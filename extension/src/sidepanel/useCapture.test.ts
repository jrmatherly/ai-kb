import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ref } from 'vue';
import { installFakeChrome, tick } from '../../test/fakeChrome';
import { ApiError, type ApiClient } from '../shared/api';
import { pollParse } from '../shared/capture';
import type { ExtractReply, Request, TabAccessReply } from '../shared/messages';
import { captureScope, type ClipError } from '../shared/session';
import type { Capture, CaptureDraft, Connection, Intent, Knowledge } from '../shared/types';
import { type Editor, useCapture } from './useCapture';

const article: CaptureDraft = { kind: 'article', title: 'Page', url: 'https://e.com/p', markdown: `# Page\n\n${'body '.repeat(20)}`, meta: {} };
const saved = (knowledgeId: string, url = 'https://e.com/p', kbId = 'kb1'): Capture =>
  ({ knowledgeId, kbId, title: 'Page', url, savedAt: '2026-10-16T00:00:00Z', status: 'completed' });

type Opts = {
  connection?: Partial<Connection> | null;
  connectionError?: Error;
  draft?: CaptureDraft;
  intent?: Intent;
  recent?: Capture[];
  extract?: ExtractReply;
  access?: TabAccessReply;
  saveError?: Error;
  poll?: { status: Capture['status']; message?: string };
  realPoll?: boolean;
  knowledge?: Knowledge | Error;
  kbError?: Error;
  clipError?: ClipError;
};

function harness(opts: Opts = {}) {
  const { session, local, listeners } = installFakeChrome();
  if (opts.draft) session.set('draft:7', opts.draft);
  if (opts.intent) session.set('intent:7', opts.intent);
  if (opts.clipError) session.set('clipError:7', opts.clipError);
  const connection: Connection | null = opts.connection === null
    ? null
    : { serverUrl: 'https://kb.example.com', auth: { kind: 'api_key', key: 'sk' }, tenantId: null, lastKbId: 'kb1', quickClip: true, ...opts.connection };
  if (connection) local.set('connection', connection);
  for (const c of opts.recent ?? []) local.set(`capture:${c.knowledgeId}`, { ...c, scope: captureScope(connection ?? { serverUrl: 'https://other', tenantId: null }) });
  const saves: unknown[][] = [];
  const sent: Request[] = [];
  const resyncs: Capture[][] = [];
  const gets: string[] = [];
  const errors: unknown[] = [];
  const access = ref<TabAccessReply>(opts.access ?? { url: 'https://e.com/p', title: 'Page' });
  const api = {
    getKnowledge: async (id: string) => {
      gets.push(id);
      await tick();
      if (opts.knowledge instanceof Error) throw opts.knowledge;
      return opts.knowledge ?? { id, title: 'T', parse_status: 'completed' };
    },
    listKnowledgeBases: async () => { if (opts.kbError) throw opts.kbError; return [{ id: 'kb1', name: 'Docs', type: 'document' }]; },
    listSharedKnowledgeBases: async () => [{ knowledge_base: { id: 'kb2', name: 'Team' }, permission: 'editor' }],
  } as unknown as ApiClient;
  let now = 0;
  const capture = useCapture({
    api,
    send: async <T>(request: Request) => {
      sent.push(request);
      return (request.type === 'EXTRACT' ? (opts.extract ?? { draft: article }) : null) as T;
    },
    tabId: () => 7,
    tabAccess: () => access.value,
    loadConnection: async () => { if (opts.connectionError) throw opts.connectionError; return connection; },
    onError: (e) => errors.push(e),
    flow: {
      saveCapture: async (a, draft, kbId, existing) => {
        saves.push([a, draft, kbId, existing]);
        await tick();
        if (opts.saveError) throw opts.saveError;
        return { knowledgeId: existing?.knowledgeId ?? 'k9', kbId: existing?.kbId ?? kbId, title: draft.title, url: draft.url, savedAt: '2026-10-17T00:00:00Z', status: 'pending' };
      },
      pollParse: opts.realPoll ? pollParse : async () => opts.poll ?? { status: 'completed' },
      resyncCaptures: async (_a, list) => { resyncs.push(list); await tick(); return list.map((c) => ({ ...c, status: 'completed' as const })); },
      clock: { sleep: (ms) => { now += ms; return tick(); }, now: () => now },
    },
  });
  const stored = () => [...local.entries()].filter(([key]) => key.startsWith('capture:')).map(([, c]) => c as Capture);
  return { capture, api, session, local, listeners, access, stored, saves, sent, resyncs, gets, errors };
}

function phase<P extends Editor['phase']>(c: ReturnType<typeof useCapture>, p: P): Extract<Editor, { phase: P }> {
  assert.equal(c.state.editor.phase, p);
  return c.state.editor as Extract<Editor, { phase: P }>;
}

test('loadFromSession opens the draft, finds the earlier capture of its URL and clears the session keys', async () => {
  const { capture, session, saves } = harness({ draft: article, intent: 'clip:article', recent: [saved('k0', 'https://e.com/p/')] });
  await capture.loadFromSession();
  const editor = phase(capture, 'editing');
  assert.deepEqual(editor.draft, article);
  assert.equal(editor.existing?.knowledgeId, 'k0');
  assert.equal(editor.duplicate?.knowledgeId, 'k0');
  assert.deepEqual([capture.state.intent, capture.state.intentDraft], ['clip:article', true]);
  assert.equal(session.has('draft:7'), false);
  assert.equal(session.has('intent:7'), false);
  assert.equal(saves.length, 0);
  assert.deepEqual(capture.state.kbs.map((kb) => kb.id), ['kb1', 'kb2']);
});

test('a failed connection read leaves the clip in the session', async () => {
  const { capture, session } = harness({ draft: article, intent: 'quick:article', connectionError: new Error('storage down') });
  await assert.rejects(capture.loadFromSession(), /storage down/);
  assert.deepEqual(session.get('draft:7'), article);
  assert.equal(session.get('intent:7'), 'quick:article');
});

test('a quick intent without lastKbId opens the editor', async () => {
  const { capture, saves } = harness({ draft: article, intent: 'quick:article', connection: { lastKbId: null } });
  await capture.loadFromSession();
  assert.deepEqual(phase(capture, 'editing').draft, article);
  assert.equal(saves.length, 0);
});

test('a quick intent with quick clip off opens the editor', async () => {
  const { capture, saves } = harness({ draft: article, intent: 'quick:article', connection: { quickClip: false } });
  await capture.loadFromSession();
  assert.deepEqual(phase(capture, 'editing').draft, article);
  assert.equal(saves.length, 0);
});

test('a quick intent with lastKbId saves to it at once as a quick save and loads the bases', async () => {
  const { capture, api, saves } = harness({ draft: article, intent: 'quick:article' });
  await capture.loadFromSession();
  assert.deepEqual(saves, [[api, article, 'kb1', undefined]]);
  const editor = phase(capture, 'saved');
  assert.deepEqual([editor.existing.knowledgeId, editor.quick], ['k9', true]);
  assert.deepEqual(capture.state.kbs.map((kb) => kb.id), ['kb1', 'kb2']);
  capture.cancel();
  assert.equal(capture.state.editor.phase, 'idle');
});

test('a quick clip on a URL with an earlier capture creates a new entry and leaves the earlier one', async () => {
  const { capture, saves, stored } = harness({ draft: article, intent: 'quick:select', recent: [saved('k0')] });
  await capture.loadFromSession();
  assert.equal(saves[0][3], undefined);
  assert.deepEqual(stored().map((c) => c.knowledgeId).sort(), ['k0', 'k9']);
  assert.deepEqual(capture.state.recent.map((c) => c.knowledgeId), ['k9', 'k0']);
});

test('a note intent without a draft leaves the editor idle for the view to start', async () => {
  const { capture } = harness({ intent: 'note' });
  await capture.loadFromSession();
  assert.deepEqual([capture.state.intent, capture.state.intentDraft, capture.state.intentSeq], ['note', false, 1]);
  assert.equal(capture.state.editor.phase, 'idle');
});

test('a draft written for this tab loads it, and stop() ends the subscription', async () => {
  const { capture, listeners } = harness();
  await chrome.storage.session.set({ 'intent:7': 'clip:article', 'draft:7': article });
  while (capture.state.editor.phase !== 'editing') await tick();
  assert.deepEqual(phase(capture, 'editing').draft, article);
  await chrome.storage.session.set({ 'draft:8': article });
  capture.stop();
  assert.equal(listeners.size, 0);
});

test('a quick intent read before its draft is cleared, and the clip written with it later saves', async () => {
  const { capture, session, saves } = harness({ intent: 'quick:select' });
  await capture.loadFromSession();
  assert.equal(session.has('intent:7'), false);
  assert.equal(capture.state.editor.phase, 'idle');
  session.set('intent:7', 'quick:select').set('draft:7', article);
  await capture.loadFromSession();
  assert.equal(saves.length, 1);
  assert.equal(phase(capture, 'saved').existing.knowledgeId, 'k9');
});

test('concurrent loads take the draft once', async () => {
  const { capture, saves } = harness({ draft: article, intent: 'quick:article' });
  await Promise.all([capture.loadFromSession(), capture.loadFromSession()]);
  assert.equal(saves.length, 1);
});

test('kbId starts as the stored lastKbId', async () => {
  const { capture } = harness({ connection: { lastKbId: 'kb1' } });
  await capture.loadFromSession();
  assert.equal(capture.state.kbId, 'kb1');
});

test('a failed knowledge base load shows in the editor instead of rejecting', async () => {
  const { capture } = harness({ draft: article, kbError: new ApiError(500, 'db down') });
  await capture.loadFromSession();
  assert.deepEqual(phase(capture, 'editing').saveError, { text: 'Could not load knowledge bases: db down' });
  assert.deepEqual(capture.state.kbs, []);
});

test('startClip maps the extract errors to page states', async () => {
  for (const [extract, pageError] of [
    [{ error: 'unreachable'}, { kind: 'unreachable' }],
    [{ error: 'no_access'}, { kind: 'cold' }],
    [{ error: 'failed', message: 'boom'}, { kind: 'failed', message: 'boom' }],
  ] as const) {
    const { capture, sent } = harness({ extract });
    await capture.startClip('article');
    assert.deepEqual(capture.state.pageError, pageError);
    assert.deepEqual(capture.pageState.value, pageError);
    assert.equal(capture.state.editor.phase, 'idle');
    assert.deepEqual(sent, [{ type: 'EXTRACT', tabId: 7, mode: 'article' }]);
  }
});

test('startClip opens the extracted draft with the earlier capture of its URL', async () => {
  const { capture } = harness({ recent: [saved('k0')] });
  await capture.startClip('select');
  const editor = phase(capture, 'editing');
  assert.deepEqual(editor.draft, article);
  assert.equal(editor.existing?.knowledgeId, 'k0');
  assert.equal(capture.state.pageError, null);
  assert.deepEqual(capture.state.kbs.map((kb) => kb.id), ['kb1', 'kb2']);
});

test('startClip on a URL captured under another scope offers no existing or duplicate', async () => {
  const { capture, local } = harness({ recent: [saved('k0')] });
  local.set('capture:k0', { ...saved('k0'), scope: captureScope({ serverUrl: 'https://elsewhere', tenantId: null }) });
  await capture.startClip('select');
  const editor = phase(capture, 'editing');
  assert.equal(editor.existing, undefined);
  assert.equal(editor.duplicate, undefined);
});

test('startClip on a tab without access shows the cold hint without asking the worker', async () => {
  const { capture, sent } = harness({ access: { access: false } });
  await capture.startClip('article');
  assert.deepEqual(capture.pageState.value, { kind: 'cold' });
  assert.deepEqual(sent, []);
});

test('pageState follows the tab access, and an extract error lasts only while the same reply stands', async () => {
  const { capture, access } = harness({ extract: { error: 'unreachable' } });
  assert.deepEqual(capture.pageState.value, { kind: 'ok' });
  await capture.startClip('article');
  assert.deepEqual(capture.pageState.value, { kind: 'unreachable' });
  access.value = { url: 'https://e.com/next', title: 'Next' };
  assert.deepEqual(capture.pageState.value, { kind: 'ok' });
  access.value = { access: false };
  assert.deepEqual(capture.pageState.value, { kind: 'cold' });
});

test('save(false) updates the existing capture and save(true) creates a new one', async () => {
  const { capture, saves } = harness({ draft: article, recent: [saved('k0')] });
  await capture.loadFromSession();
  await capture.save(false);
  assert.equal((saves[0][3] as Capture).knowledgeId, 'k0');
  await capture.save(true);
  assert.equal(saves[1][3], undefined);
  assert.equal(phase(capture, 'saved').existing.knowledgeId, 'k9');
});

test('save keeps the editor on the saved capture, remembers it and patches lastKbId when it changed', async () => {
  const { capture, sent, stored } = harness({ draft: article });
  await capture.loadFromSession();
  capture.state.kbId = 'kb2';
  await capture.save(false);
  assert.equal(phase(capture, 'saved').existing.knowledgeId, 'k9');
  assert.deepEqual(sent, [{ type: 'PATCH_CONNECTION', patch: { lastKbId: 'kb2' } }]);
  assert.deepEqual(stored().map((c) => c.knowledgeId), ['k9']);
  assert.deepEqual(capture.state.recent.map((c) => c.knowledgeId), ['k9']);
  await capture.save(false);
  assert.equal(sent.length, 1, 'no second patch for the same base');
});

test('cancel during an in-flight save leaves the editor idle', async () => {
  const { capture, saves, stored } = harness({ draft: article });
  await capture.loadFromSession();
  const saving = capture.save(false);
  assert.equal(capture.state.editor.phase, 'saving');
  capture.cancel();
  await saving;
  assert.equal(saves.length, 1);
  assert.equal(capture.state.editor.phase, 'idle');
  assert.deepEqual(stored(), []);
});

test('a poll in progress stops after cancel()', async () => {
  const { capture, gets } = harness({ draft: article, realPoll: true, knowledge: { id: 'k9', title: 'T', parse_status: 'processing' } });
  await capture.loadFromSession();
  const saving = capture.save(false);
  while (gets.length < 2) await tick();
  capture.cancel();
  const polled = gets.length;
  await saving;
  await tick(20);
  assert.equal(gets.length, polled);
  assert.equal(capture.state.editor.phase, 'idle');
});

test('save refuses an over-limit draft without calling the api, and trim lets it through', async () => {
  const { capture, saves } = harness();
  await capture.loadFromSession();
  capture.newNote();
  phase(capture, 'editing').draft.markdown = 'x'.repeat(200001);
  assert.equal(capture.charCount.value, 200001);
  await capture.save(false);
  assert.equal(phase(capture, 'editing').limit?.reason, 'too_long');
  assert.equal(saves.length, 0);
  capture.trim();
  assert.equal(capture.charCount.value, 200000);
  assert.equal(phase(capture, 'editing').limit, null);
  await capture.save(false);
  assert.equal(saves.length, 1);
});

test('the counter measures the whole body, source lines included', async () => {
  const { capture } = harness({ draft: { ...article, markdown: '\u{1F600}'.repeat(10) } });
  await capture.loadFromSession();
  assert.equal(capture.charCount.value, 'Source: https://e.com/p\n\n'.length + 10);
});

test('a 10-character note saves', async () => {
  const { capture, saves } = harness();
  await capture.loadFromSession();
  capture.newNote();
  phase(capture, 'editing').draft.markdown = '0123456789';
  await capture.save(false);
  assert.equal(saves.length, 1);
  assert.equal(capture.state.editor.phase, 'saved');
});

test('save without a knowledge base asks for one', async () => {
  const { capture, saves } = harness({ connection: { lastKbId: null } });
  await capture.loadFromSession();
  capture.newNote();
  phase(capture, 'editing').draft.markdown = 'hello';
  await capture.save(false);
  assert.deepEqual(phase(capture, 'editing').saveError, { text: 'Choose a knowledge base', field: 'kb' });
  assert.equal(saves.length, 0);
});

test('save maps server failures to editor errors', async () => {
  for (const [error, saveError] of [
    [new ApiError(403, 'forbidden'), { text: 'This key cannot write to this knowledge base', field: 'kb' }],
    [new ApiError(401, 'signed_out'), { text: 'Signed out elsewhere' }],
    [new ApiError(500, 'disk full'), { text: 'disk full' }],
  ] as const) {
    const { capture } = harness({ draft: article, saveError: error, recent: [saved('k0')] });
    await capture.loadFromSession();
    await capture.save(false);
    const editor = phase(capture, 'editing');
    assert.deepEqual(editor.saveError, saveError);
    assert.equal(editor.duplicate?.knowledgeId, 'k0', 'the duplicate offer survives a failed save');
  }
});

test('newNote opens an empty dated note', async () => {
  const { capture } = harness();
  capture.newNote();
  const { draft } = phase(capture, 'editing');
  assert.match(draft.title, /^Note \d{4}-\d{2}-\d{2}$/);
  assert.equal(draft.kind, 'note');
  assert.equal(draft.url, '');
  assert.equal(draft.markdown, '');
});

test('editCapture opens the stored content as a note, keeping its Source block', async () => {
  const knowledge: Knowledge = { id: 'k0', title: 'Old', parse_status: 'completed', metadata: '{"content":"Source: https://e.com/p\\n\\n# old","status":"publish"}' };
  const { capture } = harness({ knowledge });
  const c = saved('k0');
  await capture.editCapture(c);
  const editor = phase(capture, 'editing');
  assert.equal(editor.draft.markdown, 'Source: https://e.com/p\n\n# old');
  assert.equal(editor.draft.url, '');
  assert.equal(editor.draft.kind, 'note');
  assert.equal(editor.draft.title, 'Old');
  assert.deepEqual(editor.existing, c);
  assert.equal(editor.duplicate, undefined);
});

test('editCapture of a knowledge the server no longer has reports it and stays idle', async () => {
  const { capture } = harness({ knowledge: new ApiError(404, 'knowledge not found') });
  await capture.editCapture(saved('k0'));
  assert.deepEqual(capture.state.pageError, { kind: 'failed', message: 'knowledge not found' });
  assert.equal(capture.state.editor.phase, 'idle');
});

test('save polls the parse in this page and records a failure with its message', async () => {
  const { capture, stored } = harness({ draft: article, poll: { status: 'failed', message: 'bad' } });
  await capture.loadFromSession();
  await capture.save(false);
  assert.deepEqual(phase(capture, 'saved').lastStatus, { status: 'failed', message: 'bad' });
  assert.deepEqual(stored().map((c) => c.status), ['failed']);
  assert.deepEqual(capture.state.recent.map((c) => c.status), ['failed']);
});

test('resync refreshes the recent captures once and stores the result', async () => {
  const { capture, resyncs, stored } = harness({ recent: [{ ...saved('k1'), status: 'pending' }] });
  await capture.resync();
  assert.equal(resyncs.length, 1);
  assert.deepEqual(stored().map((c) => c.status), ['completed']);
  assert.deepEqual(capture.state.recent.map((c) => c.status), ['completed']);
});

test('a save during a resync keeps both the new capture and the synced statuses', async () => {
  const { capture, stored } = harness({ connection: { lastKbId: 'kb1' }, recent: [{ ...saved('k1', 'https://e.com/other'), status: 'pending' }] });
  await capture.loadFromSession();
  capture.newNote();
  phase(capture, 'editing').draft.markdown = 'hello';
  await Promise.all([capture.resync(), capture.save(false)]);
  assert.deepEqual(stored().map((c) => [c.knowledgeId, c.status]).sort(), [['k1', 'completed'], ['k9', 'completed']]);
});

test('cancel closes the editor', async () => {
  const { capture } = harness();
  capture.newNote();
  capture.cancel();
  assert.equal(capture.state.editor.phase, 'idle');
  assert.equal(capture.charCount.value, 0);
});

test('a repeated intent bumps intentSeq, and opening the editor keeps the intent', async () => {
  const { capture, session } = harness({ intent: 'clip:article' });
  await capture.loadFromSession();
  assert.deepEqual([capture.state.intent, capture.state.intentSeq], ['clip:article', 1]);
  session.set('intent:7', 'clip:article');
  await capture.loadFromSession();
  assert.deepEqual([capture.state.intent, capture.state.intentSeq], ['clip:article', 2]);
  session.set('intent:7', 'quick:article');
  await capture.loadFromSession();
  assert.equal(capture.state.intent, 'quick:article');
  await capture.editCapture(saved('k0'));
  assert.equal(capture.state.intent, 'quick:article');
});

test('a clip error from a menu or command shows as the page state, once', async () => {
  for (const [clipError, pageError] of [
    [{ error: 'unreachable', url: 'https://e.com/p' }, { kind: 'unreachable' }],
    [{ error: 'no_access', url: 'https://e.com/p' }, { kind: 'cold' }],
    [{ error: 'failed', message: 'boom', url: 'https://e.com/p' }, { kind: 'failed', message: 'boom' }],
  ] as const) {
    const { capture, session } = harness({ clipError });
    await capture.loadFromSession();
    assert.deepEqual(capture.pageState.value, pageError);
    assert.equal(session.has('clipError:7'), false);
    assert.equal(capture.state.editor.phase, 'idle');
  }
});

test('a clip error written while the panel is open reaches the page state', async () => {
  const { capture } = harness();
  await chrome.storage.session.set({ 'clipError:7': { error: 'unreachable', url: 'https://e.com/p' } });
  for (let i = 0; i < 20 && capture.pageState.value.kind === 'ok'; i++) await tick();
  assert.deepEqual(capture.pageState.value, { kind: 'unreachable' });
});

test('a clip error waits for the tab it describes and is dropped once the tab shows another page', async () => {
  const { capture, access } = harness({ access: { access: false }, clipError: { error: 'unreachable', url: 'chrome://newtab' } });
  await capture.loadFromSession();
  assert.deepEqual(capture.pageState.value, { kind: 'cold' }, 'the tab is not known yet');
  access.value = { url: 'chrome://newtab', title: 'New tab' };
  assert.deepEqual(capture.pageState.value, { kind: 'unreachable' });
  access.value = { url: 'https://e.com/other', title: 'Other' };
  assert.deepEqual(capture.pageState.value, { kind: 'ok' });
});

test('editing a Recent entry locks the picker to its base and leaves lastKbId alone', async () => {
  const { capture, saves, sent } = harness({ knowledge: { id: 'k0', title: 'Old', parse_status: 'completed', metadata: { content: 'old body text' } } });
  await capture.loadFromSession();
  assert.equal(capture.state.kbId, 'kb1');
  await capture.editCapture(saved('k0', 'https://e.com/p', 'kb2'));
  assert.equal(capture.lockedKbId.value, 'kb2');
  capture.state.kbId = 'kb1';
  await capture.save(false);
  assert.equal((saves[0][3] as Capture).kbId, 'kb2');
  assert.deepEqual(sent, [], 'an update writes no lastKbId');
  assert.equal(capture.lockedKbId.value, 'kb2', 'the saved entry stays locked to its base');
});

test('the picker is free for a new entry and for Save as new on a duplicate', async () => {
  const { capture } = harness({ draft: article, recent: [saved('k0', 'https://e.com/p', 'kb2')] });
  await capture.loadFromSession();
  assert.equal(phase(capture, 'editing').duplicate?.knowledgeId, 'k0');
  assert.equal(capture.lockedKbId.value, null);
  capture.newNote();
  assert.equal(capture.lockedKbId.value, null);
});
