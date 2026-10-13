/**
 * Access context for protected files (provider://, resource:// and so on).
 *
 * The backend splits the file proxy by access principal, and each has its own auth model:
 *   - `/files`                                -> signed-in Bearer + X-Tenant-ID
 *   - `/api/v1/knowledge-bases/:id/files`     -> knowledge base access (cross-tenant shared bases)
 *   - `/api/v1/sessions/:id/messages/:mid/files` -> session message ownership + shared agent permission
 *   - `/api/v1/embed/:channel_id/files`       -> the embed visitor's Embed token
 *
 * Which proxy to use depends on the auth plane of the current request, not on which component renders the image.
 * This collapses that decision into one source of truth; rendering components only declare a scope instead of building URLs themselves.
 */

export const PROVIDER_SCHEME_PATTERN = 'resource|local|minio|cos|tos|s3|oss|ks3|obs';

const PROVIDER_FILE_SCHEME_RE = new RegExp(`^(${PROVIDER_SCHEME_PATTERN}):\\/\\/\\S+$`, 'i');
const STORAGE_BACKEND_FILE_SCHEME_RE = new RegExp(
  `^storage:\\/\\/[0-9A-Za-z_-]+\\/(${PROVIDER_SCHEME_PATTERN}):\\/\\/\\S+$`,
  'i',
);

const KB_FILE_PROXY_PATH_RE = /^\/api\/v1\/knowledge-bases\/[^/]+\/files$/;
const EMBED_FILE_PROXY_PATH_RE = /^\/api\/v1\/embed\/[^/]+\/files$/;
const MESSAGE_FILE_PROXY_PATH_RE = /^\/api\/v1\/sessions\/[^/]+\/messages\/[^/]+\/files$/;

export type ProtectedFileAccessContext =
  /** Signed-in user: Bearer + selected tenant. */
  | { mode: 'tenant' }
  /** Embed visitor: holds only an Embed token, with no Bearer and no tenant context. */
  | { mode: 'embed'; channelId: string; token: string }
  /** Knowledge base scope: a signed-in user reads objects in a shared base that belong to another tenant. */
  | { mode: 'knowledgeBase'; kbId: string }
  /** Message scope: a signed-in user reads source-space resources in a shared agent's reply. */
  | { mode: 'message'; sessionId: string; messageId: string };

export interface ProtectedFileRequest {
  url: string;
  headers: Record<string, string>;
}

const TENANT_ACCESS: ProtectedFileAccessContext = { mode: 'tenant' };

interface ProtectedFileAccessState {
  current: ProtectedFileAccessContext;
}

// Kept on window like the blob cache: Vite hot reload replaces the module but does not rebuild the document,
// so a module-level variable would lose the context the embed app registered at startup.
const accessState: ProtectedFileAccessState = (() => {
  const fresh = (): ProtectedFileAccessState => ({ current: TENANT_ACCESS });
  if (typeof window === 'undefined') return fresh();
  const scope = window as typeof window & {
    __aikbProtectedFileAccessV1__?: ProtectedFileAccessState;
  };
  scope.__aikbProtectedFileAccessV1__ ||= fresh();
  return scope.__aikbProtectedFileAccessV1__;
})();

/**
 * Register the default access context for the current document. The app entry calls it once (the embed app registers after it gets
 * channelId/token), and from then on every protected file request uses the matching proxy.
 */
export function setDefaultProtectedFileAccess(
  access: ProtectedFileAccessContext | null,
): void {
  accessState.current = access ?? TENANT_ACCESS;
}

export function getDefaultProtectedFileAccess(): ProtectedFileAccessContext {
  return accessState.current;
}

/**
 * Merge the default context with the scope a component passes in.
 *
 * The default context carries the auth plane (embed visitor vs signed-in user); a component override can only
 * narrow the scope within the same plane. An embed visitor has no Bearer, so letting a `knowledgeBase` override
 * replace the embed plane would send the request to a proxy that needs sign-in and return 401.
 */
export function resolveProtectedFileAccess(
  override?: ProtectedFileAccessContext | null,
): ProtectedFileAccessContext {
  const fallback = accessState.current;
  if (fallback.mode === 'embed') return fallback;
  if (!override) return fallback;
  if (override.mode === 'knowledgeBase' && !override.kbId.trim()) return fallback;
  if (override.mode === 'message' && (!override.sessionId.trim() || !override.messageId.trim())) {
    return fallback;
  }
  return override;
}

/** Whether this is a storage path that must be fetched through a proxy (provider:// or storage://<backend>/provider://). */
export function isProviderFileURL(url: string): boolean {
  const trimmed = url.trim();
  return PROVIDER_FILE_SCHEME_RE.test(trimmed) || STORAGE_BACKEND_FILE_SCHEME_RE.test(trimmed);
}

/** Whether this is a path of one of the protected file proxies. */
export function isProtectedFileProxyPath(pathname: string): boolean {
  return (
    pathname === '/files'
    || KB_FILE_PROXY_PATH_RE.test(pathname)
    || MESSAGE_FILE_PROXY_PATH_RE.test(pathname)
    || EMBED_FILE_PROXY_PATH_RE.test(pathname)
  );
}

function tenantRequestHeaders(): Record<string, string> {
  const headers: Record<string, string> = {};
  try {
    const token = (localStorage.getItem('aikb_token') || '').trim();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }

    const selectedTenantId = (localStorage.getItem('aikb_selected_tenant_id') || '').trim();
    if (selectedTenantId) {
      // Always attach when a selected tenant is set. Same rationale as
      // utils/request.ts / api/chat/streame.ts: the
      // "selectedTenantId === defaultTenantId → skip" short-circuit
      // silently drops the header whenever any code path writes the
      // active tenant into aikb_tenant, leaving authenticated file
      // fetches landing on the home tenant.
      headers['X-Tenant-ID'] = selectedTenantId;
    }
  } catch {
    // ignore localStorage read errors
  }
  return headers;
}

/**
 * Build the proxy request for a storage path. Returns null when the current context cannot send it
 * (not a storage path, or the embed context has no token yet); the caller should skip it and retry later.
 */
export function buildProtectedFileRequest(
  sourceURL: string,
  access: ProtectedFileAccessContext,
): ProtectedFileRequest | null {
  const filePath = sourceURL.trim();
  if (!isProviderFileURL(filePath)) return null;

  const query = new URLSearchParams({ file_path: filePath }).toString();

  if (access.mode === 'embed') {
    const channelId = access.channelId.trim();
    const token = access.token.trim();
    // An embed visitor's only credential is the Embed token; falling back to /files without it only gets a 401,
    // so skip and wait for the next hydration after bootstrap completes.
    if (!channelId || !token) return null;
    return {
      url: `/api/v1/embed/${encodeURIComponent(channelId)}/files?${query}`,
      headers: { Authorization: `Embed ${token}` },
    };
  }

  if (access.mode === 'knowledgeBase') {
    return {
      url: `/api/v1/knowledge-bases/${encodeURIComponent(access.kbId.trim())}/files?${query}`,
      headers: tenantRequestHeaders(),
    };
  }

  if (access.mode === 'message') {
    return {
      url: `/api/v1/sessions/${encodeURIComponent(access.sessionId.trim())}/messages/${encodeURIComponent(access.messageId.trim())}/files?${query}`,
      headers: tenantRequestHeaders(),
    };
  }

  return { url: `/files?${query}`, headers: tenantRequestHeaders() };
}
