import assert from 'node:assert/strict';
import { readFile, stat } from 'node:fs/promises';
import { join } from 'node:path';

const output = join(process.cwd(), 'out');
const html = await readFile(join(output, 'index.html'), 'utf8');
const visible = html.replace(/<script\b[^>]*>[\s\S]*?<\/script>/g, '').replace(/<[^>]+>/g, '');
assert.equal((html.match(/<h1\b/g) || []).length, 1, 'The homepage must have one main heading');
for (const term of ['Find the answers', 'then put knowledge to work', 'v0.8.2', 'RAG', 'Agent', 'Wiki', 'ClawHub', 'SkillHub', 'Docker', 'E2B', 'Cube', 'Long-term memory', 'Interactive terminal', 'Graphical desktop', 'Local Browser', 'BrowserSkill', 'MCP Server', 'Conversation control', 'Confluence', 'DingTalk', 'GitLab', 'Tencent IMA', 'LiteLLM', 'DeepSeek Harness']) {
  assert.ok(visible.includes(term), `Missing product content: ${term}`);
}
for (const term of ['Three plans', 'Choose a plan', 'Legacy demo', '18K+', 'Turn into action', 'From scattered documents', 'To living knowledge', 'Japanese UI', 'WeChat Dialog Open Platform', 'Tencent Cloud Lighthouse']) {
  assert.ok(!visible.includes(term), `Retired content still visible: ${term}`);
}
assert.match(html, /<video[^>]+preload="none"/, 'Video should not load before user interaction');
const readmeVideo = (await readFile(join(process.cwd(), '..', '..', 'README.md'), 'utf8')).match(/https:\/\/github\.com\/user-attachments\/assets\/[0-9a-f-]+/)?.[0];
assert.ok(readmeVideo, 'README.md must embed the product video');
assert.ok(html.includes(readmeVideo), `Use the README video: ${readmeVideo}`);
assert.ok(!/<video[^>]+autoplay/i.test(html), 'Do not autoplay the product film');
const wiki = html.match(/<section id="wiki"[\s\S]*?<\/section>/)?.[0];
assert.ok(wiki, 'The homepage must include a dedicated Wiki section');
for (const term of ['/docs/_home/product/wiki-browser.png', '/docs/_home/product/wiki-graph.png', '/docs/_home/product/wiki-revision-history.png', 'id="wiki-gallery"', '/docs/03-features/14-wiki.html', 'source citations', 'Wiki graph', 'version diffs']) {
  assert.ok(wiki.includes(term), `Missing Wiki showcase content: ${term}`);
}
const ids = new Set([...html.matchAll(/\bid="([^"]+)"/g)].map(match => match[1]));
for (const [, anchor] of html.matchAll(/href="#([^"]+)"/g)) {
  assert.ok(ids.has(anchor), `Missing navigation target: #${anchor}`);
}
const assets = new Set([...html.matchAll(/(?:src|href|poster)="(\/[^"?#]*)/g)].map(match => match[1]));
for (const asset of assets) {
  const path = join(output, decodeURIComponent(asset));
  let file = await stat(path);
  if (file.isDirectory()) file = await stat(join(path, 'index.html'));
  assert.ok(file.isFile() && file.size > 0, `Missing local asset: ${asset}`);
}
const pendingShots = [...html.matchAll(/website-docs\/homepage\/public(\/docs\/_home\/product\/[\w.-]+)/g)].map(match => match[1]);
// Gallery slides without a file render their placeholder only once selected; read the flag from the page data.
for (const [, image] of html.matchAll(/\\?"image\\?":\\?"([\w-]+)\\?"[^{}]*?\\?"available\\?":false/g)) pendingShots.push(`/docs/_home/product/${image}.png`);
if (pendingShots.length) console.warn(`Product screenshots still pending (placeholder shown): ${[...new Set(pendingShots)].join(', ')}`);
assert.match(html, /aria-controls="main-navigation"/);
assert.match(html, /aria-label="Play the AI KB product introduction video/);
console.log(`Homepage audit passed: product content, README video, navigation anchors, ${assets.size} local assets, and retired design choices.`);
