export const SUPPORTED_LOCALES = ['en-US'] as const
export type SupportedLocale = (typeof SUPPORTED_LOCALES)[number]

export const BUILT_IN_DEFAULT: SupportedLocale = 'en-US'

function isSupportedLocale(value: string): value is SupportedLocale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value)
}

/** Resolve the UI locale. Anything but a supported tag falls back to the built-in default. */
export function resolveDefaultLocale(
  runtimeLocale?: string | null,
  buildTimeLocale?: string | null,
): SupportedLocale {
  const raw = runtimeLocale || buildTimeLocale || BUILT_IN_DEFAULT
  const candidate = typeof raw === 'string' ? raw.trim() : BUILT_IN_DEFAULT
  return isSupportedLocale(candidate) ? candidate : BUILT_IN_DEFAULT
}
