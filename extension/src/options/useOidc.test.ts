import { test } from 'node:test';
import assert from 'node:assert/strict';
import { OidcError, parseOidcFragment, useOidc } from './useOidc';

const redirectUrl = () => 'https://abc.chromiumapp.org/oidc';
const redirects = (async () => ({ status: 0 })) as unknown as typeof fetch;
const answers = (status: number) => (async () => ({ status })) as unknown as typeof fetch;
const encode = (value: unknown) => Buffer.from(JSON.stringify(value)).toString('base64url');

test('startUrl carries the exact return URL', () => {
  const oidc = useOidc({ launch: async () => undefined, redirectUrl, fetchFn: redirects });
  assert.equal(oidc.returnTo(), 'https://abc.chromiumapp.org/oidc');
  assert.equal(oidc.startUrl('https://kb.example.com'), 'https://kb.example.com/api/v1/auth/oidc/start?return_to=https%3A%2F%2Fabc.chromiumapp.org%2Foidc');
});

test('parseOidcFragment reads a result, an error, or nothing', () => {
  const result = encode({ success: true, token: 'a1', refresh_token: 'r1', tenant: { id: 7 }, memberships: [] });
  assert.deepEqual(parseOidcFragment(`https://abc.chromiumapp.org/oidc#oidc_result=${result}`), {
    kind: 'result', result: { success: true, token: 'a1', refresh_token: 'r1', tenant: { id: 7 }, memberships: [] },
  });
  assert.deepEqual(parseOidcFragment('https://abc.chromiumapp.org/oidc#oidc_error=access_denied&oidc_error_description=nope'), { kind: 'error', code: 'access_denied', description: 'nope' });
  assert.deepEqual(parseOidcFragment('https://abc.chromiumapp.org/oidc'), { kind: 'none' });
});

test('signIn launches interactively and returns the parsed result', async () => {
  const calls: Array<{ url: string; interactive: boolean }> = [];
  const result = encode({ success: true, token: 'a1', refresh_token: 'r1', tenant: { id: 7 }, memberships: [] });
  const oidc = useOidc({
    launch: async (details) => { calls.push({ url: details.url, interactive: details.interactive ?? false }); return `https://abc.chromiumapp.org/oidc#oidc_result=${result}`; },
    redirectUrl,
    fetchFn: redirects,
  });
  assert.equal((await oidc.signIn('https://kb.example.com')).token, 'a1');
  assert.deepEqual(calls, [{ url: 'https://kb.example.com/api/v1/auth/oidc/start?return_to=https%3A%2F%2Fabc.chromiumapp.org%2Foidc', interactive: true }]);
});

test('signIn maps the browser rejections to OidcError codes', async () => {
  const rejects = (message: string) => useOidc({ launch: async () => { throw new Error(message); }, redirectUrl, fetchFn: redirects });
  for (const [message, code] of [
    ['The user did not approve access. WINDOW_CLOSED', 'cancelled'],
    ['The user did not approve access.', 'cancelled'],
    ['Authorization page could not be loaded.', 'failed'],
    ['Something else', 'failed'],
  ] as const) {
    await assert.rejects(rejects(message).signIn('https://kb.example.com'), (e: unknown) => e instanceof OidcError && e.code === code, message);
  }
  const serverError = useOidc({ launch: async () => 'https://abc.chromiumapp.org/oidc#oidc_error=login_failed&oidc_error_description=boom', redirectUrl, fetchFn: redirects });
  await assert.rejects(serverError.signIn('https://kb.example.com'), (e: unknown) => e instanceof OidcError && e.code === 'login_failed' && e.description === 'boom');
  const empty = useOidc({ launch: async () => undefined, redirectUrl, fetchFn: redirects });
  await assert.rejects(empty.signIn('https://kb.example.com'), (e: unknown) => e instanceof OidcError && e.code === 'cancelled');
});

test('the start route decides not_allowed before any window opens', async () => {
  let launched = 0;
  const launch = async () => { launched += 1; return undefined; };
  await assert.rejects(useOidc({ launch, redirectUrl, fetchFn: answers(400) }).signIn('https://kb.example.com'), (e: unknown) => e instanceof OidcError && e.code === 'not_allowed');
  await assert.rejects(useOidc({ launch, redirectUrl, fetchFn: answers(403) }).signIn('https://kb.example.com'), (e: unknown) => e instanceof OidcError && e.code === 'failed' && /403/.test(e.description ?? ''));
  const down = (async () => { throw new Error('Failed to fetch'); }) as unknown as typeof fetch;
  await assert.rejects(useOidc({ launch, redirectUrl, fetchFn: down }).signIn('https://kb.example.com'), (e: unknown) => e instanceof OidcError && e.code === 'failed' && /Cannot reach/.test(e.description ?? ''));
  assert.equal(launched, 0);
  assert.deepEqual(parseOidcFragment('https://abc.chromiumapp.org/oidc#oidc_result=%%%'), { kind: 'error', code: 'failed', description: 'The sign-in result could not be read' });
});
