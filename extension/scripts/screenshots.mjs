// PNGs of the built extension's pages: node scripts/screenshots.mjs --out <dir>. Chrome comes from AIKB_SMOKE_CHROME.
// Shoots the signed-out pages, then the same pages connected to a stub server that serves canned agents and bases.
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { parseArgs } from 'node:util';
import { attachWorker, copyDist, extensionId, launchChrome, openTab, sleep, stopChrome, until } from './cdp.mjs';

const { values } = parseArgs({ options: { out: { type: 'string' } } });
if (!values.out || !process.env.AIKB_SMOKE_CHROME) {
  console.error('usage: AIKB_SMOKE_CHROME=<chrome binary> node scripts/screenshots.mjs --out <dir>');
  process.exit(2);
}
const out = resolve(values.out);
mkdirSync(out, { recursive: true });

const KBS = [{ id: 'kb1', name: 'Clips' }, { id: 'kb2', name: 'Engineering notes' }];
const STUB = {
  '/auth/me': { user: { email: 'demo@example.com' }, tenant: { name: 'Demo workspace' } },
  '/agents': [
    { id: 'a1', name: 'Quick Answer', config: { kb_selection_mode: 'all' } },
    { id: 'a2', name: 'Docs agent', config: { kb_selection_mode: 'selected', knowledge_bases: ['kb1'] } },
  ],
  '/knowledge-bases': KBS,
};
const J = JSON.stringify;
const stub = createServer((req, res) => {
  const path = new URL(req.url, 'http://x').pathname.replace(/^\/api\/v1/, '');
  res.writeHead(200, { 'Content-Type': 'application/json' }).end(J(path === '/auth/oidc/config' ? { enabled: false } : { success: true, data: STUB[path] ?? [] }));
}).listen(0, '127.0.0.1');
await new Promise((r) => stub.once('listening', r));
const server = `http://127.0.0.1:${stub.address().port}`;

const work = mkdtempSync(join(tmpdir(), 'aikb-shots-'));
const copy = join(work, 'extension');
const origin = `chrome-extension://${extensionId(copyDist(copy, [server]).key)}`;
let chrome;
let devtools;

// Each step is [action, predicate]: the action runs, then the shot waits until the predicate holds.
async function shoot(page, name, ready, ...steps) {
  const tab = await openTab(devtools, `${origin}/${page}`);
  await tab.call('Emulation.setDeviceMetricsOverride', { width: 1280, height: 800, deviceScaleFactor: 1, mobile: false });
  await until(tab, ready, `${name} to render`, 15_000);
  for (const [action, done] of steps) {
    await tab.evaluate(action);
    await until(tab, done, `${name}: ${done}`, 15_000);
  }
  // Nothing signals that transitions have finished.
  await sleep(500);
  const { data } = await tab.call('Page.captureScreenshot', { format: 'png' });
  const file = join(out, `${name}.png`);
  writeFileSync(file, Buffer.from(data, 'base64'));
  console.log(file);
  tab.close();
  await fetch(`${devtools}/json/close/${tab.id}`);
}

const clickText = (selector, text) => `[...document.querySelectorAll(${J(selector)})].find((b) => b.textContent.trim() === ${J(text)}).click()`;
const selectedTab = (text) => `document.querySelector('[role=tab][aria-selected=true]')?.textContent.trim() === ${J(text)}`;
const kbOptions = `!!document.querySelector('select[aria-label="Knowledge bases"] option')`;

let code = 0;
try {
  ({ chrome, devtools } = await launchChrome({ chromeBin: process.env.AIKB_SMOKE_CHROME, extensionDir: copy, profile: join(work, 'profile') }));
  const sw = await attachWorker(devtools, origin);

  await shoot('options.html', 'options-signed-out', `!!document.querySelector('input[type=url]')`);
  await shoot('sidepanel.html', 'sidepanel-signed-out', `!!document.querySelector('main')?.innerText.trim()`);
  await shoot('popup.html', 'popup-signed-out', `!!document.querySelector('main')?.innerText.trim()`);

  await sw.evaluate(`chrome.storage.local.set({ connection: ${J({ serverUrl: server, auth: { kind: 'api_key', key: 'demo' }, tenantId: null, lastKbId: 'kb1', quickClip: false })} })`);
  sw.close();

  await shoot('options.html', 'options-connected', `[...document.querySelectorAll('h2')].some((h) => h.textContent.trim() === 'Connected')`);
  await shoot('sidepanel.html', 'sidepanel-chat', kbOptions);
  await shoot('sidepanel.html', 'sidepanel-chat-locked', kbOptions,
    [`(() => { const s = document.querySelector('select[aria-label=Agent]'); s.value = 'a2'; s.dispatchEvent(new Event('change')); })()`, `document.body.innerText.includes('Set by this agent')`]);
  await shoot('sidepanel.html', 'sidepanel-capture', `!!document.querySelector('[role=tab]')`, [clickText('[role=tab]', 'Capture'), selectedTab('Capture')]);
  await shoot('sidepanel.html', 'sidepanel-editor', `!!document.querySelector('[role=tab]')`,
    [clickText('[role=tab]', 'Capture'), selectedTab('Capture')],
    [clickText('button', 'New note'), `!!document.querySelector('textarea[aria-label=Markdown]')`]);
  await shoot('popup.html', 'popup-connected', `document.body.innerText.includes('Connected to')`);
} catch (error) {
  console.error(error?.stack ?? String(error));
  code = 1;
} finally {
  await stopChrome(chrome);
  stub.close();
  rmSync(work, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
}
process.exit(code);
