import { browserFetch } from '../shared/browserFetch';
import { apiBase } from '../shared/serverUrl';
import type { OidcResult } from '../shared/types';

export class OidcError extends Error {
  constructor(public readonly code: 'cancelled' | 'not_allowed' | 'failed' | (string & {}), public readonly description?: string) {
    super(description ?? code);
    this.name = 'OidcError';
  }
}

export type OidcFragment = { kind: 'result'; result: OidcResult } | { kind: 'error'; code: string; description?: string } | { kind: 'none' };

// Same decoding as decodeOIDCResult in frontend/src/App.vue: base64url, atob, then UTF-8.
function decodeResult(encoded: string): OidcResult {
  const normalized = encoded.replace(/-/g, '+').replace(/_/g, '/');
  const binary = atob(normalized + '='.repeat((4 - (normalized.length % 4)) % 4));
  return JSON.parse(new TextDecoder().decode(Uint8Array.from(binary, (c) => c.charCodeAt(0)))) as OidcResult;
}

export function parseOidcFragment(url: string): OidcFragment {
  const hash = url.indexOf('#');
  if (hash < 0) return { kind: 'none' };
  const params = new URLSearchParams(url.slice(hash + 1));
  const code = params.get('oidc_error');
  if (code) return { kind: 'error', code, description: params.get('oidc_error_description') ?? undefined };
  const result = params.get('oidc_result');
  if (!result) return { kind: 'none' };
  try {
    return { kind: 'result', result: decodeResult(result) };
  } catch {
    return { kind: 'error', code: 'failed', description: 'The sign-in result could not be read' };
  }
}

// Browser rejection texts measured in private/findings/research-2026-10-16/extension-live-run.md.
function fromRejection(error: unknown): OidcError {
  const message = error instanceof Error ? error.message : String(error);
  if (/WINDOW_CLOSED|did not approve/.test(message)) return new OidcError('cancelled');
  if (/could not be loaded/.test(message)) return new OidcError('failed', 'The sign-in page could not be loaded');
  return new OidcError('failed', message);
}

type Deps = {
  launch: (details: { url: string; interactive?: boolean }) => Promise<string | undefined>;
  redirectUrl: () => string;
  fetchFn?: typeof fetch;
};

export function useOidc(deps: Deps) {
  const returnTo = () => deps.redirectUrl();
  const startUrl = (serverUrl: string) => `${apiBase(serverUrl)}/auth/oidc/start?return_to=${encodeURIComponent(returnTo())}`;

  // The auth window reports every failed load with one text, so the start route is asked first:
  // 400 is the unlisted return URL, a redirect (opaque to fetch) means the IdP is next.
  async function allowed(url: string): Promise<void> {
    const response = await (deps.fetchFn ?? browserFetch)(url, { redirect: 'manual' }).catch((error: Error) => {
      throw new OidcError('failed', `Cannot reach the server: ${error.message}`);
    });
    if (response.status === 400) throw new OidcError('not_allowed');
    if (response.status >= 400) throw new OidcError('failed', `The server answered ${response.status}`);
  }

  async function signIn(serverUrl: string): Promise<OidcResult> {
    const url = startUrl(serverUrl);
    await allowed(url);
    let redirected: string | undefined;
    try {
      redirected = await deps.launch({ url, interactive: true });
    } catch (error) {
      throw fromRejection(error);
    }
    const fragment = parseOidcFragment(redirected ?? '');
    if (fragment.kind === 'error') throw new OidcError(fragment.code, fragment.description);
    if (fragment.kind === 'none') throw new OidcError('cancelled');
    return fragment.result;
  }

  return { returnTo, startUrl, signIn };
}
