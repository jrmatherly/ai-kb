package session

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/application/service"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// sessionForker is the fork surface the handler needs. Declaring it here
// rather than depending on *service.SessionForkService keeps the handler
// testable with a stub.
type sessionForker interface {
	Fork(
		ctx context.Context,
		tenantID uint64,
		userID, sourceSessionID, messageID, title string,
	) (*service.ForkResult, error)
}

// ForkSessionRequest is the fork endpoint's body.
type ForkSessionRequest struct {
	// MessageID is the message to branch at. A user message copies history
	// strictly before it (the client prefills that question). An assistant
	// message copies history through that answer so the branch continues after it.
	MessageID string `json:"message_id" binding:"required"`
	// Title is optional. Empty falls back to the source title plus a suffix.
	Title string `json:"title"`
}

// ForkSession godoc
// @Summary      Fork a session
// @Description  Forks a new session from the given user or assistant message. From a user message: copies the earlier
// @Description  history and prefills that question. From an assistant message: copies the history including that answer
// @Description  and continues from that turn's sandbox state.
// @Tags         Sessions
// @Accept       json
// @Produce      json
// @Param        session_id  path      string              true  "Source session ID"
// @Param        request     body      ForkSessionRequest  true  "Fork request"
// @Success      200         {object}  map[string]interface{}  "New session"
// @Failure      400         {object}  errors.ErrorResponse         "Invalid parameters / unsupported fork point role"
// @Failure      404         {object}  errors.ErrorResponse         "Session or message not found"
// @Failure      409         {object}  errors.ErrorResponse         "Source session is generating"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /sessions/{session_id}/fork [post]
func (h *Handler) ForkSession(c *gin.Context) {
	ctx := c.Request.Context()

	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Param("id"))
	}
	if sessionID == "" {
		_ = c.Error(errors.NewBadRequestError("session ID is required"))
		return
	}

	var req ForkSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError("message_id is required"))
		return
	}
	if strings.TrimSpace(req.MessageID) == "" {
		_ = c.Error(errors.NewBadRequestError("message_id is required"))
		return
	}

	if h.forkService == nil {
		_ = c.Error(errors.NewBadRequestError("session fork is not available"))
		return
	}

	tenantID, _ := types.TenantIDFromContext(ctx)
	userID := types.SessionOwnerIDFromContext(ctx)

	result, err := h.forkService.Fork(
		ctx, tenantID, userID, sessionID,
		strings.TrimSpace(req.MessageID), strings.TrimSpace(req.Title),
	)
	if err != nil {
		// A busy source is a retryable, user-actionable state, not a fault:
		// snapshotting would pause the sandbox and interrupt the running turn.
		if stderrors.Is(err, service.ErrForkSourceBusy) {
			_ = c.Error(errors.NewConflictError("source session has an active turn").
				WithDetails(gin.H{"code": "FORK_SOURCE_BUSY"}))
			return
		}
		if stderrors.Is(err, service.ErrForkSessionNotFound) {
			_ = c.Error(errors.NewNotFoundError("session not found"))
			return
		}
		if stderrors.Is(err, service.ErrForkMessageNotFound) {
			_ = c.Error(errors.NewNotFoundError("message not found"))
			return
		}
		if stderrors.Is(err, service.ErrForkMessageNotUser) {
			_ = c.Error(errors.NewBadRequestError("fork point must be a user or assistant message"))
			return
		}
		logger.Errorf(ctx, "fork session %s failed: %v", sessionID, err)
		_ = c.Error(errors.NewInternalServerError("fork session failed"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}
