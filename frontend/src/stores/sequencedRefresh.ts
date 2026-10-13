/**
 * Request sequencer with "refresh" semantics (for singleton reads such as /auth/me).
 *
 * - `refresh()`: the result must come from a request sent after this call. When a request is in flight,
 *   queue a new one after it; concurrent refresh calls share the same queued request.
 * - `share()`: reuse any in-flight request, otherwise send a new one. Use it when several first-screen consumers
 *   want the same data at once.
 * - `invalidate()`: void in-flight requests. The `isCurrent()` that `run` receives returns false after await,
 *   so the caller skips storing the result; used on logout so a late response does not write the session back.
 */
export interface SequencedRefresh<T> {
  refresh: () => Promise<T>
  share: () => Promise<T>
  invalidate: () => void
  hasInFlightRequest: () => boolean
}

export function createSequencedRefresh<T>(
  run: (isCurrent: () => boolean) => Promise<T>,
): SequencedRefresh<T> {
  let revision = 0
  let inflight: Promise<T> | null = null
  let queued: Promise<T> | null = null

  const start = (): Promise<T> => {
    const requestRevision = revision
    const current = run(() => requestRevision === revision).finally(() => {
      if (inflight === current) inflight = null
    })
    inflight = current
    return current
  }

  const refresh = (): Promise<T> => {
    if (!inflight) return start()
    if (!queued) {
      const waitingFor = inflight
      queued = waitingFor.then(
        () => {
          queued = null
          return start()
        },
        () => {
          queued = null
          return start()
        },
      )
    }
    return queued
  }

  return {
    refresh,
    share: () => inflight ?? start(),
    invalidate: () => {
      revision += 1
    },
    hasInFlightRequest: () => inflight !== null,
  }
}
