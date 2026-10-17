let tabId = null;
const out = (v) => (document.getElementById('out').textContent += `${JSON.stringify(v)}\n`);

chrome.tabs.query({ active: true, currentWindow: true }).then(([t]) => {
  tabId = t.id;
  document.getElementById('tab').textContent = `tab: ${t.id} url: ${t.url ?? '(hidden)'}`;
});

document.getElementById('p1').addEventListener('click', () => {
  const opened = chrome.sidePanel.open({ tabId });
  chrome.storage.session.set({ [`intent:${tabId}`]: 'popup' });
  opened.then(() => probeLog('P3', 'opened as the first statement'), (e) => probeLog('P3', `rejected: ${errText(e)}`)).then(out);
});

document.getElementById('p4').addEventListener('click', async () => {
  await chrome.storage.session.set({ [`intent:${tabId}`]: 'popup-await' });
  try {
    await chrome.sidePanel.open({ tabId });
    out(await probeLog('P4', 'opened after one await'));
  } catch (e) {
    out(await probeLog('P4', `rejected: ${errText(e)}`));
  }
});

document.getElementById('p5').addEventListener('click', () => {
  chrome.runtime.sendMessage({ type: 'OPEN_PANEL', tabId }).then(out);
});
