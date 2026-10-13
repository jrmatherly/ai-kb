import { post, get, put } from '@/utils/request'
import i18n from '@/i18n'

const t = (key: string) => i18n.global.t(key)

// User login API
export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  success: boolean
  message?: string
  user?: {
    id: string
    username: string
    email: string
    avatar?: string
    tenant_id: number
    can_access_all_tenants?: boolean
    is_system_admin?: boolean
    is_active: boolean
    created_at: string
    updated_at: string
  }
  tenant?: {
    id: number
    name: string
    description: string
    status: string
    business: string
    storage_quota: number
    storage_used: number
    created_at: string
    updated_at: string
  } | null
  // active_tenant mirrors `tenant` for endpoints that distinguish home
  // tenant from current tenant (e.g. /auth/register-by-invite). Only
  // one of `tenant` / `active_tenant` is populated by any given endpoint.
  active_tenant?: {
    id: number
    name: string
    description?: string
    status?: string
    business?: string
    storage_quota?: number
    storage_used?: number
    created_at?: string
    updated_at?: string
  } | null
  memberships?: MembershipInfo[]
  token?: string
  refresh_token?: string
}

export interface OIDCAuthURLResponse {
  success: boolean
  authorization_url?: string
  state?: string
  message?: string
}

export interface OIDCConfigResponse {
  success: boolean
  enabled: boolean
  provider_display_name?: string
  message?: string
}

// User registration API
export interface RegisterRequest {
  username: string
  email: string
  password: string
}

export interface RegisterResponse {
  success: boolean
  message?: string
  data?: {
    user: {
      id: string
      username: string
      email: string
    }
    tenant: {
      id: string
      name: string
    }
  }
}

// User preferences (aligned with the backend types.UserPreferences; an optional field means never set explicitly).
// When adding a key, remember: the backend service.UpdateUserPreferences must also handle it in the merge branch;
// frontend callers read it as needed and fall back to a default.
export interface UserPreferences {
  browser_search_instructions?: string | null
  // last_active_tenant_id persists the "return to the last space after refresh / a device change / re-login"
  // preference; the backend keeps using it on Login / RefreshToken only after checking the membership is valid,
  // otherwise it falls back to home and clears the field. Sending 0 to PATCH means "clear the preference".
  last_active_tenant_id?: number | null
  // oidc_only_login true means the account was provisioned automatically by OIDC and the user has not set a known password.
  oidc_only_login?: boolean
  // gallery stores the user's gallery state: the search activation mode and per-field on/off (keyed by property ID).
  // Each change overwrites the whole block; the backend validates the mode and status values.
  gallery?: {
    mode?: 'all' | 'custom'
    status?: Record<string, string>
  }
}

// User info API
export interface UserInfo {
  id: string
  username: string
  email: string
  avatar?: string
  tenant_id: string
  can_access_all_tenants?: boolean
  preferences?: UserPreferences
  is_system_admin?: boolean
  created_at: string
  updated_at: string
}

/**
 * Normalize the user JSON returned by the backend into the frontend UserInfo.
 *
 * There used to be 4 separate setUser calls (Login, autoSetup, token rehydrate,
 * the proactive /auth/me refresh), each with its own hand-written field allowlist, so every new user field had to be
 * synced in 4 places, or the field was silently filtered out. When is_system_admin shipped, one missed copy
 * hid the "System admin" entry; this factory exists to keep that kind of
 * missed copy from happening again. **Add new user fields only here**.
 *
 * fallbackTenantId is the fallback source when tenant_id is missing:
 *   - the autoSetup response has tenant.id at the top level, but the user object has no tenant_id
 *   - /auth/me occasionally returns only user without tenant, which also uses the fallback
 * Callers pass it as needed; without it the value stays an empty string (matching past behavior).
 *
 * Fields are read with `=== true` rather than `|| false`, to strictly normalize the occasional non-boolean
 * type (the backend one day sending 1/0 or a string), so a truthy string is not
 * mistaken for a permission grant.
 */
export function userInfoFromApi(
  u: any,
  fallbackTenantId?: string | number | null,
): UserInfo {
  const rawTenantId =
    u?.tenant_id !== undefined && u?.tenant_id !== null && u.tenant_id !== ''
      ? u.tenant_id
      : fallbackTenantId ?? ''
  const tid = Number(rawTenantId) > 0 ? rawTenantId : ''
  return {
    id: u?.id || '',
    username: u?.username || '',
    email: u?.email || '',
    avatar: u?.avatar,
    tenant_id: String(tid) || '',
    can_access_all_tenants: u?.can_access_all_tenants === true,
    is_system_admin: u?.is_system_admin === true,
    preferences: u?.preferences,
    created_at: u?.created_at || new Date().toISOString(),
    updated_at: u?.updated_at || new Date().toISOString(),
  }
}

// Space info API
export interface TenantInfo {
  id: string
  name: string
  description?: string
  status?: string
  business?: string
  owner_id: string
  storage_quota?: number
  storage_used?: number
  created_at: string
  updated_at: string
  knowledge_bases?: KnowledgeBaseInfo[]
}

// Knowledge base info API
export interface KnowledgeBaseInfo {
  id: string
  name: string
  description: string
  tenant_id: string
  // creator_id is the user id of whoever originally created the KB.
  // Set by PR 5 of the multi-tenant RBAC series; nullable for legacy
  // KBs created before that migration backfilled the column.
  creator_id?: string
  // creator_name is backfilled in bulk by the backend list endpoint (username first, falling back to email),
  // used only for the origin badge on list cards; missing means it could not be resolved (deleted or old data).
  creator_name?: string
  created_at: string
  updated_at: string
  document_count?: number
  chunk_count?: number
}

// Model info API
export interface ModelInfo {
  id: string
  name: string
  type: string
  source: string
  description?: string
  is_default?: boolean
  created_at: string
  updated_at: string
}

/**
 * User login
 */
export async function login(data: LoginRequest): Promise<LoginResponse> {
  try {
    const response = await post('/api/v1/auth/login', data)
    return response as unknown as LoginResponse
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.loginFailed')
    }
  }
}

/**
 * Get the OIDC login redirect URL
 */
export async function getOIDCAuthorizationURL(redirectURI: string): Promise<OIDCAuthURLResponse> {
  try {
    const response = await get(`/api/v1/auth/oidc/url?redirect_uri=${encodeURIComponent(redirectURI)}`)
    return response as unknown as OIDCAuthURLResponse
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.loginFailed')
    }
  }
}

/**
 * Get the OIDC login config
 */
export async function getOIDCConfig(): Promise<OIDCConfigResponse> {
  try {
    const response = await get('/api/v1/auth/oidc/config')
    return response as unknown as OIDCConfigResponse
  } catch (error: any) {
    return {
      success: false,
      enabled: false,
      message: error.message || t('error.auth.loginFailed')
    }
  }
}

/**
 * Get the auth config (only the public fields the frontend needs to render, such as the registration mode).
 *
 * The backend controls self-service registration through `auth.registration_mode`:
 *   - "self_serve"  keeps the existing self-service registration entry (default)
 *   - "invite_register" only holders of a valid invitation link can register
 *   - "invite_only" registration is off; existing accounts can still accept invitations
 *
 * On failure, hide the registration entry so a disallowed registration flow is not shown.
 */
export interface AuthConfigResponse {
  success: boolean
  registration_mode: 'self_serve' | 'invite_register' | 'invite_only' | string
  complex_password_enabled: boolean
}

export async function getAuthConfig(): Promise<AuthConfigResponse> {
  try {
    const response = await get('/api/v1/auth/config')
    return response as unknown as AuthConfigResponse
  } catch {
    return { success: false, registration_mode: '', complex_password_enabled: false }
  }
}

/**
 * User registration
 */
export async function register(data: RegisterRequest): Promise<RegisterResponse> {
  try {
    const response = await post('/api/v1/auth/register', data)
    return response as unknown as RegisterResponse
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.registerFailed')
    }
  }
}

/**
 * Lite auto-initialization (creates the default user and space and issues tokens)
 */
let autoSetupPromise: Promise<LoginResponse> | null = null

export function autoSetup(): Promise<LoginResponse> {
  if (autoSetupPromise) return autoSetupPromise

  const request = (async () => {
    try {
      const nativeApp = (window as any).go?.main?.App
      if (!nativeApp?.GetAutoSetupToken) return { success: false, message: 'Desktop authentication required' }
      const token = await nativeApp.GetAutoSetupToken()
      const response = await post('/api/v1/auth/auto-setup', {}, {
        headers: { 'X-AIKB-Desktop-Token': token },
      })
      return response as unknown as LoginResponse
    } catch (error: any) {
      return {
        success: false,
        message: error.message || 'Auto-setup unavailable'
      }
    }
  })()

  autoSetupPromise = request
  void request.then(
    () => { if (autoSetupPromise === request) autoSetupPromise = null },
    () => { if (autoSetupPromise === request) autoSetupPromise = null },
  )
  return request
}

/**
 * Membership row returned alongside /auth/me. Mirrors the LoginResponse
 * shape so the frontend can refresh `currentTenantRole` on every page
 * load - without it, role changes after login (e.g. an Owner demoting
 * us in a peer tenant) stay invisible until the user logs out and back
 * in.
 */
export interface MembershipInfo {
  tenant_id: number
  tenant_name?: string
  role: string
}

/**
 * Get the current user info
 */
export interface AuthCapabilities {
  can_create_tenant: boolean
  auto_accept_invitation: boolean
}

export async function getCurrentUser(): Promise<{ success: boolean; data?: { user: UserInfo; tenant?: TenantInfo | null; memberships?: MembershipInfo[]; tenant_required?: boolean; capabilities?: AuthCapabilities; preference_defaults?: { browser_search_instructions: string } }; message?: string }> {
  try {
    const response = await get('/api/v1/auth/me')
    return response as unknown as { success: boolean; data?: { user: UserInfo; tenant?: TenantInfo | null; memberships?: MembershipInfo[]; tenant_required?: boolean; capabilities?: AuthCapabilities; preference_defaults?: { browser_search_instructions: string } }; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.getUserFailed')
    }
  }
}

/**
 * Update the current user's preferences (PATCH semantics: send only the fields to change; the backend overwrites only the keys sent
 * and leaves the other keys unchanged). The backend returns the full updated preferences object.
 */
export async function updateMyPreferences(
  patch: Partial<UserPreferences>,
): Promise<{ success: boolean; data?: UserPreferences; message?: string }> {
  try {
    const response = await put('/api/v1/auth/me/preferences', patch)
    return response as unknown as { success: boolean; data?: UserPreferences; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.updatePreferencesFailed'),
    }
  }
}

/**
 * Get the current space info
 */
export async function getCurrentTenant(): Promise<{ success: boolean; data?: TenantInfo; message?: string }> {
  try {
    const response = await get('/api/v1/auth/tenant')
    return response as unknown as { success: boolean; data?: TenantInfo; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.getTenantFailed')
    }
  }
}

/**
 * Refresh the token
 */
export async function refreshToken(refreshToken: string): Promise<{ success: boolean; data?: { token: string; refreshToken: string }; message?: string }> {
  try {
    const response: any = await post('/api/v1/auth/refresh', { refreshToken })
    if (response && response.success) {
      if (response.access_token || response.refresh_token) {
        return {
          success: true,
          data: {
            token: response.access_token,
            refreshToken: response.refresh_token,
          }
        }
      }
    }

    // Otherwise return the original message as is
    return {
      success: false,
      message: response?.message || t('error.auth.refreshTokenFailed')
    }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.refreshTokenFailed')
    }
  }
}

/**
 * User logout
 */
export async function logout(): Promise<{ success: boolean; message?: string }> {
  try {
    await post('/api/v1/auth/logout', {})
    return {
      success: true
    }
  } catch (error: any) {
    return {
      success: false,
      message: error.message || t('error.auth.logoutFailed')
    }
  }
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

/** Map change-password API failures to localized UI strings. */
export function resolveChangePasswordError(error: any): string {
  const details =
    typeof error?.error?.details === 'string'
      ? error.error.details
      : typeof error?.details === 'string'
        ? error.details
        : ''
  switch (details) {
    case 'invalid_old_password':
      return t('userProfile.changePassword.failed')
    case 'password_policy':
      return t('userProfile.changePassword.policyFailed')
    case 'same_password':
      return t('userProfile.changePassword.sameAsCurrent')
    default:
      return error?.message || t('userProfile.changePassword.failed')
  }
}

/**
 * Self-service password rotation. On success the backend revokes every
 * outstanding session for the caller, so the client should clear local
 * auth state and send the user back to /login.
 */
export async function changePassword(
  data: ChangePasswordRequest,
): Promise<{ success: boolean; message?: string }> {
  try {
    const response = await post('/api/v1/auth/change-password', data)
    return response as unknown as { success: boolean; message?: string }
  } catch (error: any) {
    return {
      success: false,
      message: resolveChangePasswordError(error),
    }
  }
}

/**
 * Validate the token
 */
export async function validateToken(): Promise<{ success: boolean; valid?: boolean; message?: string }> {
  try {
    const response = await get('/api/v1/auth/validate')
    return response as unknown as { success: boolean; valid?: boolean; message?: string }
  } catch (error: any) {
    return {
      success: false,
      valid: false,
      message: error.message || t('error.auth.validateTokenFailed')
    }
  }
}





// ---- share-link registration --------------------------------------------

// InviteLookup is the public projection of a share-link row used by
// /register?token=xxx - enough to render the registration page header
// ("X invited you to Y") without leaking sensitive inviter fields.
export interface InviteLookup {
  tenant_id: number
  tenant_name?: string
  role: string
  expires_at: string
}

export interface InviteLookupResponse {
  success: boolean
  data?: InviteLookup
  message?: string
}

export interface RegisterByInviteRequest {
  token: string
  email: string
  username: string
  password: string
}

/**
 * Resolve a share-link token (no auth) into the context the
 * registration page needs (tenant name, role, expiry). Returns 410
 * when the link is invalid / revoked / expired.
 *
 * Uses POST + body (rather than GET + path) so the plaintext token
 * never appears in access logs, browser history, or tracing spans.
 */
export async function getInvitationByToken(token: string): Promise<InviteLookupResponse> {
  try {
    const response = await post(`/api/v1/auth/invitations/lookup`, { token })
    return response as unknown as InviteLookupResponse
  } catch (error: any) {
    return { success: false, message: error.message || '' }
  }
}

/**
 * Complete registration via a share-link token. The invitee supplies
 * their own email - the token is the authorisation, not an identity
 * lock.
 */
export async function registerByInvite(data: RegisterByInviteRequest): Promise<LoginResponse> {
  try {
    const response = await post('/api/v1/auth/register-by-invite', data)
    return response as unknown as LoginResponse
  } catch (error: any) {
    return { success: false, message: error.message || t('error.auth.registerFailed') }
  }
}
