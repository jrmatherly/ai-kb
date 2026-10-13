package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/handler/dto"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

// TenantHandler implements HTTP request handlers for tenant management
// Provides functionality for creating, retrieving, updating, and deleting tenants
// through the REST API endpoints
type TenantHandler struct {
	service       interfaces.TenantService
	apiKeyService interfaces.TenantAPIKeyService
	userService   interfaces.UserService
	memberService interfaces.TenantMemberService
	kbService     interfaces.KnowledgeBaseService
	config        *config.Config
	// systemSettingSvc resolves runtime tenant policies and limits.
	// Reading goes DB > ENV >
	// in-code default, so a SystemAdmin's UI override applies on the
	// very next CreateTenant call.
	systemSettingSvc interfaces.SystemSettingService
}

// NewTenantHandler creates a new tenant handler instance with the provided service
// Parameters:
//   - service: An implementation of the TenantService interface for business logic
//   - userService: An implementation of the UserService interface for user operations
//   - memberService: An implementation of TenantMemberService used to bootstrap
//     the creator as Owner of the tenant they just created (self-service create).
//   - config: Application configuration
//
// # Returns a pointer to the newly created TenantHandler
//
// Note on RBAC: cross-tenant gating (CanAccessAllTenants /
// EnableCrossTenantAccess) and per-tenant path matching (URL :id ==
// active tenant) used to live in `authorizeTenantAccess` and the if
// blocks at the top of ListAllTenants / SearchTenants. Both moved to
// `middleware/access.go` (RequireCrossTenantAccess /
// RequirePathTenantMatch) and are wired in `router.go` so the handler
// stays focused on business logic.
func NewTenantHandler(
	service interfaces.TenantService,
	apiKeyService interfaces.TenantAPIKeyService,
	userService interfaces.UserService,
	memberService interfaces.TenantMemberService,
	kbService interfaces.KnowledgeBaseService,
	config *config.Config,
	systemSettingSvc interfaces.SystemSettingService,
) *TenantHandler {
	return &TenantHandler{
		service:          service,
		apiKeyService:    apiKeyService,
		userService:      userService,
		memberService:    memberService,
		kbService:        kbService,
		config:           config,
		systemSettingSvc: systemSettingSvc,
	}
}

// createTenantRequest is the JSON body for POST /tenants. Only fields a
// regular authenticated user is allowed to set are accepted; everything
// else (api_key, status, storage_quota, retriever_engines, etc.) is
// generated server-side by TenantService.CreateTenant so a normal user
// can't bypass quotas or self-suspend a workspace at create time.
//
// Cross-tenant superusers historically posted the full Tenant struct to
// this endpoint. We keep that path working by binding into the same
// types.Tenant when CanAccessAllTenants is true (see CreateTenant
// below), but the recommended shape going forward is name+description.
type createTenantRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=128"`
	Description string `json:"description" binding:"max=512"`
}

// updateTenantRequest is the JSON body for PUT /tenants/:id. Only the
// fields an Owner is permitted to mutate via the public API are bound;
// everything else (storage_quota, status, business, api_key, agent /
// retrieval / storage configs, ...) is intentionally NOT writable here -
// those go through dedicated endpoints (PUT /tenants/kv/:key, ...)
// that have their own validation.
//
// Pointers so we can distinguish "not sent" from "explicit empty
// string"; when nil we leave the existing column untouched.
type updateTenantRequest struct {
	Name        *string `json:"name"        binding:"omitempty,min=1,max=128"`
	Description *string `json:"description" binding:"omitempty,max=512"`
}

type apiPrincipalConfigRequest struct {
	Mode                  types.APIPrincipalMode `json:"mode"`
	DirectHeaderName      string                 `json:"direct_header_name"`
	SignedTokenHeaderName string                 `json:"signed_token_header_name"`
	RequireDirectHeader   bool                   `json:"require_direct_header"`
	HMACSecret            *string                `json:"hmac_secret"`
}

type apiPrincipalConfigResponse struct {
	Mode                  types.APIPrincipalMode `json:"mode"`
	DirectHeaderName      string                 `json:"direct_header_name"`
	SignedTokenHeaderName string                 `json:"signed_token_header_name"`
	RequireDirectHeader   bool                   `json:"require_direct_header"`
	// HasHMACSecret reports whether a signing secret is configured. The
	// plaintext secret is NEVER returned - clients only learn presence.
	HasHMACSecret bool `json:"has_hmac_secret"`
}

type apiPrincipalTestTokenRequest struct {
	ExternalUserID   string `json:"external_user_id"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type apiPrincipalTestTokenResponse struct {
	Token            string `json:"token"`
	HeaderName       string `json:"header_name"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	ExpiresAtUnix    int64  `json:"expires_at_unix"`
	ExternalUserID   string `json:"external_user_id"`
}

type tenantAPIKeyCreateRequest struct {
	Name             string   `json:"name"`
	FullAccess       bool     `json:"full_access"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	Capabilities     []string `json:"capabilities"`
	ExpiresAt        *int64   `json:"expires_at_unix"`
}

// tenantAPIKeyUpdateRequest changes an existing API key's settings; fields mean the same as on create.
type tenantAPIKeyUpdateRequest struct {
	Name             string   `json:"name"`
	FullAccess       bool     `json:"full_access"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	Capabilities     []string `json:"capabilities"`
	ExpiresAt        *int64   `json:"expires_at_unix"`
}

type tenantAPIKeyResponse struct {
	ID               uint64                `json:"id"`
	ScopeType        types.APIKeyScopeType `json:"scope_type"`
	Name             string                `json:"name"`
	APIKey           string                `json:"api_key"`
	FullAccess       bool                  `json:"full_access"`
	KnowledgeBaseIDs types.StringArray     `json:"knowledge_base_ids"`
	Capabilities     types.StringArray     `json:"capabilities"`
	LastUsedAt       *time.Time            `json:"last_used_at,omitempty"`
	ExpiresAt        *time.Time            `json:"expires_at,omitempty"`
	CreatedAt        time.Time             `json:"created_at"`
}

type tenantAPIKeyCreateResponse struct {
	tenantAPIKeyResponse
	Token string `json:"token"`
}

const (
	defaultAPIPrincipalDirectHeader  = "X-External-User-ID"
	defaultAPIPrincipalTokenHeader   = "X-External-User-Token"
	defaultAPIPrincipalTestTokenTTL  = 15 * time.Minute
	maxAPIPrincipalTestTokenTTL      = time.Hour
	maxAPIPrincipalExternalUserIDLen = 128
	// apiPrincipalSecretRedacted is the placeholder an update request may
	// send in place of the HMAC secret to signal "leave the stored secret
	// unchanged". The plaintext secret is never returned by GET, so the
	// client cannot echo the real value back.
	apiPrincipalSecretRedacted = "***"
)

// defaultMaxOwnedTenantsPerUser is the cap applied when
// config.Tenant.MaxOwnedPerUser is left at zero. Picked to comfortably
// cover legitimate "personal + a couple of side-projects" use while
// blunting drive-by abuse against POST /tenants (see CreateTenant).
const defaultMaxOwnedTenantsPerUser = 10

// resolveMaxOwnedTenantsPerUser returns the current cap, walking the
// 3-tier resolver: system_settings DB row > AIKB_TENANT_MAX_OWNED_PER_USER
// env > config.Tenant.MaxOwnedPerUser (yaml) > defaultMaxOwnedTenantsPerUser.
// We pre-compute the cfg-derived fallback so the SystemSettingService
// receives a single int64 default - its 3-tier resolver layers DB and
// env on top of that.
func (h *TenantHandler) resolveMaxOwnedTenantsPerUser(ctx context.Context) int {
	fallback := int64(defaultMaxOwnedTenantsPerUser)
	if h.config != nil && h.config.Tenant != nil && h.config.Tenant.MaxOwnedPerUser != 0 {
		fallback = int64(h.config.Tenant.MaxOwnedPerUser)
	}
	return int(h.systemSettingSvc.GetInt(
		ctx,
		"tenant.max_owned_per_user",
		"AIKB_TENANT_MAX_OWNED_PER_USER",
		fallback,
	))
}

// CreateTenant godoc
// @Summary      Create workspace
// @Description  Creates a new workspace. Any signed-in user can call it to set up their own workspace,
// @Description  and the caller becomes the workspace Owner. Cross-workspace super admins can still
// @Description  create any workspace through this endpoint.
// @Description  When tenant.auto_create_api_key (or AIKB_TENANT_AUTO_CREATE_API_KEY)
// @Description  is on, a full_access API key is created and its plaintext token returned in data.api_key.
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        request  body      handler.createTenantRequest  true  "Workspace details"
// @Success      201      {object}  map[string]interface{}  "Created workspace (optionally with api_key)"
// @Failure      400      {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Router       /tenants [post]
func (h *TenantHandler) CreateTenant(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start creating tenant")

	// Resolve the caller; required so we can bootstrap the Owner
	// membership and so we can branch on cross-tenant superuser status
	// for the legacy full-payload path.
	caller, err := h.userService.GetCurrentUser(ctx)
	if err != nil || caller == nil {
		logger.Error(ctx, "Failed to resolve current user from context", err)
		c.Error(errors.NewUnauthorizedError("authentication required"))
		return
	}
	apiKeyScope, hasAPIKeyScope := types.TenantAPIKeyScopeFromContext(ctx)
	platformCaller := hasAPIKeyScope && apiKeyScope.IsPlatform()
	catalogManager := caller.CanAccessAllTenants || platformCaller

	// Deployment-level policy: ordinary users may be restricted to joining
	// existing workspaces by invitation. This check is authoritative; the
	// frontend capability only improves UX and cannot bypass it. Cross-tenant
	// superusers retain the catalog-management create path.
	if !catalogManager &&
		!resolveTenantSelfServiceCreationEnabled(ctx, h.config, h.systemSettingSvc) {
		logger.Warnf(ctx, "Self-service tenant creation denied by policy for user %s", caller.ID)
		c.Error(errors.NewTenantCreationDisabledError())
		return
	}

	var tenantData types.Tenant

	if catalogManager {
		// Backward-compatible path for cross-tenant superusers: accept
		// the full Tenant payload (status, storage_quota, retriever
		// engines, configs...) so existing tooling keeps working.
		if err := c.ShouldBindJSON(&tenantData); err != nil {
			logger.Error(ctx, "Failed to parse request parameters", err)
			appErr := errors.NewValidationError("Invalid request parameters").WithDetails(err.Error())
			c.Error(appErr)
			return
		}
		// Reset client-supplied primary key so we don't accidentally
		// insert with a chosen ID that collides with a future
		// auto-increment value. Tenant IDs must always be DB-generated.
		tenantData.ID = 0
	} else {
		// Self-service path: a regular user can only set name and
		// description. Everything else is server-generated by
		// TenantService.CreateTenant (status="active", storage_quota
		// default, retriever engines from RETRIEVE_DRIVER). API keys are
		// created explicitly through the integration API-key list.
		var req createTenantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error(ctx, "Failed to parse request parameters", err)
			appErr := errors.NewValidationError("Invalid request parameters").WithDetails(err.Error())
			c.Error(appErr)
			return
		}

		// Per-user quota: cap how many tenants a regular user can spin
		// up via self-service. Without this any authenticated client
		// can flood `tenants` (and saturate validateStorageBucketUniqueness
		// which scans the whole table). Superusers above are exempt
		// because they're already trusted to manage the catalog.
		if h.memberService != nil {
			memberships, listErr := h.memberService.ListByUser(ctx, caller.ID)
			if listErr != nil {
				logger.Errorf(ctx, "Failed to count owned tenants for user %s: %v", caller.ID, listErr)
				c.Error(errors.NewInternalServerError("Failed to validate workspace quota").WithDetails(listErr.Error()))
				return
			}
			ownedCount := 0
			for _, m := range memberships {
				if m != nil && m.Role == types.TenantRoleOwner {
					ownedCount++
				}
			}
			cap := h.resolveMaxOwnedTenantsPerUser(ctx)
			if cap > 0 && ownedCount >= cap {
				logger.Warnf(ctx,
					"User %s reached self-service tenant quota (%d/%d)",
					caller.ID, ownedCount, cap,
				)
				c.Error(errors.NewTooManyRequestsError(
					"reached self-service workspace quota; contact an administrator to raise the limit",
				))
				return
			}
		}

		tenantData = types.Tenant{
			Name:        strings.TrimSpace(req.Name),
			Description: strings.TrimSpace(req.Description),
		}
	}

	// Apply the system-setting-driven default storage quota when the
	// caller didn't specify one (always true for self-serve; sometimes
	// true for the superuser branch when the JSON omits storage_quota).
	// We resolve at create time on purpose - the on-disk row should
	// carry an explicit value, so changing the setting later doesn't
	// silently shrink/grow established tenants. Negative values are
	// treated as "use default" so a misconfigured setting can't yield
	// a negative quota that the storage-used checks would interpret as
	// "unlimited" (StorageQuota <= 0 disables enforcement in
	// knowledge_create.go).
	if tenantData.StorageQuota <= 0 {
		gb := h.systemSettingSvc.GetInt(
			ctx,
			"tenant.default_storage_quota_gb",
			"AIKB_TENANT_DEFAULT_STORAGE_QUOTA_GB",
			10,
		)
		if gb <= 0 {
			gb = 10
		}
		tenantData.StorageQuota = gb * 1024 * 1024 * 1024
	}

	logger.Infof(ctx, "Creating tenant, name: %s", secutils.SanitizeForLog(tenantData.Name))

	createdTenant, err := h.service.CreateTenant(ctx, &tenantData)
	if err != nil {
		// Check if this is an application-specific error
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to create workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to create workspace").WithDetails(err.Error()))
		}
		return
	}

	// Bootstrap an Owner membership so the caller immediately has full
	// control over the tenant they just created. We MUST roll the tenant
	// back if this fails: without a membership row the new tenant is
	// unreachable (middleware/auth.go's orphan-recovery only fires for a
	// user's home tenant, never for a freshly-created side workspace),
	// yet still occupies storage_bucket / name uniqueness slots.
	// Idempotent: EnsureOwner is a no-op when the row already exists,
	// so cross-tenant superusers create-and-own through the same path.
	if h.memberService != nil && !platformCaller {
		if _, err := h.memberService.EnsureOwner(ctx, caller.ID, createdTenant.ID); err != nil {
			logger.Errorf(ctx,
				"Failed to bootstrap owner membership for user %s tenant %d: %v - rolling back tenant",
				caller.ID, createdTenant.ID, err)
			if delErr := h.service.DeleteTenant(ctx, createdTenant.ID); delErr != nil {
				logger.Errorf(ctx,
					"Rollback DeleteTenant failed for orphan tenant %d: %v",
					createdTenant.ID, delErr,
				)
			}
			c.Error(errors.NewInternalServerError("Failed to finalise workspace ownership").WithDetails(err.Error()))
			return
		}

		// Quota TOCTOU guard. The earlier ownedCount check is racy:
		// N concurrent CreateTenant calls all read ownedCount < cap,
		// all proceed, all insert. Re-count AFTER the Owner membership
		// is committed; if we landed over the cap, roll back this
		// tenant + its membership so the bound holds in steady state.
		// We only do this for non-superusers (the only path that has
		// a cap) - superusers are exempt above.
		if !caller.CanAccessAllTenants {
			memberships, listErr := h.memberService.ListByUser(ctx, caller.ID)
			if listErr != nil {
				logger.Errorf(ctx, "Post-create quota recount failed for user %s tenant %d: %v",
					caller.ID, createdTenant.ID, listErr)
			} else {
				ownedNow := 0
				for _, m := range memberships {
					if m != nil && m.Role == types.TenantRoleOwner {
						ownedNow++
					}
				}
				cap := h.resolveMaxOwnedTenantsPerUser(ctx)
				if cap > 0 && ownedNow > cap {
					logger.Warnf(ctx,
						"User %s exceeded tenant quota after concurrent create (%d/%d), rolling back tenant %d",
						caller.ID, ownedNow, cap, createdTenant.ID,
					)
					if rmErr := h.memberService.RemoveMember(ctx, caller.ID, createdTenant.ID); rmErr != nil {
						logger.Errorf(ctx,
							"Rollback RemoveMember failed for user %s tenant %d: %v",
							caller.ID, createdTenant.ID, rmErr,
						)
					}
					if delErr := h.service.DeleteTenant(ctx, createdTenant.ID); delErr != nil {
						logger.Errorf(ctx,
							"Rollback DeleteTenant failed for over-quota tenant %d: %v",
							createdTenant.ID, delErr,
						)
					}
					c.Error(errors.NewTooManyRequestsError(
						"reached self-service workspace quota; contact an administrator to raise the limit",
					))
					return
				}
			}
		}
	}

	// When a tenantless user creates their first workspace, make it their
	// default login tenant. Roll the just-created resources back if this
	// finalisation fails so the user is not left with an unreachable tenant.
	if caller.TenantID == 0 && !platformCaller {
		caller.TenantID = createdTenant.ID
		if err := h.userService.UpdateUser(ctx, caller); err != nil {
			logger.Errorf(ctx, "Failed to set first tenant %d as default for user %s: %v",
				createdTenant.ID, caller.ID, err)
			if h.memberService != nil {
				_ = h.memberService.RemoveMember(ctx, caller.ID, createdTenant.ID)
			}
			_ = h.service.DeleteTenant(ctx, createdTenant.ID)
			c.Error(errors.NewInternalServerError("Failed to finalise default workspace").WithDetails(err.Error()))
			return
		}
	}

	logger.Infof(
		ctx,
		"Tenant created successfully, ID: %d, name: %s",
		createdTenant.ID,
		secutils.SanitizeForLog(createdTenant.Name),
	)

	// data carries the created tenant. When the legacy auto-create-key
	// behaviour is enabled we embed the plaintext token as data.api_key so
	// the response shape mirrors the pre-break-change behaviour integrations
	// relied on.
	var data any = createdTenant

	// Optional legacy compatibility: mint a full-access API key on tenant
	// creation and return its plaintext token, gated by the
	// tenant.auto_create_api_key setting (env AIKB_TENANT_AUTO_CREATE_API_KEY).
	// Default off - modern deployments create keys explicitly via
	// tenant_api_keys. Failing to create the convenience key must NOT fail
	// the whole tenant creation (the tenant is fully usable without a key);
	// we log a warning and return the tenant as usual.
	// A platform key with system_tenants_manage may create workspaces, but it
	// must not bootstrap a full-access workspace key and escalate beyond its
	// own explicit capabilities.
	if !platformCaller && h.autoCreateTenantAPIKey(ctx) && h.apiKeyService != nil {
		result, keyErr := h.apiKeyService.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
			TenantID:   createdTenant.ID,
			Name:       "default",
			FullAccess: true,
		})
		if keyErr != nil {
			logger.Errorf(ctx,
				"Auto-create default API key failed for tenant %d: %v - returning tenant without key",
				createdTenant.ID, keyErr)
		} else if merged, mErr := tenantWithAPIKey(createdTenant, result.Token); mErr != nil {
			// Round-trip failure is unexpected; degrade gracefully by
			// returning the tenant without embedding the key rather than
			// failing the whole request.
			logger.Errorf(ctx, "Failed to embed api_key into tenant response for tenant %d: %v",
				createdTenant.ID, mErr)
		} else {
			data = merged
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    data,
	})
}

// tenantWithAPIKey returns the tenant serialized as a map with an extra
// api_key field, so the create response can embed the plaintext token inside
// data (mirroring the pre-break-change shape) without adding a persisted
// api_key column back onto types.Tenant.
func tenantWithAPIKey(tenant *types.Tenant, token string) (map[string]any, error) {
	raw, err := json.Marshal(tenant)
	if err != nil {
		return nil, err
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m["api_key"] = token
	return m, nil
}

// autoCreateTenantAPIKey resolves whether tenant creation should also mint a
// full-access API key (legacy compatibility). 3-tier resolver:
// system_settings DB row > AIKB_TENANT_AUTO_CREATE_API_KEY env > false.
func (h *TenantHandler) autoCreateTenantAPIKey(ctx context.Context) bool {
	if h.systemSettingSvc == nil {
		return false
	}
	return h.systemSettingSvc.GetBool(
		ctx,
		"tenant.auto_create_api_key",
		"AIKB_TENANT_AUTO_CREATE_API_KEY",
		false,
	)
}

// GetTenant godoc
// @Summary      Get workspace
// @Description  Gets a workspace by ID
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Workspace ID"
// @Success      200  {object}  map[string]interface{}  "Workspace details"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Failure      404  {object}  errors.ErrorResponse         "Workspace not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/{id} [get]
func (h *TenantHandler) GetTenant(c *gin.Context) {
	ctx := c.Request.Context()

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Errorf(ctx, "Invalid workspace ID: %s", secutils.SanitizeForLog(c.Param("id")))
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to retrieve workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to retrieve workspace").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    dto.NewTenantResponse(ctx, tenant),
	})
}

// UpdateTenant godoc
// @Summary      Update workspace
// @Description  Updates workspace details
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        id       path      int           true  "Workspace ID"
// @Param        request  body      types.Tenant  true  "Workspace details"
// @Success      200      {object}  map[string]interface{}  "Updated workspace"
// @Failure      400      {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Router       /tenants/{id} [put]
func (h *TenantHandler) UpdateTenant(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start updating tenant")

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Errorf(ctx, "Invalid workspace ID: %s", secutils.SanitizeForLog(c.Param("id")))
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	// Strict whitelist: only Name / Description are mutable through the
	// public PUT. Storage quota, status, business, configs, api_key and
	// every other privileged column live behind dedicated endpoints
	// (PUT /tenants/kv/:key, ...). Without this, an
	// Owner - including any user who just self-served a tenant - could
	// flip status / bump storage_quota by simply crafting an extended
	// JSON body. Pointers distinguish "field omitted" from "explicit
	// empty string" so we can leave untouched columns alone.
	var req updateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	// Load the persisted tenant so any column the request omits keeps
	// its current value through the GORM `Updates(struct)` zero-skip
	// behaviour (we always pass back the full struct).
	existing, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			c.Error(errors.NewValidationError("name cannot be blank"))
			return
		}
		existing.Name = trimmed
	}
	if req.Description != nil {
		existing.Description = strings.TrimSpace(*req.Description)
	}

	logger.Infof(ctx, "Updating tenant, ID: %d, Name: %s", id, secutils.SanitizeForLog(existing.Name))

	updatedTenant, err := h.service.UpdateTenant(ctx, existing)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to update workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace").WithDetails(err.Error()))
		}
		return
	}

	logger.Infof(
		ctx,
		"Tenant updated successfully, ID: %d, Name: %s",
		updatedTenant.ID,
		secutils.SanitizeForLog(updatedTenant.Name),
	)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    dto.NewTenantResponse(ctx, updatedTenant),
	})
}

func (h *TenantHandler) ListAPIKeys(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	keys, err := h.apiKeyService.ListAPIKeys(ctx, id)
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to list API keys").WithDetails(err.Error()))
		return
	}
	resp := make([]tenantAPIKeyResponse, 0, len(keys))
	for _, key := range keys {
		resp = append(resp, tenantAPIKeyForResponse(key))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": resp})
}

func (h *TenantHandler) CreateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	var req tenantAPIKeyCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if err := validateTenantAPIKeyRequest(ctx, h.kbService, id, req); err != nil {
		c.Error(err)
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		if !t.After(time.Now().UTC()) {
			c.Error(errors.NewValidationError("expires_at_unix must be in the future"))
			return
		}
		expiresAt = &t
	}
	result, err := h.apiKeyService.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID:         id,
		Name:             req.Name,
		FullAccess:       req.FullAccess,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs,
		Capabilities:     req.Capabilities,
		ExpiresAt:        expiresAt,
	})
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to create API key").WithDetails(err.Error()))
		return
	}
	item := tenantAPIKeyForResponse(result.APIKey)
	item.APIKey = result.Token
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    tenantAPIKeyCreateResponse{tenantAPIKeyResponse: item, Token: result.Token},
	})
}

// UpdateAPIKey changes the scope and other settings of an existing workspace API key.
// The route requires the workspace Owner; field validation matches create.
func (h *TenantHandler) UpdateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || tenantID == 0 {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	keyID, err := strconv.ParseUint(c.Param("key_id"), 10, 64)
	if err != nil || keyID == 0 {
		c.Error(errors.NewBadRequestError("Invalid API key ID"))
		return
	}
	var req tenantAPIKeyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if appErr := validateTenantAPIKeyRequest(ctx, h.kbService, tenantID, tenantAPIKeyCreateRequest(req)); appErr != nil {
		c.Error(appErr)
		return
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		expiresAt = &t
	}

	updated, err := h.apiKeyService.UpdateAPIKey(ctx, interfaces.TenantAPIKeyUpdateRequest{
		TenantID: tenantID, APIKeyID: keyID, Name: req.Name, FullAccess: req.FullAccess,
		KnowledgeBaseIDs: req.KnowledgeBaseIDs, Capabilities: req.Capabilities, ExpiresAt: expiresAt,
	})
	if err != nil {
		c.Error(errors.NewNotFoundError("API key not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tenantAPIKeyForResponse(updated)})
}

func (h *TenantHandler) DeleteAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	keyID, err := strconv.ParseUint(c.Param("key_id"), 10, 64)
	if err != nil || keyID == 0 {
		c.Error(errors.NewBadRequestError("Invalid API key ID"))
		return
	}
	if err := h.apiKeyService.RevokeAPIKey(ctx, tenantID, keyID); err != nil {
		c.Error(errors.NewNotFoundError("API key not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// tenantAPIKeyForResponse masks api_key: the plaintext goes out only once, in the create response.
func tenantAPIKeyForResponse(key *types.TenantAPIKey) tenantAPIKeyResponse {
	if key == nil {
		return tenantAPIKeyResponse{}
	}
	return tenantAPIKeyResponse{
		ID:               key.ID,
		ScopeType:        types.NormalizeAPIKeyScopeType(key.ScopeType),
		Name:             key.Name,
		APIKey:           key.KeyHint,
		FullAccess:       key.FullAccess,
		KnowledgeBaseIDs: key.KnowledgeBaseIDs,
		Capabilities:     types.NormalizeAPIKeyCapabilities(key.Capabilities),
		LastUsedAt:       key.LastUsedAt,
		ExpiresAt:        key.ExpiresAt,
		CreatedAt:        key.CreatedAt,
	}
}

func validateTenantAPIKeyRequest(
	ctx context.Context,
	kbService interfaces.KnowledgeBaseService,
	tenantID uint64,
	req tenantAPIKeyCreateRequest,
) *errors.AppError {
	if strings.TrimSpace(req.Name) == "" {
		return errors.NewValidationError("name is required")
	}
	if req.FullAccess {
		return nil
	}
	caps := types.NormalizeAPIKeyCapabilities(types.StringArray(req.Capabilities))
	if len(caps) == 0 {
		return errors.NewValidationError("capabilities are required for scoped API keys")
	}
	for _, cap := range req.Capabilities {
		if strings.TrimSpace(cap) == "" {
			continue
		}
		if types.NormalizeAPIKeyCapability(types.APIKeyCapability(cap)) == "" {
			return errors.NewValidationError("capabilities contains an unknown capability")
		}
	}
	return validateTenantAPIKeyKnowledgeBaseIDs(ctx, kbService, tenantID, req.KnowledgeBaseIDs)
}

// validateTenantAPIKeyKnowledgeBaseIDs checks that every allowlisted knowledge base exists and belongs to the target
// workspace.
// On failure it returns an app error ready to send as the response.
func validateTenantAPIKeyKnowledgeBaseIDs(
	ctx context.Context,
	kbService interfaces.KnowledgeBaseService,
	tenantID uint64,
	knowledgeBaseIDs []string,
) *errors.AppError {
	if len(knowledgeBaseIDs) == 0 {
		return nil
	}
	return validateTenantAPIKeyKnowledgeBaseIDsWithLookup(
		ctx, tenantID, knowledgeBaseIDs, kbService.GetKnowledgeBaseByID,
	)
}

// validateTenantAPIKeyKnowledgeBaseIDsWithLookup decouples the ownership check from the large knowledge base service
// interface so boundary cases are testable.
// lookup returns the real knowledge base for an ID; the result is nil or a validation error ready to send.
func validateTenantAPIKeyKnowledgeBaseIDsWithLookup(
	ctx context.Context,
	tenantID uint64,
	knowledgeBaseIDs []string,
	lookup func(context.Context, string) (*types.KnowledgeBase, error),
) *errors.AppError {
	for _, kbID := range knowledgeBaseIDs {
		kbID = strings.TrimSpace(kbID)
		if kbID == "" {
			continue
		}
		kb, err := lookup(ctx, kbID)
		if err != nil || kb == nil {
			return errors.NewValidationError("knowledge_base_ids contains an unknown knowledge base")
		}
		if kb.TenantID != tenantID {
			return errors.NewForbiddenError("knowledge_base_ids contains a knowledge base outside this workspace")
		}
	}
	return nil
}

func apiPrincipalConfigForResponse(cfg *types.APIPrincipalConfig) apiPrincipalConfigResponse {
	if cfg == nil {
		cfg = &types.APIPrincipalConfig{}
	}
	mode := cfg.Mode
	if mode == "" {
		mode = types.APIPrincipalModeTenant
	}
	return apiPrincipalConfigResponse{
		Mode:                  mode,
		DirectHeaderName:      defaultAPIPrincipalDirectHeader,
		SignedTokenHeaderName: defaultAPIPrincipalTokenHeader,
		RequireDirectHeader:   cfg.RequireDirectHeader,
		HasHMACSecret:         strings.TrimSpace(cfg.HMACSecret) != "",
	}
}

// GetAPIPrincipalConfig godoc
// @Summary      Get workspace API key identity config
// @Description  Returns how X-API-Key requests map to an end-user Principal (Owner)
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Workspace ID"
// @Success      200  {object}  map[string]interface{}  "API principal config"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Failure      403  {object}  errors.ErrorResponse         "Insufficient permissions"
// @Security     Bearer
// @Router       /tenants/{id}/api-principal-config [get]
func (h *TenantHandler) GetAPIPrincipalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apiPrincipalConfigForResponse(tenant.APIPrincipalConfig),
	})
}

// UpdateAPIPrincipalConfig godoc
// @Summary      Update workspace API key identity config
// @Description  Configures how X-API-Key requests map to an end-user Principal (Owner)
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        id       path      int                           true  "Workspace ID"
// @Param        request  body      handler.apiPrincipalConfigRequest  true  "API principal config"
// @Success      200      {object}  map[string]interface{}        "Updated config"
// @Failure      400      {object}  errors.ErrorResponse               "Invalid request parameters"
// @Failure      403      {object}  errors.ErrorResponse               "Insufficient permissions"
// @Security     Bearer
// @Router       /tenants/{id}/api-principal-config [put]
func (h *TenantHandler) UpdateAPIPrincipalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}
	var req apiPrincipalConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if req.Mode == "" {
		req.Mode = types.APIPrincipalModeTenant
	}
	switch req.Mode {
	case types.APIPrincipalModeTenant, types.APIPrincipalModeDirect, types.APIPrincipalModeSignedToken:
	default:
		c.Error(errors.NewValidationError("mode must be tenant, direct_header, or signed_token"))
		return
	}

	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}

	existingSecret := ""
	if tenant.APIPrincipalConfig != nil {
		existingSecret = tenant.APIPrincipalConfig.HMACSecret
	}
	hmacSecret := existingSecret
	if req.HMACSecret != nil {
		provided := strings.TrimSpace(*req.HMACSecret)
		// GET no longer discloses the plaintext secret, so a client that
		// edits the config re-submits the redaction placeholder to mean
		// "keep the existing secret". Treat it as a no-op instead of
		// overwriting the real secret with "***".
		if provided != apiPrincipalSecretRedacted {
			hmacSecret = provided
		}
	}
	cfg := &types.APIPrincipalConfig{
		Mode:                  req.Mode,
		DirectHeaderName:      defaultAPIPrincipalDirectHeader,
		SignedTokenHeaderName: defaultAPIPrincipalTokenHeader,
		RequireDirectHeader:   req.RequireDirectHeader,
		HMACSecret:            hmacSecret,
	}
	if cfg.Mode == types.APIPrincipalModeSignedToken && strings.TrimSpace(cfg.HMACSecret) == "" {
		c.Error(errors.NewValidationError("hmac_secret is required for signed_token mode"))
		return
	}
	tenant.APIPrincipalConfig = cfg

	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to update API principal config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apiPrincipalConfigForResponse(updatedTenant.APIPrincipalConfig),
	})
}

// CreateAPIPrincipalTestToken godoc
// @Summary      Generate API Playground test JWT
// @Description  Signs a short-lived external user JWT with the workspace's saved HMAC secret (Owner)
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        id       path      int                                  true  "Workspace ID"
// @Param        request  body      handler.apiPrincipalTestTokenRequest true  "Test token parameters"
// @Success      200      {object}  map[string]interface{}               "Short-lived JWT"
// @Failure      400      {object}  errors.ErrorResponse                      "Invalid request parameters"
// @Failure      403      {object}  errors.ErrorResponse                      "Insufficient permissions"
// @Security     Bearer
// @Router       /tenants/{id}/api-principal-test-token [post]
func (h *TenantHandler) CreateAPIPrincipalTestToken(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	var req apiPrincipalTestTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	externalUserID := strings.TrimSpace(req.ExternalUserID)
	if err := validateAPIPrincipalExternalUserID(externalUserID); err != nil {
		c.Error(errors.NewValidationError("external_user_id is invalid").WithDetails(err.Error()))
		return
	}

	tenant, err := h.service.GetTenantByID(ctx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			c.Error(errors.NewInternalServerError("Failed to load workspace").WithDetails(err.Error()))
		}
		return
	}

	cfg := tenant.APIPrincipalConfig
	if cfg == nil || cfg.Mode != types.APIPrincipalModeSignedToken {
		c.Error(errors.NewValidationError("signed_token mode is required"))
		return
	}
	secret := strings.TrimSpace(cfg.HMACSecret)
	if secret == "" {
		c.Error(errors.NewValidationError("hmac_secret is required for signed_token mode"))
		return
	}

	ttl := defaultAPIPrincipalTestTokenTTL
	if req.ExpiresInSeconds > 0 {
		ttl = time.Duration(req.ExpiresInSeconds) * time.Second
	}
	if ttl <= 0 || ttl > maxAPIPrincipalTestTokenTTL {
		c.Error(errors.NewValidationError("expires_in_seconds must be between 1 and 3600"))
		return
	}

	now := time.Now()
	expiresAt := now.Add(ttl)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       externalUserID,
		"tenant_id": strconv.FormatUint(id, 10),
		"aud":       "aikb",
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
	}).SignedString([]byte(secret))
	if err != nil {
		c.Error(errors.NewInternalServerError("Failed to create API principal test token").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": apiPrincipalTestTokenResponse{
			Token:            token,
			HeaderName:       defaultAPIPrincipalTokenHeader,
			ExpiresInSeconds: int(ttl.Seconds()),
			ExpiresAtUnix:    expiresAt.Unix(),
			ExternalUserID:   externalUserID,
		},
	})
}

func validateAPIPrincipalExternalUserID(id string) error {
	if id == "" {
		return errors.NewValidationError("external_user_id is required")
	}
	if len(id) > maxAPIPrincipalExternalUserIDLen {
		return errors.NewValidationError("external_user_id is too long")
	}
	for _, r := range id {
		if r < 0x20 || r == 0x7f {
			return errors.NewValidationError("external_user_id contains invalid characters")
		}
	}
	return nil
}

// DeleteTenant godoc
// @Summary      Delete workspace
// @Description  Deletes the given workspace
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Workspace ID"
// @Success      200  {object}  map[string]interface{}  "Deleted"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Router       /tenants/{id} [delete]
func (h *TenantHandler) DeleteTenant(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start deleting tenant")

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		logger.Errorf(ctx, "Invalid workspace ID: %s", secutils.SanitizeForLog(c.Param("id")))
		c.Error(errors.NewBadRequestError("Invalid workspace ID"))
		return
	}

	logger.Infof(ctx, "Deleting tenant, ID: %d", id)

	if err := h.service.DeleteTenant(ctx, id); err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to delete workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to delete workspace").WithDetails(err.Error()))
		}
		return
	}

	logger.Infof(ctx, "Workspace deleted successfully, ID: %d", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Workspace deleted successfully",
	})
}

// ListTenants godoc
// @Summary      List workspaces
// @Description  Lists the workspaces the current user can access
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Workspaces"
// @Failure      500  {object}  errors.ErrorResponse         "Internal server error"
// @Security     Bearer
// @Router       /tenants [get]
func (h *TenantHandler) ListTenants(c *gin.Context) {
	ctx := c.Request.Context()

	tenant, ok := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
	if !ok || tenant == nil {
		c.Error(errors.NewUnauthorizedError("Authentication required"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": []*dto.TenantResponse{dto.NewTenantResponse(ctx, tenant)},
		},
	})
}

// ListAllTenants godoc
// @Summary      List all workspaces
// @Description  Lists every workspace in the system (requires cross-workspace access)
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "All workspaces"
// @Failure      403  {object}  errors.ErrorResponse         "Insufficient permissions"
// @Security     Bearer
// @Router       /tenants/all [get]
func (h *TenantHandler) ListAllTenants(c *gin.Context) {
	ctx := c.Request.Context()

	// Cross-tenant gating (CanAccessAllTenants + EnableCrossTenantAccess)
	// is enforced at the route layer via middleware.RequireCrossTenantAccess
	// (router.go). The handler stays focused on listing.
	tenants, err := h.service.ListAllTenants(ctx)
	if err != nil {
		// Check if this is an application-specific error
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to retrieve all workspaces list: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to retrieve all workspaces list").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": dto.NewTenantResponsesCrossTenant(tenants),
		},
	})
}

// SearchTenants godoc
// @Summary      Search workspaces
// @Description  Searches workspaces with pagination (requires cross-workspace access)
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        keyword    query     string  false  "Search keyword"
// @Param        tenant_id  query     int     false  "Filter by workspace ID"
// @Param        page       query     int     false  "Page number"  default(1)
// @Param        page_size  query     int     false  "Page size"  default(20)
// @Success      200        {object}  map[string]interface{}  "Search results"
// @Failure      403        {object}  errors.ErrorResponse         "Insufficient permissions"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/search [get]
func (h *TenantHandler) SearchTenants(c *gin.Context) {
	ctx := c.Request.Context()

	// Cross-tenant gating is enforced at the route layer via
	// middleware.RequireCrossTenantAccess (router.go); the handler only
	// parses query params and delegates to the service.

	// Parse query parameters
	keyword := c.Query("keyword")
	tenantIDStr := c.Query("tenant_id")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")

	var tenantID uint64
	if tenantIDStr != "" {
		parsedID, err := strconv.ParseUint(tenantIDStr, 10, 64)
		if err == nil {
			tenantID = parsedID
		}
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100 // Limit max page size
	}

	tenants, total, err := h.service.SearchTenants(ctx, keyword, tenantID, page, pageSize)
	if err != nil {
		// Check if this is an application-specific error
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to search workspaces: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to search workspaces").WithDetails(err.Error()))
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":     dto.NewTenantResponsesCrossTenant(tenants),
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetTenantKV godoc
// @Summary      Get workspace KV config
// @Description  Gets a workspace-level KV config (supports web-search-config, prompt-templates, parser-engine-config,
// @Description  storage-engine-config, chat-history-config, retrieval-config)
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        key  path      string  true  "Config key"
// @Success      200  {object}  map[string]interface{}  "Config value"
// @Failure      400  {object}  errors.ErrorResponse         "Unsupported key"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/{key} [get]
func (h *TenantHandler) GetTenantKV(c *gin.Context) {
	ctx := c.Request.Context()
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "web-search-config", "parser-engine-config", "storage-engine-config":
		if !dto.CanViewIntegrationSecrets(ctx) {
			c.Error(errors.NewForbiddenError("integration configuration requires admin access"))
			return
		}
	}

	switch key {
	case "web-search-config":
		h.GetTenantWebSearchConfig(c)
		return
	case "prompt-templates":
		h.GetPromptTemplates(c)
		return
	case "parser-engine-config":
		h.GetTenantParserEngineConfig(c)
		return
	case "storage-engine-config":
		h.GetTenantStorageEngineConfig(c)
		return
	case "chat-history-config":
		h.GetTenantChatHistoryConfig(c)
		return
	case "retrieval-config":
		h.GetTenantRetrievalConfig(c)
		return
	case "memory-config":
		h.GetTenantMemoryConfig(c)
		return
	default:
		logger.Info(ctx, "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
		return
	}
}

// UpdateTenantKV godoc
// @Summary      Update workspace KV config
// @Description  Updates a workspace-level KV config (supports web-search-config, parser-engine-config,
// @Description  storage-engine-config, chat-history-config, retrieval-config)
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Param        key      path      string  true  "Config key"
// @Param        request  body      object  true  "Config value"
// @Success      200      {object}  map[string]interface{}  "Updated"
// @Failure      400      {object}  errors.ErrorResponse         "Unsupported key"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/{key} [put]
func (h *TenantHandler) UpdateTenantKV(c *gin.Context) {
	ctx := c.Request.Context()
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "web-search-config", "parser-engine-config", "storage-engine-config":
		if !dto.CanViewIntegrationSecrets(ctx) {
			c.Error(errors.NewForbiddenError("integration configuration requires admin access"))
			return
		}
	}

	switch key {
	case "web-search-config":
		h.updateTenantWebSearchConfigInternal(c)
		return
	case "parser-engine-config":
		h.updateTenantParserEngineConfigInternal(c)
		return
	case "storage-engine-config":
		h.updateTenantStorageEngineConfigInternal(c)
		return
	case "chat-history-config":
		h.updateTenantChatHistoryConfigInternal(c)
		return
	case "retrieval-config":
		h.updateTenantRetrievalConfigInternal(c)
		return
	case "memory-config":
		h.updateTenantMemoryConfigInternal(c)
		return
	default:
		logger.Info(ctx, "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
		return
	}
}

// updateTenantWebSearchConfigInternal updates tenant's web search config
func (h *TenantHandler) updateTenantWebSearchConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	// Bind directly into the strong typed struct
	var cfg types.WebSearchConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	cfg = *types.MergeWebSearchConfigForUpdate(&cfg, tenant.WebSearchConfig)

	// Validate configuration
	if cfg.MaxResults < 1 || cfg.MaxResults > 50 {
		c.Error(errors.NewBadRequestError("max_results must be between 1 and 50"))
		return
	}

	tenant.WebSearchConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			logger.Error(ctx, "Failed to update workspace: application error", appErr)
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace web search config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.WebSearchConfigForResponse(updatedTenant.WebSearchConfig, true),
		"message": "Web search configuration updated successfully",
	})
}

// GetTenantWebSearchConfig godoc
// @Summary      Get workspace web search config
// @Description  Gets the workspace's web search config
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Web search config"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/web-search-config [get]
func (h *TenantHandler) GetTenantWebSearchConfig(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start getting tenant web search config")
	// Get tenant
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	logger.Infof(ctx, "Tenant web search config retrieved successfully, Tenant ID: %d", tenant.ID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.WebSearchConfigForResponse(tenant.WebSearchConfig, true),
	})
}

// GetTenantParserEngineConfig returns the tenant's parser engine config (MinerU endpoint, API key, etc.).
func (h *TenantHandler) GetTenantParserEngineConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := types.ParserEngineConfigForResponse(tenant.ParserEngineConfig, true)
	if data == nil {
		data = &types.ParserEngineConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantParserEngineConfigInternal updates the tenant's parser engine config.
func (h *TenantHandler) updateTenantParserEngineConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()
	var cfg types.ParserEngineConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	merged := types.MergeParserEngineConfigForUpdate(&cfg, tenant.ParserEngineConfig)
	if err := validateParserEngineOutboundURLs(merged); err != nil {
		c.Error(errors.NewValidationError(err.Error()))
		return
	}
	tenant.ParserEngineConfig = merged
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace parser engine config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.ParserEngineConfigForResponse(updatedTenant.ParserEngineConfig, true),
		"message": "Parser engine config updated",
	})
}

// GetTenantStorageEngineConfig returns the tenant's storage engine config (Local, MinIO, COS parameters).
func (h *TenantHandler) GetTenantStorageEngineConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := types.StorageEngineConfigForResponse(tenant.StorageEngineConfig, true)
	if data == nil {
		data = &types.StorageEngineConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantStorageEngineConfigInternal updates the tenant's storage engine config.
func (h *TenantHandler) updateTenantStorageEngineConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()
	var cfg types.StorageEngineConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.DefaultProvider))
	if provider == "" {
		provider = firstAllowedStorageProvider()
	}
	if provider == "" {
		c.Error(errors.NewBadRequestError("No storage provider is allowed by STORAGE_ALLOW_LIST"))
		return
	}
	if !isStorageProviderAllowed(provider) {
		c.Error(errors.NewBadRequestError("Storage provider is not allowed by STORAGE_ALLOW_LIST"))
		return
	}
	cfg.DefaultProvider = provider
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	merged := types.MergeStorageEngineConfigForUpdate(&cfg, tenant.StorageEngineConfig)
	tenant.StorageEngineConfig = merged
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update workspace storage engine config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    types.StorageEngineConfigForResponse(updatedTenant.StorageEngineConfig, true),
		"message": "Storage engine config updated",
	})
}

// GetPromptTemplates godoc
// @Summary      Get prompt templates
// @Description  Gets the system-configured prompt templates
// @Tags         Workspaces
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Prompt template config"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /tenants/kv/prompt-templates [get]
func (h *TenantHandler) GetPromptTemplates(c *gin.Context) {
	// Return prompt templates from config.yaml
	templates := h.config.PromptTemplates
	if templates == nil {
		templates = &config.PromptTemplatesConfig{}
	}

	// Determine user language from context (set by Language middleware)
	lang := types.LanguageFromContextOrDefault(c.Request.Context())

	// Build a localized copy so the original config is never mutated
	localized := &config.PromptTemplatesConfig{
		SystemPrompt:          config.LocalizeTemplates(templates.SystemPrompt, lang),
		ContextTemplate:       config.LocalizeTemplates(templates.ContextTemplate, lang),
		Rewrite:               config.LocalizeTemplates(templates.Rewrite, lang),
		Fallback:              config.LocalizeTemplates(templates.Fallback, lang),
		GenerateSessionTitle:  templates.GenerateSessionTitle,
		GenerateSummary:       templates.GenerateSummary,
		GenerateKBDescription: templates.GenerateKBDescription,
		KeywordsExtraction:    templates.KeywordsExtraction,
		AgentSystemPrompt:     config.LocalizeTemplates(templates.AgentSystemPrompt, lang),
		IntentPrompts:         config.LocalizeTemplates(templates.IntentPrompts, lang),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    localized,
	})
}

// GetTenantChatHistoryConfig returns the tenant's chat history KB configuration.
func (h *TenantHandler) GetTenantChatHistoryConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.ChatHistoryConfig
	if data == nil {
		data = &types.ChatHistoryConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantChatHistoryConfigInternal updates the tenant's chat history KB configuration.
// When enabled with an embedding model and no KB exists yet, it auto-creates a hidden KB.
func (h *TenantHandler) updateTenantChatHistoryConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	// The frontend sends: enabled, embedding_model_id
	// knowledge_base_id is managed internally.
	var req types.ChatHistoryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	existing := tenant.ChatHistoryConfig

	// Build the new config, preserving the internally-managed knowledge_base_id
	cfg := &types.ChatHistoryConfig{
		Enabled:          req.Enabled,
		EmbeddingModelID: req.EmbeddingModelID,
		KnowledgeBaseID:  "", // will be set below
	}

	// Carry over existing KB ID if the embedding model hasn't changed
	if existing != nil && existing.KnowledgeBaseID != "" {
		if existing.EmbeddingModelID == req.EmbeddingModelID {
			cfg.KnowledgeBaseID = existing.KnowledgeBaseID
		} else {
			// Embedding model changed - the old KB is incompatible.
			// We'll create a new one below. The old KB remains but is orphaned (can be cleaned up later).
			logger.Infof(ctx, "Embedding model changed from %s to %s, will create new chat history KB", existing.EmbeddingModelID, req.EmbeddingModelID)
		}
	}

	// Auto-create hidden KB if enabled + model set + no KB yet
	if cfg.Enabled && cfg.EmbeddingModelID != "" && cfg.KnowledgeBaseID == "" {
		kb := &types.KnowledgeBase{
			Name:             "__chat_history__",
			Type:             types.KnowledgeBaseTypeDocument,
			IsTemporary:      true,
			Description:      "Auto-managed knowledge base for chat history message indexing",
			EmbeddingModelID: cfg.EmbeddingModelID,
		}
		createdKB, err := h.kbService.CreateKnowledgeBase(ctx, kb)
		if err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to create chat history knowledge base").WithDetails(err.Error()))
			return
		}
		cfg.KnowledgeBaseID = createdKB.ID
		logger.Infof(ctx, "Auto-created chat history KB: id=%s, embedding_model=%s", createdKB.ID, cfg.EmbeddingModelID)
	}

	tenant.ChatHistoryConfig = cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update chat history config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.ChatHistoryConfig,
		"message": "Chat history configuration updated successfully",
	})
}

// GetTenantRetrievalConfig returns the tenant's global retrieval configuration.
func (h *TenantHandler) GetTenantRetrievalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.RetrievalConfig
	if data == nil {
		data = &types.RetrievalConfig{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantRetrievalConfigInternal updates the tenant's global retrieval configuration.
func (h *TenantHandler) updateTenantRetrievalConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	var cfg types.RetrievalConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	// Validate thresholds
	if cfg.VectorThreshold < 0 || cfg.VectorThreshold > 1 {
		c.Error(errors.NewBadRequestError("vector_threshold must be between 0 and 1"))
		return
	}
	if cfg.KeywordThreshold < 0 || cfg.KeywordThreshold > 1 {
		c.Error(errors.NewBadRequestError("keyword_threshold must be between 0 and 1"))
		return
	}
	if cfg.RerankThreshold < -10 || cfg.RerankThreshold > 10 {
		c.Error(errors.NewBadRequestError("rerank_threshold must be between -10 and 10"))
		return
	}
	if cfg.EmbeddingTopK < 0 || cfg.EmbeddingTopK > 200 {
		c.Error(errors.NewBadRequestError("embedding_top_k must be between 0 and 200"))
		return
	}
	if cfg.RerankTopK < 0 || cfg.RerankTopK > 200 {
		c.Error(errors.NewBadRequestError("rerank_top_k must be between 0 and 200"))
		return
	}

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	tenant.RetrievalConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update retrieval config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.RetrievalConfig,
		"message": "Retrieval configuration updated successfully",
	})
}

// GetTenantMemoryConfig returns the workspace long-term memory configuration.
func (h *TenantHandler) GetTenantMemoryConfig(c *gin.Context) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}
	data := tenant.MemoryConfig
	if data == nil {
		// Memory is off until an admin turns it on: the feature retains what
		// users say across sessions, so it must not arrive enabled by default.
		data = &types.MemoryConfig{}
	}
	data.Normalize()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// updateTenantMemoryConfigInternal updates the workspace memory configuration.
func (h *TenantHandler) updateTenantMemoryConfigInternal(c *gin.Context) {
	ctx := c.Request.Context()

	var cfg types.MemoryConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if cfg.WriteMode != "" &&
		cfg.WriteMode != types.MemoryWriteExplicitOnly &&
		cfg.WriteMode != types.MemoryWriteAuto {
		c.Error(errors.NewBadRequestError("write_mode must be explicit_only or auto"))
		return
	}
	if cfg.MaxItems < 0 || cfg.MaxItems > 2000 {
		c.Error(errors.NewBadRequestError("max_items must be between 0 and 2000"))
		return
	}
	if cfg.ExtractDelaySeconds < 0 || cfg.ExtractDelaySeconds > types.MaxMemoryExtractDelaySeconds {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"extract_delay_seconds must be between 0 and %d", types.MaxMemoryExtractDelaySeconds)))
		return
	}
	if cfg.ExtractMinIntervalSeconds < 0 ||
		cfg.ExtractMinIntervalSeconds > types.MaxMemoryExtractMinIntervalSeconds {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"extract_min_interval_seconds must be between 0 and %d",
			types.MaxMemoryExtractMinIntervalSeconds)))
		return
	}
	if len(cfg.EmbeddingModelID) > 64 {
		c.Error(errors.NewBadRequestError("embedding_model_id is too long"))
		return
	}
	if cfg.InterestThreshold < 0 || cfg.InterestThreshold > types.MaxMemoryInterestThreshold {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"interest_threshold must be between 1 and %d", types.MaxMemoryInterestThreshold)))
		return
	}
	if len([]rune(cfg.ExtractInstructions)) > types.MaxMemoryExtractInstructionsRunes {
		c.Error(errors.NewBadRequestError(fmt.Sprintf(
			"extract_instructions must be at most %d characters",
			types.MaxMemoryExtractInstructionsRunes)))
		return
	}
	cfg.Normalize()

	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Workspace is empty")
		c.Error(errors.NewBadRequestError("Workspace is empty"))
		return
	}

	tenant.MemoryConfig = &cfg
	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
		} else {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError("Failed to update memory config").WithDetails(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.MemoryConfig,
		"message": "Memory configuration updated successfully",
	})
}

func validateParserEngineOutboundURLs(cfg *types.ParserEngineConfig) error {
	if cfg == nil {
		return nil
	}
	if endpoint := strings.TrimSpace(cfg.MinerUEndpoint); endpoint != "" {
		if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
			return fmt.Errorf("mineru_endpoint failed SSRF validation: %v", err)
		}
	}
	if vlmURL := strings.TrimSpace(cfg.MinerUVLMServerURL); vlmURL != "" {
		if err := secutils.ValidateURLForSSRF(vlmURL); err != nil {
			return fmt.Errorf("mineru_vlm_server_url failed SSRF validation: %v", err)
		}
	}
	if odlURL := strings.TrimSpace(cfg.ODLHybridURL); odlURL != "" {
		if err := secutils.ValidateURLForSSRF(odlURL); err != nil {
			return fmt.Errorf("odl_hybrid_url failed SSRF validation: %v", err)
		}
	}
	if endpoint := strings.TrimSpace(cfg.PaddleOCRVLEndpoint); endpoint != "" {
		if err := secutils.ValidateURLForSSRF(endpoint); err != nil {
			return fmt.Errorf("paddleocr_vl_endpoint failed SSRF validation: %v", err)
		}
	}
	return nil
}
