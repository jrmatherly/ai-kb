// Shared by every probe page: append one outcome under storage.local.probe.
globalThis.probeLog = async (name, value) => {
  const { probe = [] } = await chrome.storage.local.get('probe');
  probe.push({ name, value, at: new Date().toISOString(), ua: navigator.userAgent });
  await chrome.storage.local.set({ probe });
  return value;
};
globalThis.errText = (e) => (e && e.message) || String(e);
globalThis.out = (v) => (document.getElementById('out').textContent += `${JSON.stringify(v)}\n`);
