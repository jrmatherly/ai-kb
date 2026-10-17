// Asserts the manifest constraints the design relies on (no tabs, no per-site grants, popup-opening action).
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const PERMISSIONS = ['storage', 'sidePanel', 'activeTab', 'scripting', 'contextMenus', 'identity'];
const OPTIONAL_HOSTS = ['https://*/*', 'http://*/*'];
const ABSENT = ['host_permissions', 'content_scripts', 'web_accessible_resources', 'externally_connectable'];

const same = (a, b) => Array.isArray(a) && a.length === b.length && a.every((x, i) => x === b[i]);

export function checkManifest(manifest, expectedVersion) {
  const errors = [];
  if (manifest.version !== expectedVersion) errors.push(`version ${manifest.version} does not match ${expectedVersion}`);
  if (!same(manifest.permissions, PERMISSIONS)) errors.push(`permissions must be exactly ${PERMISSIONS.join(', ')}`);
  if (!same(manifest.optional_host_permissions, OPTIONAL_HOSTS)) {
    errors.push(`optional_host_permissions must be exactly ${OPTIONAL_HOSTS.join(', ')}`);
  }
  for (const key of ABSENT) if (key in manifest) errors.push(`${key} must be absent`);
  if (manifest.action?.default_popup !== 'popup.html') errors.push('action.default_popup must be popup.html');
  const ui = manifest.options_ui;
  if (!ui || ui.page !== 'options.html' || ui.open_in_tab !== true) {
    errors.push('options_ui must be { page: options.html, open_in_tab: true }');
  }
  const keyed = Object.values(manifest.commands ?? {}).filter((c) => c.suggested_key).length;
  if (keyed > 4) errors.push('at most four commands may carry suggested_key');
  return errors;
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const path = process.argv[2];
  if (!path) {
    console.error('usage: check-manifest.mjs <manifest.json>');
    process.exit(2);
  }
  const pkg = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
  const errors = checkManifest(JSON.parse(readFileSync(path, 'utf8')), pkg.version);
  for (const e of errors) console.error(`${path}: ${e}`);
  process.exit(errors.length ? 1 : 0);
}
