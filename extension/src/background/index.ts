import type { ExtractMode, ExtractResult } from '../shared/messages';
import { readIntent, writeComposer, writeDraft, writeIntent } from '../shared/session';
import { createRefresher, loadConnection, saveConnection } from './auth';
import { askFromGesture, clipFromGesture, type Deps, handle, noteFromGesture } from './router';

// content.js is 700 KB, so it is injected once per page and reused while the page lives.
async function inject(tabId: number, mode: ExtractMode): Promise<ExtractResult> {
  const target = { tabId };
  const run = () => chrome.scripting.executeScript({
    target,
    func: (m: ExtractMode) => (typeof globalThis.__aikbExtract === 'function' ? globalThis.__aikbExtract(m) : null),
    args: [mode],
  });
  let [first] = await run();
  if (first?.result == null) {
    await chrome.scripting.executeScript({ target, files: ['content.js'] });
    [first] = await run();
  }
  if (first?.result == null) throw new Error('the extractor did not load');
  return first.result as ExtractResult;
}

const deps: Deps = {
  load: loadConnection,
  save: saveConnection,
  refresh: createRefresher({ load: loadConnection, save: saveConnection }),
  tabInfo: (tabId) => chrome.tabs.get(tabId),
  inject,
  commands: async () => (await chrome.commands.getAll()).map((c) => ({ name: c.name ?? '', description: c.description ?? '', shortcut: c.shortcut ?? '' })),
  openPanel: (tabId) => chrome.sidePanel.open({ tabId }),
  writeIntent,
  readIntent,
  writeDraft,
  writeComposer,
  pageSelection: async (tabId) => {
    const [result] = await chrome.scripting.executeScript({ target: { tabId }, func: () => getSelection()?.toString() ?? '' });
    return (result?.result as string | undefined) ?? '';
  },
  extensionId: chrome.runtime.id,
  extensionOrigin: chrome.runtime.getURL(''),
};

chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  handle(request, sender, deps).then(sendResponse, (error: unknown) => {
    console.error('message failed', request?.type, error);
    sendResponse(undefined);
  });
  return true;
});

// Idempotent and cheap, so it runs on every worker start rather than only on install.
chrome.storage.local.setAccessLevel({ accessLevel: 'TRUSTED_CONTEXTS' }).catch((error: unknown) => console.error('setAccessLevel', error));

// One table serves the context menu and the keyboard commands; the menu ids are the command names.
const GESTURES: Record<string, (tabId: number, selection?: string) => Promise<void>> = {
  'open-sidepanel': (tabId) => chrome.sidePanel.open({ tabId }),
  'quick-ask': (tabId, selection) => askFromGesture(tabId, deps, selection),
  'smart-clip': (tabId) => clipFromGesture(tabId, 'article', deps),
  'select-clip': (tabId) => clipFromGesture(tabId, 'select', deps),
  'quick-note': (tabId) => noteFromGesture(tabId, deps),
};

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.removeAll(() => {
    chrome.contextMenus.create({ id: 'quick-ask', title: 'Ask AI KB about this', contexts: ['selection'] });
    chrome.contextMenus.create({ id: 'smart-clip', title: 'Clip page to AI KB', contexts: ['page'] });
    chrome.contextMenus.create({ id: 'select-clip', title: 'Clip selection to AI KB', contexts: ['selection'] });
    chrome.contextMenus.create({ id: 'quick-note', title: 'Quick note', contexts: ['page'] });
  });
});

chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (tab?.id) void GESTURES[String(info.menuItemId)]?.(tab.id, info.selectionText ?? '');
});

chrome.commands.onCommand.addListener((command, tab) => {
  if (tab?.id) void GESTURES[command]?.(tab.id);
});

// For the scripted smoke run, which cannot click a menu.
globalThis.__aikb = { clipFromGesture: (tabId: number, mode: ExtractMode) => clipFromGesture(tabId, mode, deps) };

declare global {
  // eslint-disable-next-line no-var
  var __aikb: { clipFromGesture: (tabId: number, mode: ExtractMode) => Promise<void> };
}
