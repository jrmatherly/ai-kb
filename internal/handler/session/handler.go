package session

import (
	stderrors "errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/browserskill"
	"github.com/jrmatherly/ai-kb/internal/config"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/infrastructure/docparser"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/redis/go-redis/v9"
)

// Handler handles all HTTP requests related to conversation sessions
type Handler struct {
	browserSkill         *browserskill.Manager
	messageService       interfaces.MessageService // Service for managing messages
	suggestionService    interfaces.MessageSuggestionService
	sessionService       interfaces.SessionService       // Service for managing sessions
	streamManager        interfaces.StreamManager        // Manager for handling streaming responses
	config               *config.Config                  // Application configuration
	knowledgebaseService interfaces.KnowledgeBaseService // Service for managing knowledge bases
	customAgentService   interfaces.CustomAgentService   // Service for managing custom agents
	tenantService        interfaces.TenantService        // Service for loading tenant (shared agent context)
	agentShareService    interfaces.AgentShareService    // Service for resolving shared agents (KB scope in retrieval)
	kbShareService       interfaces.KBShareService       // Service for resolving shared KB permissions
	fileService          interfaces.FileService          // Service for file storage (image uploads)
	resourceCatalog      interfaces.ResourceCatalog
	storageResolver      interfaces.StorageBackendResolver
	modelService         interfaces.ModelService // Service for model management (VLM access)
	attachmentProcessor  *AttachmentProcessor    // Processor for file attachments
	temporaryDocuments   interfaces.TemporaryDocumentService
	// artifactCollector drains skill-generated files from the session sandbox
	// after an agent turn completes. May be nil when the sandbox backend does
	// not support artifact collection; handlers must check before using.
	artifactCollector *service.ArtifactCollector
	// workspaceCheckpointer commits the sandbox /workspace at the end of each
	// agent turn so session fork can roll back to a specific message. May be
	// nil when the deployment has no sandbox backend.
	workspaceCheckpointer *service.WorkspaceCheckpointer
	// sandboxIDLookup resolves a session's bound sandbox without provisioning.
	sandboxIDLookup SandboxIDLookup
	memoryService   interfaces.MemoryService // Service for cross-session long-term memory
	// userService / memberService back the sandbox terminal's self-contained
	// handshake (browser WebSocket upgrades cannot send Authorization).
	userService   interfaces.UserService
	memberService interfaces.TenantMemberService
	// terminalService opens PTYs on the sandbox bound to a session. It also
	// owns first-use provisioning: the WS handshake carries the chat page's
	// selected agent so the sandbox is created with the same config a
	// conversation turn would use.
	terminalService *service.SandboxTerminalService
	desktopService  *service.SandboxDesktopService
	desktopTickets  service.SandboxDesktopTicketStore
	desktopLast     service.SandboxDesktopLastStore
	// redis backs the distributed desktop slot. Nil in Lite mode, where the
	// in-process limiter is the correct degradation.
	redis *redis.Client
	// forkService branches a session at a chosen user message. May be nil in
	// deployments where fork is not wired; ForkSession checks.
	forkService sessionForker
	// rewindService truncates the current session at a chosen message. May
	// be nil in deployments where rewind is not wired; RewindSession checks.
	rewindService sessionRewinder
	// approvedProjectDirs is the user-approved ProjectDirs list used to
	// validate CreateSession's optional project_dir. Nil means none are
	// approved, so a non-empty project_dir is rejected.
	approvedProjectDirs HostProjectDirsLoader
}

// NewHandler creates a new instance of Handler with all necessary dependencies
func NewHandler(
	sessionService interfaces.SessionService,
	messageService interfaces.MessageService,
	suggestionService interfaces.MessageSuggestionService,
	streamManager interfaces.StreamManager,
	config *config.Config,
	knowledgebaseService interfaces.KnowledgeBaseService,
	customAgentService interfaces.CustomAgentService,
	tenantService interfaces.TenantService,
	agentShareService interfaces.AgentShareService,
	kbShareService interfaces.KBShareService,
	fileService interfaces.FileService,
	resourceCatalog interfaces.ResourceCatalog,
	storageResolver interfaces.StorageBackendResolver,
	modelService interfaces.ModelService,
	documentReader interfaces.DocumentReader,
	imageResolver *docparser.ImageResolver,
	temporaryDocuments interfaces.TemporaryDocumentService,
	artifactCollector *service.ArtifactCollector,
	workspaceCheckpointer *service.WorkspaceCheckpointer,
	sandboxIDLookup SandboxIDLookup,
	memoryService interfaces.MemoryService,
	userService interfaces.UserService,
	memberService interfaces.TenantMemberService,
	terminalService *service.SandboxTerminalService,
	browserSkill *browserskill.Manager,
	desktopService *service.SandboxDesktopService,
	desktopTickets service.SandboxDesktopTicketStore,
	desktopLast service.SandboxDesktopLastStore,
	rdb *redis.Client,
	forkService *service.SessionForkService,
	rewindService *service.SessionRewindService,
	approvedProjectDirs HostProjectDirsLoader,
) *Handler {
	h := &Handler{
		browserSkill:          browserSkill,
		sessionService:        sessionService,
		messageService:        messageService,
		suggestionService:     suggestionService,
		streamManager:         streamManager,
		config:                config,
		knowledgebaseService:  knowledgebaseService,
		customAgentService:    customAgentService,
		tenantService:         tenantService,
		agentShareService:     agentShareService,
		kbShareService:        kbShareService,
		fileService:           fileService,
		resourceCatalog:       resourceCatalog,
		storageResolver:       storageResolver,
		modelService:          modelService,
		temporaryDocuments:    temporaryDocuments,
		artifactCollector:     artifactCollector,
		workspaceCheckpointer: workspaceCheckpointer,
		sandboxIDLookup:       sandboxIDLookup,
		memoryService:         memoryService,
		userService:           userService,
		memberService:         memberService,
		terminalService:       terminalService,
		desktopService:        desktopService,
		desktopTickets:        desktopTickets,
		desktopLast:           desktopLast,
		redis:                 rdb,
		approvedProjectDirs:   approvedProjectDirs,
		attachmentProcessor: NewAttachmentProcessor(
			fileService,
			documentReader,
			imageResolver,
			modelService,
		),
	}
	if forkService != nil {
		h.forkService = forkService
	}
	if rewindService != nil {
		h.rewindService = rewindService
	}
	return h
}

// CreateSession godoc
// @Summary      Create a session
// @Description  Creates a new conversation session
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        request  body      CreateSessionRequest  true  "Session creation request"
// @Success      201      {object}  map[string]interface{}  "Created session"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions [post]
func (h *Handler) CreateSession(c *gin.Context) {
	ctx := c.Request.Context()
	// Parse and validate the request body
	var request CreateSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(ctx, "Failed to validate session creation parameters", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	// Get tenant ID from context
	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		logger.Error(ctx, "Failed to get tenant ID")
		c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Sessions are now knowledge-base-independent:
	// - All configuration comes from custom agent at query time
	// - Session only stores basic info (tenant ID, title, description)
	logger.Infof(
		ctx,
		"Processing session creation request, tenant ID: %d",
		tenantID,
	)

	hostDir, ok := bindHostWorkspaceDir(request.ProjectDir, h.approvedDirs())
	if !ok {
		_ = c.Error(errors.NewBadRequestError("project_dir is not an approved project directory"))
		return
	}

	// Create session object with base properties
	createdSession := &types.Session{
		TenantID:         tenantID.(uint64),
		Title:            request.Title,
		Description:      types.SanitizeClientSessionDescription(request.Description, ""),
		HostWorkspaceDir: hostDir,
	}
	// Attach the calling user as the session owner when available.
	// API-key callers scope sessions per external user when configured;
	// otherwise they fall back to the synthetic tenant user.
	if ownerID := types.SessionOwnerIDFromContext(ctx); ownerID != "" {
		createdSession.UserID = ownerID
	}

	// Call service to create session
	logger.Infof(ctx, "Calling session service to create session")
	createdSession, err := h.sessionService.CreateSession(ctx, createdSession)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Return created session
	logger.Infof(ctx, "Session created successfully, ID: %s", createdSession.ID)
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    createdSession,
	})
}

// GetSession godoc
// @Summary      Get session details
// @Description  Gets session details by ID
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Session ID"
// @Success      200  {object}  map[string]interface{}  "Session details"
// @Failure      404  {object}  errors.AppError         "Session not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id} [get]
func (h *Handler) GetSession(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start retrieving session")

	// Get session ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	// Call service to get session details
	logger.Infof(ctx, "Retrieving session, ID: %s", id)
	session, err := h.sessionService.GetSession(ctx, id)
	if err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Return session data
	logger.Infof(ctx, "Session retrieved successfully, ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    session,
	})
}

// GetSessionsByTenant godoc
// @Summary      List sessions
// @Description  Lists the current workspace's sessions, with pagination, keyword search and filtering by source/agent
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "Page number"
// @Param        page_size  query     int     false  "Page size"
// @Param        keyword    query     string  false  "Fuzzy title search"
// @Param        source     query     string  false  "web/embed/api/feishu/wechat/slack/...; api/embed/IM need Admin+"
// @Param        agent_id   query     string  false  "Filter by agent (IM sessions only)"
// @Success      200        {object}  map[string]interface{}  "Session list"
// @Failure      400        {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions [get]
func (h *Handler) GetSessionsByTenant(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination parameters from query
	var pagination types.Pagination
	if err := c.ShouldBindQuery(&pagination); err != nil {
		logger.Error(ctx, "Failed to parse pagination parameters", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	// Response items always include pin state and (when available) IM origin
	// fields so the frontend can render pin icons / source badges without a
	// second roundtrip. Unset filter params behave like "no filter".
	result, err := h.sessionService.ListSessions(ctx, &types.SessionListQuery{
		Keyword:  c.Query("keyword"),
		Source:   c.Query("source"),
		AgentID:  c.Query("agent_id"),
		Page:     pagination.Page,
		PageSize: pagination.PageSize,
	})
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      result.Data,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// UpdateSession godoc
// @Summary      Update a session
// @Description  Updates session properties
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        id       path      string         true  "Session ID"
// @Param        request  body      types.Session  true  "Session info"
// @Success      200      {object}  map[string]interface{}  "Updated session"
// @Failure      404      {object}  errors.AppError         "Session not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id} [put]
func (h *Handler) UpdateSession(c *gin.Context) {
	ctx := c.Request.Context()

	// Get session ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	// Verify tenant ID from context for authorization
	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		logger.Error(ctx, "Failed to get tenant ID")
		c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Parse request body to session object
	var session types.Session
	if err := c.ShouldBindJSON(&session); err != nil {
		logger.Error(ctx, "Failed to parse session data", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	session.ID = id
	session.TenantID = tenantID.(uint64)

	// Call service to update session
	if err := h.sessionService.UpdateSession(ctx, &session); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Reload session from database to return complete timestamps and stored fields
	updatedSession, err := h.sessionService.GetSession(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Return updated session
	logger.Infof(ctx, "Session updated successfully, ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedSession,
	})
}

// DeleteSession godoc
// @Summary      Delete a session
// @Description  Deletes the given session
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Session ID"
// @Success      200  {object}  map[string]interface{}  "Deleted"
// @Failure      404  {object}  errors.AppError         "Session not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id} [delete]
func (h *Handler) DeleteSession(c *gin.Context) {
	ctx := c.Request.Context()

	// Get session ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	// Call service to delete session
	if err := h.sessionService.DeleteSession(ctx, id); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	h.browserSkill.Forget(browserSkillScope(ctx), []string{id})

	// Return success message
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session deleted successfully",
	})
}

// ClearSessionMessages godoc
// @Summary      Clear session messages
// @Description  Deletes every message in the session and clears the LLM context and chat-history knowledge base
// @Description  entries. The session itself is kept.
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Session ID"
// @Success      200  {object}  map[string]interface{}  "Cleared"
// @Failure      400  {object}  errors.AppError         "Invalid request parameters"
// @Failure      404  {object}  errors.AppError         "Session not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id}/messages [delete]
func (h *Handler) ClearSessionMessages(c *gin.Context) {
	ctx := c.Request.Context()

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	logger.Infof(ctx, "Clearing all messages for session: %s", id)

	if err := h.messageService.ClearSessionMessages(ctx, id); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "Session not found, ID: %s", id)
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{"session_id": id})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Session messages cleared successfully, ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Session messages cleared successfully",
	})
}

// batchDeleteRequest represents the request body for batch deleting sessions
type batchDeleteRequest struct {
	IDs       []string `json:"ids"`
	DeleteAll bool     `json:"delete_all"`
}

// BatchDeleteSessions godoc
// @Summary      Batch delete sessions
// @Description  Deletes sessions by a list of IDs, or with delete_all=true deletes every session in the current
// @Description  workspace
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        request  body      batchDeleteRequest  true  "Batch delete request"
// @Success      200      {object}  map[string]interface{}  "Delete result"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/batch [delete]
func (h *Handler) BatchDeleteSessions(c *gin.Context) {
	ctx := c.Request.Context()

	var req batchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf(ctx, "Invalid batch delete request: %v", err)
		c.Error(errors.NewBadRequestError("invalid request"))
		return
	}

	if req.DeleteAll {
		if err := h.sessionService.DeleteAllSessions(ctx); err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(errors.NewInternalServerError(err.Error()))
			return
		}
		h.browserSkill.ForgetAll(browserSkillScope(ctx))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "All sessions deleted successfully",
		})
		return
	}

	if len(req.IDs) == 0 {
		c.Error(errors.NewBadRequestError("ids are required when delete_all is false"))
		return
	}

	// Sanitize all IDs
	sanitizedIDs := make([]string, 0, len(req.IDs))
	for _, id := range req.IDs {
		sanitized := secutils.SanitizeForLog(id)
		if sanitized != "" {
			sanitizedIDs = append(sanitizedIDs, sanitized)
		}
	}

	if len(sanitizedIDs) == 0 {
		c.Error(errors.NewBadRequestError("no valid session IDs provided"))
		return
	}

	if err := h.sessionService.BatchDeleteSessions(ctx, sanitizedIDs); err != nil {
		if stderrors.Is(err, errors.ErrSessionNotFound) {
			logger.Warnf(ctx, "No visible sessions found for batch delete")
			c.Error(errors.NewNotFoundError(err.Error()))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	h.browserSkill.Forget(browserSkillScope(ctx), sanitizedIDs)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Sessions deleted successfully",
	})
}

// PinSession godoc
// @Summary      Pin a session
// @Description  Pins the given session (per user)
// @Tags         Sessions
// @Produce      json
// @Param        session_id   path      string  true  "Session ID"
// @Success      200  {object}  map[string]interface{}  "Pinned"
// @Failure      404  {object}  errors.AppError         "Session not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{session_id}/pin [post]
func (h *Handler) PinSession(c *gin.Context) {
	h.setSessionPinned(c, true)
}

// UnpinSession godoc
// @Summary      Unpin a session
// @Description  Unpins the given session
// @Tags         Sessions
// @Produce      json
// @Param        id   path      string  true  "Session ID"
// @Success      200  {object}  map[string]interface{}  "Unpinned"
// @Failure      404  {object}  errors.AppError         "Session not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{id}/pin [delete]
func (h *Handler) UnpinSession(c *gin.Context) {
	h.setSessionPinned(c, false)
}

func (h *Handler) setSessionPinned(c *gin.Context, pinned bool) {
	ctx := c.Request.Context()

	// POST and DELETE for /sessions/.../pin register under different wildcards
	// (POST :session_id, DELETE :id - see router.go). Accept whichever is set.
	rawID := c.Param("session_id")
	if rawID == "" {
		rawID = c.Param("id")
	}
	id := secutils.SanitizeForLog(rawID)
	if id == "" {
		logger.Error(ctx, "Session ID is empty")
		c.Error(errors.NewBadRequestError(errors.ErrInvalidSessionID.Error()))
		return
	}

	rows, err := h.sessionService.SetSessionPinned(ctx, id, pinned)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": id,
			"pinned":     pinned,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	// Zero rows means the session doesn't exist or isn't visible to this user;
	// tell the client rather than reporting success.
	if rows == 0 {
		c.Error(errors.NewNotFoundError(errors.ErrSessionNotFound.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"is_pinned": pinned,
	})
}
