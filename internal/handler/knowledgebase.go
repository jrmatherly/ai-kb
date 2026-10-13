package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jrmatherly/ai-kb/internal/application/access"
	"github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/errors"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/middleware"
	"github.com/jrmatherly/ai-kb/internal/storageurl"
	"github.com/jrmatherly/ai-kb/internal/tracing/langfuse"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/jrmatherly/ai-kb/internal/utils"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

// KnowledgeBaseHandler defines the HTTP handler for knowledge base operations
type KnowledgeBaseHandler struct {
	cfg                *config.Config
	service            interfaces.KnowledgeBaseService
	profileService     interfaces.KnowledgeBaseProfileService
	knowledgeService   interfaces.KnowledgeService
	kbShareService     interfaces.KBShareService
	agentShareService  interfaces.AgentShareService
	asynqClient        interfaces.TaskEnqueuer
	vectorStoreService interfaces.VectorStoreService // enriches KB responses with bound store display
	// userService only backfills creator_name in list endpoints.
	// Authorization is done by the RBAC middleware and Lookup, not here.
	userService interfaces.UserService
	// fileService and storageResolver back the optional `resource_urls=public`
	// mode on hybrid-search. Both may be nil in tests, in which case only the
	// default handle mode is available.
	fileService     interfaces.FileService
	storageResolver interfaces.StorageBackendResolver
}

// NewKnowledgeBaseHandler creates a new knowledge base handler instance
func NewKnowledgeBaseHandler(
	cfg *config.Config,
	service interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	kbShareService interfaces.KBShareService,
	agentShareService interfaces.AgentShareService,
	asynqClient interfaces.TaskEnqueuer,
	vectorStoreService interfaces.VectorStoreService,
	userService interfaces.UserService,
	fileService interfaces.FileService,
	storageResolver interfaces.StorageBackendResolver,
	profileService interfaces.KnowledgeBaseProfileService,
) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{
		cfg:                cfg,
		profileService:     profileService,
		service:            service,
		knowledgeService:   knowledgeService,
		kbShareService:     kbShareService,
		agentShareService:  agentShareService,
		asynqClient:        asynqClient,
		vectorStoreService: vectorStoreService,
		userService:        userService,
		fileService:        fileService,
		storageResolver:    storageResolver,
	}
}

// resolveResourceRewriter builds the storage-reference rewriter for one response
// from the request's `resource_urls` parameter, falling back to the deployment
// default. The returned error is already an AppError the caller can hand to
// c.Error: a rejected scope is a 403, a typo in the parameter is a 400.
func (h *KnowledgeBaseHandler) resolveResourceRewriter(c *gin.Context) (*storageurl.Rewriter, error) {
	ctx := c.Request.Context()
	mode, err := storageurl.ResolveMode(ctx, c.Query(storageurl.QueryParam))
	if err != nil {
		if stderrors.Is(err, storageurl.ErrPublicModeForbidden) {
			return nil, apperrors.NewForbiddenError(err.Error())
		}
		return nil, apperrors.NewBadRequestError(err.Error())
	}
	return storageurl.NewRequestRewriter(ctx, mode, h.fileService, h.storageResolver), nil
}

// buildKBResponse turns a knowledge base into a JSON-ready response shape,
// merging the bound vector store's display metadata and any caller-supplied
// extras (e.g., my_permission for shared KBs). Returns the kb pointer
// unchanged on serialization failure so the request still succeeds.
//
// The map-merge approach (rather than a wrapper struct embedding the kb)
// is deliberate: KnowledgeBase has a custom MarshalJSON, and embedding
// would promote it to any wrapper struct and silently swallow the extra
// fields. The same pattern is already used by GetKnowledgeBase to add the
// my_permission field for shared knowledge bases.
//
// Shared-KB suppression: when storeView.Source == StoreSourceShared (the
// caller is not the KB owner), the raw vector_store_id UUID is stripped
// from the response so the owner-tenant's store inventory cannot be
// correlated across multiple shared KBs. Name / engine type / status are
// already empty in the SharedStoreDisplay payload; suppressing the UUID
// completes the cross-tenant metadata hiding.
func buildKBResponse(
	kb *types.KnowledgeBase,
	storeView types.StoreDisplay,
	extras map[string]interface{},
) interface{} {
	b, err := json.Marshal(kb)
	if err != nil {
		return kb
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return kb
	}
	if storeView.Source == types.StoreSourceShared {
		delete(m, "vector_store_id")
	}
	if storeView.Name != "" {
		m["vector_store_name"] = storeView.Name
	}
	if storeView.Source != "" {
		m["vector_store_source"] = storeView.Source
	}
	if storeView.EngineType != "" {
		m["vector_store_engine_type"] = storeView.EngineType
	}
	if storeView.Status != "" {
		m["vector_store_status"] = storeView.Status
	}
	for k, v := range extras {
		m[k] = v
	}
	return m
}

// buildKBListResponse turns a slice of knowledge bases into a JSON-ready
// slice that mirrors the single-KB enrichment in buildKBResponse. Store
// views are batch-resolved once via BatchResolveStoreView to keep the
// list endpoint O(1) in vector-store service calls - the per-KB
// resolveKBStoreView would otherwise be N+1.
//
// Caller-vs-owner semantics match the single-KB path:
//   - KB has no binding         → DefaultStoreDisplay()
//   - KB is owned by another tenant (cross-tenant shared) → SharedStoreDisplay()
//   - KB is own-tenant bound    → look up in the batch result; misses
//     fall back to UnavailableStoreDisplay()
//
// Resolver failures degrade gracefully: every own-tenant bound KB renders
// as unavailable instead of breaking the list response.
func (h *KnowledgeBaseHandler) buildKBListResponse(
	ctx context.Context, kbs []*types.KnowledgeBase, callerTenantID uint64,
) []interface{} {
	defaultView := h.envDefaultStoreView(ctx)
	storeViews := h.batchResolveKBStoreViews(ctx, kbs, callerTenantID)
	out := make([]interface{}, 0, len(kbs))
	for _, kb := range kbs {
		var view types.StoreDisplay
		switch {
		case !kb.HasVectorStore():
			view = defaultView
		case kb.TenantID != callerTenantID:
			view = types.SharedStoreDisplay()
		default:
			v, ok := storeViews[*kb.VectorStoreID]
			if !ok || v.Source == "" {
				view = types.UnavailableStoreDisplay()
			} else {
				view = v
			}
		}
		out = append(out, buildKBResponse(kb, view, nil))
	}
	return out
}

// sharedKBRow projects a SharedKnowledgeBaseInfo into a response payload
// that respects the cross-tenant strip rule: the embedded KnowledgeBase
// row runs through buildKBResponse with SharedStoreDisplay() so its
// vector_store_id and any owner-tenant store metadata never reach the
// wire. The share-record fields (share_id, organization_id, etc.) are
// kept intact alongside the stripped KB. Callers can pass extras to
// merge view-specific keys such as is_mine or source_from_agent.
//
// Always uses SharedStoreDisplay() regardless of whether the caller is
// the owner; the cross-tenant share endpoints serve mixed audiences and
// the owner's "rich" view of their own bindings is already served by
// ListKnowledgeBases / single-KB GET on the standard knowledge-base
// routes. Trying to enrich own-row entries here would either require
// threading the vector-store service through the organization handler
// or duplicating the lookup logic - both larger than the security fix
// warrants and easy to follow up on once needed.
func sharedKBRow(
	info *types.SharedKnowledgeBaseInfo, extras map[string]interface{},
) map[string]interface{} {
	kbView := buildKBResponse(info.KnowledgeBase, types.SharedStoreDisplay(), nil)
	row := map[string]interface{}{
		"knowledge_base":   kbView,
		"share_id":         info.ShareID,
		"organization_id":  info.OrganizationID,
		"org_name":         info.OrgName,
		"permission":       info.Permission,
		"source_tenant_id": info.SourceTenantID,
		"shared_at":        info.SharedAt,
	}
	for k, v := range extras {
		row[k] = v
	}
	return row
}

// envDefaultStoreView returns the env-fallback store display enriched with
// the configured env-store engine type when the service is available. The
// service path populates EngineType so the caller can show "postgres" or
// "qdrant" on the env-default badge instead of leaving it blank. A nil
// service (e.g. in narrow unit-test setups) falls back to the bare default
// display rather than failing the list response.
func (h *KnowledgeBaseHandler) envDefaultStoreView(ctx context.Context) types.StoreDisplay {
	if h.vectorStoreService == nil {
		return types.DefaultStoreDisplay()
	}
	return h.vectorStoreService.EnvDefaultStoreView(ctx)
}

// batchResolveKBStoreViews collects the unique own-tenant store IDs across
// the KB slice and resolves them in one BatchResolveStoreView call.
// Cross-tenant shared KBs never enter the batch - they always render
// via SharedStoreDisplay, which deliberately suppresses the owner
// tenant's store name and engine type so cross-tenant viewers cannot
// correlate the owner's store inventory from KB responses alone.
func (h *KnowledgeBaseHandler) batchResolveKBStoreViews(
	ctx context.Context, kbs []*types.KnowledgeBase, callerTenantID uint64,
) map[string]types.StoreDisplay {
	if h.vectorStoreService == nil {
		return nil
	}
	storeIDs := make([]string, 0, len(kbs))
	seen := make(map[string]bool, len(kbs))
	for _, kb := range kbs {
		if !kb.HasVectorStore() || kb.TenantID != callerTenantID {
			continue
		}
		sid := *kb.VectorStoreID
		if !seen[sid] {
			seen[sid] = true
			storeIDs = append(storeIDs, sid)
		}
	}
	if len(storeIDs) == 0 {
		return nil
	}
	views, err := h.vectorStoreService.BatchResolveStoreView(ctx, callerTenantID, storeIDs)
	if err != nil {
		logger.WarnWithFields(ctx, logger.Fields{
			"tenant_id":   callerTenantID,
			"store_count": len(storeIDs),
		}, "[kb.list] batch store view resolve failed; rendering bound KBs as unavailable")
		return nil
	}
	return views
}

// resolveKBStoreView returns the store display payload to embed in the KB
// response. It applies two policies on top of the service-level resolver:
//
//   - When the KB does not have a DB-managed vector store binding,
//     the env-fallback display is returned without touching the service.
//   - When the caller is not the KB owner (shared access), the underlying
//     store's name and engine are suppressed so operator-chosen names do
//     not leak across tenants. The Source value is set to "shared".
//
// On resolution error, an unavailable display is returned and the failure
// is logged for ops; the request itself still succeeds.
func (h *KnowledgeBaseHandler) resolveKBStoreView(
	ctx context.Context, kb *types.KnowledgeBase, callerTenantID uint64,
) types.StoreDisplay {
	if !kb.HasVectorStore() {
		return h.envDefaultStoreView(ctx)
	}
	if kb.TenantID != callerTenantID {
		return types.SharedStoreDisplay()
	}
	if h.vectorStoreService == nil {
		return types.UnavailableStoreDisplay()
	}
	view, err := h.vectorStoreService.ResolveStoreView(ctx, kb.TenantID, *kb.VectorStoreID)
	if err != nil {
		logger.WarnWithFields(ctx, logger.Fields{
			"kb_id":     secutils.SanitizeForLog(kb.ID),
			"tenant_id": kb.TenantID,
		}, "[kb.view] vector store resolve failed; returning unavailable")
		return types.UnavailableStoreDisplay()
	}
	return view
}

// HybridSearch godoc
// @Summary      Hybrid search
// @Description  Low-level recall: hybrid vector and keyword retrieval, without rerank by default (enable it with the
// @Description  rerank field). For general search use /knowledge-search. Prefer POST; GET with a JSON body exists only
// @Description  for old clients.
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        id             path      string             true   "Knowledge base ID"
// @Param        request        body      types.SearchParams true   "Search parameters"
// @Param resource_urls query string false "public returns direct file links" Enums(handle, public) default(handle)
// @Success      200            {object}  map[string]interface{}  "Search results"
// @Failure      400            {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/hybrid-search [post]
// @Router       /knowledge-bases/{id}/hybrid-search [get]
func (h *KnowledgeBaseHandler) HybridSearch(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start hybrid search")

	// Validate and check permission for knowledge base access
	_, id, effectiveTenantID, _, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Parse request body
	var req types.SearchParams
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	precomputedVectorOnly := len(req.QueryEmbedding) > 0 && req.DisableKeywordsMatch && !req.DisableVectorMatch
	if strings.TrimSpace(req.QueryText) == "" && !precomputedVectorOnly {
		_ = c.Error(apperrors.NewBadRequestError("query_text is required"))
		return
	}
	if err := req.Rerank.Validate(); err != nil {
		_ = c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	if req.Rerank.IsEnabled() && strings.TrimSpace(req.QueryText) == "" {
		// The rerank model scores passages against the query text.
		_ = c.Error(apperrors.NewBadRequestError("query_text is required when rerank is enabled"))
		return
	}

	logger.Infof(ctx, "Executing hybrid search, knowledge base ID: %s, query: %s, effectiveTenantID: %d",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(req.QueryText), effectiveTenantID)

	// Resolve before retrieving so a typo or a rejected scope costs nothing.
	rewriter, err := h.resolveResourceRewriter(c)
	if err != nil {
		logger.Warnf(ctx, "Rejected resource URL mode: %v", err)
		_ = c.Error(err)
		return
	}

	// Execute hybrid search. Without a rerank object this is the raw recall
	// primitive it has always been; with one, the response carries meta.
	// Note: For shared KBs, the service uses effectiveTenantID internally via context
	var retrieval *types.RetrievalResult
	if req.Rerank != nil {
		retrieval, err = h.service.HybridSearchWithRerank(c.Request.Context(), id, req)
	} else {
		var results []*types.SearchResult
		results, err = h.service.HybridSearch(c.Request.Context(), id, req)
		retrieval = &types.RetrievalResult{Results: results}
	}
	if err != nil {
		// Service-layer typed AppErrors (e.g. ErrVectorStoreBindingInvalid,
		// ErrVectorStoreUnavailable, BadRequest from multi-store fan-out)
		// must reach the client with their original code rather than be
		// downgraded to InternalServerError. Mirrors the pattern used in
		// CreateKnowledgeBase.
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Hybrid search completed, knowledge base ID: %s, result count: %d",
		secutils.SanitizeForLog(id), len(retrieval.Results))
	response := gin.H{
		"success": true,
		"data":    rewriter.CopyReferences(ctx, retrieval.Results),
	}
	if retrieval.Meta.Rerank != nil {
		response["meta"] = retrieval.Meta
	}
	c.JSON(http.StatusOK, response)
}

// CreateKnowledgeBase godoc
// @Summary      Create a knowledge base
// @Description  Create a new knowledge base
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        request  body      types.KnowledgeBase  true  "Knowledge base info"
// @Success      201      {object}  map[string]interface{}  "Created knowledge base"
// @Failure      400      {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases [post]
func (h *KnowledgeBaseHandler) CreateKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start creating knowledge base")

	// Parse request body
	var req types.KnowledgeBase
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	if err := validateExtractConfig(req.ExtractConfig); err != nil {
		logger.Error(ctx, "Invalid extract configuration", err)
		c.Error(err)
		return
	}
	types.NormalizeKnowledgeBasePromptInstructions(&req)
	if err := validateKnowledgeBasePromptInstructions(&req); err != nil {
		c.Error(err)
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.GetStorageProvider()))
	if provider != "" && !isStorageProviderAllowed(provider) {
		c.Error(apperrors.NewBadRequestError("Storage provider is not allowed by STORAGE_ALLOW_LIST"))
		return
	}

	logger.Infof(ctx, "Creating knowledge base, name: %s", secutils.SanitizeForLog(req.Name))
	// Create knowledge base using the service
	kb, err := h.service.CreateKnowledgeBase(ctx, &req)
	if err != nil {
		// Surface typed AppErrors (notably the 400-class codes
		// ErrVectorStoreBindingInvalid and ErrVectorStoreUnavailable
		// returned by validateVectorStoreBinding) instead of wrapping them
		// into a generic 500. The middleware renders the original code and
		// HTTP status verbatim. Falls through to 500 only for raw infra
		// errors that the service did not classify.
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base created successfully, ID: %s, name: %s",
		secutils.SanitizeForLog(kb.ID), secutils.SanitizeForLog(kb.Name))
	callerTenantID := c.GetUint64(types.TenantIDContextKey.String())
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    buildKBResponse(kb, h.resolveKBStoreView(ctx, kb, callerTenantID), nil),
	})
}

// validateAndGetKnowledgeBase validates request parameters and retrieves the knowledge base.
// Enforces per-API-key KB scope before tenant/share/agent resolution.
// Returns the knowledge base, knowledge base ID, effective tenant ID for embedding, permission level, and any errors encountered
// For owned KBs, effectiveTenantID is the caller's tenant ID
// For shared KBs, effectiveTenantID is the source tenant ID (owner's tenant)
func (h *KnowledgeBaseHandler) validateAndGetKnowledgeBase(
	c *gin.Context,
) (*types.KnowledgeBase, string, uint64, types.OrgMemberRole, error) {
	id := secutils.SanitizeForLog(c.Param("id"))
	grant, err := resolveHandlerKBAccess(c, id, h.service, h.kbShareService, h.agentShareService)
	if err != nil {
		return nil, id, 0, "", err
	}
	return grant.KnowledgeBase, id, grant.EffectiveTenantID, grant.Permission, nil
}

// GetKnowledgeBase godoc
// @Summary      Get knowledge base details
// @Description  Get a knowledge base by ID. When using a shared agent, pass agent_id to check that the agent may access
// @Description  this knowledge base.
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        id         path      string  true   "Knowledge base ID"
// @Param agent_id query string false "Shared agent ID (checks that the agent may access this knowledge base)"
// @Success      200  {object}  map[string]interface{}  "Knowledge base details"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Failure      404  {object}  errors.ErrorResponse         "Knowledge base not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id} [get]
func (h *KnowledgeBaseHandler) GetKnowledgeBase(c *gin.Context) {
	// Validate and get the knowledge base
	kb, _, _, permission, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}
	// Fill counts (knowledge_count, chunk_count, is_processing) so hover/detail shows correct numbers
	if fillErr := h.service.FillKnowledgeBaseCounts(c.Request.Context(), kb); fillErr != nil {
		logger.Warnf(c.Request.Context(), "Failed to fill KB counts for %s: %v", kb.ID, fillErr)
	}
	tenantID := c.GetUint64(types.TenantIDContextKey.String())
	storeView := h.resolveKBStoreView(c.Request.Context(), kb, tenantID)
	var extras map[string]interface{}
	if kb.TenantID != tenantID && permission != "" {
		// Include my_permission in data so frontend can show role (e.g. "Read-only") instead of "--" for agent-visible
		// KBs
		extras = map[string]interface{}{"my_permission": permission}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": buildKBResponse(kb, storeView, extras)})
}

// ListKnowledgeBases godoc
// @Summary      List knowledge bases
// @Description  List all knowledge bases in the current workspace; with agent_id (a shared agent), check permission and
// @Description  return the knowledge bases configured for that agent (used for @ mentions)
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param agent_id query string false "Shared agent ID (returns the knowledge bases available to that agent)"
// @Success      200  {object}  map[string]interface{}  "Knowledge base list"
// @Failure      500  {object}  errors.ErrorResponse         "Internal server error"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases [get]
func (h *KnowledgeBaseHandler) ListKnowledgeBases(c *gin.Context) {
	ctx := c.Request.Context()

	agentID := c.Query("agent_id")
	if agentID != "" {
		agent, err := resolveSharedAgentForRequest(c, agentID, h.agentShareService)
		if err != nil {
			_ = c.Error(err)
			return
		}
		currentTenantID := middleware.KBAccessRequest(c).Caller.TenantID
		scope := types.NewSharedAgentKBScope(agent)
		if scope.IsEmpty() {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []interface{}{}})
			return
		}
		kbs, err := h.service.ListKnowledgeBasesByTenantID(ctx, agent.TenantID)
		if err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(apperrors.NewInternalServerError(err.Error()))
			return
		}
		kbs = filterKnowledgeBasesForSharedAgent(kbs, agent)
		kbs = filterKnowledgeBasesForAPIKeyScope(ctx, kbs)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    h.buildKBListResponse(ctx, kbs, currentTenantID),
		})
		return
	}

	// Get all knowledge bases for this tenant
	kbs, err := h.service.ListKnowledgeBases(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	// Optional creator filter - drives the [All | Mine | Others] segmented
	// control on the list page. We filter in-process rather than pushing
	// down into SQL because the tenant-bounded KB list is small (typically
	// <100 rows) and adding a creator predicate to ListKnowledgeBases would
	// ripple through every other caller (chat pipeline, agent editor, …).
	// Rows with empty CreatorID predate the RBAC migration (PR 5); we treat
	// them as "not anyone in particular" so they never appear under "mine"
	// or "others" - they fall out of both filters cleanly.
	creatorFilter := strings.ToLower(strings.TrimSpace(c.Query("creator")))
	if creatorFilter == "mine" || creatorFilter == "others" {
		callerUserID, _ := c.Get(types.UserIDContextKey.String())
		callerUserIDStr, _ := callerUserID.(string)
		filtered := make([]*types.KnowledgeBase, 0, len(kbs))
		for _, kb := range kbs {
			if kb.CreatorID == "" {
				continue
			}
			if creatorFilter == "mine" && kb.CreatorID == callerUserIDStr {
				filtered = append(filtered, kb)
			} else if creatorFilter == "others" && kb.CreatorID != callerUserIDStr {
				filtered = append(filtered, kb)
			}
		}
		kbs = filtered
	}
	kbs = filterKnowledgeBasesForAPIKeyScope(ctx, kbs)

	// Get share counts for all knowledge bases
	if len(kbs) > 0 && h.kbShareService != nil {
		kbIDs := make([]string, len(kbs))
		for i, kb := range kbs {
			kbIDs[i] = kb.ID
		}

		shareCounts, err := h.kbShareService.CountSharesByKnowledgeBaseIDs(ctx, kbIDs)
		if err != nil {
			logger.Warnf(ctx, "Failed to get share counts: %v", err)
		} else {
			for _, kb := range kbs {
				if count, ok := shareCounts[kb.ID]; ok {
					kb.ShareCount = count
				}
			}
		}
	}

	// Backfill creator_name so the list can tell "created by me" from "created by another workspace member".
	// Only the list endpoint needs it; detail and edit views do not. When resolution fails (deleted user,
	// old rows with an empty CreatorID) the field stays empty and the frontend renders a fallback.
	enrichKBCreatorNames(ctx, h.userService, kbs)

	callerTenantID := c.GetUint64(types.TenantIDContextKey.String())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    h.buildKBListResponse(ctx, kbs, callerTenantID),
	})
}

func filterKnowledgeBasesForAPIKeyScope(ctx context.Context, kbs []*types.KnowledgeBase) []*types.KnowledgeBase {
	scope, ok := types.TenantAPIKeyScopeFromContext(ctx)
	if !ok || len(scope.KnowledgeBaseIDs) == 0 {
		return kbs
	}
	filtered := make([]*types.KnowledgeBase, 0, len(kbs))
	for _, kb := range kbs {
		if kb != nil && scope.AllowsKnowledgeBase(kb.ID) {
			filtered = append(filtered, kb)
		}
	}
	return filtered
}

// enrichKBCreatorNames resolves the CreatorID of each KB to a display name (username
// first, then email). Every error is swallowed: a missing creator_name only affects
// the badge on the card, and must not break the list.
func enrichKBCreatorNames(ctx context.Context, userSvc interfaces.UserService, kbs []*types.KnowledgeBase) {
	if userSvc == nil || len(kbs) == 0 {
		return
	}
	idSet := make(map[string]struct{}, len(kbs))
	for _, kb := range kbs {
		if kb.CreatorID != "" {
			idSet[kb.CreatorID] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	users, err := userSvc.GetUsersByIDs(ctx, ids)
	if err != nil {
		logger.Warnf(ctx, "Failed to resolve KB creator names: %v", err)
		return
	}
	for _, kb := range kbs {
		if kb.CreatorID == "" {
			continue
		}
		u, ok := users[kb.CreatorID]
		if !ok || u == nil {
			continue
		}
		kb.CreatorName = pickUserDisplayName(u)
	}
}

// pickUserDisplayName picks the field most users will recognise: Username
// if present (it's required at registration), Email as a fallback. Used by
// both KB and Agent list enrichment so the badge text stays consistent.
func pickUserDisplayName(u *types.User) string {
	if u == nil {
		return ""
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}

// TogglePinKnowledgeBase godoc
// @Summary      Pin or unpin a knowledge base
// @Description  Toggle the pinned state of a knowledge base
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        id  path      string  true  "Knowledge base ID"
// @Success      200  {object}  map[string]interface{}  "Updated knowledge base"
// @Failure      404  {object}  errors.ErrorResponse         "Knowledge base not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/pin [put]
func (h *KnowledgeBaseHandler) TogglePinKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewBadRequestError("knowledge base ID is required"))
		return
	}

	kb, err := h.service.TogglePinKnowledgeBase(ctx, id)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(apperrors.NewNotFoundError("knowledge base not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	callerTenantID := c.GetUint64(types.TenantIDContextKey.String())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    buildKBResponse(kb, h.resolveKBStoreView(ctx, kb, callerTenantID), nil),
	})
}

// UpdateKnowledgeBaseRequest defines the request body structure for updating a knowledge base
type UpdateKnowledgeBaseRequest struct {
	Name        string                     `json:"name"        binding:"required"`
	Description string                     `json:"description"`
	Config      *types.KnowledgeBaseConfig `json:"config"`
	// VLMConfig updates the knowledge base's multimodal (vision) config.
	// Optional: nil means "no change", mirroring how CreateKnowledgeBase
	// accepts a top-level vlm_config.
	VLMConfig *types.VLMConfig `json:"vlm_config"`
}

// UpdateKnowledgeBase godoc
// @Summary      Update a knowledge base
// @Description  Update the name, description and config of a knowledge base
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        id       path      string                     true  "Knowledge base ID"
// @Param        request  body      UpdateKnowledgeBaseRequest true  "Update request"
// @Success      200      {object}  map[string]interface{}     "Updated knowledge base"
// @Failure      400      {object}  errors.ErrorResponse            "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id} [put]
func (h *KnowledgeBaseHandler) UpdateKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start updating knowledge base")

	// Validate and get the knowledge base
	currentKB, id, _, permission, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Only admin/editor can update knowledge base
	if permission != types.OrgRoleAdmin && permission != types.OrgRoleEditor {
		c.Error(apperrors.NewForbiddenError("No permission to update knowledge base"))
		return
	}

	// Parse request body
	var req UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	// The VLM model is a KB setting: like PUT /initialization/config, only the
	// owner workspace or an admin share may change it, not a share editor.
	if req.VLMConfig != nil {
		if _, err := kbSettingsAccess(c, currentKB); err != nil {
			_ = c.Error(err)
			return
		}
	}
	if req.Config != nil {
		probe := &types.KnowledgeBase{
			ChunkingConfig: req.Config.ChunkingConfig,
			WikiConfig:     req.Config.WikiConfig,
			ProfileConfig:  req.Config.ProfileConfig,
		}
		if req.Config.ImageProcessingConfig != nil {
			probe.ImageProcessingConfig = *req.Config.ImageProcessingConfig
		}
		if err := validateKnowledgeBasePromptInstructions(probe); err != nil {
			c.Error(err)
			return
		}
	}

	logger.Infof(ctx, "Updating knowledge base, ID: %s, name: %s",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(req.Name))

	// Update the knowledge base
	kb, err := h.service.UpdateKnowledgeBase(ctx, id, req.Name, req.Description, req.Config, req.VLMConfig)
	if err != nil {
		// A rejected vlm_config is the caller's mistake, not a server failure.
		if appErr, ok := apperrors.IsAppError(err); ok {
			_ = c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base updated successfully, ID: %s",
		secutils.SanitizeForLog(id))
	callerTenantID := c.GetUint64(types.TenantIDContextKey.String())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    buildKBResponse(kb, h.resolveKBStoreView(ctx, kb, callerTenantID), nil),
	})
}

// GenerateKnowledgeBaseProfile godoc
// @Summary      Generate a knowledge base description
// @Description  Regenerate the AI description of a knowledge base now, from aggregated document profiles (a
// @Description  hand-written description is not overwritten)
// @Tags         Knowledge Bases
// @Produce      json
// @Param        id   path      string  true  "Knowledge base ID"
// @Success      200  {object}  map[string]interface{}  "Generated knowledge base profile"
// @Failure      400  {object}  errors.ErrorResponse         "Knowledge base type not supported or no model configured"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/profile/generate [post]
func (h *KnowledgeBaseHandler) GenerateKnowledgeBaseProfile(c *gin.Context) {
	ctx := c.Request.Context()
	kb, id, _, permission, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if permission != types.OrgRoleAdmin && permission != types.OrgRoleEditor {
		_ = c.Error(apperrors.NewForbiddenError("No permission to update knowledge base"))
		return
	}
	if h.profileService == nil {
		_ = c.Error(apperrors.NewInternalServerError("knowledge base profile service unavailable"))
		return
	}
	genCtx, cancel := context.WithTimeout(ctx, knowledgeBaseProfileRequestTimeout)
	defer cancel()
	profile, err := h.profileService.GenerateKnowledgeBaseProfile(genCtx, kb, true)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"knowledge_base_id": id})
		switch {
		case stderrors.Is(err, types.ErrKnowledgeBaseProfileUnsupported),
			stderrors.Is(err, types.ErrKnowledgeBaseProfileModelNotConfigured):
			_ = c.Error(apperrors.NewBadRequestError(err.Error()))
		default:
			_ = c.Error(apperrors.NewInternalServerError(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profile})
}

// knowledgeBaseProfileRequestTimeout bounds the synchronous regeneration a
// user triggers from the settings dialog: one aggregation plus one small
// model call.
const knowledgeBaseProfileRequestTimeout = 2 * time.Minute

// DeleteKnowledgeBase godoc
// @Summary      Delete a knowledge base
// @Description  Delete a knowledge base and all its contents
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Knowledge base ID"
// @Success      200  {object}  map[string]interface{}  "Deleted"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id} [delete]
func (h *KnowledgeBaseHandler) DeleteKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start deleting knowledge base")

	// Validate and get the knowledge base
	kb, id, _, permission, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Only owner (admin with matching tenant) can delete knowledge base
	tenantID, _ := c.Get(types.TenantIDContextKey.String())
	if kb.TenantID != tenantID.(uint64) || permission != types.OrgRoleAdmin {
		c.Error(apperrors.NewForbiddenError("Only knowledge base owner can delete"))
		return
	}

	logger.Infof(ctx, "Deleting knowledge base, ID: %s, name: %s",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(kb.Name))

	// Delete the knowledge base
	if err := h.service.DeleteKnowledgeBase(ctx, id); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base deleted successfully, ID: %s",
		secutils.SanitizeForLog(id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge base deleted successfully",
	})
}

type CopyKnowledgeBaseRequest struct {
	TaskID   string `json:"task_id"`
	SourceID string `json:"source_id" binding:"required"`
	TargetID string `json:"target_id"`
}

// CopyKnowledgeBaseResponse defines the response for copy knowledge base
type CopyKnowledgeBaseResponse struct {
	TaskID   string `json:"task_id"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Message  string `json:"message"`
}

type DuplicateKnowledgeBaseResponse struct {
	SourceID      string      `json:"source_id"`
	TargetID      string      `json:"target_id"`
	Message       string      `json:"message"`
	KnowledgeBase interface{} `json:"knowledge_base"`
}

// CopyKnowledgeBase godoc
// @Summary      Copy a knowledge base
// @Description  Copy the contents of one knowledge base into another (async task)
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        request  body      CopyKnowledgeBaseRequest   true  "Copy request"
// @Success      200      {object}  map[string]interface{}     "Task ID"
// @Failure      400      {object}  errors.ErrorResponse            "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/copy [post]
func (h *KnowledgeBaseHandler) CopyKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	var req CopyKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	caller := types.CallerFromContext(ctx)
	if caller.TenantID == 0 {
		_ = c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}
	tenantID := caller.TenantID
	sourceGrant, err := resolveHandlerKBAccessFor(c, req.SourceID, h.service, nil, nil, types.OrgRoleViewer)
	if err != nil {
		c.Error(err)
		return
	}
	sourceKB := sourceGrant.KnowledgeBase
	if sourceKB.TenantID != caller.TenantID {
		_ = c.Error(errors.NewForbiddenError("No permission to copy this knowledge base"))
		return
	}
	taskID := req.TaskID
	if taskID == "" {
		taskID = utils.GenerateTaskID("kb_clone", caller.TenantID, req.SourceID)
	} else if err := requireTaskProgressTenant(ctx, taskID); err != nil {
		_ = c.Error(err)
		return
	}
	create := req.TargetID == ""
	targetKB := &types.KnowledgeBase{ID: uuid.NewString(), TenantID: caller.TenantID}
	if create {
		if !types.IsSyntheticUserID(caller.UserID) {
			targetKB.CreatorID = caller.UserID
		}
	} else {
		targetGrant, err := resolveHandlerKBAccessFor(c, req.TargetID, h.service, nil, nil, types.OrgRoleEditor)
		if err != nil {
			_ = c.Error(err)
			return
		}
		targetKB = targetGrant.KnowledgeBase
		if targetKB.TenantID != caller.TenantID {
			c.Error(errors.NewForbiddenError("No permission to copy to this knowledge base"))
			return
		}
		if err := middleware.EvaluateOwnershipOrRole(c.Request.Context(),
			h.cfg,
			types.TenantRoleAdmin,
			func() (string,
				error,
			) {
				return targetKB.CreatorID,
					nil
			}); err != nil {
			_ = c.Error(errors.NewForbiddenError("No permission to replace this knowledge base's contents"))
			return
		}
		tenant, _ := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
		if err := access.ValidateKBTransferCompatibility(sourceKB,
			targetKB,
			access.KBTransferClone,
			"",
			tenant); err != nil {
			_ = c.Error(errors.NewBadRequestError(err.Error()))
			return
		}
	}
	ctx, err = access.WithKBTransfer(c.Request.Context(), sourceKB, targetKB, access.KBTransferClone, taskID, create)
	if err != nil {
		_ = c.Error(kbAccessHTTPError(err))
		return
	}
	// Reserve the destination in the payload; enqueue failures do not leave
	// empty KBs and every worker delivery creates/resumes the same destination.
	req.TargetID = targetKB.ID

	// Create KB clone payload
	payload := types.KBClonePayload{
		TenantID:     tenantID,
		TaskID:       taskID,
		SourceID:     req.SourceID,
		TargetID:     req.TargetID,
		CreateTarget: create,
		CreatorID:    targetKB.CreatorID,
		Initiator:    types.TaskInitiatorFromContext(ctx),
	}
	langfuse.InjectTracing(ctx, &payload)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Errorf(ctx, "Failed to marshal KB clone payload: %v", err)
		c.Error(apperrors.NewInternalServerError("Failed to create task"))
		return
	}

	// Enqueue KB clone task to Asynq
	task := asynq.NewTask(types.TypeKBClone, payloadBytes,
		asynq.TaskID(taskID), asynq.Queue(types.QueueMaintenance),
		asynq.MaxRetry(3), asynq.Timeout(2*time.Hour))
	info, err := h.asynqClient.Enqueue(task)
	if err != nil {
		logger.Errorf(ctx, "Failed to enqueue KB clone task: %v", err)
		c.Error(apperrors.NewInternalServerError("Failed to enqueue task"))
		return
	}

	logger.Infof(ctx, "KB clone task enqueued: %s, asynq task ID: %s, source: %s, target: %s",
		taskID, info.ID, secutils.SanitizeForLog(req.SourceID), secutils.SanitizeForLog(req.TargetID))

	// Save initial progress to Redis so frontend can query immediately
	initialProgress := &types.KBCloneProgress{
		TaskID:    taskID,
		SourceID:  req.SourceID,
		TargetID:  req.TargetID,
		Status:    types.KBCloneStatusPending,
		Progress:  0,
		Message:   "Task queued, waiting to start...",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	if err := h.knowledgeService.SaveKBCloneProgress(ctx, initialProgress); err != nil {
		logger.Warnf(ctx, "Failed to save initial KB clone progress: %v", err)
		// Don't fail the request, task is already enqueued
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": CopyKnowledgeBaseResponse{
			TaskID:   taskID,
			SourceID: req.SourceID,
			TargetID: req.TargetID,
			Message:  "Knowledge base copy task started",
		},
	})
}

// DuplicateKnowledgeBase godoc
// @Summary      Duplicate a knowledge base
// @Description  Create a new knowledge base with the same settings only; knowledge, FAQ content, chunks, indexes, Wiki
// @Description  pages, shares and pinned state are not copied
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        id       path      string                  true  "Source knowledge base ID"
// @Success      201      {object}  map[string]interface{}  "Created knowledge base copy"
// @Failure      400      {object}  errors.ErrorResponse                 "Invalid request parameters"
// @Security     Bearer
// @Router       /knowledge-bases/{id}/duplicate [post]
func (h *KnowledgeBaseHandler) DuplicateKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	sourceID := c.Param("id")
	if sourceID == "" {
		c.Error(apperrors.NewBadRequestError("Knowledge base ID cannot be empty"))
		return
	}

	callerTenantID := c.GetUint64(types.TenantIDContextKey.String())
	sourceKB, err := h.service.GetKnowledgeBaseByID(ctx, sourceID)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Source knowledge base not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if sourceKB.TenantID != callerTenantID {
		logger.Warnf(ctx,
			"Knowledge base duplicate rejected: source belongs to another tenant, source_id: %s, caller_tenant: %d, kb_tenant: %d",
			secutils.SanitizeForLog(sourceID), callerTenantID, sourceKB.TenantID)
		c.Error(errors.NewForbiddenError("No permission to duplicate this knowledge base"))
		return
	}

	targetKB, err := h.service.DuplicateKnowledgeBase(ctx, sourceID)
	if err != nil {
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Source knowledge base not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base duplicate created, source: %s, target: %s",
		secutils.SanitizeForLog(sourceID), secutils.SanitizeForLog(targetKB.ID))
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": DuplicateKnowledgeBaseResponse{
			SourceID:      sourceID,
			TargetID:      targetKB.ID,
			Message:       "Knowledge base duplicate created",
			KnowledgeBase: buildKBResponse(targetKB, h.resolveKBStoreView(ctx, targetKB, callerTenantID), nil),
		},
	})
}

// GetKBCloneProgress godoc
// @Summary      Get knowledge base copy progress
// @Description  Get the progress of a knowledge base copy task
// @Tags         Knowledge Bases
// @Accept       json
// @Produce      json
// @Param        task_id  path      string  true  "Task ID"
// @Success      200      {object}  map[string]interface{}  "Progress"
// @Failure      404      {object}  errors.ErrorResponse         "Task not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/copy/progress/{task_id} [get]
func (h *KnowledgeBaseHandler) GetKBCloneProgress(c *gin.Context) {
	ctx := c.Request.Context()

	taskID := c.Param("task_id")
	if taskID == "" {
		logger.Error(ctx, "Task ID is empty")
		c.Error(apperrors.NewBadRequestError("Task ID cannot be empty"))
		return
	}
	if err := requireTaskProgressTenant(ctx, taskID); err != nil {
		c.Error(err)
		return
	}

	progress, err := h.knowledgeService.GetKBCloneProgress(ctx, taskID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    progress,
	})
}

// validateExtractConfig validates the graph configuration parameters
func validateExtractConfig(config *types.ExtractConfig) error {
	if config == nil {
		return nil
	}
	if !config.Enabled {
		config.Enabled = false
		return nil
	}
	// Validate text field
	if config.Text == "" {
		return apperrors.NewBadRequestError("text cannot be empty")
	}

	// Validate tags field
	if len(config.Tags) == 0 {
		return apperrors.NewBadRequestError("tags cannot be empty")
	}
	for i, tag := range config.Tags {
		if tag == "" {
			return apperrors.NewBadRequestError("tag cannot be empty at index " + strconv.Itoa(i))
		}
	}

	// Validate nodes
	if len(config.Nodes) == 0 {
		return apperrors.NewBadRequestError("nodes cannot be empty")
	}
	nodeNames := make(map[string]bool)
	for i, node := range config.Nodes {
		if node.Name == "" {
			return apperrors.NewBadRequestError("node name cannot be empty at index " + strconv.Itoa(i))
		}
		// Check for duplicate node names
		if nodeNames[node.Name] {
			return apperrors.NewBadRequestError("duplicate node name: " + node.Name)
		}
		nodeNames[node.Name] = true
	}

	if len(config.Relations) == 0 {
		return apperrors.NewBadRequestError("relations cannot be empty")
	}
	// Validate relations
	for i, relation := range config.Relations {
		if relation.Node1 == "" {
			return apperrors.NewBadRequestError("relation node1 cannot be empty at index " + strconv.Itoa(i))
		}
		if relation.Node2 == "" {
			return apperrors.NewBadRequestError("relation node2 cannot be empty at index " + strconv.Itoa(i))
		}
		if relation.Type == "" {
			return apperrors.NewBadRequestError("relation type cannot be empty at index " + strconv.Itoa(i))
		}
		// Check if referenced nodes exist
		if !nodeNames[relation.Node1] {
			return apperrors.NewBadRequestError("relation references non-existent node1: " + relation.Node1)
		}
		if !nodeNames[relation.Node2] {
			return apperrors.NewBadRequestError("relation references non-existent node2: " + relation.Node2)
		}
	}

	return nil
}

func validateKnowledgeBasePromptInstructions(kb *types.KnowledgeBase) error {
	if err := types.ValidateKnowledgeBasePromptInstructions(kb); err != nil {
		return apperrors.NewBadRequestError(err.Error())
	}
	return nil
}

// ListMoveTargets returns knowledge bases eligible as move targets for the given source KB.
// Filters: same Type, same EmbeddingModelID, different ID, not temporary.
//
// ListMoveTargets godoc
// @Summary      List move target knowledge bases
// @Description  Returns the knowledge bases with the same Type and EmbeddingModelID as the source, that are not
// @Description  temporary and not the source itself
// @Tags         Knowledge Bases
// @Produce      json
// @Param        id   path      string                  true  "Source knowledge base ID"
// @Success      200  {object}  map[string]interface{}  "Move targets"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Failure      404  {object}  errors.ErrorResponse         "Knowledge base not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/move-targets [get]
func (h *KnowledgeBaseHandler) ListMoveTargets(c *gin.Context) {
	ctx := c.Request.Context()

	sourceKBID := c.Param("id")
	if sourceKBID == "" {
		c.Error(apperrors.NewBadRequestError("Knowledge base ID is required"))
		return
	}

	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		c.Error(apperrors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Get source knowledge base
	sourceKB, err := h.service.GetKnowledgeBaseByID(ctx, sourceKBID)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Source knowledge base not found"))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if sourceKB.TenantID != tenantID.(uint64) {
		c.Error(errors.NewForbiddenError("No permission to access this knowledge base"))
		return
	}

	// Get all knowledge bases
	allKBs, err := h.service.ListKnowledgeBases(ctx)
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Filter eligible targets
	targets := make([]*types.KnowledgeBase, 0)
	for _, kb := range allKBs {
		if kb.ID == sourceKBID {
			continue
		}
		if kb.IsTemporary {
			continue
		}
		if kb.Type != sourceKB.Type {
			continue
		}
		if kb.EmbeddingModelID != sourceKB.EmbeddingModelID {
			continue
		}
		targets = append(targets, kb)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    targets,
	})
}

// GetImageAttrsSchema returns the canonical image-attribute registry for this
// release. It is the single source of truth that drives the frontend attribute
// panel - both the attributes and their display text (label, description, the
// meaning of each value) - so adding an attribute later is a backend-only
// change (one registry row) and the UI follows automatically. Read-only; the
// registry is global, not per-KB, so it carries no KB id and only the Viewer
// role is required.
func (h *KnowledgeBaseHandler) GetImageAttrsSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"version":         types.ImageAttrSchemaVersion,
			"prompt":          types.ImageAttrPromptVersion,
			"attributes":      types.ImageAttrRegistry,
			"default_actions": types.DefaultImageActions(),
		},
	})
}
