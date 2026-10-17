const originPattern = () => `${new URL(document.getElementById('server').value).origin}/*`;

document.getElementById('redirect').textContent = `redirect URL: ${chrome.identity.getRedirectURL('oidc')}`;
chrome.permissions.contains({ origins: [originPattern()] }).then(async (v) => {
  document.getElementById('contains').textContent = `P9 contains on load: ${v}`;
  await probeLog('P9-contains-on-load', { origin: originPattern(), contains: v });
});

document.getElementById('connect').addEventListener('click', () => {
  const granted = chrome.permissions.request({ origins: [originPattern()] });
  granted.then((v) => probeLog('P9-request', { origin: originPattern(), granted: v }), (e) => probeLog('P9-request', { error: errText(e) })).then(out);
});

document.getElementById('auth').addEventListener('click', async () => {
  const url = document.getElementById('oidc').value;
  try {
    const result = await chrome.identity.launchWebAuthFlow({ url, interactive: true });
    out(await probeLog('O1-options', { resolved: result }));
  } catch (e) {
    out(await probeLog('O1-options', { rejected: errText(e) }));
  }
});

document.getElementById('dump').addEventListener('click', async () => {
  const { probe = [] } = await chrome.storage.local.get('probe');
  document.getElementById('out').textContent = probe.map((p) => `${p.at} ${p.name} ${JSON.stringify(p.value)}`).join('\n');
});
