import { test } from 'node:test';
import assert from 'node:assert/strict';
import { installFakeChrome } from '../../test/fakeChrome';
import { onSessionChange, readDraft, readIntent, readRecent, rememberCapture, resyncRecent, updateCaptures, writeClip, writeDraft } from './session';
import type { ApiClient } from './api';
import type { Capture, CaptureDraft } from './types';

const capture = (i: number, status: Capture['status'] = 'completed'): Capture =>
  ({ knowledgeId: `k${i}`, kbId: 'kb1', title: 'T', url: `https://e.com/${i}`, savedAt: new Date(Date.UTC(2026, 9, 17, 0, 0, i)).toISOString(), status });

const serverA = { serverUrl: 'https://a', tenantId: 1 };
const serverB = { serverUrl: 'https://b', tenantId: 1 };
const ids = async () => (await readRecent()).map((c) => c.knowledgeId);

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
  const { local } = installFakeChrome();
  local.set('connection', serverA);
  await rememberCapture(capture(1, 'pending'));
  await rememberCapture(capture(2, 'pending'));
  await updateCaptures([capture(1, 'completed')]);
  assert.deepEqual((await readRecent()).map((c) => [c.knowledgeId, c.status]), [['k2', 'pending'], ['k1', 'completed']]);
});

test('onSessionChange reports writes of session keys and skips removals', async () => {
  const { local } = installFakeChrome();
  local.set('connection', serverA);
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

test('readRecent returns [] when there is no connection', async () => {
  const { local } = installFakeChrome();
  local.set('capture:k1', capture(1));
  assert.deepEqual(await readRecent(), []);
});

test('captures remembered under server A are hidden under server B and back', async () => {
  const { local } = installFakeChrome();
  local.set('connection', serverA);
  await rememberCapture(capture(1));
  local.set('connection', serverB);
  await rememberCapture(capture(2));
  assert.deepEqual(await ids(), ['k2']);
  local.set('connection', serverA);
  assert.deepEqual(await ids(), ['k1']);
});

test("changing only the tenant hides the other tenant's captures", async () => {
  const { local } = installFakeChrome();
  local.set('connection', serverA);
  await rememberCapture(capture(1));
  local.set('connection', { ...serverA, tenantId: 2 });
  assert.deepEqual(await ids(), []);
});

test('the 50-item prune only touches the current scope and removes unscoped keys', async () => {
  const { local } = installFakeChrome();
  local.set('capture:k90', capture(90));
  local.set('capture:k91', capture(91));
  local.set('connection', serverB);
  for (let i = 60; i < 63; i++) await rememberCapture(capture(i));
  local.set('connection', serverA);
  for (let i = 0; i < 55; i++) await rememberCapture(capture(i));
  assert.equal((await readRecent()).length, 50);
  local.set('connection', serverB);
  assert.deepEqual(await ids(), ['k62', 'k61', 'k60']);
  assert.equal(local.has('capture:k90'), false, 'unscoped pruned');
  assert.equal(local.has('capture:k91'), false, 'unscoped pruned');
});

test("resyncRecent under B never sends A's ids", async () => {
  const { local } = installFakeChrome();
  local.set('connection', serverA);
  await rememberCapture(capture(1, 'pending'));
  local.set('connection', serverB);
  await rememberCapture(capture(2, 'pending'));
  const sent: string[][] = [];
  await resyncRecent({} as ApiClient, async (_api, list) => { sent.push(list.map((c) => c.knowledgeId)); return list; });
  assert.deepEqual(sent, [['k2']]);
});

test('a write-back without a scope keeps the stored one', async () => {
  const { local } = installFakeChrome();
  local.set('connection', serverA);
  await rememberCapture(capture(1, 'pending'));
  local.set('connection', serverB);
  await updateCaptures([capture(1, 'completed')]);
  local.set('connection', serverA);
  assert.deepEqual((await readRecent()).map((c) => [c.knowledgeId, c.status]), [['k1', 'completed']]);
});

test('a status write-back after a switch keeps the original scope', async () => {
  const { local } = installFakeChrome();
  local.set('connection', serverA);
  await rememberCapture(capture(1, 'pending'));
  const [stored] = await readRecent();
  local.set('connection', serverB);
  await updateCaptures([{ ...stored, status: 'completed' }]);
  assert.deepEqual(await ids(), []);
  local.set('connection', serverA);
  assert.deepEqual((await readRecent()).map((c) => [c.knowledgeId, c.status]), [['k1', 'completed']]);
});

test('resyncRecent skips the write-back when the connection changed during the resync', async () => {
  const { local } = installFakeChrome();
  local.set('connection', serverA);
  await rememberCapture(capture(1, 'pending'));
  await resyncRecent({} as ApiClient, async (_api, list) => {
    local.set('connection', serverB);
    return list.map((c) => ({ ...c, status: 'cancelled' as const }));
  });
  local.set('connection', serverA);
  assert.deepEqual((await readRecent()).map((c) => c.status), ['pending']);
});
