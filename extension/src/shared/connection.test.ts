import { test } from 'node:test';
import assert from 'node:assert/strict';
import { installFakeChrome } from '../../test/fakeChrome';
import { onIdentityChange, saveConnection } from './connection';
import type { Connection } from './types';

const bearer: Connection = { serverUrl: 'https://kb.example.com', auth: { kind: 'bearer', token: 'a1', refreshToken: 'r1' }, tenantId: 7, lastKbId: 'kb1', quickClip: true };

test('onIdentityChange fires on connect, sign-out, workspace and forget, not on a token refresh or a preference', async () => {
  installFakeChrome();
  let fired = 0;
  const stop = onIdentityChange(() => { fired += 1; });
  const steps: Array<[Connection | null, number]> = [
    [bearer, 1],
    [{ ...bearer, auth: { kind: 'bearer', token: 'a2', refreshToken: 'r2' } }, 1],
    [{ ...bearer, lastKbId: 'kb2' }, 1],
    [{ ...bearer, tenantId: 8 }, 2],
    [{ ...bearer, tenantId: 8, auth: { kind: 'signed_out' } }, 3],
    [null, 4],
  ];
  for (const [connection, expected] of steps) {
    await saveConnection(connection);
    assert.equal(fired, expected, JSON.stringify(connection));
  }
  stop();
  await saveConnection(bearer);
  assert.equal(fired, 4);
});
