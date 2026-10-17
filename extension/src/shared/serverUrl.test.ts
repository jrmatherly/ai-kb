import { test } from 'node:test';
import assert from 'node:assert/strict';
import { apiBase, normalizeServerUrl } from './serverUrl';

test('normalizes a trailing slash, an /api/v1 suffix and uppercase', () => {
  for (const input of ['https://kb.example.com/', 'https://kb.example.com/api/v1', 'HTTPS://KB.EXAMPLE.COM']) {
    assert.deepEqual(normalizeServerUrl(input), { ok: true, url: 'https://kb.example.com' }, input);
  }
});

test('accepts plain http with the insecure warning', () => {
  assert.deepEqual(normalizeServerUrl('http://kb.lan'), { ok: true, url: 'http://kb.lan', warning: 'insecure' });
});

test('assumes https when the scheme is missing', () => {
  assert.deepEqual(normalizeServerUrl('kb.example.com'), { ok: true, url: 'https://kb.example.com' });
});

test('rejects text that is not a URL', () => {
  assert.deepEqual(normalizeServerUrl('not a url'), { ok: false, reason: 'invalid' });
});

test('apiBase appends /api/v1', () => {
  assert.equal(apiBase('https://kb.example.com'), 'https://kb.example.com/api/v1');
});

test('rejects credentials, a query or a fragment in the URL', () => {
  for (const input of ['https://user:pw@kb.example.com', 'https://kb.example.com/?x=1', 'https://kb.example.com/#top']) {
    assert.deepEqual(normalizeServerUrl(input), { ok: false, reason: 'invalid' }, input);
  }
});
