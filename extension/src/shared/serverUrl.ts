export type NormalizedServerUrl = { ok: true; url: string; warning?: 'insecure' } | { ok: false; reason: 'invalid' };

const INVALID = { ok: false, reason: 'invalid' } as const;

export function normalizeServerUrl(input: string): NormalizedServerUrl {
  const text = input.trim();
  const withScheme = /^[a-z]+:\/\//i.test(text) ? text : `https://${text}`;
  let parsed: URL;
  try {
    parsed = new URL(withScheme);
  } catch {
    return INVALID;
  }
  if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') return INVALID;
  // Credentials, a query or a fragment would be dropped silently, so refuse them instead.
  if (parsed.username || parsed.password || parsed.search || parsed.hash) return INVALID;
  const path = parsed.pathname.replace(/\/api\/v1\/?$/, '').replace(/\/+$/, '');
  const url = `${parsed.protocol}//${parsed.host}${path}`;
  return parsed.protocol === 'http:' ? { ok: true, url, warning: 'insecure' } : { ok: true, url };
}

export const apiBase = (serverUrl: string): string => `${serverUrl}/api/v1`;
export const kbUrl = (serverUrl: string, kbId: string): string => `${serverUrl}/platform/knowledge-bases/${kbId}`;
export const sessionUrl = (serverUrl: string, sessionId: string): string => `${serverUrl}/platform/chat/${sessionId}`;
