import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const zipScript = fileURLToPath(new URL('zip.mjs', import.meta.url));
const missing = ['zip', 'unzip'].filter((cmd) => spawnSync('sh', ['-c', `command -v ${cmd}`]).status !== 0);

test('the store zip drops key and dist keeps it', { skip: missing.length ? `missing ${missing.join(', ')}` : false }, (t) => {
  const work = mkdtempSync(join(tmpdir(), 'aikb-zip-test-'));
  t.after(() => rmSync(work, { recursive: true, force: true }));
  const dist = join(work, 'dist');
  mkdirSync(join(dist, 'icons'), { recursive: true });
  writeFileSync(join(dist, 'icons', 'a.png'), 'png');
  const manifest = `${JSON.stringify({ manifest_version: 3, name: 'x', version: '1', key: 'KEY' }, null, 2)}\n`;
  writeFileSync(join(dist, 'manifest.json'), manifest);
  const out = join(work, 'out.zip');

  execFileSync(process.execPath, [zipScript, dist, out], { stdio: 'ignore' });

  assert.equal(execFileSync('unzip', ['-p', out, 'manifest.json'], { encoding: 'utf8' }),
    `${JSON.stringify({ manifest_version: 3, name: 'x', version: '1' }, null, 2)}\n`);
  assert.match(execFileSync('unzip', ['-l', out], { encoding: 'utf8' }), /icons\/a\.png/);
  assert.equal(readFileSync(join(dist, 'manifest.json'), 'utf8'), manifest);
});
