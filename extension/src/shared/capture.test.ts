import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import type { ApiClient } from './api';
import {
  MAX_CONTENT_CHARS, SERVER_REJECT_PATTERNS, checkLimits, codePoints, draftToBody, findRecentByUrl, pollParse, resyncCaptures, saveCapture, trimDraft, trimToLimit, writableKnowledgeBases,
} from './capture';
import type { Capture, CaptureDraft, Knowledge, KnowledgeBase, SharedKnowledgeBase } from './types';

const draft: CaptureDraft = { kind: 'article', title: 'T', url: 'https://e.com/p', markdown: '# x', meta: { author: 'A' } };

test('the reject patterns match the server source, pattern for pattern', () => {
  const go = readFileSync(new URL('../../../internal/utils/security.go', import.meta.url), 'utf8');
  const block = go.slice(go.indexOf('xssPatterns = []*regexp.Regexp{'), go.indexOf('}', go.indexOf('xssPatterns = []*regexp.Regexp{')));
  const serverSources = [...block.matchAll(/regexp\.MustCompile\(`\(\?i\)(.*?)`\)/g)].map((m) => m[1]);
  assert.deepEqual(SERVER_REJECT_PATTERNS.map((p) => p.source.replace(/\\\//g, '/')), serverSources);
  assert.ok(SERVER_REJECT_PATTERNS.every((p) => p.flags === 'i'));
});

const note = (markdown: string, title = 'T'): CaptureDraft => ({ kind: 'note', title, url: '', markdown, meta: {} });

test('checkLimits measures the whole body for the limit and the markdown alone for the minimum', () => {
  assert.deepEqual(checkLimits(note('x'.repeat(200001))), { ok: false, reason: 'too_long', length: 200001 });
  assert.deepEqual(checkLimits({ ...draft, markdown: 'x'.repeat(10) }), { ok: false, reason: 'too_short', length: 45 });
  assert.deepEqual(checkLimits(note('x'.repeat(10))), { ok: true });
  assert.deepEqual(checkLimits(note('x'.repeat(60), 'Click onclick=x here')), { ok: false, reason: 'bad_title', length: 60, cleanedTitle: 'Click x here' });
  assert.deepEqual(checkLimits(note('x'.repeat(60), 'Bad\u0000title\f')), { ok: false, reason: 'bad_title', length: 60, cleanedTitle: 'Badtitle' });
  assert.deepEqual(checkLimits({ ...draft, kind: 'select', markdown: ' \n\t ' }), { ok: false, reason: 'too_short', length: 39 });
  assert.deepEqual(checkLimits(note('\u{1F600}'.repeat(10))), { ok: true });
  assert.deepEqual(checkLimits(note('\u{1F600}'.repeat(200001))), { ok: false, reason: 'too_long', length: 200001 });
});

test('trimDraft keeps the whole body within the limit and cuts at a line break when one is near', () => {
  const trimmed = trimDraft({ ...draft, markdown: 'y'.repeat(250000) });
  assert.equal(codePoints(draftToBody(trimmed).content), 200000);
  const lines = Array.from({ length: 3000 }, (_, i) => `line ${i} ${'z'.repeat(90)}`).join('\n');
  const atBreak = trimDraft({ ...draft, markdown: lines });
  assert.ok(codePoints(draftToBody(atBreak).content) <= 200000);
  assert.ok(atBreak.markdown.endsWith('z'), 'ends on a whole line');
  assert.ok(!atBreak.markdown.endsWith('\n'));
  assert.equal(trimDraft(draft), draft);
});

test('trimToLimit cuts at the last line break in the final 2,000 characters, else at the limit', () => {
  const limit = MAX_CONTENT_CHARS;
  assert.equal(trimToLimit(`${'x'.repeat(199990)}\n${'x'.repeat(19)}`, limit), 'x'.repeat(199990));
  assert.equal(trimToLimit('x'.repeat(200010), limit), 'x'.repeat(200000));
  assert.equal(trimToLimit(`${'x'.repeat(197000)}\n${'x'.repeat(3010)}`, limit), `${'x'.repeat(197000)}\n${'x'.repeat(2999)}`);
  assert.equal(trimToLimit(`${'\u{1F600}'.repeat(199990)}\n${'x'.repeat(19)}`, limit), '\u{1F600}'.repeat(199990));
  assert.equal(trimToLimit('\u{1F600}'.repeat(200010), limit), '\u{1F600}'.repeat(200000));
  assert.equal(trimToLimit('short\n', limit), 'short\n');
  assert.equal(trimToLimit('x'.repeat(200000), limit), 'x'.repeat(200000));
});

test('trimToLimit with no room left returns nothing', () => {
  assert.equal(trimToLimit('anything', 0), '');
  assert.equal(trimToLimit('anything', -5), '');
});

test('draftToBody prefixes the source lines', () => {
  assert.deepEqual(draftToBody(draft), { title: 'T', content: 'Source: https://e.com/p\nAuthor: A\n\n# x' });
  assert.equal(draftToBody({ ...draft, kind: 'note', url: '', meta: {} }).content, '# x');
  assert.equal(draftToBody({ ...draft, meta: { author: 'A', published: '2026-10-17' } }).content, 'Source: https://e.com/p\nAuthor: A\nPublished: 2026-10-17\n\n# x');
});

test('writableKnowledgeBases keeps document bases the user can write to', () => {
  const own: KnowledgeBase[] = [{ id: 'a', name: 'A', type: 'document' }, { id: 'f', name: 'F', type: 'faq' }, { id: 'n', name: 'N' }];
  const shared: SharedKnowledgeBase[] = [
    { knowledge_base: { id: 's', name: 'S', type: 'document' }, permission: 'editor' },
    { knowledge_base: { id: 'v', name: 'V' }, permission: 'viewer' },
    { knowledge_base: { id: 'a', name: 'A', type: 'document' }, permission: 'admin' },
    { knowledge_base: null, permission: 'admin' },
  ];
  assert.deepEqual(writableKnowledgeBases(own, shared).map((kb) => kb.id), ['a', 'n', 's']);
});

type FakeApi = ApiClient & { calls: string[] };

function fakeApi(statuses: Array<Partial<Knowledge>> = []): FakeApi {
  const calls: string[] = [];
  const fake: Pick<ApiClient, 'createManual' | 'updateManual' | 'getKnowledge' | 'getKnowledgeBatch'> & { calls: string[] } = {
    calls,
    createManual: async (kbId, body) => { calls.push(`create ${kbId} ${body.title}`); return { id: 'k1', title: body.title, parse_status: 'pending' }; },
    updateManual: async (id, body) => { calls.push(`update ${id} ${body.title}`); },
    getKnowledge: async (id) => { calls.push(`get ${id}`); const next = statuses.shift() ?? {}; return { id, title: 'T', parse_status: 'pending', ...next }; },
    getKnowledgeBatch: async (ids) => { calls.push(`batch ${ids.join(',')}`); return ids.map((id) => ({ id, title: 'T', parse_status: 'completed' })); },
  };
  return fake as FakeApi;
}

test('saveCapture creates or updates and returns a pending capture', async () => {
  const api = fakeApi();
  const created = await saveCapture(api, draft, 'kb1');
  assert.deepEqual(api.calls, ['create kb1 T']);
  assert.equal(created.knowledgeId, 'k1');
  assert.equal(created.kbId, 'kb1');
  assert.equal(created.status, 'pending');
  assert.equal(created.url, 'https://e.com/p');
  const existing: Capture = { knowledgeId: 'k0', kbId: 'kbA', title: 'old', url: draft.url, savedAt: '2026-10-16T00:00:00Z', status: 'completed' };
  const updated = await saveCapture(api, draft, 'kb1', existing);
  assert.deepEqual(api.calls, ['create kb1 T', 'update k0 T']);
  assert.equal(updated.knowledgeId, 'k0');
  assert.equal(updated.kbId, 'kbA');
  const note = await saveCapture(api, { ...draft, kind: 'note', url: '' }, 'kb1', existing);
  assert.equal(note.url, draft.url);
});

// A clock that only advances when something sleeps.
function sleeper() {
  const slept: number[] = [];
  let clock = 0;
  return { slept, sleep: async (ms: number) => { slept.push(ms); clock += ms; }, now: () => clock };
}

test('pollParse resolves on completed after the pending states', async () => {
  const api = fakeApi([{ parse_status: 'pending' }, { parse_status: 'processing' }, { parse_status: 'completed' }]);
  const timing = sleeper();
  assert.deepEqual(await pollParse(api, 'k1', { intervalMs: 2000, timeoutMs: 120000 }, timing), { status: 'completed' });
  assert.deepEqual(timing.slept, [2000, 2000]);
});

test('pollParse reports a message only for a failure', async () => {
  assert.deepEqual(await pollParse(fakeApi([{ parse_status: 'completed', error_message: 'old failure' }]), 'k1', { intervalMs: 2000, timeoutMs: 120000 }, sleeper()), { status: 'completed' });
});

test('pollParse treats finalizing as completed, since the document is already queryable', async () => {
  assert.deepEqual(await pollParse(fakeApi([{ parse_status: 'finalizing' }]), 'k1', { intervalMs: 2000, timeoutMs: 120000 }, sleeper()), { status: 'completed' });
});

test('pollParse times out on wall-clock time, not request count', async () => {
  const timing = sleeper();
  const slowApi = fakeApi();
  const original = slowApi.getKnowledge;
  slowApi.getKnowledge = async (id) => { timing.sleep(3000); return original(id); };
  assert.deepEqual(await pollParse(slowApi, 'k1', { intervalMs: 2000, timeoutMs: 120000 }, timing), { status: 'timeout' });
  assert.ok(timing.slept.length < 60, `polled ${timing.slept.length} times`);
});

test('pollParse reports failed with the message, maps deleting to cancelled, and times out', async () => {
  const opts = { intervalMs: 2000, timeoutMs: 120000 };
  assert.deepEqual(await pollParse(fakeApi([{ parse_status: 'failed', error_message: 'bad pdf' }]), 'k1', opts, sleeper()), { status: 'failed', message: 'bad pdf' });
  assert.deepEqual(await pollParse(fakeApi([{ parse_status: 'cancelled' }]), 'k1', opts, sleeper()), { status: 'cancelled' });
  assert.deepEqual(await pollParse(fakeApi([{ parse_status: 'deleting' }]), 'k1', opts, sleeper()), { status: 'cancelled' });
  const timing = sleeper();
  assert.deepEqual(await pollParse(fakeApi(), 'k1', opts, timing), { status: 'timeout' });
  assert.equal(timing.slept.length, 60);
});

test('pollParse stops before its next request once aborted', async () => {
  const api = fakeApi();
  const stop = new AbortController();
  const clock = { sleep: async () => { stop.abort(new Error('left')); }, now: () => 0 };
  await assert.rejects(pollParse(api, 'k1', { intervalMs: 2000, timeoutMs: 120000 }, clock, stop.signal), /left/);
  assert.deepEqual(api.calls, ['get k1']);
});

const capture = (knowledgeId: string, url: string, status: Capture['status'] = 'completed'): Capture =>
  ({ knowledgeId, kbId: 'kb1', title: 'T', url, savedAt: '2026-10-17T00:00:00Z', status });

test('findRecentByUrl ignores a trailing slash and the fragment', () => {
  const list = [capture('k1', 'https://e.com/p')];
  assert.equal(findRecentByUrl(list, 'https://e.com/p/#top')?.knowledgeId, 'k1');
  assert.equal(findRecentByUrl(list, 'https://e.com/q'), undefined);
  assert.equal(findRecentByUrl(list, ''), undefined);
});

test('resyncCaptures asks about the pending and timed-out ones and drops what the server no longer has', async () => {
  const api = fakeApi();
  api.getKnowledgeBatch = async (ids) => { api.calls.push(`batch ${ids.join(',')}`); return ids.filter((id) => id !== 'k3').map((id) => ({ id, title: 'T', parse_status: 'completed' })); };
  const list = [capture('k1', 'https://e.com/1', 'pending'), capture('k2', 'https://e.com/2', 'completed'), capture('k3', 'https://e.com/3', 'timeout'), capture('k4', 'https://e.com/4', 'timeout')];
  const synced = await resyncCaptures(api, list);
  assert.deepEqual(api.calls, ['batch k1,k3,k4']);
  assert.deepEqual(synced.map((c) => c.status), ['completed', 'completed', 'cancelled', 'completed']);
});
