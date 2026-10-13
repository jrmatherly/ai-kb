package handler

import (
	"context"
	stderrors "errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

// TagHandler handles knowledge base tag operations.
//
// All KB-access checks (own / org-shared / via shared agent) are now
// performed by the route-level g.KBAccessRead / g.KBAccessWrite
// guards in router.go - the guard rewrites c.Request.Context() to
// carry the effective tenant ID, so handlers below just use
// c.Request.Context() the way they always did.
type TagHandler struct {
	tagService interfaces.KnowledgeTagService
	tagRepo    interfaces.KnowledgeTagRepository
	chunkRepo  interfaces.ChunkRepository
}

// DeleteTagRequest represents the request body for deleting a tag
type DeleteTagRequest struct {
	ExcludeIDs []int64 `json:"exclude_ids"` // Chunk seq_ids to exclude from deletion
}

// NewTagHandler creates a new TagHandler.
func NewTagHandler(
	tagService interfaces.KnowledgeTagService,
	tagRepo interfaces.KnowledgeTagRepository,
	chunkRepo interfaces.ChunkRepository,
) *TagHandler {
	return &TagHandler{tagService: tagService, tagRepo: tagRepo, chunkRepo: chunkRepo}
}

// resolveTagID resolves tag_id parameter which can be either UUID or seq_id (integer).
// Uses tenant from c's context - which the route-level KB-access guard
// has already rewritten to the effective tenant for shared KBs.
func (h *TagHandler) resolveTagID(c *gin.Context) (string, error) {
	return h.resolveTagIDWithCtx(c, c.Request.Context())
}

// resolveTagIDWithCtx resolves tag_id using the given context for tenant.
func (h *TagHandler) resolveTagIDWithCtx(c *gin.Context, ctx context.Context) (string, error) {
	tagIDParam := secutils.SanitizeForLog(c.Param("tag_id"))

	if seqID, err := strconv.ParseInt(tagIDParam, 10, 64); err == nil {
		tenantID := types.MustTenantIDFromContext(ctx)
		tag, err := h.tagRepo.GetBySeqID(ctx, tenantID, seqID)
		if err != nil {
			return "", errors.NewNotFoundError("Tag not found")
		}
		return tag.ID, nil
	}
	return tagIDParam, nil
}

// getChunksBySeqIDs retrieves chunks by their seq_ids.
func (h *TagHandler) getChunksBySeqIDs(ctx context.Context, tenantID uint64, seqIDs []int64) ([]*types.Chunk, error) {
	return h.chunkRepo.ListChunksBySeqID(ctx, tenantID, seqIDs)
}

// ListTags godoc
// @Summary      List tags
// @Description  List all tags in a knowledge base with their statistics
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Param        id         path      string  true   "Knowledge base ID"
// @Param        page       query     int     false  "Page number"
// @Param        page_size  query     int     false  "Page size"
// @Param        keyword    query     string  false  "Keyword search"
// @Success      200        {object}  map[string]interface{}  "Tag list"
// @Failure      400        {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/tags [get]
func (h *TagHandler) ListTags(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var page types.Pagination
	if err := c.ShouldBindQuery(&page); err != nil {
		logger.Error(ctx, "Failed to bind pagination query", err)
		_ = c.Error(errors.NewBadRequestError("Invalid pagination parameters").WithDetails(err.Error()))
		return
	}

	keyword := secutils.SanitizeForLog(c.Query("keyword"))

	tags, err := h.tagService.ListTags(ctx, kbID, &page, keyword)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tags,
	})
}

type createTagRequest struct {
	Name      string `json:"name"       binding:"required"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

// CreateTag godoc
// @Summary      Create a tag
// @Description  Create a new tag in a knowledge base
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "Knowledge base ID"
// @Param        request  body      object{name=string,color=string,sort_order=int}  true  "Tag info"
// @Success      200      {object}  map[string]interface{}  "Created tag"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/tags [post]
func (h *TagHandler) CreateTag(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req createTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind create tag payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	tag, err := h.tagService.CreateTag(ctx, kbID,
		secutils.SanitizeForLog(req.Name), secutils.SanitizeForLog(req.Color), req.SortOrder)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"kb_id": kbID,
		})
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tag,
	})
}

type updateTagRequest struct {
	Name      *string `json:"name"`
	Color     *string `json:"color"`
	SortOrder *int    `json:"sort_order"`
}

// UpdateTag godoc
// @Summary      Update a tag
// @Description  Update tag info
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "Knowledge base ID"
// @Param        tag_id   path      string  true  "Tag ID (UUID or seq_id)"
// @Param        request  body      object  true  "Tag update"
// @Success      200      {object}  map[string]interface{}  "Updated tag"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/tags/{tag_id} [put]
func (h *TagHandler) UpdateTag(c *gin.Context) {
	ctx := c.Request.Context()

	tagID, err := h.resolveTagID(c)
	if err != nil {
		c.Error(err)
		return
	}

	var req updateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind update tag payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	tag, err := h.tagService.UpdateTag(ctx, tagID, req.Name, req.Color, req.SortOrder)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tag_id": tagID,
		})
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tag,
	})
}

// DeleteTag godoc
// @Summary      Delete a tag
// @Description  Delete a tag. force=true deletes a tag that is still referenced; content_only=true deletes only the
// @Description  content under the tag and keeps the tag
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Param        id            path      string              true   "Knowledge base ID"
// @Param        tag_id        path      string              true   "Tag ID (UUID or seq_id)"
// @Param        force         query     bool                false  "Force delete"
// @Param        content_only  query     bool                false  "Delete only the content, keep the tag"
// @Param        body          body      DeleteTagRequest    false  "Delete options"
// @Success      200           {object}  map[string]interface{}  "Deleted"
// @Failure      400           {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/tags/{tag_id} [delete]
func (h *TagHandler) DeleteTag(c *gin.Context) {
	ctx := c.Request.Context()

	tagID, err := h.resolveTagID(c)
	if err != nil {
		c.Error(err)
		return
	}

	force := c.Query("force") == "true"
	contentOnly := c.Query("content_only") == "true"

	var req DeleteTagRequest
	if err := c.ShouldBindJSON(&req); err != nil && !stderrors.Is(err, io.EOF) {
		_ = c.Error(errors.NewBadRequestError("Invalid delete options"))
		return
	}

	var excludeUUIDs []string
	if len(req.ExcludeIDs) > 0 {
		tenantID := types.MustTenantIDFromContext(ctx)
		wanted := make(map[int64]bool, len(req.ExcludeIDs))
		for _, id := range req.ExcludeIDs {
			if id <= 0 {
				_ = c.Error(errors.NewBadRequestError("Excluded entry IDs must be positive integers"))
				return
			}
			wanted[id] = true
		}
		chunks, err := h.getChunksBySeqIDs(ctx, tenantID, req.ExcludeIDs)
		if err != nil {
			_ = c.Error(err)
			return
		}
		for _, chunk := range chunks {
			if chunk == nil || !wanted[chunk.SeqID] {
				continue
			}
			if chunk.TenantID != tenantID || chunk.KnowledgeBaseID != c.Param("id") ||
				chunk.ChunkType != types.ChunkTypeFAQ {
				_ = c.Error(errors.NewForbiddenError("Excluded entry does not belong to this knowledge base"))
				return
			}
			excludeUUIDs = append(excludeUUIDs, chunk.ID)
			delete(wanted, chunk.SeqID)
		}
		if len(wanted) != 0 {
			_ = c.Error(errors.NewNotFoundError("Excluded entry not found"))
			return
		}
	}

	if err := h.tagService.DeleteTag(ctx, tagID, force, contentOnly, excludeUUIDs); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tag_id": tagID,
		})
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// NOTE: TagHandler currently exposes CRUD for tags and statistics.
// Knowledge / Chunk tagging is handled via dedicated knowledge and FAQ APIs.
