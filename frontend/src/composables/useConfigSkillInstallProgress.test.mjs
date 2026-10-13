import assert from 'node:assert/strict'
import test from 'node:test'

import {
  clampInstallPercent,
  formatBusyInstallStatus,
  liveInstallPercent,
  progressKey,
} from './skillInstallProgress.ts'

test('progress keys isolate one install per sandbox', () => {
  assert.equal(progressKey('cfg-a', 'skill-1'), 'cfg-a:skill-1')
})

test('percent values are clamped to 0-100', () => {
  assert.equal(clampInstallPercent(37.4), 37)
  assert.equal(clampInstallPercent(-8), 0)
  assert.equal(clampInstallPercent(140), 100)
  assert.equal(clampInstallPercent('37'), null)
  assert.equal(clampInstallPercent(Number.NaN), null)
})

test('live percent hides a closed stream that never reported progress', () => {
  assert.equal(liveInstallPercent(undefined), null)
  assert.equal(liveInstallPercent({ percent: 0, stage: 'installing', done: true }), null)
  assert.equal(liveInstallPercent({ percent: 0, stage: 'accepted', done: false }), 0)
  assert.equal(liveInstallPercent({ percent: 37, stage: 'installing', done: false }), 37)
  assert.equal(liveInstallPercent({ percent: 100, stage: 'done', done: true }), 100)
})

test('busy status appends a live percent when one exists', () => {
  assert.equal(formatBusyInstallStatus('Installing', null), 'Installing')
  assert.equal(formatBusyInstallStatus('Installing', 37), 'Installing · 37%')
})
