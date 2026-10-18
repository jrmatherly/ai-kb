// Shared harness for driving the built extension in headless Chrome over CDP.
import { spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { cpSync, readFileSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export const extensionId = (key) => {
  if (!key) throw new Error('dist/manifest.json has no key, so the extension ID cannot be derived; run npm run build first');
  return [...createHash('sha256').update(Buffer.from(key, 'base64')).digest('hex').slice(0, 32)]
    .map((c) => String.fromCharCode(97 + parseInt(c, 16)))
    .join('');
};

export const freePort = () => new Promise((resolve) => {
  const s = createServer().listen(0, '127.0.0.1', () => {
    const { port } = s.address();
    s.close(() => resolve(port));
  });
});

// Copies dist to dest with host_permissions replaced by origins, and returns the original manifest.
export function copyDist(dest, origins) {
  cpSync(fileURLToPath(new URL('../dist', import.meta.url)), dest, { recursive: true });
  const manifestPath = join(dest, 'manifest.json');
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
  writeFileSync(manifestPath, JSON.stringify({ ...manifest, host_permissions: origins.map((o) => `${new URL(o).origin}/*`) }, null, 2));
  return manifest;
}

export async function stopChrome(chrome) {
  if (!chrome?.pid || chrome.exitCode !== null || chrome.signalCode !== null) return;
  const exited = new Promise((r) => chrome.once('exit', r));
  chrome.kill('SIGTERM');
  if (!(await Promise.race([exited.then(() => true), sleep(5000)]))) {
    chrome.kill('SIGKILL');
    await exited;
  }
}

export async function launchChrome({ chromeBin, extensionDir, profile }) {
  const port = await freePort();
  const devtools = `http://127.0.0.1:${port}`;
  const chrome = spawn(chromeBin, [
    '--headless=new', `--user-data-dir=${profile}`, `--remote-debugging-port=${port}`, '--no-first-run', '--no-default-browser-check', `--load-extension=${extensionDir}`,
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
  try {
    return { chrome, devtools, browser: await Promise.race([answered, spawnError]) };
  } catch (error) {
    await stopChrome(chrome);
    throw error;
  }
}

export async function connectTarget(target) {
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
    ws.send(JSON.stringify({ id, method, params }));
  });
  const evaluate = async (expression, ms = 60_000) => {
    let timer;
    const timeout = new Promise((_, reject) => (timer = setTimeout(() => reject(new Error(`evaluate timed out after ${ms / 1000} s on ${target.url}`)), ms)));
    const r = await Promise.race([call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }), timeout]).finally(() => clearTimeout(timer));
    if (r.exceptionDetails) throw new Error(r.exceptionDetails.exception?.description ?? r.exceptionDetails.text);
    return r.result.value;
  };
  return { id: target.id, url: target.url, call, evaluate, close: () => ws.close() };
}

export const targets = async (devtools) => (await fetch(`${devtools}/json`)).json();

export async function openTab(devtools, url) {
  const target = await (await fetch(`${devtools}/json/new?${encodeURIComponent(url)}`, { method: 'PUT' })).json();
  return connectTarget(target);
}

// Matches the URL as well as the type: Chrome runs component extensions that also have a background.js worker.
export async function attachWorker(devtools, origin) {
  const end = Date.now() + 15_000;
  let target;
  while (!(target = (await targets(devtools)).find((t) => t.type === 'service_worker' && t.url === `${origin}/background.js`))) {
    if (Date.now() > end) throw new Error(`no worker for ${origin}: the derived id does not match the loaded extension, or this Chrome ignores --load-extension (branded Chrome does; use Chrome for Testing)`);
    await sleep(250);
  }
  return connectTarget(target);
}

export async function until(page, expression, what, ms = 30_000) {
  const end = Date.now() + ms;
  for (;;) {
    const value = await page.evaluate(expression).catch(() => undefined);
    if (value) return value;
    if (Date.now() > end) throw new Error(`timed out after ${ms / 1000} s waiting for ${what}`);
    await sleep(300);
  }
}
