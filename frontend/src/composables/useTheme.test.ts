import assert from 'node:assert/strict'
import test from 'node:test'

test('a user with no stored theme follows the OS', async () => {
  const saved = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: { getItem: () => null, setItem: () => {}, removeItem: () => {} },
  })
  try {
    const { useTheme } = await import('./useTheme.ts')
    assert.equal(useTheme().currentTheme.value, 'system')
  } finally {
    if (saved) Object.defineProperty(globalThis, 'localStorage', saved)
    else Reflect.deleteProperty(globalThis, 'localStorage')
  }
})
