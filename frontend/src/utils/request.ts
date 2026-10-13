// src/utils/request.js
import axios from "axios";
import { generateRandomString, MAX_FILE_SIZE_MB, MAX_SKILL_BUNDLE_SIZE_MB } from "./index";
import i18n from '@/i18n'
import { getApiBaseUrl } from './api-base';
import { isSkillBundleUploadUrl } from './uploadLimit';
import { isTimeoutError, uploadTimeoutMs } from './requestTimeouts';
import {
  forceReloginRedirect,
  isEmbedPage,
  refreshAccessTokenShared,
} from './authRefresh';

export { forceReloginRedirect, refreshAccessTokenShared };

const t = (key: string) => i18n.global.t(key)

const BASE_URL = getApiBaseUrl();

/**
 * Response payload augmented with the HTTP status code.
 *
 * `$httpStatus` lets callers distinguish outcomes that share a success shape.
 * Defined as a non-enumerable property, so it stays invisible to object spread,
 * JSON.stringify and Object.keys and never leaks into downstream payloads.
 *
 * Guaranteed only for JSON responses (objects/arrays). Blob, string and SSE
 * stream responses do not carry it at runtime, so only read `$httpStatus`
 * when the payload is known to be an object.
 */
export type WithStatus<T> = T & {
  /** HTTP status code of the response. Non-enumerable. See {@link WithStatus}. */
  readonly $httpStatus: number
};

const HTTP_STATUS_KEY = '$httpStatus';

/**
 * Attach the non-enumerable `$httpStatus` property to a response payload
 * in place and return it. Primitives pass through untouched.
 * See {@link WithStatus} for where the property is guaranteed.
 */
function withHttpStatus<T>(data: T, status: number): T {
  if (data !== null && typeof data === 'object') {
    Object.defineProperty(data, HTTP_STATUS_KEY, {
      value: status,
      enumerable: false,
      configurable: true,
      writable: false,
    });
  }
  return data;
}

const instance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
  headers: {
    "Content-Type": "application/json",
    "X-Request-ID": `${generateRandomString(12)}`,
  },
});

// Current UI language, sent as the Accept-Language header.
export function getCurrentLanguage(): string {
  return i18n.global.locale?.value || localStorage.getItem('locale') || 'en-US'
}


instance.interceptors.request.use(
  (config) => {
    const existingAuth = config.headers?.Authorization ?? config.headers?.authorization;
    const isEmbedAuth = typeof existingAuth === 'string' && existingAuth.startsWith('Embed ');
    const isEmbedPath = typeof config.url === 'string' && config.url.includes('/api/v1/embed/');

    // Embed channels authenticate with an Embed token; overwriting it with the local JWT makes the debug page return 401.
    if (!isEmbedAuth) {
      const token = localStorage.getItem('aikb_token');
      if (token) {
        config.headers["Authorization"] = `Bearer ${token}`;
      }
    }
    
    config.headers["Accept-Language"] = getCurrentLanguage();
    
    // Send X-Tenant-ID on every request once setSelectedTenant has stored an
    // active workspace. Skipping it when selectedTenantId === defaultTenantId
    // broke switching: any code that writes aikb_tenant as the active workspace
    // (OIDC callback, UserMenu loadUserInfo, router hydrate) makes the two
    // equal, later requests silently drop the header, and the UI looks switched
    // while requests still run in the home workspace. The backend's
    // IsTenantAccessible already accepts a header pointing at the home
    // workspace, so always sending it adds no risk.
    if (!isEmbedAuth && !isEmbedPath) {
      const selectedTenantId = localStorage.getItem('aikb_selected_tenant_id');
      if (selectedTenantId) {
        config.headers["X-Tenant-ID"] = selectedTenantId;
      }
    }
    
    config.headers["X-Request-ID"] = `${generateRandomString(12)}`;
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Share-link endpoints (/auth/invitations/lookup, /auth/register-by-invite)
// are reachable by anonymous users opening an invite link. A 401 from these
// must surface to the page (e.g. expired token), not trigger the
// refresh-then-redirect-to-login flow (issue #1617). '/auth/register' already
// covers '/auth/register-by-invite' via substring match.
const PUBLIC_AUTH_PATHS = ['/auth/auto-setup', '/auth/login', '/auth/register', '/auth/oidc/', '/auth/invitations/lookup', '/api/v1/embed/'];

function isPublicAuthRequest(url?: string): boolean {
  if (!url) return false;
  return PUBLIC_AUTH_PATHS.some(p => url.includes(p));
}

instance.interceptors.response.use(
  (response) => {
    const { status, data } = response;
    if (status >= 200 && status < 300) {
      return withHttpStatus(data, status);
    } else {
      return Promise.reject(withHttpStatus(data, status));
    }
  },
  async (error: any) => {
    const originalRequest = error.config;
    
    if (!error.response) {
      // A timeout and an unreachable server both arrive without a response, but
      // telling someone whose upload timed out to "check your connection" sends
      // them after the wrong problem.
      return Promise.reject({
        message: t(isTimeoutError(error) ? 'error.requestTimeout' : 'error.networkError'),
      });
    }

    // A failed file download still returns JSON; decode it so the Blob does not hide the error.
    // Content-Type is not trusted: a gateway may rewrite errors as text/plain or no type.
    if (typeof Blob !== 'undefined' && error.response.data instanceof Blob) {
      try {
        const text = (await error.response.data.text()).trim();
        if (text.startsWith('{') || text.startsWith('[') || error.response.data.type.includes('json')) {
          error.response.data = JSON.parse(text);
        }
      } catch {
        // Invalid JSON falls through to the normal error handling.
      }
    }
    
    // A 401 from a public endpoint (auto-setup, login, register, oidc) is returned as is, without refresh.
    if ((error.response.status === 401 || error.response.status === 403) && isPublicAuthRequest(originalRequest?.url)) {
      const { status, data } = error.response;
      const msg = typeof data === 'object'
        ? (typeof data?.error === 'string' ? data.error : (data?.error?.message || data?.message))
        : data;
      return Promise.reject(withHttpStatus({ status, message: msg || t('error.invalidCredentials') }, status));
    }

    // Embed debug page and widget have no JWT: reject instead of refresh and redirect to /login.
    if (error.response.status === 401 && isEmbedPage()) {
      const { status, data } = error.response;
      const msg = typeof data === 'object'
        ? (typeof data?.error === 'string' ? data.error : (data?.error?.message || data?.message))
        : data;
      return Promise.reject(withHttpStatus({ status, message: msg || t('error.invalidCredentials') }, status));
    }

    if (error.response.status === 401 && !originalRequest._retry && !originalRequest.url?.includes('/auth/refresh')) {
      originalRequest._retry = true;
      try {
        const token = await refreshAccessTokenShared({
          messages: {
            pleaseRelogin: t('error.pleaseRelogin'),
            tokenRefreshFailed: t('error.tokenRefreshFailed'),
          },
        });
        originalRequest.headers['Authorization'] = 'Bearer ' + token;
        return instance(originalRequest);
      } catch (refreshError) {
        // refreshAccessTokenShared already cleared credentials and redirected.
        return Promise.reject(refreshError);
      }
    }
    
    // Nginx 413 Request Entity Too Large
    const ERR_ENTITY_TOO_LARGE = 413;
    if (error.response.status === ERR_ENTITY_TOO_LARGE) {
      const skillUpload = isSkillBundleUploadUrl(error.config?.url)
      return Promise.reject(withHttpStatus({
        status: ERR_ENTITY_TOO_LARGE,
        message: skillUpload
          ? i18n.global.t('settings.sandbox.skillBundleTooLarge', { size: MAX_SKILL_BUNDLE_SIZE_MB })
          : i18n.global.t('error.fileSizeExceeded', { size: MAX_FILE_SIZE_MB }),
        success: false
      }, ERR_ENTITY_TOO_LARGE));
    }

    const { status, data } = error.response;
    // Backend shape: { success: false, error: { code, message, details } }.
    // error.message is lifted to the top-level message, and the HTTP status is attached for callers.
    let errorMessage: string | undefined;
    if (typeof data === 'object') {
      if (typeof data?.error === 'string') {
        errorMessage = data.error;
      } else if (data?.error?.message) {
        errorMessage = data.error.message;
      } else {
        errorMessage = data?.message;
      }
    } else if (typeof data === 'string') {
      errorMessage = data;
    }
    return Promise.reject(withHttpStatus({
      status,
      message: errorMessage,
      ...(typeof data === 'object' ? data : {}) 
    }, status));
  }
);

export function get<T = any>(url: string, config?: any): Promise<WithStatus<T>> {
  return instance.get<T>(url, config) as unknown as Promise<WithStatus<T>>;
}

export async function getDown(url: string): Promise<Blob> {
  const res = await instance.get<Blob>(url, {
    responseType: "blob",
  }) as unknown as Blob;
  return res
}

export function postUpload(
  url: string,
  data = {},
  onUploadProgress?: (progressEvent: any) => void,
  config: any = {},
): Promise<WithStatus<any>> {
  return instance.post(url, data, {
    // Uploads are bounded by transfer time, not by the 30s default that suits
    // JSON calls. Derive the budget from the payload so a deployment raising
    // MAX_FILE_SIZE_MB doesn't silently abort its own uploads; an explicit
    // `config.timeout` still wins.
    timeout: uploadTimeoutMs(data),
    ...config,
    headers: {
      "Content-Type": "multipart/form-data",
      "X-Request-ID": `${generateRandomString(12)}`,
      ...(config.headers || {}),
    },
    onUploadProgress: onUploadProgress || config.onUploadProgress,
  }) as unknown as Promise<any>;
}

export function postChat<T = any>(url: string, data = {}): Promise<T> {
  // SSE stream: body is a string, so no `$httpStatus` is attached (see WithStatus).
  return instance.post(url, data, {
    headers: {
      "Content-Type": "text/event-stream;charset=utf-8",
      "X-Request-ID": `${generateRandomString(12)}`,
    },
  }) as unknown as Promise<T>;
}

export function post<T = any>(url: string, data = {}, config?: any): Promise<WithStatus<T>> {
  return instance.post<T>(url, data, config) as unknown as Promise<WithStatus<T>>;
}

export function put<T = any>(url: string, data = {}, config?: any): Promise<WithStatus<T>> {
  return instance.put<T>(url, data, config) as unknown as Promise<WithStatus<T>>;
}

export function patch<T = any>(url: string, data = {}, config?: any): Promise<WithStatus<T>> {
  return instance.patch<T>(url, data, config) as unknown as Promise<WithStatus<T>>;
}

export function del<T = any>(url: string, data?: any): Promise<WithStatus<T>> {
  return instance.delete<T>(url, { data }) as unknown as Promise<WithStatus<T>>;
}
