importScripts('log.js');

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({ id: 'probe-menu', title: 'Probe: clip this page', contexts: ['page', 'selection'] });
});

// The design's TAB_ACCESS: without the tabs permission, url and title are present only under an activeTab grant.
async function tabAccess(tabId) {
  const tab = await chrome.tabs.get(tabId);
  return tab.url ? { url: tab.url, title: tab.title ?? '' } : { access: false };
}

async function extract(tabId, mode) {
  try {
    await chrome.scripting.executeScript({ target: { tabId }, files: ['content.js'] });
    const [r] = await chrome.scripting.executeScript({
      target: { tabId },
      func: (m) => globalThis.__probeExtract(m),
      args: [mode],
    });
    return { result: r.result };
  } catch (e) {
    return { error: errText(e) };
  }
}

chrome.runtime.onMessage.addListener((req, sender, reply) => {
  (async () => {
    switch (req.type) {
      case 'TAB_ACCESS':
        return tabAccess(req.tabId);
      case 'EXTRACT':
        return extract(req.tabId, req.mode ?? 'article');
      case 'OPEN_PANEL': {
        // P5: the gesture crossed one message hop; nothing awaited before the call.
        try {
          await chrome.sidePanel.open({ tabId: req.tabId });
          return probeLog('P5', 'opened');
        } catch (e) {
          return probeLog('P5', `rejected: ${errText(e)}`);
        }
      }
      default:
        return { error: `unknown ${req.type}` };
    }
  })().then(reply, (e) => reply({ error: errText(e) }));
  return true;
});

chrome.commands.onCommand.addListener(async (command, tab) => {
  if (!tab?.id) return;
  await probeLog('P1-command', { command, url: tab.url ?? null, title: tab.title ?? null });
  await chrome.storage.session.set({ [`intent:${tab.id}`]: 'command' });
  try {
    await chrome.sidePanel.open({ tabId: tab.id });
    await probeLog('P1-command-open', 'opened after one await');
  } catch (e) {
    await probeLog('P1-command-open', `rejected: ${errText(e)}`);
  }
});

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  if (!tab?.id) return;
  await probeLog('P1-menu', { url: tab.url ?? null, title: tab.title ?? null, selection: info.selectionText ?? null });
  // P8: one awaited write, then the open call.
  await chrome.storage.session.set({ [`intent:${tab.id}`]: 'menu' });
  try {
    await chrome.sidePanel.open({ tabId: tab.id });
    await probeLog('P8', 'opened after one await');
  } catch (e) {
    await probeLog('P8', `rejected: ${errText(e)}`);
  }
});
