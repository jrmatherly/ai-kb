import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { checkManifest } from './check-manifest.mjs';

const load = (p) => JSON.parse(readFileSync(new URL(p, import.meta.url), 'utf8'));
const good = load('../manifest.json');

test('accepts the shipped manifest', () => assert.deepEqual(checkManifest(good, '1.0.0'), []));

test('rejects broad host permissions', () => assert.deepEqual(
  checkManifest({ ...good, host_permissions: ['<all_urls>'] }, '1.0.0'), ['host_permissions must be absent']));

test('rejects a version mismatch', () => assert.deepEqual(
  checkManifest(good, '1.0.1'), ['version 1.0.0 does not match 1.0.1']));

test('rejects a permission set that differs from the six', () => {
  assert.deepEqual(checkManifest({ ...good, permissions: [...good.permissions, 'tabs'] }, '1.0.0'),
    ['permissions must be exactly storage, sidePanel, activeTab, scripting, contextMenus, identity']);
  assert.deepEqual(checkManifest({ ...good, permissions: good.permissions.slice(1) }, '1.0.0'),
    ['permissions must be exactly storage, sidePanel, activeTab, scripting, contextMenus, identity']);
});

test('rejects optional host permissions other than the two wildcards', () => assert.deepEqual(
  checkManifest({ ...good, optional_host_permissions: ['https://*/*'] }, '1.0.0'),
  ['optional_host_permissions must be exactly https://*/*, http://*/*']));

test('rejects a panel that opens on the action click or a missing popup', () => {
  assert.deepEqual(checkManifest({ ...good, action: {} }, '1.0.0'), ['action.default_popup must be popup.html']);
  assert.deepEqual(checkManifest({ ...good, side_panel: { ...good.side_panel, openPanelOnActionClick: true } }, '1.0.0'),
    ['side_panel.openPanelOnActionClick must be absent']);
});

test('rejects more than four suggested keys, content_scripts, web_accessible_resources, externally_connectable, and a missing options_ui', () => {
  const commands = { ...good.commands, extra: { suggested_key: { default: 'Alt+Shift+Z' }, description: 'x' } };
  const { options_ui, ...noOptions } = good;
  assert.deepEqual(checkManifest({ ...good, commands }, '1.0.0'), ['at most four commands may carry suggested_key']);
  assert.deepEqual(checkManifest({ ...good, content_scripts: [] }, '1.0.0'), ['content_scripts must be absent']);
  assert.deepEqual(checkManifest({ ...good, web_accessible_resources: [] }, '1.0.0'), ['web_accessible_resources must be absent']);
  assert.deepEqual(checkManifest({ ...good, externally_connectable: {} }, '1.0.0'), ['externally_connectable must be absent']);
  assert.deepEqual(checkManifest(noOptions, '1.0.0'),
    ['options_ui must be { page: options.html, open_in_tab: true }']);
});
