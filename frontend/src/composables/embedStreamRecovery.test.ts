import assert from 'node:assert/strict'
import { test } from 'node:test'
import { isTurnOpen, serverTurnDone } from './embedStreamRecovery.ts'

test('isTurnOpen', () => {
  const user = { role: 'user', content: 'q' }
  assert.equal(isTurnOpen(false, [user, { role: 'assistant', is_completed: true }]), false)
  assert.equal(isTurnOpen(false, [user, { role: 'assistant', is_completed: false }]), true)
  assert.equal(isTurnOpen(true, [user]), true, 'cut before agent_query created a row')
  assert.equal(isTurnOpen(false, []), false)
  const stale = { role: 'assistant', is_completed: false }
  assert.equal(
    isTurnOpen(false, [stale, user, { role: 'assistant', is_completed: true }]),
    false,
    'an interrupted row earlier in history does not reopen a finished turn',
  )
})

test('serverTurnDone', () => {
  assert.equal(serverTurnDone(undefined), false)
  assert.equal(serverTurnDone([]), false)
  assert.equal(serverTurnDone([{ role: 'user' }]), false, 'assistant row not persisted yet')
  assert.equal(serverTurnDone([{ role: 'user' }, { role: 'assistant', is_completed: false }]), false)
  assert.equal(serverTurnDone([{ role: 'user' }, { role: 'assistant', is_completed: true }]), true)
})
