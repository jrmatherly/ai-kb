import assert from 'node:assert/strict'
import test from 'node:test'
import { filterPlatformOptions } from './imPlatforms'

const options = [
  { value: 'wecom', label: 'WeCom' },
  { value: 'slack', label: 'Slack' },
  { value: 'telegram', label: 'Telegram' },
]

test('an old payload without im_platforms keeps every platform', () => {
  assert.deepEqual(filterPlatformOptions(options, undefined), options)
})

test('only enabled platforms are offered, in option order', () => {
  assert.deepEqual(filterPlatformOptions(options, ['telegram', 'slack']).map((o) => o.value), ['slack', 'telegram'])
})

test('an empty allow-list offers nothing', () => {
  assert.deepEqual(filterPlatformOptions(options, []), [])
})

test('the platform of the channel being edited stays listed', () => {
  assert.deepEqual(filterPlatformOptions(options, ['slack'], 'wecom').map((o) => o.value), ['wecom', 'slack'])
})
