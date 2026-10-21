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

test('every applied theme is mirrored into the key the pre-paint script reads', async () => {
  const saved = Object.fromEntries(['localStorage', 'window', 'document'].map(k => [k, Object.getOwnPropertyDescriptor(globalThis, k)]))
  const store = new Map<string, string>([['aikb_user', JSON.stringify({ id: 'u1' })], ['AIKB_u1_theme', 'light']])
  const define = (key: string, value: unknown) => Object.defineProperty(globalThis, key, { configurable: true, value })
  define('localStorage', {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => { store.set(k, v) },
    removeItem: (k: string) => { store.delete(k) },
  })
  define('window', { matchMedia: () => ({ matches: true, addEventListener() {} }) })
  define('document', { documentElement: { setAttribute() {}, style: {} }, getElementById: () => null })
  try {
    const { useTheme, initTheme, reloadThemeFromStorage, BOOT_THEME_KEY } = await import('./useTheme.ts')
    assert.equal(BOOT_THEME_KEY, 'AIKB_boot_theme')
    initTheme()
    assert.equal(store.get('AIKB_boot_theme'), 'light')
    useTheme().setTheme('dark')
    assert.equal(store.get('AIKB_boot_theme'), 'dark')
    store.set('AIKB_u1_theme', 'system')
    reloadThemeFromStorage()
    assert.equal(store.get('AIKB_boot_theme'), 'system')
  } finally {
    for (const [k, d] of Object.entries(saved)) {
      if (d) Object.defineProperty(globalThis, k, d)
      else Reflect.deleteProperty(globalThis, k)
    }
  }
})
