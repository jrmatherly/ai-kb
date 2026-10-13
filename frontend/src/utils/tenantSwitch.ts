// Tenant-switch navigation helper.
//
// Switching the active tenant always lands the user on the platform's KB
// list. It used to "reload on the current path" with a fallback to the KB list for a few sensitive paths, but even
// pages without a resource id (settings, agent list and so on) often showed an empty state after reload because the new space
// had no matching data, which felt about the same as jumping to a fixed home page. So it always goes to the KB list,
// and one full navigation resets every store, SSE stream and request.

import { updateMyPreferences } from '@/api/auth'

const SAFE_FALLBACK_PATH = '/platform/knowledge-bases'

/**
 * Return the URL to navigate to after a tenant switch. Always returns the KB list today
 * as the landing page; the signature stays so route-specific handling can be added later.
 */
export function tenantSwitchTargetPath(_currentPath: string): string {
  return SAFE_FALLBACK_PATH
}

/**
 * Perform the post-switch navigation. Always goes to the KB list.
 */
export function navigateAfterTenantSwitch(): void {
  window.location.href = tenantSwitchTargetPath(window.location.pathname)
}

// The post-switch toast survives the hard reload: the caller puts the info in
// sessionStorage before reloading, and App.vue consumes it once at startup and shows it. Calling
// NotifyPlugin right before the reload would be wiped before anyone could read it.
const PENDING_TOAST_KEY = 'aikb_pending_tenant_switch_toast'

export interface PendingTenantSwitchToast {
  name: string
  /** Human-readable role label, e.g. "Owner". Drives the role chip text. */
  role?: string
  /** Raw role enum, e.g. "owner". Drives the role chip color + icon. */
  roleEnum?: string
}

export function stashTenantSwitchToast(payload: PendingTenantSwitchToast): void {
  try {
    sessionStorage.setItem(PENDING_TOAST_KEY, JSON.stringify(payload))
  } catch {
    // If writing sessionStorage fails (private mode and so on), give up silently; the toast is a nicety
  }
}

export function consumePendingTenantSwitchToast(): PendingTenantSwitchToast | null {
  try {
    const raw = sessionStorage.getItem(PENDING_TOAST_KEY)
    if (!raw) return null
    sessionStorage.removeItem(PENDING_TOAST_KEY)
    const parsed = JSON.parse(raw)
    if (parsed && typeof parsed.name === 'string') {
      return {
        name: parsed.name,
        role: typeof parsed.role === 'string' ? parsed.role : undefined,
        roleEnum: typeof parsed.roleEnum === 'string' ? parsed.roleEnum : undefined,
      }
    }
    return null
  } catch {
    return null
  }
}

/**
 * Persist the user's "last active tenant" preference server-side so a
 * fresh login (new device, new refresh token, cleared browser) drops
 * them back into this workspace instead of always bouncing to their
 * home tenant.
 *
 * Conventions:
 *   - Pass the target tenant id when switching to a peer tenant.
 *   - Pass `null` (which sends `0`) when switching back to the home
 *     tenant — that clears the preference and reverts the user to the
 *     "always start at home" default.
 *
 * Fire-and-forget: callers usually trigger a full-page reload right
 * after; this returns the in-flight promise so callers can race it
 * against a small budget if they want best-effort completion before
 * navigation. Failures are logged but never thrown — losing one
 * persist is recoverable on the next switch.
 */
export function persistLastActiveTenantPreference(
  tenantId: number | null,
): Promise<void> {
  const payload = { last_active_tenant_id: tenantId == null ? 0 : tenantId }
  return updateMyPreferences(payload)
    .then((res) => {
      if (!res.success) {
        console.warn('persistLastActiveTenantPreference: server rejected update', res.message)
      }
    })
    .catch((err) => {
      console.warn('persistLastActiveTenantPreference: network/serialization error', err)
    })
}
