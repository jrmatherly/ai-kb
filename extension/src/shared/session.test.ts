import { test } from 'node:test';
import assert from 'node:assert/strict';
import { installFakeChrome } from '../../test/fakeChrome';
import { onSessionChange, readDraft, readIntent, readRecent, rememberCapture, updateCaptures, writeClip, writeDraft } from './session';
import type { Capture, CaptureDraft } from './types';

const capture = (i: number, status: Capture['status'] = 'completed'): Capture =>
  ({ knowledgeId: `k${i}`, kbId: 'kb1', title: 'T', url: `https://e.com/${i}`, savedAt: new Date(Date.UTC(2026, 9, 17, 0, 0, i)).toISOString(), status });

test('writeClip stores the draft and its intent in one write', async () => {
  const { session } = installFakeChrome();
  const draft: CaptureDraft = { kind: 'article', title: 'P', url: 'https://e.com/p', markdown: 'x', meta: {} };
  await writeClip(3, 'quick:article', draft);
  assert.deepEqual([...session.keys()], ['intent:3', 'draft:3']);
  assert.equal(await readIntent(3), 'quick:article');
  assert.deepEqual(await readDraft(3), draft);
});

test('rememberCapture keeps the newest 50, newest first, and replaces by knowledgeId', async () => {
  const { local } = installFakeChrome();
  local.set('connection', { serverUrl: 'https://kb.example.com' });
  for (let i = 0; i < 55; i++) await rememberCapture(capture(i));
  const recent = await readRecent();
  assert.equal(recent.length, 50);
  assert.equal(recent[0].knowledgeId, 'k54');
  assert.equal(recent[49].knowledgeId, 'k5');
  assert.equal(local.has('capture:k4'), false, 'pruned');
  assert.equal(local.has('connection'), true, 'other keys untouched');
  await rememberCapture(capture(54, 'failed'));
  assert.deepEqual((await readRecent()).slice(0, 2).map((c) => [c.knowledgeId, c.status]), [['k54', 'failed'], ['k53', 'completed']]);
});

test('updateCaptures rewrites each capture under its own key', async () => {
  installFakeChrome();
  await rememberCapture(capture(1, 'pending'));
  await rememberCapture(capture(2, 'pending'));
  await updateCaptures([capture(1, 'completed')]);
  assert.deepEqual((await readRecent()).map((c) => [c.knowledgeId, c.status]), [['k2', 'pending'], ['k1', 'completed']]);
});

test('onSessionChange reports writes of session keys and skips removals', async () => {
  installFakeChrome();
  const keys: string[] = [];
  const stop = onSessionChange((key) => keys.push(key));
  const draft: CaptureDraft = { kind: 'note', title: 'N', url: '', markdown: 'x', meta: {} };
  await writeDraft(3, draft);
  await writeDraft(3, null);
  await rememberCapture(capture(1));
  stop();
  await writeDraft(4, draft);
  assert.deepEqual(keys, ['draft:3']);
});
