// CDP smoke run of the built extension against a live server: node scripts/smoke.mjs --identity <name>.
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

const IDENTITIES = {
  // restricted differs from full only in the key's scope; the steps and the history mode are the same.
  restricted: { tab: 'API key', env: ['AIKB_SMOKE_API_KEY'], history: 'handle' },
  full: { tab: 'API key', env: ['AIKB_SMOKE_FULL_KEY'], history: 'handle' },
  'no-ingest': { tab: 'API key', env: ['AIKB_SMOKE_NO_INGEST_KEY'], history: 'handle', saveError: 'This key cannot write to this knowledge base' },
  bearer: { tab: 'Email and password', env: ['AIKB_SMOKE_EMAIL', 'AIKB_SMOKE_PASSWORD'], history: 'public' },
  oidc: { tab: 'Sign in with', env: ['AIKB_SMOKE_DEX_EMAIL', 'AIKB_SMOKE_DEX_PASSWORD'], history: 'public' },
};

const root = fileURLToPath(new URL('..', import.meta.url));
const { values } = parseArgs({ options: { identity: { type: 'string' } } });
const identity = IDENTITIES[values.identity];
if (!identity) {
  console.error(`usage: node scripts/smoke.mjs --identity ${Object.keys(IDENTITIES).join('|')}`);
  process.exit(2);
}
const missing = ['AIKB_SMOKE_CHROME', ...identity.env].filter((name) => !process.env[name]);
if (missing.length) {
  console.error(`missing environment: ${missing.join(', ')}`);
  process.exit(2);
}
const [credential, secret] = identity.env.map((name) => process.env[name]);
// dist/manifest.json carries the store item's public key, so the smoke copy runs under the store ID and its OIDC return URL.
let PUBLIC_KEY;
try {
  ({ key: PUBLIC_KEY } = JSON.parse(readFileSync(new URL('../dist/manifest.json', import.meta.url), 'utf8')));
} catch {
  console.error('dist/manifest.json is missing; run npm run build first');
  process.exit(2);
}
if (!PUBLIC_KEY) {
  console.error('dist/manifest.json has no key, so the extension ID cannot be derived');
  process.exit(2);
}
const EXTENSION_ID = [...createHash('sha256').update(Buffer.from(PUBLIC_KEY, 'base64')).digest('hex').slice(0, 32)]
  .map((c) => String.fromCharCode(97 + parseInt(c, 16)))
  .join('');
const REDIRECT_URL = `https://${EXTENSION_ID}.chromiumapp.org/oidc`;
const ORIGIN = `chrome-extension://${EXTENSION_ID}`;
const server = (process.env.AIKB_SMOKE_SERVER ?? 'http://127.0.0.1:8080').replace(/\/+$/, '');
const kbName = process.env.AIKB_SMOKE_KB ?? 'Clips';

console.log(`extension id ${EXTENSION_ID}`);
console.log(`redirect URL ${REDIRECT_URL}`);
console.log(`  oidc needs OIDC_AUTH_EXTENSION_RETURN_URLS=${REDIRECT_URL} on the server; with it removed, the options page must end at "This server does not allow extension sign-in yet" (checked by hand)`);
console.log(`identity ${values.identity}, server ${server}`);

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const J = JSON.stringify;
const button = (label) => `[...document.querySelectorAll('button')].find((b) => b.textContent.trim().startsWith(${J(label)}))`;

async function connectTarget(target) {
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    ws.onopen = resolve;
    ws.onerror = () => reject(new Error(`cannot attach to ${target.url}`));
  });
  let seq = 0;
  let closed = null;
  const pending = new Map();
  ws.onmessage = (m) => {
    const msg = JSON.parse(m.data);
    pending.get(msg.id)?.(msg);
    pending.delete(msg.id);
  };
  const fail = () => {
    closed ??= new Error(`the connection to ${target.url} closed`);
    for (const settle of pending.values()) settle({ error: { message: closed.message } });
    pending.clear();
  };
  ws.onclose = fail;
  ws.onerror = fail;
  const call = (method, params = {}) => new Promise((resolve, reject) => {
    if (closed) return reject(closed);
    const id = ++seq;
    pending.set(id, (msg) => (msg.error ? reject(new Error(`${method}: ${msg.error.message}`)) : resolve(msg.result)));
    ws.send(J({ id, method, params }));
  });
  const evaluate = async (expression, ms = 60_000) => {
    let timer;
    const timeout = new Promise((_, reject) => (timer = setTimeout(() => reject(new Error(`evaluate timed out after ${ms / 1000} s on ${target.url}`)), ms)));
    const r = await Promise.race([call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }), timeout]).finally(() => clearTimeout(timer));
    if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description ?? r.exceptionDetails.text);
    return r.result.value;
  };
  return { url: target.url, call, evaluate, close: () => ws.close() };
}

async function until(page, expression, what, ms = 30_000) {
  const end = Date.now() + ms;
  for (;;) {
    const value = await page.evaluate(expression).catch(() => undefined);
    if (value) return value;
    if (Date.now() > end) throw new Error(`timed out after ${ms / 1000} s waiting for ${what}`);
    await sleep(300);
  }
}

// A real input event, so a handler that needs a user gesture gets one.
async function click(page, element) {
  const box = await page.evaluate(`(() => { const el = ${element}; if (!el) return null; el.scrollIntoView({ block: 'center' }); const r = el.getBoundingClientRect(); return { x: r.x + r.width / 2, y: r.y + r.height / 2 }; })()`);
  if (!box) throw new Error(`nothing to click: ${element}`);
  for (const type of ['mousePressed', 'mouseReleased']) await page.call('Input.dispatchMouseEvent', { type, x: box.x, y: box.y, button: 'left', clickCount: 1 });
}

const fill = (page, selector, value, change = false) => page.evaluate(`(() => {
  const el = document.querySelector(${J(selector)});
  if (!el) throw new Error('no field ' + ${J(selector)});
  el.value = ${J(value)};
  el.dispatchEvent(new Event('input', { bubbles: true }));
  ${change ? "el.dispatchEvent(new Event('change', { bubbles: true }));" : ''}
})()`);

const choose = (page, selector, text) => page.evaluate(`(() => {
  const s = document.querySelector(${J(selector)});
  const o = s && [...s.options].find((o) => o.textContent.trim() === ${J(text)});
  if (!o) return false;
  if (s.multiple) for (const x of s.options) x.selected = x === o;
  else s.value = o.value;
  s.dispatchEvent(new Event('change', { bubbles: true }));
  return o.value;
})()`);

const freePort = () => new Promise((resolve) => {
  const s = createServer().listen(0, '127.0.0.1', () => {
    const { port } = s.address();
    s.close(() => resolve(port));
  });
});

const work = mkdtempSync(join(tmpdir(), 'aikb-smoke-'));
const copy = join(work, 'extension');
const profile = join(work, 'profile');
let chrome;
let fixture;
let current;
let worker;
let headers;
const created = [];
let deleted = false;

let cleaning;
const cleanup = () => (cleaning ??= (async () => {
  if (created.length && !deleted && headers) {
    await deleteCreated().then((ids) => console.log(`cleanup deleted knowledge ${ids}`), (e) => console.error(`cleanup: ${e.message}`));
  }
  if (chrome?.pid && chrome.exitCode === null && chrome.signalCode === null) {
    const exited = new Promise((r) => chrome.once('exit', r));
    chrome.kill('SIGTERM');
    if (!(await Promise.race([exited.then(() => true), sleep(5000)]))) {
      chrome.kill('SIGKILL');
      await exited;
    }
  }
  fixture?.close();
  rmSync(work, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
})());

async function deleteCreated() {
  for (const id of created) {
    const r = await fetch(`${server}/api/v1/knowledge/${id}`, { method: 'DELETE', headers });
    if (!r.ok) throw new Error(`DELETE /knowledge/${id} answered ${r.status}: ${(await r.text()).slice(0, 200)}`);
  }
  deleted = true;
  return created.join(', ');
}

async function step(name, fn) {
  try {
    const note = await fn();
    console.log(`ok ${name}${typeof note === 'string' ? `: ${note}` : ''}`);
  } catch (error) {
    const text = await current?.evaluate('document.body.innerText').catch(() => null);
    console.error(`FAIL ${name}: ${error.message}\n--- page text (${current?.url ?? 'no page'}) ---\n${text ?? '(none)'}`);
    throw error;
  }
}

process.on('SIGINT', () => void cleanup().finally(() => process.exit(130)));

let devtools;
const targets = async () => (await fetch(`${devtools}/json`)).json();
async function openTab(url) {
  const target = await (await fetch(`${devtools}/json/new?${encodeURIComponent(url)}`, { method: 'PUT' })).json();
  return connectTarget(target);
}

async function run() {
  const fixtureHtml = readFileSync(join(root, 'test/fixtures/article.html'));
  const fixturePort = await freePort();
  fixture = createServer((_, res) => res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' }).end(fixtureHtml)).listen(fixturePort, '127.0.0.1');
  const fixtureUrl = `http://127.0.0.1:${fixturePort}/article.html`;

  await step('copy dist with host permissions', () => {
    cpSync(join(root, 'dist'), copy, { recursive: true });
    const manifestPath = join(copy, 'manifest.json');
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    writeFileSync(manifestPath, J({ ...manifest, host_permissions: [`${new URL(server).origin}/*`, `${new URL(fixtureUrl).origin}/*`] }, null, 2));
    return `version ${manifest.version}`;
  });

  await step('launch Chrome', async () => {
    const port = await freePort();
    devtools = `http://127.0.0.1:${port}`;
    chrome = spawn(process.env.AIKB_SMOKE_CHROME, [
      '--headless=new', `--user-data-dir=${profile}`, `--remote-debugging-port=${port}`, '--no-first-run', '--no-default-browser-check', `--load-extension=${copy}`,
    ], { stdio: 'ignore' });
    const spawnError = new Promise((_, reject) => chrome.once('error', reject));
    const answered = (async () => {
      const end = Date.now() + 15_000;
      for (;;) {
        const version = await fetch(`${devtools}/json/version`).then((r) => r.json(), () => null);
        if (version) return version.Browser;
        if (chrome.exitCode !== null || Date.now() > end) throw new Error('Chrome did not answer /json/version');
        await sleep(250);
      }
    })();
    return Promise.race([answered, spawnError]);
  });

  await step('attach to the service worker', async () => {
    const end = Date.now() + 15_000;
    let target;
    while (!(target = (await targets()).find((t) => t.type === 'service_worker' && t.url === `${ORIGIN}/background.js`))) {
      if (Date.now() > end) throw new Error(`no worker for ${ORIGIN}; the derived id does not match the loaded extension`);
      await sleep(250);
    }
    worker = await connectTarget(target);
    const url = await worker.evaluate("chrome.identity.getRedirectURL('oidc')");
    if (url !== REDIRECT_URL) throw new Error(`getRedirectURL returned ${url}`);
    return url;
  });

  const options = (current = await openTab(`${ORIGIN}/options.html`));
  await step('open options.html', () => until(options, `!!document.querySelector('input[type=url]')`, 'the server field'));

  await step('set the server URL', async () => {
    await fill(options, 'input[type=url]', server, true);
    await until(options, `!!(${button(identity.tab)})`, `the "${identity.tab}" tab`);
  });

  await step(`pick the ${identity.tab} tab`, () => click(options, button(identity.tab)));

  if (values.identity !== 'oidc') {
    await step('fill the credentials', async () => {
      if (identity.tab === 'API key') await fill(options, 'input[type=password]', credential);
      else {
        await fill(options, 'input[type=email]', credential);
        await fill(options, 'input[type=password]', secret);
      }
    });
  }

  const before = new Set((await targets()).map((t) => t.id));
  await step('click Connect', () => click(options, button('Connect')));

  if (values.identity === 'oidc') {
    await step('sign in on the Dex page', async () => {
      const pages = new Map();
      const end = Date.now() + 30_000;
      try {
        for (;;) {
          for (const t of (await targets()).filter((t) => !before.has(t.id) && t.webSocketDebuggerUrl && t.type !== 'service_worker')) {
            if (!pages.has(t.id)) pages.set(t.id, await connectTarget(t).catch(() => null));
            const page = pages.get(t.id);
            if (page && (await page.evaluate(`!!document.querySelector('#login')`).catch(() => false))) {
              current = page;
              await fill(page, '#login', credential);
              await fill(page, '#password', secret);
              await page.evaluate(`document.querySelector('#login').form.requestSubmit()`);
              current = options;
              return new URL(t.url).host;
            }
          }
          if (Date.now() > end) throw new Error('no auth window with a #login field appeared');
          await sleep(500);
        }
      } finally {
        for (const page of pages.values()) page?.close();
      }
    });
  }

  await step('wait for Connected', () => until(options, `[...document.querySelectorAll('h2')].some((h) => h.textContent.trim() === 'Connected')`, 'the Connected card'));

  const connection = await worker.evaluate(`chrome.storage.local.get('connection').then((s) => s.connection)`);
  if (values.identity === 'oidc') {
    await step('the worker holds a bearer connection', () => {
      if (connection?.auth?.kind !== 'bearer') throw new Error(`auth.kind is ${connection?.auth?.kind}`);
    });
  }
  const auth = connection.auth;
  headers = auth.kind === 'api_key' ? { 'X-API-Key': auth.key } : { Authorization: `Bearer ${auth.token}` };
  if (connection.tenantId != null) headers['X-Tenant-ID'] = String(connection.tenantId);

  await step(`set the last used base to ${kbName}`, async () => {
    await until(options, `[...(document.querySelector('.kb-picker')?.options ?? [])].some((o) => o.textContent.trim() === ${J(kbName)})`, `"${kbName}" in the picker`);
    const kbId = await choose(options, '.kb-picker', kbName);
    await until(options, `chrome.storage.local.get('connection').then((s) => s.connection.lastKbId === ${J(kbId)})`, 'lastKbId in storage');
    return kbId;
  });

  let tabId;
  await step('open the fixture in a tab', async () => {
    await openTab(fixtureUrl);
    tabId = await until(worker, `chrome.tabs.query({ url: ${J(fixtureUrl)} }).then(([t]) => t?.status === 'complete' && t.id)`, 'the fixture tab');
    return `tab ${tabId}, ${fixtureUrl}`;
  });

  await step(`clip the fixture tab ${tabId} through the worker`, () => worker.evaluate(`globalThis.__aikb.clipFromGesture(${tabId}, 'article').then(() => true)`));

  const panel = (current = await openTab(`${ORIGIN}/sidepanel.html?tabId=${tabId}`));
  await step('wait for the editor', () => until(panel, `(() => { const t = document.querySelector('input[aria-label=Title]'); return document.querySelector('textarea[aria-label=Markdown]')?.value && t?.value; })()`, 'the editor with the clipped draft'));

  await step('click Save', () => click(panel, `[...document.querySelectorAll('button')].find((b) => b.textContent.trim() === 'Save')`));

  const captured = () => worker.evaluate(`chrome.storage.local.get(null).then((s) => Object.keys(s).filter((k) => k.startsWith('capture:')).map((k) => s[k].knowledgeId))`);

  if (identity.saveError) {
    await step(`the save ends at "${identity.saveError}"`, async () => {
      await until(panel, `document.body.innerText.includes(${J(identity.saveError)}) || [...document.querySelectorAll('.toast')].some((t) => t.textContent.trim().startsWith('Saved'))`, 'the save result');
      created.push(...(await captured()));
      if (created.length) throw new Error(`the save went through as knowledge ${created.join(', ')}: this key can write to the base`);
    });
    return;
  }

  await step('wait for the Saved toast', async () => {
    await until(panel, `[...document.querySelectorAll('.toast')].some((t) => /^(Saved|Parsing failed.*|Parsing was cancelled|Parsing is taking longer than two minutes)$/.test(t.textContent.trim()))`, 'the parse result', 150_000);
    created.push(...(await captured()));
    const toast = await panel.evaluate(`[...document.querySelectorAll('.toast')].map((t) => t.textContent.trim()).join(' | ')`);
    if (!/(^| \| )Saved($| \| )/.test(toast)) throw new Error(`toast reads "${toast}"`);
    return `knowledge ${created.join(', ')}`;
  });

  await step('switch to Chat', () => click(panel, `[...document.querySelectorAll('[role=tab]')].find((b) => b.textContent.trim() === 'Chat')`));

  await step(`select the ${kbName} base`, async () => {
    await until(panel, `[...document.querySelector('select[aria-label="Knowledge bases"]').options].some((o) => o.textContent.trim() === ${J(kbName)})`, `"${kbName}" in the chat picker`);
    await choose(panel, 'select[aria-label="Knowledge bases"]', kbName);
  });

  await step('ask "What does the fixture say?"', async () => {
    await fill(panel, '.composer textarea', 'What does the fixture say?');
    await click(panel, button('Send'));
  });

  await step('wait for the stream to end with an answer and a reference', async () => {
    const last = `[...document.querySelectorAll('.message.assistant')].at(-1)`;
    await until(panel, `(() => { const m = ${last}; return m && !(${button('Stop')}) && (m.querySelector('.answer')?.textContent.trim() || m.querySelector('.error')); })()`, 'the end of the stream', 180_000);
    const result = await panel.evaluate(`(() => { const m = ${last}; return { answer: m.querySelector('.answer').textContent.trim(), error: m.querySelector('.error')?.textContent ?? null, refs: [...m.querySelectorAll('.citations li')].map((li) => li.textContent.trim()) }; })()`);
    if (result.error) throw new Error(`the answer ended with "${result.error}"`);
    if (!result.answer) throw new Error('empty answer');
    if (!result.refs.length) throw new Error('no reference rendered');
    return `${result.answer.length} characters, references ${result.refs.join('; ')}`;
  });

  await step(`History loads the session with resource_urls=${identity.history}`, async () => {
    await panel.evaluate(`(() => {
      window.__urls = [];
      const real = window.fetch;
      window.fetch = (input, init) => { window.__urls.push(String(input instanceof Request ? input.url : input)); return real(input, init); };
    })()`);
    await click(panel, button('History'));
    await until(panel, `!!document.querySelector('.sessions li button')`, 'the session list');
    await click(panel, `document.querySelector('.sessions li button.current') ?? document.querySelector('.sessions li button')`);
    const url = await until(panel, `window.__urls.find((u) => u.includes('/load?'))`, 'the messages load request');
    await until(panel, `!!document.querySelector('.message.assistant .answer')?.textContent.trim()`, 'the loaded conversation');
    if (!url.includes(`resource_urls=${identity.history}`)) throw new Error(`load went to ${url}`);
    return new URL(url).pathname + new URL(url).search;
  });

  await step('delete the created knowledge', deleteCreated);
}

let code = 0;
try {
  await run();
} catch (error) {
  console.error(error?.stack ?? String(error));
  code = 1;
} finally {
  await cleanup();
}
process.exit(code);
