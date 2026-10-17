let tabId = null;
const out = (v) => (document.getElementById('out').textContent += `${JSON.stringify(v)}\n`);

async function refresh(reason) {
  const [t] = await chrome.tabs.query({ active: true, currentWindow: true });
  tabId = t.id;
  const access = await chrome.runtime.sendMessage({ type: 'TAB_ACCESS', tabId });
  document.getElementById('access').textContent = `TAB_ACCESS(${tabId}): ${JSON.stringify(access)}`;
  await probeLog('P2', { reason, tabId, access });
}
refresh('load');
chrome.tabs.onActivated.addListener(() => refresh('activated'));
chrome.tabs.onUpdated.addListener((id, info) => { if (id === tabId && info.status === 'complete') refresh('updated'); });

document.getElementById('inject').addEventListener('click', async () => {
  out(await probeLog('P1-inject', await chrome.runtime.sendMessage({ type: 'EXTRACT', tabId, mode: 'article' })));
});

document.getElementById('fetch').addEventListener('click', async () => {
  const base = document.getElementById('server').value;
  try {
    const r = await fetch(`${base}/api/v1/auth/oidc/config`);
    out(await probeLog('L1', { base, status: r.status, body: (await r.text()).slice(0, 120) }));
  } catch (e) {
    out(await probeLog('L1', { base, error: errText(e) }));
  }
});

document.getElementById('drop').addEventListener('click', async () => {
  const base = document.getElementById('server').value;
  out(await probeLog('L2-remove', await chrome.permissions.remove({ origins: [`${new URL(base).origin}/*`] })));
});

document.getElementById('auth').addEventListener('click', async () => {
  const url = document.getElementById('oidc').value;
  try {
    const result = await chrome.identity.launchWebAuthFlow({ url, interactive: true });
    out(await probeLog('O2-panel', { resolved: result }));
  } catch (e) {
    out(await probeLog('O2-panel', { rejected: errText(e) }));
  }
});
