package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/application/service/memory"
	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// MemoryHandler exposes the caller's own long-term memory.
//
// Every route operates on the memory space derived from the request context,
// so no endpoint takes a subject id. That is deliberate: it removes the entire
// class of "can I read another user's memories by changing an id" bugs instead
// of relying on a per-route ownership check.
type MemoryHandler struct {
	memoryService interfaces.MemoryService
}

func NewMemoryHandler(memoryService interfaces.MemoryService) *MemoryHandler {
	return &MemoryHandler{memoryService: memoryService}
}

// GetSettings godoc
// @Summary      Get my memory settings
// @Description  Returns the effective memory switch (workspace and personal) and the memory count
// @Tags         Long-term Memory
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Memory settings"
// @Security     Bearer
// @Router       /memory/settings [get]
func (h *MemoryHandler) GetSettings(c *gin.Context) {
	ctx := c.Request.Context()
	settings, err := h.memoryService.GetSettings(ctx)
	if err != nil {
		h.fail(c, err, "Failed to load memory settings")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": settings})
}

type updateMemorySettingsRequest struct {
	Enabled *bool `json:"enabled"`
}

// UpdateSettings godoc
// @Summary      Update my memory settings
// @Description  Turns long-term memory on or off for the current user
// @Tags         Long-term Memory
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Settings"
// @Success      200      {object}  map[string]interface{}  "Updated settings"
// @Security     Bearer
// @Router       /memory/settings [put]
func (h *MemoryHandler) UpdateSettings(c *gin.Context) {
	ctx := c.Request.Context()
	var req updateMemorySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	if req.Enabled == nil {
		c.Error(apperrors.NewBadRequestError("enabled is required"))
		return
	}
	if err := h.memoryService.SetEnabled(ctx, *req.Enabled); err != nil {
		h.fail(c, err, "Failed to update memory settings")
		return
	}
	settings, err := h.memoryService.GetSettings(ctx)
	if err != nil {
		h.fail(c, err, "Failed to load memory settings")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": settings})
}

// ListItems godoc
// @Summary      List my memories
// @Description  Returns the current user's memories, paginated, optionally filtered by status
// @Tags         Long-term Memory
// @Produce      json
// @Param        status  query     string  false  "Status filter"  Enums(active, superseded, archived, pending)
// @Param        limit   query     int     false  "Page size"  default(50)
// @Param        offset  query     int     false  "Offset"
// @Success      200     {object}  map[string]interface{}  "Memories"
// @Security     Bearer
// @Router       /memory/items [get]
func (h *MemoryHandler) ListItems(c *gin.Context) {
	ctx := c.Request.Context()
	status := c.Query("status")
	switch status {
	case "", types.MemoryStatusActive, types.MemoryStatusSuperseded,
		types.MemoryStatusArchived, types.MemoryStatusPending:
	default:
		c.Error(apperrors.NewBadRequestError("unsupported status"))
		return
	}
	limit, offset := memoryListPaging(c)

	items, total, err := h.memoryService.ListItems(ctx, status, limit, offset)
	if err != nil {
		h.fail(c, err, "Failed to list memories")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
		"total":   total,
	})
}

const (
	// memoryExportPageSize is how many rows one export page reads.
	memoryExportPageSize = 500
	// memoryExportMaxItems bounds a single export so one enormous store cannot
	// turn a download into an unbounded read.
	memoryExportMaxItems = 20000
)

func memoryListPaging(c *gin.Context) (limit, offset int) {
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// ListTopics godoc
// @Summary      List observed topics
// @Description  Returns counted topics not yet promoted to a long-term interest, with how many more mentions each needs
// @Tags         Long-term Memory
// @Produce      json
// @Param        limit   query     int  false  "Page size"  default(50)
// @Param        offset  query     int  false  "Offset"
// @Success      200     {object}  map[string]interface{}  "Topics"
// @Security     Bearer
// @Router       /memory/topics [get]
func (h *MemoryHandler) ListTopics(c *gin.Context) {
	ctx := c.Request.Context()
	limit, offset := memoryListPaging(c)
	topics, total, err := h.memoryService.ListTopics(ctx, limit, offset)
	if err != nil {
		h.fail(c, err, "Failed to list topics")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    topics,
		"total":   total,
	})
}

// PromoteTopic godoc
// @Summary      Promote to long-term interest now
// @Description  Promotes an observed topic to a long-term interest memory without waiting for the remaining mentions
// @Tags         Long-term Memory
// @Produce      json
// @Param        id   path      string  true  "Topic ID"
// @Success      200  {object}  map[string]interface{}  "Created memory"
// @Security     Bearer
// @Router       /memory/topics/{id}/promote [post]
func (h *MemoryHandler) PromoteTopic(c *gin.Context) {
	ctx := c.Request.Context()
	item, err := h.memoryService.PromoteTopic(ctx, c.Param("id"))
	if err != nil {
		h.fail(c, err, "Failed to promote topic")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

// DeleteTopic godoc
// @Summary      Stop tracking a topic
// @Description  Deletes the count of an unpromoted topic and records the rejection, so it is never promoted
// @Description  automatically
// @Tags         Long-term Memory
// @Produce      json
// @Param        id   path      string  true  "Topic ID"
// @Success      200  {object}  map[string]interface{}  "Deleted"
// @Security     Bearer
// @Router       /memory/topics/{id} [delete]
func (h *MemoryHandler) DeleteTopic(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.memoryService.DeleteTopic(ctx, c.Param("id")); err != nil {
		h.fail(c, err, "Failed to delete topic")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListDocuments godoc
// @Summary      List frequently used documents
// @Description  Returns documents repeatedly cited in the current user's answers; documents below the habit threshold
// @Description  are omitted
// @Tags         Long-term Memory
// @Produce      json
// @Param        limit   query     int  false  "Page size"  default(50)
// @Param        offset  query     int  false  "Offset"
// @Success      200     {object}  map[string]interface{}  "Documents"
// @Security     Bearer
// @Router       /memory/documents [get]
func (h *MemoryHandler) ListDocuments(c *gin.Context) {
	ctx := c.Request.Context()
	limit, offset := memoryListPaging(c)
	docs, total, err := h.memoryService.ListDocuments(ctx, limit, offset)
	if err != nil {
		h.fail(c, err, "Failed to list documents")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    docs,
		"total":   total,
	})
}

// DeleteDocument godoc
// @Summary      Stop personalizing retrieval with a document
// @Description  Deletes a document affinity count, so retrieval no longer boosts that document
// @Tags         Long-term Memory
// @Produce      json
// @Param        id   path      string  true  "Affinity ID"
// @Success      200  {object}  map[string]interface{}  "Deleted"
// @Security     Bearer
// @Router       /memory/documents/{id} [delete]
func (h *MemoryHandler) DeleteDocument(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.memoryService.DeleteDocument(ctx, c.Param("id")); err != nil {
		h.fail(c, err, "Failed to delete document affinity")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

type createMemoryItemRequest struct {
	Kind       string `json:"kind"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

// CreateItem godoc
// @Summary      Create a memory
// @Description  Adds a long-term memory manually
// @Tags         Long-term Memory
// @Accept       json
// @Produce      json
// @Param        request  body      object  true  "Memory content"
// @Success      200      {object}  map[string]interface{}  "Created memory"
// @Security     Bearer
// @Router       /memory/items [post]
func (h *MemoryHandler) CreateItem(c *gin.Context) {
	ctx := c.Request.Context()
	var req createMemoryItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	item, err := h.memoryService.CreateItem(ctx, req.Kind, req.Content, req.Importance)
	if err != nil {
		h.fail(c, err, "Failed to create memory")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

type updateMemoryItemRequest struct {
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

// UpdateItem godoc
// @Summary      Update a memory
// @Description  Updates a memory's content and importance; an edited memory is never overwritten by background
// @Description  extraction
// @Tags         Long-term Memory
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "Memory ID"
// @Param        request  body      object  true  "Memory content"
// @Success      200      {object}  map[string]interface{}  "Updated memory"
// @Security     Bearer
// @Router       /memory/items/{id} [put]
func (h *MemoryHandler) UpdateItem(c *gin.Context) {
	ctx := c.Request.Context()
	var req updateMemoryItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	item, err := h.memoryService.UpdateItem(ctx, c.Param("id"), req.Content, req.Importance)
	if err != nil {
		h.fail(c, err, "Failed to update memory")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

// DeleteItem godoc
// @Summary      Delete a memory
// @Description  Permanently deletes a memory
// @Tags         Long-term Memory
// @Produce      json
// @Param        id  path      string  true  "Memory ID"
// @Success      200  {object}  map[string]interface{}  "Deleted"
// @Security     Bearer
// @Router       /memory/items/{id} [delete]
func (h *MemoryHandler) DeleteItem(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.memoryService.DeleteItem(ctx, c.Param("id")); err != nil {
		h.fail(c, err, "Failed to delete memory")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ConfirmItem godoc
// @Summary      Confirm an inferred memory
// @Description  Accepts a memory the system inferred, so it takes effect
// @Tags         Long-term Memory
// @Produce      json
// @Param        id   path      string  true  "Memory ID"
// @Success      200  {object}  map[string]interface{}  "Confirmed"
// @Security     Bearer
// @Router       /memory/items/{id}/confirm [post]
//
// Inferred memories are the ones worth having and the ones most likely to be
// wrong, so they wait here rather than taking effect silently.
func (h *MemoryHandler) ConfirmItem(c *gin.Context) {
	ctx := c.Request.Context()
	item, err := h.memoryService.ConfirmItem(ctx, c.Param("id"))
	if err != nil {
		h.fail(c, err, "Failed to confirm memory")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

// RejectItem godoc
// @Summary      Reject an inferred memory
// @Description  Rejects a memory the system inferred and records the rejection
// @Tags         Long-term Memory
// @Produce      json
// @Param        id   path      string  true  "Memory ID"
// @Success      200  {object}  map[string]interface{}  "Rejected"
// @Security     Bearer
// @Router       /memory/items/{id}/reject [post]
func (h *MemoryHandler) RejectItem(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.memoryService.RejectItem(ctx, c.Param("id")); err != nil {
		h.fail(c, err, "Failed to reject memory")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Clear godoc
// @Summary      Clear my memories
// @Description  Permanently deletes all of the current user's memories
// @Tags         Long-term Memory
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Cleared"
// @Security     Bearer
// @Router       /memory/items [delete]
func (h *MemoryHandler) Clear(c *gin.Context) {
	ctx := c.Request.Context()
	removed, err := h.memoryService.Clear(ctx)
	if err != nil {
		h.fail(c, err, "Failed to clear memories")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "removed": removed})
}

// Export godoc
// @Summary      Export my memories
// @Description  Exports all of the current user's memories as JSON
// @Tags         Long-term Memory
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Memory export"
// @Security     Bearer
// @Router       /memory/export [get]
func (h *MemoryHandler) Export(c *gin.Context) {
	ctx := c.Request.Context()
	// Export is a snapshot, not a page, so it walks every status to the end.
	//
	// A single fixed page used to serve this on the grounds that it matched the
	// largest capacity a workspace can configure. It does not: max_items caps
	// active memories only, while superseded and archived rows accumulate
	// without limit, so a long-lived store holds far more than its capacity and
	// the export quietly returned a prefix of it.
	var items []*types.MemoryItem
	var total int64
	for {
		page, pageTotal, err := h.memoryService.ListItems(ctx, "", memoryExportPageSize, len(items))
		if err != nil {
			h.fail(c, err, "Failed to export memories")
			return
		}
		total = pageTotal
		items = append(items, page...)
		if len(page) < memoryExportPageSize || int64(len(items)) >= total {
			break
		}
		if len(items) >= memoryExportMaxItems {
			break
		}
	}
	c.Header("Content-Disposition", `attachment; filename="ai-kb-memories.json"`)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   total,
		// Say so rather than letting a partial file look complete. Only the
		// safety ceiling can trigger this, so it stays false in practice.
		"truncated": int64(len(items)) < total,
		"data":      items,
	})
}

// Consolidate godoc
// @Summary      Consolidate my memories now
// @Description  Merges near-duplicate entries and archives expired items without waiting for the daily background run
// @Tags         Long-term Memory
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Consolidation result"
// @Security     Bearer
// @Router       /memory/consolidate [post]
func (h *MemoryHandler) Consolidate(c *gin.Context) {
	ctx := c.Request.Context()
	result, err := h.memoryService.ConsolidateNow(ctx)
	if err != nil {
		h.fail(c, err, "Failed to consolidate memories")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// fail maps service errors onto HTTP responses. A missing item and an item
// belonging to someone else produce the same 404 on purpose.
func (h *MemoryHandler) fail(c *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, memory.ErrNoMemoryScope):
		c.Error(apperrors.NewUnauthorizedError("no principal in request"))
	case errors.Is(err, memory.ErrItemNotFound):
		c.Error(apperrors.NewNotFoundError("memory not found"))
	case errors.Is(err, types.ErrMemoryConflict):
		c.Error(apperrors.NewConflictError(err.Error()))
	case errors.Is(err, memory.ErrSensitiveContent):
		c.Error(apperrors.NewBadRequestError(err.Error()))
	case errors.Is(err, memory.ErrMemoryDisabled):
		c.Error(apperrors.NewBadRequestError("memory is disabled"))
	default:
		logger.ErrorWithFields(c.Request.Context(), err, nil)
		c.Error(apperrors.NewInternalServerError(message).WithDetails(err.Error()))
	}
}
