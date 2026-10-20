import { test } from 'node:test';
import assert from 'node:assert/strict';
import { send } from './messages';

function fakeRuntime(replies: Array<Promise<unknown>>) {
  const calls: unknown[] = [];
  (globalThis as unknown as { chrome: unknown }).chrome = {
    runtime: { sendMessage: (request: unknown) => { calls.push(request); return replies.shift() ?? Promise.resolve(undefined); } },
  };
  return calls;
}

test('send resends once when the worker stays silent', async () => {
  const calls = fakeRuntime([new Promise(() => {}), Promise.resolve({ serverUrl: 'https://kb' })]);
  const reply = await send<{ serverUrl: string }>({ type: 'GET_HEADERS' }, 10);
  assert.equal(reply.serverUrl, 'https://kb');
  assert.equal(calls.length, 2);
});

test('send takes the first reply without resending', async () => {
  const calls = fakeRuntime([Promise.resolve(null)]);
  assert.equal(await send({ type: 'GET_HEADERS' }, 10), null);
  assert.equal(calls.length, 1);
});

test('send waits for a slow EXTRACT instead of running it twice', async () => {
  const calls = fakeRuntime([new Promise((r) => setTimeout(() => r({ draft: null, reason: 'cancelled' }), 30))]);
  const reply = await send<{ draft: null; reason: string }>({ type: 'EXTRACT', tabId: 1, mode: 'article' }, 10);
  assert.equal(reply.reason, 'cancelled');
  assert.equal(calls.length, 1);
});

test('send reports a missing listener after the resend', async () => {
  fakeRuntime([new Promise(() => {}), Promise.resolve(undefined)]);
  await assert.rejects(send({ type: 'GET_HEADERS' }, 10), /no reply to GET_HEADERS/);
});
