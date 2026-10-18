import { execFileSync } from 'node:child_process';
import { cpSync, existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Usage: node scripts/zip.mjs [dist dir] [output zip]. The defaults are dist/ and ai-kb-extension-<version>.zip.
const root = fileURLToPath(new URL('..', import.meta.url));
const dist = resolve(process.argv[2] ?? join(root, 'dist'));
if (!existsSync(resolve(dist, 'manifest.json'))) {
  console.error(`${dist}/manifest.json is missing; run npm run build first`);
  process.exit(1);
}
const { version } = JSON.parse(readFileSync(resolve(root, 'package.json'), 'utf8'));
const out = resolve(process.argv[3] ?? join(root, `ai-kb-extension-${version}.zip`));
rmSync(out, { force: true });
// The store signs with its own key, so the upload drops `key`; dist/ keeps it so unpacked loads share the store ID.
const stage = mkdtempSync(join(tmpdir(), 'aikb-zip-'));
try {
  cpSync(dist, stage, { recursive: true });
  const manifestPath = join(stage, 'manifest.json');
  const { key, ...manifest } = JSON.parse(readFileSync(manifestPath, 'utf8'));
  writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
  execFileSync('zip', ['-r', '-X', out, '.'], { cwd: stage, stdio: ['ignore', 'ignore', 'inherit'] });
} catch (e) {
  console.error(e.code === 'ENOENT' ? 'the zip command is not installed' : `zip failed: ${e.message}`);
  process.exit(1);
} finally {
  rmSync(stage, { recursive: true, force: true });
}
console.log(`wrote ${out}`);
