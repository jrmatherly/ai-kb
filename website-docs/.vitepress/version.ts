import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const versionFile = resolve(import.meta.dirname, '../VERSION')

/** Site release version from website-docs/VERSION, so a standalone build need not read the parent directory */
export function getRepoVersion(): string {
  if (!existsSync(versionFile)) return 'unknown'
  return readFileSync(versionFile, 'utf-8').trim() || 'unknown'
}

export const repoVersion = getRepoVersion()

export const repoVersionLabel =
  repoVersion === 'unknown' || repoVersion.startsWith('v') ? repoVersion : `v${repoVersion}`
