package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/handler/dto"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

var liteSetupToken string

// SetLiteSetupToken is called by the native desktop host before serving requests.
// Web deployments leave it empty and cannot issue anonymous administrator tokens.
func SetLiteSetupToken(token string) { liteSetupToken = token }

const (
	oidcNonceCookieName   = "aikb_oidc_nonce"
	oidcNonceCookieMaxAge = 600
)

// AuthHandler implements HTTP request handlers for user authentication
// Provides functionality for user registration, login, logout, and token management
// through the REST API endpoints
type AuthHandler struct {
	userService      interfaces.UserService
	tenantService    interfaces.TenantService
	configInfo       *config.Config
	systemSettingSvc interfaces.SystemSettingService
	// invitationSvc is required for the share-link registration path
	// (POST /auth/register-by-invite). When nil - e.g. legacy test
	// fixtures - the share-link endpoints respond 503 rather than
	// blocking the rest of the auth surface.
	invitationSvc interfaces.TenantInvitationService
	auditSvc      interfaces.AuditLogService
}

// NewAuthHandler creates a new auth handler instance with the provided services
// Parameters:
//   - userService: An implementation of the UserService interface for business logic
//   - tenantService: An implementation of the TenantService interface for tenant management
//   - systemSettingSvc: 3-tier resolver for runtime-tunable settings such as
//     auth.registration_mode (P3). When DB has a row, it overrides cfg's
//     startup value; otherwise we fall back to cfg.Auth.RegistrationMode
//     (which already accounted for the legacy DISABLE_REGISTRATION env coerce
//     during config load). Mismatch impossible by construction since the
//     handler always passes cfg's value as the def parameter to GetString.
//
// Returns a pointer to the newly created AuthHandler
func NewAuthHandler(configInfo *config.Config,
	userService interfaces.UserService, tenantService interfaces.TenantService,
	systemSettingSvc interfaces.SystemSettingService,
	invitationSvc interfaces.TenantInvitationService,
	auditSvc interfaces.AuditLogService,
) *AuthHandler {
	// Boot-time guard: a nil-or-empty Auth section silently disables the
	// invite_only gate (see Register below). Emit a loud one-shot log
	// pointing at the misconfiguration so operators notice on startup
	// instead of discovering it the day someone hits /auth/register.
	if configInfo == nil || configInfo.Auth == nil {
		logger.Errorf(context.Background(),
			"[auth] AuthHandler constructed with nil/incomplete config (cfg=%v); "+
				"registration_mode enforcement is disabled. This is almost certainly a wiring bug.",
			configInfo)
	}
	return &AuthHandler{
		configInfo:       configInfo,
		userService:      userService,
		tenantService:    tenantService,
		systemSettingSvc: systemSettingSvc,
		invitationSvc:    invitationSvc,
		auditSvc:         auditSvc,
	}
}

// resolveRegistrationMode returns the currently active registration mode.
// Priority: DB system_settings > cfg (which already absorbed the legacy
// DISABLE_REGISTRATION env coerce at startup) > "self_serve" hard default.
//
// Centralised here so /auth/register and /auth/config stay in lock-step -
// otherwise a SystemAdmin's UI edit could affect one path and not the other.
func (h *AuthHandler) resolveRegistrationMode(ctx context.Context) string {
	// cfg-derived default: empty is impossible after applyAuthAndTenantDefaults,
	// but be defensive in case AuthHandler was constructed before that ran
	// (the NewAuthHandler guard already logged in that case).
	def := config.AuthRegistrationModeSelfServe
	if h.configInfo != nil && h.configInfo.Auth != nil {
		if m := strings.TrimSpace(h.configInfo.Auth.RegistrationMode); m != "" {
			def = m
		}
	}
	if h.systemSettingSvc == nil {
		return def
	}
	// envName = "" because DISABLE_REGISTRATION is a boolean and
	// auth.registration_mode is a string - the legacy env was already
	// coerced into `def` above. Mixing the two semantics at the resolver
	// layer would mean a UI delete (DB row absent) silently flipped to
	// the legacy boolean read again, which is surprising.
	return h.systemSettingSvc.GetString(ctx, "auth.registration_mode", "", def)
}

// resolveDefaultTenantMode returns the provisioning policy for a new
// local user account.
// Priority: DB system_settings > cfg.Auth > hard default (create_personal).
// Shared by public registration and the SystemAdmin create-user endpoint.
//
// Invitation registration never uses this value: the invitation itself
// supplies the target tenant.
func resolveDefaultTenantMode(
	ctx context.Context,
	configInfo *config.Config,
	systemSettingSvc interfaces.SystemSettingService,
) types.TenantProvisioningMode {
	def := config.AuthDefaultTenantModeCreatePersonal
	if configInfo != nil && configInfo.Auth != nil {
		if mode := strings.TrimSpace(configInfo.Auth.DefaultTenantMode); mode != "" {
			def = mode
		}
	}
	mode := def
	if systemSettingSvc != nil {
		mode = systemSettingSvc.GetString(
			ctx,
			"auth.default_tenant_mode",
			"AIKB_AUTH_DEFAULT_TENANT_MODE",
			def,
		)
	}
	if mode == config.AuthDefaultTenantModeTenantless {
		return types.TenantProvisioningTenantless
	}
	return types.TenantProvisioningCreatePersonal
}

// resolveDefaultTenantMode returns the provisioning policy for ordinary
// public password registrations.
func (h *AuthHandler) resolveDefaultTenantMode(ctx context.Context) types.TenantProvisioningMode {
	return resolveDefaultTenantMode(ctx, h.configInfo, h.systemSettingSvc)
}

// resolveDefaultTenantMode resolves the same policy for users provisioned
// by a SystemAdmin via POST /api/v1/system/admin/users/create.
func (h *SystemHandler) resolveDefaultTenantMode(ctx context.Context) types.TenantProvisioningMode {
	return resolveDefaultTenantMode(ctx, h.cfg, h.systemSettingSvc)
}

func (h *AuthHandler) complexPasswordEnabled(ctx context.Context) bool {
	return service.ResolveComplexPasswordEnabled(ctx, h.configInfo, h.systemSettingSvc)
}

func (h *SystemHandler) complexPasswordEnabled(ctx context.Context) bool {
	return service.ResolveComplexPasswordEnabled(ctx, h.cfg, h.systemSettingSvc)
}

// Register godoc
// @Summary      Register
// @Description  Registers a new user account
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      types.RegisterRequest  true  "Registration request"
// @Success      201      {object}  types.RegisterResponse
// @Failure      400      {object}  errors.ErrorResponse  "Invalid request parameters"
// @Failure      403      {object}  errors.ErrorResponse  "Registration is disabled"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start user registration")

	// Only self_serve permits registration without an invitation. Resolve
	// each request so system-settings changes take effect immediately.
	if h.resolveRegistrationMode(ctx) != config.AuthRegistrationModeSelfServe {
		logger.Warn(ctx, "Public registration rejected by auth.registration_mode")
		_ = c.Error(errors.NewForbiddenError("Public registration is disabled"))
		return
	}

	var req types.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse registration request parameters", err)
		appErr := errors.NewValidationError("Invalid registration parameters").WithDetails(err.Error())
		c.Error(appErr)
		return
	}
	req.Username = secutils.SanitizeForLog(req.Username)
	req.Email = secutils.SanitizeForLog(req.Email)
	// Password is intentionally NOT sanitized: SanitizeForLog replaces
	// \n, \r, \t and strips other control characters so a string is safe
	// to write into a log line. Applying it to a real password would
	// silently rewrite the credential before hashing, so registration
	// would succeed but login with the original password would fail.
	// Passwords must never be logged, so they don't need that defence.

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		logger.Error(ctx, "Missing required registration fields")
		appErr := errors.NewValidationError("Username, email and password are required")
		c.Error(appErr)
		return
	}

	// Validate password against the runtime policy (DB system_settings
	// first). Register itself does not enforce this because OIDC
	// auto-provision uses an untyped random secret the user never types.
	if err := service.ValidatePasswordPolicy(req.Password, h.complexPasswordEnabled(ctx)); err != nil {
		logger.Error(ctx, "Invalid password policy")
		appErr := errors.NewValidationError(err.Error())
		_ = c.Error(appErr)
		return
	}

	req.Username = secutils.SanitizeForLog(req.Username)
	req.Email = secutils.SanitizeForLog(req.Email)
	req.TenantProvisioning = h.resolveDefaultTenantMode(ctx)
	// Call service to register user
	user, err := h.userService.Register(ctx, &req)
	if err != nil {
		logger.Errorf(ctx, "Failed to register user: %v", err)
		appErr := errors.NewBadRequestError(err.Error())
		c.Error(appErr)
		return
	}

	// Return success response
	response := &types.RegisterResponse{
		Success: true,
		Message: "Registration successful",
		User:    user,
	}

	logger.Infof(ctx, "User registered successfully: %s", secutils.SanitizeForLog(user.Email))
	c.JSON(http.StatusCreated, response)
}

// Login godoc
// @Summary      Log in
// @Description  Logs a user in and returns an access token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      types.LoginRequest  true  "Login request"
// @Success      200      {object}  types.LoginResponse
// @Failure      401      {object}  errors.ErrorResponse  "Authentication failed"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start user login")

	var req types.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse login request parameters", err)
		appErr := errors.NewValidationError("Invalid login parameters").WithDetails(err.Error())
		c.Error(appErr)
		return
	}
	email := secutils.SanitizeForLog(req.Email)

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		logger.Error(ctx, "Missing required login fields")
		appErr := errors.NewValidationError("Email and password are required")
		c.Error(appErr)
		return
	}

	// Call service to authenticate user
	response, err := h.userService.Login(ctx, &req)
	if err != nil {
		logger.Errorf(ctx, "Failed to login user: %v", err)
		h.emitLoginFailed(ctx, c, "password", "service_error")
		_ = c.Error(errors.NewUnauthorizedError("Login failed"))
		return
	}

	// Check if login was successful
	if !response.Success {
		logger.Warnf(ctx, "Login failed: %s", response.Message)
		h.emitLoginFailed(ctx, c, "password", "invalid_credentials")
		_ = c.Error(errors.NewUnauthorizedError(response.Message))
		return
	}
	h.emitLoginSucceeded(ctx, c, "password", response.User, response.ActiveTenant)

	// User is already in the correct format from service

	logger.Infof(ctx, "User logged in successfully, email: %s", email)
	c.JSON(http.StatusOK, dto.NewAuthLoginResponse(response))
}

// GetOIDCAuthorizationURL godoc
// @Summary      Get the OIDC authorization URL
// @Description  Builds the third-party login redirect URL from the backend OIDC config
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        redirect_uri  query     string  true  "OIDC callback URL"
// @Success      200           {object}  types.OIDCAuthURLResponse
// @Failure      400           {object}  errors.ErrorResponse  "Invalid request parameters"
// @Failure      403           {object}  errors.ErrorResponse  "OIDC is not enabled"
// @Router       /auth/oidc/url [get]
func (h *AuthHandler) GetOIDCAuthorizationURL(c *gin.Context) {
	ctx := c.Request.Context()
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	if redirectURI == "" {
		appErr := errors.NewValidationError("redirect_uri is required")
		c.Error(appErr)
		return
	}

	resp, err := h.userService.GetOIDCAuthorizationURL(ctx, redirectURI, "")
	if err != nil {
		logger.Errorf(ctx, "Failed to generate OIDC authorization URL: %v", err)
		appErr := errors.NewForbiddenError("OIDC authorization unavailable")
		_ = c.Error(authoredError(appErr, err, appErr))
		return
	}

	// Bind the state nonce to this browser so an attacker cannot replay
	// their own authorization code into a victim's callback.
	setOIDCNonceCookie(c, resp.Nonce)

	c.JSON(http.StatusOK, resp)
}

// setOIDCNonceCookie binds the OIDC state nonce to this browser so an
// attacker cannot replay their own authorization code into a victim's
// callback. Shared by /auth/oidc/url (JSON) and /auth/oidc/start (302).
func setOIDCNonceCookie(c *gin.Context, nonce string) {
	if nonce == "" {
		return
	}
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(oidcNonceCookieName, nonce, oidcNonceCookieMaxAge, "/", "", secure, true)
}

// oidcCallbackURL derives the absolute /auth/oidc/callback URL from the
// request's own origin (scheme + host), so external platforms can deep-link
// to /auth/oidc/start without supplying a redirect_uri.
func oidcCallbackURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/api/v1/auth/oidc/callback"
}

// OIDCStart godoc
// @Summary      Start OIDC login (direct 302)
// @Description  Unlike /auth/oidc/url, this endpoint redirects (302) straight to the OIDC provider's
// @Description  authorization page, with no frontend JS. An external platform (such as a company portal)
// @Description  can link to it directly to start the OIDC authorization code flow; the IdP's SSO session
// @Description  spares the user another password prompt.
// @Tags         Auth
// @Param        return_to  query  string  false  "Exact URL from OIDC_AUTH_EXTENSION_RETURN_URLS to redirect to"
// @Success      302
// @Failure      400  {object}  errors.ErrorResponse  "return_to is not allow-listed"
// @Router       /auth/oidc/start [get]
func (h *AuthHandler) OIDCStart(c *gin.Context) {
	ctx := c.Request.Context()
	returnTo := strings.TrimSpace(c.Query("return_to"))
	// With OIDC off the service answers 403 below, which names the real cause.
	if returnTo != "" && oidcEnabled(h.configInfo) && !allowedReturnTo(h.configInfo, returnTo) {
		_ = c.Error(errors.NewBadRequestError("return_to is not an allowed extension return URL"))
		return
	}
	resp, err := h.userService.GetOIDCAuthorizationURL(ctx, oidcCallbackURL(c), returnTo)
	if err != nil {
		logger.Errorf(ctx, "Failed to generate OIDC authorization URL: %v", err)
		appErr := errors.NewForbiddenError("OIDC authorization unavailable")
		_ = c.Error(authoredError(appErr, err, appErr))
		return
	}
	setOIDCNonceCookie(c, resp.Nonce)
	c.Redirect(http.StatusFound, resp.AuthorizationURL)
}

// GetOIDCConfig godoc
// @Summary      Get the OIDC login config
// @Description  Returns whether OIDC is enabled and the provider display name, so the frontend can decide whether to
// @Description  show the OIDC login entry
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.OIDCConfigResponse
// @Router       /auth/oidc/config [get]
func (h *AuthHandler) GetOIDCConfig(c *gin.Context) {
	providerDisplayName := ""
	enabled := false

	if h.configInfo != nil && h.configInfo.OIDCAuth != nil {
		enabled = h.configInfo.OIDCAuth.Enable
		providerDisplayName = strings.TrimSpace(h.configInfo.OIDCAuth.ProviderDisplayName)
	}

	c.JSON(http.StatusOK, &types.OIDCConfigResponse{
		Success:             true,
		Enabled:             enabled,
		ProviderDisplayName: providerDisplayName,
	})
}

// OIDCRedirectCallback godoc
// @Summary      OIDC login redirect callback
// @Description  Receives the OIDC provider callback, exchanges the code on the backend, then redirects to the frontend
// @Description  login page
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        code   query string false "OIDC authorization code"
// @Param        state  query string false "OIDC state"
// @Param        error  query string false "OIDC error code"
// @Success      302
// @Router       /auth/oidc/callback [get]
func (h *AuthHandler) OIDCRedirectCallback(c *gin.Context) {
	ctx := c.Request.Context()
	state := strings.TrimSpace(c.Query("state"))
	payload, stateErr := secutils.VerifyOIDCState(state)
	// The extension's auth window only closes on its own return URL, so every
	// outcome, errors included, lands there when the state names one.
	frontendRedirectURI := h.oidcReturnTarget(state)

	if providerError := strings.TrimSpace(c.Query("error")); providerError != "" {
		redirectURL := frontendRedirectURI + "#oidc_error=" + urlQueryEscape(providerError)
		if description := strings.TrimSpace(c.Query("error_description")); description != "" {
			redirectURL += "&oidc_error_description=" + urlQueryEscape(description)
		}
		c.Redirect(http.StatusFound, redirectURL)
		return
	}

	if stateErr == nil {
		stateErr = verifyOIDCNonce(payload, c.Request)
	}
	if stateErr != nil {
		logger.Errorf(ctx, "Failed to decode OIDC state: %v", stateErr)
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("invalid_state"))
		return
	}
	// One-time use: clear the binding cookie as soon as it is checked.
	c.SetCookie(oidcNonceCookieName, "", -1, "/", "", false, true)

	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("missing_code"))
		return
	}

	resp, err := h.userService.LoginWithOIDC(
		ctx, code, strings.TrimSpace(payload.RedirectURI), h.resolveDefaultTenantMode(ctx),
	)
	if err != nil {
		logger.Errorf(ctx, "Failed to complete OIDC login via redirect callback: %v", err)
		h.emitLoginFailed(ctx, c, "oidc", "service_error")
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("login_failed")+"&oidc_error_description="+urlQueryEscape(err.Error()))
		return
	}
	if !resp.Success {
		h.emitLoginFailed(ctx, c, "oidc", "invalid_credentials")
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("login_failed")+"&oidc_error_description="+urlQueryEscape(resp.Message))
		return
	}
	h.emitLoginSucceeded(ctx, c, "oidc", resp.User, resp.Tenant)

	encoded, err := encodeOIDCCallbackPayload(resp)
	if err != nil {
		logger.Errorf(ctx, "Failed to encode OIDC callback payload: %v", err)
		c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_error="+urlQueryEscape("payload_encode_failed"))
		return
	}

	c.Redirect(http.StatusFound, frontendRedirectURI+"#oidc_result="+urlQueryEscape(encoded))
}

func oidcEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.OIDCAuth != nil && cfg.OIDCAuth.Enable
}

// allowedReturnTo accepts only an exact entry of the configured list; an empty
// list rejects every value.
func allowedReturnTo(cfg *config.Config, value string) bool {
	return cfg != nil && cfg.OIDCAuth != nil && slices.Contains(cfg.OIDCAuth.ExtensionReturnURLs, value)
}

// oidcReturnTarget is "/" unless the state names a return URL that is still
// allow-listed. The signature is not consulted: the allow-list is what makes the
// redirect safe, and a stranded auth window would never report an expired login.
func (h *AuthHandler) oidcReturnTarget(state string) string {
	payload, err := secutils.DecodeOIDCStateUnverified(state)
	if err != nil || !allowedReturnTo(h.configInfo, payload.ReturnTo) {
		return "/"
	}
	return payload.ReturnTo
}

func encodeOIDCCallbackPayload(resp *types.OIDCCallbackResponse) (string, error) {
	payload, err := json.Marshal(dto.NewAuthOIDCCallbackResponse(resp))
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// verifyOIDCNonce binds a verified state to the browser that started the login.
func verifyOIDCNonce(payload *secutils.OIDCStatePayload, req *http.Request) error {
	cookieNonce, err := req.Cookie(oidcNonceCookieName)
	if err != nil || cookieNonce == nil || strings.TrimSpace(cookieNonce.Value) == "" {
		return errors.NewValidationError("oidc nonce cookie missing")
	}
	if cookieNonce.Value != payload.Nonce {
		return errors.NewValidationError("oidc nonce mismatch")
	}
	return nil
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		" ", "%20",
		"#", "%23",
		"&", "%26",
		"+", "%2B",
		"=", "%3D",
		"?", "%3F",
	)
	return replacer.Replace(value)
}

// Logout godoc
// @Summary      Log out
// @Description  Revokes the current access token and logs out
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Logged out"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Router       /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start user logout")

	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		logger.Error(ctx, "Missing Authorization header")
		appErr := errors.NewValidationError("Authorization header is required")
		c.Error(appErr)
		return
	}

	// Parse Bearer token
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		logger.Error(ctx, "Invalid Authorization header format")
		appErr := errors.NewValidationError("Invalid Authorization header format")
		c.Error(appErr)
		return
	}

	token := tokenParts[1]

	// Revoke every outstanding session for this user so refresh tokens
	// cannot keep working after logout.
	err := h.userService.Logout(ctx, token)
	if err != nil {
		logger.Errorf(ctx, "Failed to revoke token: %v", err)
		appErr := errors.NewInternalServerError("Logout failed").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	logger.Info(ctx, "User logged out successfully")
	actorID, _ := types.UserIDFromContext(ctx)
	emitAudit(ctx, h.auditSvc, &types.AuditLog{
		Action: types.AuditActionAuthLogout, TargetType: "user", TargetUserID: actorID,
		RequestPath: c.FullPath(), RequestMethod: c.Request.Method,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logout successful",
	})
}

// emitLoginSucceeded records a completed login as the user it created a
// session for; the tenant is the workspace the token is scoped to.
func (h *AuthHandler) emitLoginSucceeded(
	ctx context.Context, c *gin.Context, method string, user *types.User, tenant *types.Tenant,
) {
	if user == nil {
		return
	}
	var tenantID uint64
	if tenant != nil {
		tenantID = tenant.ID
	}
	emitAudit(ctx, h.auditSvc, &types.AuditLog{
		TenantID: tenantID, ActorUserID: user.ID, ActorRole: "user",
		Action: types.AuditActionAuthLoginSucceeded, TargetType: "user", TargetUserID: user.ID,
		RequestPath: c.FullPath(), RequestMethod: c.Request.Method,
		Details: changeDetails("login", method),
	})
}

// emitLoginFailed records a rejected login without any identity: the row says
// that a login failed at this path and why (bad credentials or a service
// error), not whose.
func (h *AuthHandler) emitLoginFailed(ctx context.Context, c *gin.Context, method, reason string) {
	emitAudit(ctx, h.auditSvc, &types.AuditLog{
		ActorRole: "anonymous", Action: types.AuditActionAuthLoginFailed, TargetType: "user",
		Outcome: types.AuditOutcomeDenied, RequestPath: c.FullPath(), RequestMethod: c.Request.Method,
		Details: changeDetails("login", method, reason),
	})
}

// RefreshToken godoc
// @Summary      Refresh token
// @Description  Exchanges a refresh token for a new access token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      object{refreshToken=string}  true  "Refresh token"
// @Success      200      {object}  map[string]interface{}       "New tokens"
// @Failure      401      {object}  errors.ErrorResponse              "Invalid token"
// @Router       /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start token refresh")

	var req struct {
		RefreshToken string `json:"refreshToken" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse refresh token request", err)
		appErr := errors.NewValidationError("Invalid refresh token request").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	// Call service to refresh token
	accessToken, newRefreshToken, err := h.userService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		logger.Errorf(ctx, "Failed to refresh token: %v", err)
		_ = c.Error(authoredError(errors.NewUnauthorizedError("Token refresh failed"), err, err))
		return
	}

	logger.Info(ctx, "Token refreshed successfully")
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "Token refreshed successfully",
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
	})
}

// GetCurrentUser godoc
// @Summary      Get the current user
// @Description  Returns the details of the logged-in user
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "User info"
// @Failure      401  {object}  errors.ErrorResponse         "Unauthorized"
// @Security     Bearer
// @Router       /auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get current user from service (which extracts from context)
	user, err := h.userService.GetCurrentUser(ctx)
	if err != nil {
		logger.Errorf(ctx, "Failed to get current user: %v", err)
		appErr := errors.NewUnauthorizedError("Failed to get user information").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	// Get tenant information for the *active* tenant (the one the
	// auth middleware resolved against the X-Tenant-ID header), not
	// the user's home tenant. user.TenantID is the row stored on the
	// users table at signup time and never changes; reading it here
	// would make /auth/me always return the home tenant even after
	// the user switched into a peer tenant. The frontend then re-keys
	// `authStore.tenant.id` to the home tenant, and every UI gate
	// computed against it (currentTenantRole, isOwner, ...) leaks
	// the wrong role. Pull the active tenant id from context instead.
	var tenant *types.Tenant
	activeTenantID, _ := types.TenantIDFromContext(ctx)
	if activeTenantID == 0 {
		activeTenantID = user.TenantID
	}
	if activeTenantID > 0 {
		tenant, err = h.tenantService.GetTenantByID(ctx, activeTenantID)
		if err != nil {
			logger.Warnf(ctx, "Failed to get tenant info for user %s, tenant ID %d: %v", user.Email, activeTenantID, err)
			// Don't fail the request if tenant info is not available
		}
	}
	userInfo := user.ToUserInfo()
	userInfo.CanAccessAllTenants = user.CanAccessAllTenants && h.configInfo.Tenant.EnableCrossTenantAccess
	// Return the memberships too, so a page reload (which only hits /auth/me) can restore
	// currentTenantRole; otherwise the role is only known right after login.
	memberships := h.userService.BuildLoginMemberships(ctx, user, tenant)
	canCreateTenant := user.CanAccessAllTenants ||
		resolveTenantSelfServiceCreationEnabled(ctx, h.configInfo, h.systemSettingSvc)
	autoAcceptInvitation := h.systemSettingSvc != nil &&
		h.systemSettingSvc.GetBool(ctx, "tenant.auto_accept_invitation", "AIKB_TENANT_AUTO_ACCEPT_INVITATION", false)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user":                userInfo,
			"preference_defaults": gin.H{"browser_search_instructions": types.DefaultBrowserSearchInstructions},
			"tenant":              dto.NewTenantResponse(ctx, tenant),
			"memberships":         memberships,
			"tenant_required":     tenant == nil,
			"capabilities": gin.H{
				"can_create_tenant":      canCreateTenant,
				"auto_accept_invitation": autoAcceptInvitation,
			},
		},
	})
}

// updateMyPreferencesRequest is the body for PUT /auth/me/preferences.
// Fields are pointers so the handler can distinguish "key not present"
// (preserve existing value) from "explicit false". See
// types.UserPreferences for the persistence-layer counterpart.
type updateMyPreferencesRequest struct {
	BrowserSearchInstructions *string `json:"browser_search_instructions" binding:"omitempty,max=4000"`
	// LastActiveTenantID lets clients persist "after a fresh login,
	// drop me back into this workspace" across devices. The SPA sends
	// this after every tenant switch; POST /auth/switch-tenant records
	// the same preference server-side. Send a positive workspace id to
	// set / replace, or 0 to clear. Membership is validated at next
	// login, not here. Nil = field omitted from the PATCH and stays
	// untouched.
	LastActiveTenantID *uint64 `json:"last_active_tenant_id"`
}

// UpdateMyPreferences godoc
// @Summary      Update the current user's preferences
// @Description  Merges user preferences with PATCH semantics (only fields present in the body are overwritten);
// @Description  stored in users.preferences (JSON), so they sync across devices and browsers.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      updateMyPreferencesRequest  true  "Preferences patch"
// @Success      200      {object}  map[string]interface{}      "Updated preferences"
// @Failure      400      {object}  errors.ErrorResponse             "Invalid request parameters"
// @Failure      401      {object}  errors.ErrorResponse             "Unauthorized"
// @Security     Bearer
// @Router       /auth/me/preferences [put]
func (h *AuthHandler) UpdateMyPreferences(c *gin.Context) {
	ctx := c.Request.Context()

	user, err := h.userService.GetCurrentUser(ctx)
	if err != nil {
		appErr := errors.NewUnauthorizedError("Failed to get user information").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	var req updateMyPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid preferences request").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	patch := types.UserPreferences{
		LastActiveTenantID:        req.LastActiveTenantID,
		BrowserSearchInstructions: req.BrowserSearchInstructions,
	}
	prefs, err := h.userService.UpdateUserPreferences(ctx, user.ID, patch)
	if err != nil {
		logger.Errorf(ctx, "Failed to update preferences for user %s: %v", user.Email, err)
		_ = c.Error(authoredError(errors.NewBadRequestError("Failed to update preferences"), err, err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    prefs,
	})
}

// ChangePassword godoc
// @Summary      Change password
// @Description  Changes the current user's password. The new password must be 8-32 characters with both letters and
// @Description  digits; with complex passwords enabled it also needs upper and lower case and a special character. On
// @Description  success every session is revoked and the user must log in again.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      object{old_password=string,new_password=string}  true  "Password change request"
// @Success      200      {object}  map[string]interface{}                           "Changed"
// @Failure      400      {object}  errors.ErrorResponse                                  "Invalid request parameters"
// @Failure      500      {object}  errors.ErrorResponse                                  "Internal server error"
// @Security     Bearer
// @Router       /auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start password change")

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse password change request", err)
		appErr := errors.NewValidationError("Invalid password change request").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	// Get current user
	user, err := h.userService.GetCurrentUser(ctx)
	if err != nil {
		logger.Errorf(ctx, "Failed to get current user: %v", err)
		appErr := errors.NewUnauthorizedError("Failed to get user information").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	// Change password. Policy is enforced in the service after the old
	// password is verified so a wrong current credential is not masked
	// by a complexity error.
	err = h.userService.ChangePassword(ctx, user.ID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case stderrors.Is(err, service.ErrPasswordTooLong):
			_ = c.Error(errors.NewValidationError("Password must be at most 72 bytes").
				WithDetails(service.DetailPasswordPolicy))
			return
		case service.IsPasswordPolicyError(err):
			appErr := errors.NewValidationError("Password policy violation").
				WithDetails(service.DetailPasswordPolicy)
			_ = c.Error(appErr)
			return
		case stderrors.Is(err, service.ErrInvalidOldPassword):
			appErr := errors.NewBadRequestError("Current password is incorrect").
				WithDetails(service.DetailInvalidOldPassword)
			c.Error(appErr)
			return
		case stderrors.Is(err, service.ErrSamePassword):
			appErr := errors.NewValidationError("New password must differ from current password").
				WithDetails(service.DetailSamePassword)
			c.Error(appErr)
			return
		default:
			logger.Errorf(ctx, "Failed to change password: %v", err)
			_ = c.Error(err)
			return
		}
	}

	logger.Infof(ctx, "Password changed successfully for user: %s", user.Email)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Password changed successfully",
	})
}

// GetAuthConfig godoc
// @Summary      Get the auth config
// @Description  Returns this deployment's registration mode and password complexity switch, so the frontend can decide
// @Description  whether to show registration and which password rules to check
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Auth config"
// @Router       /auth/config [get]
//
// GetAuthConfig is intentionally a no-auth endpoint: the frontend reads
// it on app load to decide whether to show the Register tab and which
// password complexity rules to apply. We expose only what the UI
// strictly needs; other config stays internal.
func (h *AuthHandler) GetAuthConfig(c *gin.Context) {
	// Same source-of-truth as Register's gate, so the UI hide-the-button
	// signal can never disagree with the API enforcement signal.
	mode := h.resolveRegistrationMode(c.Request.Context())

	complexPasswordEnabled := service.ResolveComplexPasswordEnabled(
		c.Request.Context(),
		h.configInfo,
		h.systemSettingSvc,
	)
	c.JSON(http.StatusOK, gin.H{
		"success":                  true,
		"registration_mode":        mode,
		"complex_password_enabled": complexPasswordEnabled,
	})
}

// SwitchTenant godoc
// @Summary      Switch the active workspace
// @Description  Reissues the access token for the current user in the target workspace; the user needs an active
// @Description  membership there (cross-tenant superusers excepted).
// @Description  A successful switch writes the target workspace to the last-active-tenant preference, so the next login
// @Description  and refresh land there (the refresh JWT carries no tenant_id).
// @Description  The preference is per account: one switch changes where the next login or refresh lands on every
// @Description  device. If writing the preference fails, the switch fails and no new token is issued.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request  body      object{tenant_id=integer,refresh_token=string}  true  "Switch request"
// @Success      200      {object}  types.LoginResponse
// @Failure      400      {object}  errors.ErrorResponse  "Invalid parameters"
// @Failure      403      {object}  errors.ErrorResponse  "No membership in the workspace"
// @Failure      500      {object}  errors.ErrorResponse  "Internal server error, including a failed preference write"
// @Security     Bearer
// @Router       /auth/switch-tenant [post]
//
// SwitchTenant is the v1 backend hook for the tenant-switcher UI added
// in PR 3. The current PR ships the endpoint so multi-tenant tests can
// exercise the membership flow end-to-end before the frontend lands.
func (h *AuthHandler) SwitchTenant(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		TenantID     uint64 `json:"tenant_id"     binding:"required"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		appErr := errors.NewValidationError("Invalid workspace switch request").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	user, err := h.userService.GetCurrentUser(ctx)
	if err != nil || user == nil {
		appErr := errors.NewUnauthorizedError("not authenticated")
		c.Error(appErr)
		return
	}

	resp, err := h.userService.SwitchTenant(ctx, user, req.TenantID, req.RefreshToken)
	if err != nil {
		logger.Errorf(ctx, "SwitchTenant failed user=%s target=%d: %v", user.ID, req.TenantID, err)
		appErr := errors.NewForbiddenError("workspace switch failed")
		if stderrors.Is(err, service.ErrMembershipNotFound) {
			_ = c.Error(appErr.WithDetails(service.ErrMembershipNotFound.Error()))
			return
		}
		_ = c.Error(authoredError(appErr, err, err))
		return
	}

	c.JSON(http.StatusOK, dto.NewAuthLoginResponse(resp))
}

// AutoSetup godoc
// @Summary      Auto setup (Lite desktop edition)
// @Description  Lite only: on first launch, creates the default user and workspace and returns tokens. Every launch
// @Description  authenticates with the desktop's native credential, replacing manual registration and login
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  types.LoginResponse
// @Failure      403  {object}  errors.ErrorResponse  "Not the Lite edition"
// @Router       /auth/auto-setup [post]
func (h *AuthHandler) AutoSetup(c *gin.Context) {
	ctx := c.Request.Context()

	if Edition != "lite" {
		appErr := errors.NewForbiddenError("auto-setup is only available in lite edition")
		c.Error(appErr)
		return
	}

	if liteSetupToken == "" ||
		subtle.ConstantTimeCompare([]byte(c.GetHeader("X-AIKB-Desktop-Token")), []byte(liteSetupToken)) != 1 {
		_ = c.Error(errors.NewUnauthorizedError("desktop authentication is required"))
		return
	}

	const defaultEmail = "admin@ai-kb.local"

	user, _ := h.userService.GetUserByEmail(ctx, defaultEmail)
	if user == nil {
		logger.Info(ctx, "Auto-setup: creating default user and tenant for lite edition")

		randomBytes := make([]byte, 24)
		if _, err := rand.Read(randomBytes); err != nil {
			appErr := errors.NewInternalServerError("auto-setup failed: unable to generate credentials")
			c.Error(appErr)
			return
		}
		randomPassword := base64.RawURLEncoding.EncodeToString(randomBytes)
		randomUsername := fmt.Sprintf("user_%s", base64.RawURLEncoding.EncodeToString(randomBytes[:6]))

		_, err := h.userService.Register(ctx, &types.RegisterRequest{
			Username: randomUsername,
			Email:    defaultEmail,
			Password: randomPassword,
		})
		if err != nil {
			logger.Errorf(ctx, "Auto-setup: failed to register default user: %v", err)
			appErr := errors.NewInternalServerError("auto-setup failed").WithDetails(err.Error())
			c.Error(appErr)
			return
		}
		user, _ = h.userService.GetUserByEmail(ctx, defaultEmail)
		if user == nil {
			appErr := errors.NewInternalServerError("auto-setup failed: user not found after registration")
			c.Error(appErr)
			return
		}
	}

	accessToken, refreshToken, err := h.userService.GenerateTokens(ctx, user)
	if err != nil {
		logger.Errorf(ctx, "Auto-setup: failed to generate tokens: %v", err)
		appErr := errors.NewInternalServerError("auto-setup failed").WithDetails(err.Error())
		c.Error(appErr)
		return
	}

	tenant, _ := h.tenantService.GetTenantByID(ctx, user.TenantID)

	logger.Info(ctx, "Auto-setup: completed successfully")
	c.JSON(http.StatusOK, dto.NewAuthLoginResponse(&types.LoginResponse{
		Success:      true,
		Message:      "Auto-setup successful",
		User:         user,
		ActiveTenant: tenant,
		Memberships: []types.Membership{{
			TenantID:   user.TenantID,
			TenantName: tenantNameOrEmpty(tenant),
			Role:       types.TenantRoleOwner,
		}},
		Token:        accessToken,
		RefreshToken: refreshToken,
	}))
}

// tenantNameOrEmpty returns t.Name when t is non-nil, "" otherwise.
// Used by AutoSetup to populate Membership.TenantName without crashing
// if the tenant lookup failed.
func tenantNameOrEmpty(t *types.Tenant) string {
	if t == nil {
		return ""
	}
	return t.Name
}

// ValidateToken godoc
// @Summary      Validate token
// @Description  Checks whether the access token is valid
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Token is valid"
// @Failure      401  {object}  errors.ErrorResponse         "Invalid token"
// @Security     Bearer
// @Router       /auth/validate [get]
func (h *AuthHandler) ValidateToken(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start token validation")

	// Extract token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		logger.Error(ctx, "Missing Authorization header")
		appErr := errors.NewValidationError("Authorization header is required")
		c.Error(appErr)
		return
	}

	// Parse Bearer token
	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		logger.Error(ctx, "Invalid Authorization header format")
		appErr := errors.NewValidationError("Invalid Authorization header format")
		c.Error(appErr)
		return
	}

	token := tokenParts[1]

	// Validate token
	user, _, err := h.userService.ValidateToken(ctx, token)
	if err != nil {
		logger.Errorf(ctx, "Failed to validate token: %v", err)
		_ = c.Error(authoredError(errors.NewUnauthorizedError("Token validation failed"), err, err))
		return
	}

	logger.Infof(ctx, "Token validated successfully for user: %s", user.Email)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Token is valid",
		"user":    user.ToUserInfo(),
	})
}

// authoredError returns appErr carrying the service's message as details when
// the service authored err as an AppError, and fallback for any other cause.
// A fallback of err sends an internal failure (repository, token signing) out
// as the scrubbed 500; OIDC setup falls back to its bare 403 instead, since
// discovery errors carry the provider URL and body.
func authoredError(appErr *errors.AppError, err, fallback error) error {
	if cause, ok := errors.IsAppError(err); ok {
		return appErr.WithDetails(cause.Message)
	}
	return fallback
}
