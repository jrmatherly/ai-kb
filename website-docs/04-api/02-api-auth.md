# API reference: authentication and users

Registration, login, token refresh, profile and invitation handling. The authentication requirement varies per route; public routes are marked in their entries.

Unless an entry says otherwise, routes in this group sit behind the authentication middleware and require only a logged-in user (no role floor). Unauthenticated routes are marked in their entries.

## Authentication (/api/v1/auth)

### POST /api/v1/auth/register

Purpose: register a new user (self-serve mode). No authentication. Handler: `internal/handler/auth.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `username` | string | yes (`binding:"required,min=2,max=50"`) | Username, 2 to 50 characters |
| `email` | string | yes (`binding:"required,email"`) | Email address |
| `password` | string | yes (`binding:"required,min=6"`) | Password (8 to 32 characters with at least one letter and one digit; complex mode also requires upper and lower case and a special character) |

Whether a personal workspace is created automatically is decided by the server setting `auth.default_tenant_mode`; the request body cannot choose it. In `invite_register` (registration by invitation only) and `invite_only` (registration disabled) modes the route returns 403. A password that fails the policy, or a duplicate email, returns 400.

Response: 201 `{"success":true,"message":"Registration successful","user":{User}}`

```bash
curl -X POST $BASE/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"a@ex.com","password":"secret123"}'
```

### POST /api/v1/auth/register-by-invite

Only the `self_serve` and `invite_register` modes allow registration through a valid invitation link; `invite_only` returns 403.

Purpose: register through an invitation or share-link token and join the workspace. No authentication, rate limited per IP to 30 requests per minute (the budget is shared with `invitations/lookup`). Handler: `internal/handler/auth_register_by_invite.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `token` | string | yes (`binding:"required"`) | Invitation token |
| `email` | string | yes (`binding:"required,email"`) | Email address |
| `username` | string | yes (`binding:"required"`) | Username |
| `password` | string | yes (`binding:"required,min=6"`) | Password (8 to 32 characters with at least one letter and one digit; complex mode also requires upper and lower case and a special character) |

The binding tag only checks length; the full password policy is applied after the token is resolved. An unknown, expired or revoked token returns 410; an email that already has an account returns 409 (log in and use `POST /api/v1/me/invitations/accept-by-token` instead).

Response: 201, same shape as Login (`user/active_tenant/memberships/token/refresh_token`).

```bash
curl -X POST $BASE/api/v1/auth/register-by-invite -H 'Content-Type: application/json' \
  -d '{"token":"<invite_token>","email":"a@ex.com","username":"alice","password":"secret123"}'
```

### POST /api/v1/auth/invitations/lookup

Purpose: anonymously resolve an invitation token to its workspace (preview before registering). No authentication, rate limited per IP. The token travels in the body rather than the URL so it stays out of access logs and browser history. Handler: `internal/handler/auth_register_by_invite.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `token` | string | yes (`binding:"required"`) | Invitation token |

Response: 200 `{"success":true,"data":{"tenant_id","tenant_name","role","expires_at"}}`. An unknown, expired or revoked token returns 410.

```bash
curl -X POST $BASE/api/v1/auth/invitations/lookup -H 'Content-Type: application/json' -d '{"token":"<invite_token>"}'
```

### POST /api/v1/auth/login

Purpose: log in with email and password. No authentication. Handler: `internal/handler/auth.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `email` | string | yes (`binding:"required,email"`) | Email address |
| `password` | string | yes (`binding:"required,min=6"`) | Password |

Response: 200 `{"success":true,"user":{...},"active_tenant":{...},"memberships":[...],"token":"...","refresh_token":"..."}`. A wrong email or password returns 401.

```bash
curl -X POST $BASE/api/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"a@ex.com","password":"secret123"}'
```

### POST /api/v1/auth/auto-setup

Purpose: native desktop Lite auto-creates the account and workspace and logs in. The request must carry the per-process random `X-AIKB-Desktop-Token` supplied by the desktop native bridge; anonymous HTTP requests are rejected. Regular browsers use the register and login routes. Handler: `internal/handler/auth.go`

Response: 200, same shape as Login. A non-Lite edition returns 403; a missing or wrong desktop token returns 401.

### GET /api/v1/auth/config

Purpose: read the authentication configuration, such as the registration mode. No authentication. Handler: `internal/handler/auth.go`

Response: 200 `{"success":true,"registration_mode":"self_serve|invite_register|invite_only","complex_password_enabled":false}`

```bash
curl $BASE/api/v1/auth/config
```

### POST /api/v1/auth/switch-tenant

Purpose: switch the active workspace and reissue tokens. Requires a logged-in user (callable without a workspace). Handler: `internal/handler/auth.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `tenant_id` | uint64 | yes (`binding:"required"`) | Target workspace ID |
| `refresh_token` | string | no | Used to issue the new token pair |

Response: 200, same shape as Login. No active membership in the target workspace (cross-workspace superusers excepted) returns 403.

A successful switch writes the target workspace into the account-level "last active workspace" preference, so the next login (password, OIDC or another device) and every refresh land there. The refresh JWT carries no `tenant_id`, so if the preference write fails the whole switch fails and no new token is issued. API clients do not need a follow-up `PUT /auth/me/preferences`. The web UI does not switch workspaces through this route. One switch changes where that user lands next on every device.

```bash
curl -X POST $BASE/api/v1/auth/switch-tenant -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"tenant_id":2}'
```

### GET /api/v1/auth/oidc/config

Purpose: read whether OIDC is enabled and the provider display name. No authentication. Handler: `internal/handler/auth.go`

Response: 200 `{"success":true,"enabled":bool,"provider_display_name":"..."}`

```bash
curl $BASE/api/v1/auth/oidc/config
```

### GET /api/v1/auth/oidc/start

No login required. Returns 302 with a `Location` pointing at the identity provider, so an enterprise portal can link to it directly. No prior request for the JSON authorization URL is needed; the callback is built from the request origin as `/api/v1/auth/oidc/callback`. The login result after the callback is the same as the regular OIDC flow. OIDC disabled returns 403.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `return_to` | string | no | Exact URL from `OIDC_AUTH_EXTENSION_RETURN_URLS`; the callback redirects here instead of `/` |

With OIDC enabled, a `return_to` that is not allow-listed, or any value while the list is empty, returns 400.

```bash
curl -i "$BASE/api/v1/auth/oidc/start"
curl -i "$BASE/api/v1/auth/oidc/start?return_to=https://<extension-id>.chromiumapp.org/oidc"
```

### GET /api/v1/auth/oidc/url

Purpose: get the OIDC authorization redirect URL. No authentication. Handler: `internal/handler/auth.go`

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `redirect_uri` | string | yes | Callback address |

Response: 200 `{"success":true,"authorization_url":"...","state":"...","provider_display_name":"..."}`. OIDC disabled returns 403.

The signed `state` carries the nonce, the `redirect_uri` and an issue time. The nonce is also bound to an HttpOnly cookie set by this response (and by `oidc/start`), so the callback must come from the same browser.

```bash
curl "$BASE/api/v1/auth/oidc/url?redirect_uri=https://app.example.com/callback"
```

### GET /api/v1/auth/oidc/callback

Purpose: OIDC authorization callback (entered through a browser redirect). No authentication. Handler: `internal/handler/auth.go`

Query parameters: `code`, `state`, `error`, `error_description` (all returned by the OIDC provider).

Response: 302 redirect to the frontend (`/`), or to the allow-listed `return_to` carried in the signed state, with the same fragment. On success the fragment is `#oidc_result=<base64url>` (a JSON object with `user`, `tenant`, `memberships`, `token`, `refresh_token` and `is_new_user`); on failure it is `#oidc_error=...`, with `&oidc_error_description=...` when the provider or the login step supplied one. Error values: the provider's own error code, `invalid_state`, `missing_code`, `login_failed`, `payload_encode_failed`.

An account auto-provisioned by OIDC gets a 32-byte random password it never receives and the `oidc_only_login` preference, which the profile UI uses to hide self-service password rotation until `change-password` sets a known password.

```bash
curl -i "$BASE/api/v1/auth/oidc/callback?code=xxx&state=yyy"
```

### POST /api/v1/auth/refresh

Purpose: exchange a refresh token for a new token pair. No authentication. Handler: `internal/handler/auth.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `refreshToken` | string | yes (`binding:"required"`) | Refresh token |

Response: 200 `{"success":true,"message":"Token refreshed successfully","access_token":"...","refresh_token":"..."}`. An invalid refresh token returns 401.

```bash
curl -X POST $BASE/api/v1/auth/refresh -H 'Content-Type: application/json' -d '{"refreshToken":"<rt>"}'
```

### GET /api/v1/auth/validate

Purpose: check whether the current token is valid. Requires a logged-in user (callable without a workspace). Handler: `internal/handler/auth.go`

The check rejects refresh tokens and sandbox terminal tickets presented as access tokens, revoked tokens and disabled accounts; a failed token lookup is reported as an error rather than treated as valid.

Response: 200 `{"success":true,"message":"Token is valid","user":{UserInfo}}`. A rejected token returns 401.

```bash
curl $BASE/api/v1/auth/validate -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/auth/logout

Purpose: log out. Revokes every outstanding session of the user, refresh tokens included, not only the presented token. Requires a logged-in user. No request body. Handler: `internal/handler/auth.go`

Response: 200 `{"success":true,"message":"Logout successful"}`

```bash
curl -X POST $BASE/api/v1/auth/logout -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/auth/me

Purpose: read the caller's identity (user, workspace, memberships, capabilities). Requires a logged-in user; an API key also works (policy `apiKeyAny()`, any valid key). Handler: `internal/handler/auth.go`

Response: 200 `{"success":true,"data":{"user":{UserInfo},"preference_defaults":{"browser_search_instructions":"..."},"tenant":{TenantResponse},"memberships":[...],"tenant_required":bool,"capabilities":{"can_create_tenant":bool,"auto_accept_invitation":bool}}}`

```bash
curl $BASE/api/v1/auth/me -H "X-API-Key: $API_KEY"
```

### PUT /api/v1/auth/me/preferences

Purpose: update personal preferences (PATCH semantics: only fields present in the body change). Requires a logged-in user. Handler: `internal/handler/auth.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `last_active_tenant_id` | *uint64 | no | A positive value sets or replaces; `0` clears (the next login lands in the home workspace); omitted leaves it unchanged. `POST /auth/switch-tenant` writes the same field on success. |
| `browser_search_instructions` | *string | no | Local Browser search instructions, at most 4000 characters (`binding:"omitempty,max=4000"`) |

Response: 200 `{"success":true,"data":{UserPreferences}}`

```bash
curl -X PUT $BASE/api/v1/auth/me/preferences -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"last_active_tenant_id":2}'
```

The password policy is published by `GET /auth/config`. The special characters accepted in complex mode are `!@#$%^&*()_+-=[]{}|;:,.<>?`; the full validation rule cannot be derived from the struct binding length tags alone.

### POST /api/v1/auth/change-password

Purpose: change the password. Requires a logged-in user. Handler: `internal/handler/auth.go`

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `old_password` | string | yes (`binding:"required"`) | Current password |
| `new_password` | string | yes (`binding:"required"`) | New password (8 to 32 characters with at least one letter and one digit; complex mode also requires upper and lower case and a special character) |

Response: 200 `{"success":true,"message":"Password changed successfully"}`

The current password must be correct and the new password must differ from it; these two checks run before the policy check, so a wrong current password is never masked by a policy error. On success every session is revoked and the user must log in again, and an `oidc_only_login` flag is cleared. The default policy is 8 to 32 characters with letters and digits; complex mode requires upper and lower case letters, digits and a special character. A new password longer than 72 bytes in UTF-8 (bcrypt's input limit, reachable with multi-byte characters inside 32 characters) fails the policy with the message `Password must be at most 72 bytes`. The error detail is `invalid_old_password`, `password_policy` or `same_password`, all with status 400.

```bash
curl -X POST $BASE/api/v1/auth/change-password -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"old_password":"old","new_password":"NewPass123!"}'
```

### POST /api/v1/me/invitations/accept-by-token

Requires a logged-in user, acts only on the current user, and a new user without a workspace may call it. Request `{"token":"<invite-token>"}`; a valid share-link invitation adds the current user to its workspace, and a user who is already a member gets the same result again. An invalid, expired or revoked link returns 410; an empty token returns 400. The success response is `{success:true,data:{membership:{tenant_id,role,status,joined_at},tenant_name}}`, which the frontend uses to switch workspaces.

```bash
curl -X POST "$BASE/api/v1/me/invitations/accept-by-token" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"token":"<invite-token>"}'
```

## My invitations (/api/v1/me/invitations)

The service layer guarantees that only the invitee can accept or decline; there is no role floor (a new user without a workspace can use these). Handler: `internal/handler/tenant_invitation.go`

### GET /api/v1/me/invitations

Purpose: list the invitations addressed to me.

| Query parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `include_terminal` | bool | no | `true` also returns finished invitations (accepted, declined, revoked, expired) |

Response: 200 `{"success":true,"data":{"invitations":[TenantInvitationResponse],"total":N}}`

```bash
curl $BASE/api/v1/me/invitations -H "Authorization: Bearer $TOKEN"
```

### GET /api/v1/me/invitations/pending-count

Purpose: count of pending invitations (lightweight polling).

Response: 200 `{"success":true,"data":{"pending_count":N}}`

```bash
curl $BASE/api/v1/me/invitations/pending-count -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/me/invitations/:inv_id/accept

Purpose: accept an invitation and write the membership. Path parameter: `inv_id`, the invitation ID. No request body. A user without a workspace adopts the first accepted workspace as the default.

Response: 200 `{"success":true,"data":{"membership":{"tenant_id","role","status","joined_at"}}}`. Unknown invitation 404; not the invitee 403; not pending or expired 409.

```bash
curl -X POST $BASE/api/v1/me/invitations/12/accept -H "Authorization: Bearer $TOKEN"
```

### POST /api/v1/me/invitations/:inv_id/decline

Purpose: decline an invitation. Path parameter: `inv_id`. No request body.

Response: 200 `{"success":true}`. Same error codes as accept.

```bash
curl -X POST $BASE/api/v1/me/invitations/12/decline -H "Authorization: Bearer $TOKEN"
```

## Implementation reference

Route registration: `RegisterAuthRoutes` and `RegisterMyInvitationRoutes` in `internal/router/routes_auth_tenant.go`. Handlers: `internal/handler/auth.go`, `internal/handler/auth_register_by_invite.go`, `internal/handler/tenant_invitation.go`. Registration mode and the complex-password switch: `internal/config/config.go` (`AuthConfig`). Password policy: `internal/application/service/password_policy.go`. OIDC state signing: `internal/utils/oidc_state.go`.
