export interface IMPlatformOption {
  value: string
  label: string
}

/**
 * Narrow the platform picker to the deployment's AIKB_IM_PLATFORMS allow-list
 * (`im_platforms` in GET /system/capabilities). `undefined` means an older
 * backend without the field: offer everything. The platform of a channel being
 * edited stays listed so its form still shows what it is.
 */
export function filterPlatformOptions<T extends IMPlatformOption>(
  options: T[],
  enabled: string[] | undefined,
  current?: string,
): T[] {
  if (enabled === undefined) return options
  return options.filter((option) => option.value === current || enabled.includes(option.value))
}
