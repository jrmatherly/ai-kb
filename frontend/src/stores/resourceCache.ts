import { createVersionedRequestCoordinator } from './versionedRequest'

/**
 * Read primitives for space-level resources. No TTL:
 *
 * - `ensure()`: at most one request per resource is in flight at a time, and concurrent calls share it; when none is in flight
 *   it sends a new one. The returned Promise resolves only after the latest data is written to the store, so
 *   a read after `await ensure()` always sees this request's result, not an old snapshot.
 * - `ensure(true)`: call after a write. If a request is in flight, another one is sent after it finishes,
 *   so the data reflects the write.
 * - `invalidate()`: drop the snapshot and void the in-flight old response; the next `ensure()` always sends a new request.
 * - `markLoaded()`: the caller already fetched the latest list through another endpoint and wrote it to the store (such as
 *   `replaceModels` in settings); void the in-flight old request so it does not write old data back.
 *
 * The old snapshot stays in the store's ref so pages can render at once; freshness depends only on "was a request just sent"
 * and explicit invalidation, with no 60s window that always reads the cache.
 */
export interface CachedResource {
  ensure: (force?: boolean) => Promise<void>
  invalidate: () => void
  markLoaded: () => void
  isLoaded: () => boolean
  hasInFlightRequest: () => boolean
}

export function createCachedResource<T>(
  request: () => Promise<T>,
  apply: (value: T) => void,
): CachedResource {
  let loaded = false
  // If an old request is still in flight after invalidate, the next ensure must queue after it and send again,
  // otherwise the caller would await a Promise whose result was already voided.
  let needsFreshRequest = false

  const coordinator = createVersionedRequestCoordinator(request, (value) => {
    apply(value)
    loaded = true
  })

  return {
    ensure(force = false) {
      const mustForce = force || needsFreshRequest
      needsFreshRequest = false
      return coordinator.fetch(mustForce)
    },
    invalidate() {
      loaded = false
      coordinator.invalidate()
      needsFreshRequest = coordinator.hasInFlightRequest()
    },
    markLoaded() {
      coordinator.invalidate()
      needsFreshRequest = false
      loaded = true
    },
    isLoaded: () => loaded,
    hasInFlightRequest: () => coordinator.hasInFlightRequest(),
  }
}

/**
 * Per-key cached single resources (knowledge base details, knowledge bases visible to an agent and so on).
 * This data is not refetched on every ensure: once fetched it is reused until explicitly invalidated.
 * Callers ensure entries one by one in a loop (such as filling counts for the @ mention list), so it cannot hit the API every time.
 */
export function createKeyedSnapshotCache<T>(load: (key: string) => Promise<T>) {
  const snapshots = new Map<string, T>()
  const inflight = new Map<string, { promise: Promise<T>; revision: number }>()
  // One generation per key; invalidate and force both advance it. If the generation changed when a request returns,
  // an invalidation or a newer request happened meanwhile, so this response goes only to the caller, not into the snapshot.
  const revisions = new Map<string, number>()

  const bump = (key: string) => {
    const next = (revisions.get(key) ?? 0) + 1
    revisions.set(key, next)
    return next
  }

  return {
    async ensure(key: string, force = false): Promise<T> {
      if (!force && snapshots.has(key)) return snapshots.get(key) as T
      const existing = inflight.get(key)
      if (existing && !force) return existing.promise
      const revision = force ? bump(key) : (revisions.get(key) ?? 0)
      const promise = (async () => {
        try {
          const value = await load(key)
          const stillCurrent = (revisions.get(key) ?? 0) === revision
          // A load failure, represented as null, is not stored, so the next read retries.
          if (stillCurrent && value != null) snapshots.set(key, value)
          return value
        } finally {
          if (inflight.get(key)?.revision === revision) inflight.delete(key)
        }
      })()
      inflight.set(key, { promise, revision })
      return promise
    },
    invalidate(key?: string) {
      if (key === undefined) {
        for (const k of new Set([...snapshots.keys(), ...inflight.keys()])) bump(k)
        snapshots.clear()
        inflight.clear()
        return
      }
      bump(key)
      snapshots.delete(key)
      inflight.delete(key)
    },
  }
}
