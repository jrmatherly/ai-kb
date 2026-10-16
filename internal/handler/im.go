package handler

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/im"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"gorm.io/gorm"
)

// validIMPlatforms is the set of supported IM platforms.
var validIMPlatforms = map[string]bool{
	"wecom": true, "feishu": true, "lark": true, "slack": true, "telegram": true, "dingtalk": true,
	"mattermost": true, "wechat": true, "qqbot": true, "yunzhijia": true,
}

// invalidIMPlatformError is the 400 message listing the accepted platforms. It
// is derived from validIMPlatforms so the two cannot drift apart as platforms
// are added.
var invalidIMPlatformError = func() string {
	names := make([]string, 0, len(validIMPlatforms))
	for p := range validIMPlatforms {
		names = append(names, "'"+p+"'")
	}
	sort.Strings(names)
	return "platform must be one of: " + strings.Join(names, ", ")
}()

// IMHandler handles IM platform callback requests and channel CRUD.
type IMHandler struct {
	imService *im.Service
}

// NewIMHandler creates a new IM handler.
func NewIMHandler(imService *im.Service) *IMHandler {
	return &IMHandler{
		imService: imService,
	}
}

// ── Channel CRUD handlers ──

// CreateIMChannel creates a new IM channel for an agent.
func (h *IMHandler) CreateIMChannel(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		_ = c.Error(apperrors.NewBadRequestError("agent_id is required"))
		return
	}

	tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("unauthorized"))
		return
	}

	var req struct {
		Platform        string     `json:"platform" binding:"required"`
		Name            string     `json:"name"`
		Mode            string     `json:"mode"`
		OutputMode      string     `json:"output_mode"`
		Locale          string     `json:"locale"`
		SessionMode     string     `json:"session_mode"`
		KnowledgeBaseID string     `json:"knowledge_base_id"`
		Credentials     types.JSON `json:"credentials"`
		Enabled         *bool      `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}

	if !validIMPlatforms[req.Platform] {
		_ = c.Error(apperrors.NewBadRequestError(invalidIMPlatformError))
		return
	}
	locale, err := normalizeLocale(req.Locale)
	if err != nil {
		_ = c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}

	channel := &im.IMChannel{
		TenantID:    tenantID,
		AgentID:     agentID,
		Platform:    req.Platform,
		Name:        req.Name,
		Mode:        req.Mode,
		OutputMode:  req.OutputMode,
		Locale:      locale,
		SessionMode: req.SessionMode,
		Credentials: req.Credentials,
		Enabled:     true,
	}
	// The route guard passes an agent it cannot find in this workspace; both
	// the agent and the file-saving KB must belong to the channel's workspace.
	if err := h.imService.SetChannelAgentID(c.Request.Context(), channel, agentID); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("agent not found"))
		return
	}
	if err := h.imService.SetChannelKnowledgeBaseID(c.Request.Context(), channel, req.KnowledgeBaseID); err != nil {
		_ = c.Error(apperrors.NewBadRequestError("knowledge base not found"))
		return
	}
	if req.Enabled != nil {
		channel.Enabled = *req.Enabled
	}
	// WeChat uses long-polling mode and full output only
	if req.Platform == "wechat" {
		channel.Mode = "longpoll"
		channel.OutputMode = "full"
	} else {
		if channel.Mode == "" {
			if channel.Platform == "mattermost" || channel.Platform == "yunzhijia" {
				channel.Mode = "webhook"
			} else {
				channel.Mode = "websocket"
			}
		}
		if channel.OutputMode == "" {
			channel.OutputMode = "stream"
		}
	}
	if channel.Credentials == nil {
		channel.Credentials = types.JSON("{}")
	}

	if err := h.imService.CreateChannel(channel); err != nil {
		logger.Errorf(c.Request.Context(), "[IM] Create channel failed: %v", err)
		if strings.HasPrefix(err.Error(), "duplicate_bot:") {
			_ = c.Error(apperrors.NewConflictError(strings.TrimPrefix(err.Error(), "duplicate_bot: ")))
			return
		}
		_ = c.Error(apperrors.NewInternalServerError("failed to create channel"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": channel})
}

// ListIMChannels lists all IM channels for an agent.
func (h *IMHandler) ListIMChannels(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		_ = c.Error(apperrors.NewBadRequestError("agent_id is required"))
		return
	}

	tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("unauthorized"))
		return
	}

	channels, err := h.imService.ListChannelsByAgent(agentID, tenantID)
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError("failed to list channels"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": im.SummarizeIMChannels(channels)})
}

// ListAllIMChannels lists every IM channel in the current tenant, across
// agents, for the cross-agent overview page. Credentials are intentionally
// NOT included in the response.
func (h *IMHandler) ListAllIMChannels(c *gin.Context) {
	tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("unauthorized"))
		return
	}

	channels, err := h.imService.ListChannelsByTenant(c.Request.Context(), tenantID)
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError("failed to list channels"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": channels})
}

// UpdateIMChannel updates an IM channel.
//
// UpdateIMChannel godoc
// @Summary      Update IM channel
// @Description  Update an IM channel's name, mode, reply language, knowledge bases, credentials or enabled state
// @Tags         IM Channels
// @Accept       json
// @Produce      json
// @Param        id       path      string                  true  "Channel ID"
// @Param        request  body      map[string]interface{}  true  "Fields to update (locale included)"
// @Success      200      {object}  map[string]interface{}  "Updated channel"
// @Failure      400      {object}  apperrors.ErrorResponse  "Invalid request parameters"
// @Failure      404      {object}  apperrors.ErrorResponse  "Channel not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /im-channels/{id} [put]
func (h *IMHandler) UpdateIMChannel(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		_ = c.Error(apperrors.NewBadRequestError("channel id is required"))
		return
	}

	tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("unauthorized"))
		return
	}

	channel, err := h.imService.GetChannelByIDAndTenant(channelID, tenantID)
	if err != nil {
		_ = c.Error(apperrors.NewNotFoundError("channel not found"))
		return
	}

	var req struct {
		Name            *string    `json:"name"`
		Mode            *string    `json:"mode"`
		OutputMode      *string    `json:"output_mode"`
		Locale          *string    `json:"locale"`
		SessionMode     *string    `json:"session_mode"`
		KnowledgeBaseID *string    `json:"knowledge_base_id"`
		Credentials     types.JSON `json:"credentials"`
		Enabled         *bool      `json:"enabled"`
		AgentID         *string    `json:"agent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}

	if req.Name != nil {
		channel.Name = *req.Name
	}
	if req.Mode != nil {
		channel.Mode = *req.Mode
	}
	if req.OutputMode != nil {
		channel.OutputMode = *req.OutputMode
	}
	if req.Locale != nil {
		locale, err := normalizeLocale(*req.Locale)
		if err != nil {
			_ = c.Error(apperrors.NewBadRequestError(err.Error()))
			return
		}
		channel.Locale = locale
	}
	if req.SessionMode != nil {
		channel.SessionMode = *req.SessionMode
	}
	if req.KnowledgeBaseID != nil {
		ctx := c.Request.Context()
		if err := h.imService.SetChannelKnowledgeBaseID(ctx, channel, *req.KnowledgeBaseID); err != nil {
			_ = c.Error(apperrors.NewBadRequestError("knowledge base not found"))
			return
		}
	}
	if req.Credentials != nil {
		channel.Credentials = req.Credentials
	}
	if req.Enabled != nil {
		channel.Enabled = *req.Enabled
	}
	if req.AgentID != nil {
		newAgentID := strings.TrimSpace(*req.AgentID)
		if newAgentID != "" && newAgentID != channel.AgentID {
			if err := h.imService.SetChannelAgentID(c.Request.Context(), channel, newAgentID); err != nil {
				logger.Errorf(c.Request.Context(), "[IM] Update channel agent failed: %v", err)
				_ = c.Error(apperrors.NewBadRequestError("agent not found"))
				return
			}
		}
	}

	if err := h.imService.UpdateChannel(channel); err != nil {
		logger.Errorf(c.Request.Context(), "[IM] Update channel failed: %v", err)
		if strings.HasPrefix(err.Error(), "duplicate_bot:") {
			_ = c.Error(apperrors.NewConflictError(strings.TrimPrefix(err.Error(), "duplicate_bot: ")))
			return
		}
		_ = c.Error(apperrors.NewInternalServerError("failed to update channel"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": channel})
}

func normalizeLocale(locale string) (string, error) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "", nil
	}
	if normalized := types.NormalizeSupportedLocale(locale); normalized != "" {
		return normalized, nil
	}
	return "", errors.New("locale must be one of: 'en-US', 'ja-JP', 'ko-KR', 'ru-RU', 'zh-CN', 'zh-TW'")
}

// DeleteIMChannel deletes an IM channel.
//
// DeleteIMChannel godoc
// @Summary      Delete IM channel
// @Description  Delete the specified IM channel
// @Tags         IM Channels
// @Produce      json
// @Param        id   path      string                  true  "Channel ID"
// @Success      200  {object}  map[string]interface{}  "success: true"
// @Failure      400  {object}  apperrors.ErrorResponse  "Invalid request parameters"
// @Failure      404  {object}  apperrors.ErrorResponse  "Channel not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /im-channels/{id} [delete]
func (h *IMHandler) DeleteIMChannel(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		_ = c.Error(apperrors.NewBadRequestError("channel id is required"))
		return
	}

	tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("unauthorized"))
		return
	}

	if err := h.imService.DeleteChannel(channelID, tenantID); err != nil {
		_ = c.Error(apperrors.NewInternalServerError("failed to delete channel"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ToggleIMChannel toggles the enabled state of an IM channel.
//
// ToggleIMChannel godoc
// @Summary      Enable or disable IM channel
// @Description  Toggle the enabled state of the specified IM channel
// @Tags         IM Channels
// @Produce      json
// @Param        id   path      string                  true  "Channel ID"
// @Success      200  {object}  map[string]interface{}  "Updated channel"
// @Failure      400  {object}  apperrors.ErrorResponse  "Invalid request parameters"
// @Failure      404  {object}  apperrors.ErrorResponse  "Channel not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /im-channels/{id}/toggle [post]
func (h *IMHandler) ToggleIMChannel(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		_ = c.Error(apperrors.NewBadRequestError("channel id is required"))
		return
	}

	tenantID, ok := c.Request.Context().Value(types.TenantIDContextKey).(uint64)
	if !ok {
		_ = c.Error(apperrors.NewUnauthorizedError("unauthorized"))
		return
	}

	channel, err := h.imService.ToggleChannel(channelID, tenantID)
	if err != nil {
		_ = c.Error(apperrors.NewInternalServerError("failed to toggle channel"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": channel})
}

func writeIMCallbackACK(c *gin.Context, platform string) {
	if platform == "yunzhijia" {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"type":    2,
				"content": "",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── Callback handlers ──

// IMCallback handles IM platform callback requests for a specific channel.
// Route: POST /api/v1/im/callback/:channel_id
//
// IMCallback godoc
// @Summary      IM platform callback
// @Description  Receive event callbacks from IM platforms. Verified with each platform's own signature, not an API key
// @Tags         IM Callback
// @Accept       json
// @Produce      json
// @Param        channel_id  path      string                  true  "Channel ID"
// @Success      200         {object}  map[string]interface{}  "Processing result"
// @Failure      400         {object}  map[string]interface{}  "Invalid request parameters"
// @Failure      401         {object}  map[string]interface{}  "Signature verification failed"
// @Router       /im/callback/{channel_id} [get]
// @Router       /im/callback/{channel_id} [post]
func (h *IMHandler) IMCallback(c *gin.Context) {
	ctx := c.Request.Context()
	channelID := c.Param("channel_id")

	// Always validate the durable row before using a cached webhook adapter.
	// This is the correctness fallback when a replica missed Redis invalidation
	// while disconnected, and prevents stale credentials/config from being used.
	adapter, channel, err := h.imService.EnsureChannelAdapter(channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Errorf(ctx, "[IM] Channel not found for callback: %s", channelID)
			_ = c.Error(apperrors.NewNotFoundError("channel not found"))
			return
		}
		if errors.Is(err, im.ErrChannelDisabled) {
			logger.Errorf(ctx, "[IM] Channel disabled for callback: %s", channelID)
			_ = c.Error(apperrors.NewServiceUnavailableError("channel is disabled"))
			return
		}
		logger.Errorf(ctx, "[IM] Channel unavailable for callback %s: %v", channelID, err)
		_ = c.Error(apperrors.NewServiceUnavailableError("channel not available"))
		return
	}

	defaultMode := "websocket"
	if channel.Platform == "mattermost" || channel.Platform == "yunzhijia" {
		defaultMode = "webhook"
	}
	if im.ResolveMode(channel, defaultMode) != "webhook" ||
		channel.Platform == "qqbot" || channel.Platform == "wechat" || channel.Platform == "dingtalk" {
		_ = c.Error(apperrors.NewForbiddenError("HTTP callbacks are disabled for this channel"))
		return
	}

	logger.Infof(ctx, "[IM] Callback received platform=%s path_channel_id=%s", channel.Platform, channelID)

	// Handle URL verification
	if adapter.HandleURLVerification(c) {
		return
	}

	// Verify callback signature
	if err := adapter.VerifyCallback(c); err != nil {
		logger.Errorf(ctx, "[IM] Callback verification failed for channel %s: %v", channelID, err)
		_ = c.Error(apperrors.NewForbiddenError("verification failed"))
		return
	}

	// Parse the callback message
	msg, err := adapter.ParseCallback(c)
	if err != nil {
		logger.Errorf(ctx, "[IM] Parse callback failed for channel %s: %v", channelID, err)
		_ = c.Error(apperrors.NewBadRequestError("parse failed"))
		return
	}

	// If nil, it's a non-message event - just acknowledge
	if msg == nil {
		if channel.Platform == "mattermost" {
			logger.Infof(ctx, "[IM] Mattermost callback ignored (no message): path_channel_id=%s - check: (1) "+
				"trigger word must be the *first word* of the post; (2) if channel+trigger are both set, post must "+
				"be in that channel; (3) bot_user_id must not match the sender", channelID)
		} else {
			logger.Infof(ctx, "[IM] Callback parsed no message to process platform=%s path_channel_id=%s", channel.Platform, channelID)
		}
		writeIMCallbackACK(c, channel.Platform)
		return
	}

	// Respond immediately to avoid platform timeout
	writeIMCallbackACK(c, channel.Platform)

	// Detach from gin request context
	asyncCtx := context.WithoutCancel(ctx)

	// Process message asynchronously
	go func() {
		if err := h.imService.HandleMessage(asyncCtx, msg, channelID); err != nil {
			logger.Errorf(asyncCtx, "[IM] Handle message error for channel %s: %v", channelID, err)
		}
	}()
}
