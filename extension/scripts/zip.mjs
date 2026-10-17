import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, rmSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
const dist = resolve(root, 'dist');
if (!existsSync(resolve(dist, 'manifest.json'))) {
  console.error('dist/manifest.json is missing; run npm run build first');
  process.exit(1);
}
const { version } = JSON.parse(readFileSync(resolve(root, 'package.json'), 'utf8'));
const out = resolve(root, `ai-kb-extension-${version}.zip`);
rmSync(out, { force: true });
try {
  execFileSync('zip', ['-r', '-X', out, '.'], { cwd: dist, stdio: ['ignore', 'ignore', 'inherit'] });
} catch (e) {
  console.error(e.code === 'ENOENT' ? 'the zip command is not installed' : `zip failed: ${e.message}`);
  process.exit(1);
}
console.log(`wrote ${out}`);
