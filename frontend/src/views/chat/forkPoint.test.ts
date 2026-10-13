import assert from 'node:assert/strict'
import test from 'node:test'
import { resolveForkAffordance } from './forkPoint'

test('the first user message can be forked', () => {
  const messages = [
    { id: 'u1', role: 'user' },
    { id: 'a1', role: 'assistant' },
  ]
  assert.deepEqual(resolveForkAffordance(messages, 'u1'), { canFork: true })
})

test('a preceding assistant message can be forked with or without a checkpoint', () => {
  const messages = [
    { id: 'u1', role: 'user' },
    { id: 'a1', role: 'assistant' },
    { id: 'u2', role: 'user' },
  ]
  assert.deepEqual(resolveForkAffordance(messages, 'u2'), { canFork: true })
})

test('an assistant message can be forked', () => {
  const messages = [
    { id: 'u1', role: 'user' },
    { id: 'a1', role: 'assistant', is_completed: true },
  ]
  assert.deepEqual(resolveForkAffordance(messages, 'a1'), { canFork: true })
})

test('an unknown role cannot be a fork point', () => {
  const messages = [
    { id: 'u1', role: 'user' },
    { id: 's1', role: 'system' },
  ]
  assert.deepEqual(resolveForkAffordance(messages, 's1'), { canFork: false })
})

test('an unknown message ID cannot be forked', () => {
  assert.deepEqual(
    resolveForkAffordance([{ id: 'u1', role: 'user' }], 'nope'),
    { canFork: false },
  )
})

test('an unfinished assistant message means the turn is still running, so it cannot be forked', () => {
  const messages = [
    { id: 'u1', role: 'user' },
    { id: 'a1', role: 'assistant', is_completed: false },
  ]
  assert.deepEqual(resolveForkAffordance(messages, 'u1'), { canFork: false })
})
