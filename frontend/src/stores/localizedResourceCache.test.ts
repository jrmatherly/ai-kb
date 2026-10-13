import assert from 'node:assert/strict'
import test from 'node:test'
import {
  isLocalizedSnapshotUsable,
  shouldForceLocalizedRefetch,
  shouldReuseLocalizedInflight,
} from './localizedResourceCache.ts'

test('localized snapshot is usable only for the locale it was fetched in', () => {
  assert.equal(isLocalizedSnapshotUsable(true, 'en-GB', 'en-GB'), true)
  assert.equal(isLocalizedSnapshotUsable(true, 'en-GB', 'en-US'), false)
  assert.equal(isLocalizedSnapshotUsable(false, 'en-GB', 'en-GB'), false)
  assert.equal(isLocalizedSnapshotUsable(true, '', 'en-GB'), false)
})

test('landing prefetch must not be reused after the UI language changes', () => {
  // Request started as en-GB; user switched to en-US before it settled.
  assert.equal(shouldReuseLocalizedInflight(true, 'en-GB', 'en-US'), false)
  assert.equal(shouldForceLocalizedRefetch(true, 'en-GB', 'en-US'), true)

  assert.equal(shouldReuseLocalizedInflight(true, 'en-US', 'en-US'), true)
  assert.equal(shouldForceLocalizedRefetch(true, 'en-US', 'en-US'), false)
  assert.equal(shouldReuseLocalizedInflight(false, 'en-GB', 'en-GB'), false)
  assert.equal(shouldReuseLocalizedInflight(true, '', 'en-US'), false)
})

test('stamping the request locale keeps a late en-GB payload from looking usable for en-US', () => {
  const requestLocale = 'en-GB'
  // Wrong (old bug): stamp getCurrentLanguage() after await → 'en-US'.
  assert.equal(isLocalizedSnapshotUsable(true, 'en-US', 'en-US'), true)
  // Correct: stamp the locale the HTTP call was started with.
  assert.equal(isLocalizedSnapshotUsable(true, requestLocale, 'en-US'), false)
})
