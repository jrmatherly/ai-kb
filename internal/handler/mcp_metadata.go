package handler

import (
	"context"
	stderrors "errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

// GetMCPMetadata godoc
// @Summary      Read the saved MCP tool catalog
// @Description  Reads the database only and never contacts upstream. data is null before the first sync; stale is true
// @Description  after the connection config changes. OAuth catalogs are isolated per authorized principal.
// @Tags         MCP Services
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "MCP service ID"
// @Success      200  {object}  map[string]interface{}  "Catalog snapshot"
// @Failure      400  {object}  errors.ErrorResponse         "Invalid request parameters"
// @Failure      401  {object}  errors.ErrorResponse         "OAuth catalog has no authorized principal"
// @Failure      404  {object}  errors.ErrorResponse         "Service not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /mcp-services/{id}/metadata [get]
func (h *MCPServiceHandler) GetMCPMetadata(c *gin.Context) { h.mcpMetadata(c, false) }

// RefreshMCPMetadata godoc
// @Summary      Sync the MCP tool catalog
// @Description  Connects upstream explicitly and atomically replaces the whole catalog. An OAuth service writes the
// @Description  current user's snapshot and needs Viewer or above; static auth writes the workspace-shared snapshot and
// @Description  needs Admin.
// @Tags         MCP Services
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "MCP service ID"
// @Success      200  {object}  map[string]interface{}  "Catalog snapshot after sync"
// @Failure      400  {object}  errors.ErrorResponse         "Catalog incomplete or failed validation"
// @Failure      401  {object}  errors.ErrorResponse         "OAuth catalog has no authorized principal"
// @Failure      403  {object}  errors.ErrorResponse         "A static-auth catalog must be refreshed by an admin"
// @Failure      404  {object}  errors.ErrorResponse         "Service not found"
// @Failure      409  {object}  errors.ErrorResponse         "Connection config changed during the refresh"
// @Failure      503  {object}  errors.ErrorResponse         "Metadata store unavailable"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /mcp-services/{id}/metadata/refresh [post]
func (h *MCPServiceHandler) RefreshMCPMetadata(c *gin.Context) { h.mcpMetadata(c, true) }

func (h *MCPServiceHandler) mcpMetadata(c *gin.Context, refresh bool) {
	ctx := c.Request.Context()
	tenant := c.GetUint64(types.TenantIDContextKey.String())
	if tenant == 0 {
		_ = c.Error(errors.NewBadRequestError("Workspace ID cannot be empty"))
		return
	}
	svc, ok := h.mcpServiceService.(interfaces.MCPMetadataService)
	if !ok {
		_ = c.Error(errors.NewServiceUnavailableError("MCP metadata storage is unavailable"))
		return
	}
	id := c.Param("id")
	var snapshot *types.MCPMetadata
	var err error
	if refresh {
		service, getErr := h.mcpServiceService.GetMCPServiceByID(ctx, tenant, id)
		if getErr != nil || service == nil {
			logger.ErrorWithFields(ctx, getErr, map[string]interface{}{
				"service_id": secutils.SanitizeForLog(id),
				"refresh":    true,
			})
			_ = c.Error(mcpMetadataAppError(types.ErrMCPServiceNotFound, true))
			return
		}
		if !service.AuthConfig.IsOAuth() && !mayWriteSharedMCPMetadata(ctx) {
			_ = c.Error(errors.NewForbiddenError("Refreshing a shared MCP directory requires an administrator"))
			return
		}
		snapshot, err = svc.RefreshMCPMetadata(ctx, tenant, id)
	} else {
		snapshot, err = svc.GetMCPMetadata(ctx, tenant, id)
	}
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"service_id": secutils.SanitizeForLog(id),
			"refresh":    refresh,
		})
		_ = c.Error(mcpMetadataAppError(err, refresh))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": snapshot})
}

// mayWriteSharedMCPMetadata is the extra gate for static-auth catalogs. The
// route stays Viewer+ so OAuth users can persist their own snapshot after
// authorizing in chat. API keys already passed manage-MCP; JWT callers need Admin.
func mayWriteSharedMCPMetadata(ctx context.Context) bool {
	if _, ok := types.TenantAPIKeyScopeFromContext(ctx); ok {
		return true
	}
	if types.IsSystemAdminFromContext(ctx) {
		return true
	}
	return types.CallerFromContext(ctx).Role.HasPermission(types.TenantRoleAdmin)
}

func mcpMetadataAppError(err error, refresh bool) *errors.AppError {
	switch {
	case stderrors.Is(err, types.ErrMCPServiceNotFound):
		return errors.NewNotFoundError("MCP service not found")
	case stderrors.Is(err, types.ErrMCPOAuthPrincipalRequired):
		return errors.NewUnauthorizedError("OAuth metadata requires an authenticated user")
	case stderrors.Is(err, types.ErrMCPMetadataStorage):
		return errors.NewServiceUnavailableError("MCP metadata storage is unavailable")
	case stderrors.Is(err, types.ErrMCPMetadataConnectionChanged):
		return errors.NewConflictError("MCP connection changed during refresh; save the configuration and sync again")
	case stderrors.Is(err, types.ErrMCPMetadataTooLarge), stderrors.Is(err, types.ErrMCPMetadataInvalidTools):
		return errors.NewBadRequestError("MCP directory is invalid or too large")
	default:
		if refresh {
			return errors.NewBadRequestError("Failed to refresh MCP tools. Check the connection and try again.")
		}
		return errors.NewInternalServerError("Failed to read MCP metadata")
	}
}
