package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

// FAQHandler handles FAQ knowledge base operations.
//
// All KB-access checks (own / org-shared / via shared agent) are now
// performed by the route-level g.KBAccessRead / g.KBAccessWrite
// guards in router.go - the guard rewrites c.Request.Context() to
// carry the effective tenant ID for the duration of the handler, so
// the handler reads tenant from context the way it always did.
type FAQHandler struct {
	knowledgeService interfaces.KnowledgeService
	kbService        interfaces.KnowledgeBaseService
}

// NewFAQHandler creates a new FAQ handler.
func NewFAQHandler(
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
) *FAQHandler {
	return &FAQHandler{
		knowledgeService: knowledgeService,
		kbService:        kbService,
	}
}

// faqDeleteRequest is a request for deleting FAQ entries in batch
type faqDeleteRequest struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// faqEntryTagBatchRequest is a request for updating tags for FAQ entries in batch
// key: entry seq_id, value: tag seq_id (nil to remove tag)
type faqEntryTagBatchRequest struct {
	Updates map[int64]*int64 `json:"updates" binding:"required,min=1"`
}

// addSimilarQuestionsRequest is a request for adding similar questions to a FAQ entry
type addSimilarQuestionsRequest struct {
	SimilarQuestions []string `json:"similar_questions" binding:"required,min=1"`
}

// updateLastFAQImportResultDisplayStatusRequest is the request payload for UpdateLastImportResultDisplayStatus
type updateLastFAQImportResultDisplayStatusRequest struct {
	DisplayStatus string `json:"display_status" binding:"required,oneof=open close"`
}

// ListEntries godoc
// @Summary      List FAQ entries
// @Description  Lists the FAQ entries of a knowledge base, with pagination and filters
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id           path      string  true   "Knowledge base ID"
// @Param        page         query     int     false  "Page number"
// @Param        page_size    query     int     false  "Page size"
// @Param        tag_id       query     int     false  "Filter by tag ID (seq_id); legacy single-tag filter"
// @Param        tag_ids      query     string  false  "Filter by tag UUIDs, comma-separated (OR)"
// @Param        keyword      query     string  false  "Keyword search"
// @Param        search_field query     string  false  "standard_question, similar_questions or answers (default all)"
// @Param        sort_order   query     string  false  "asc: oldest update first (default newest)"
// @Param        is_enabled   query     bool    false  "Filter by enabled state; all entries when omitted"
// @Success      200        {object}  map[string]interface{}  "FAQ list"
// @Failure      400        {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries [get]
func (h *FAQHandler) ListEntries(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var page types.Pagination
	if err := c.ShouldBindQuery(&page); err != nil {
		logger.Error(ctx, "Failed to bind pagination query", err)
		_ = c.Error(errors.NewBadRequestError("Invalid pagination parameters").WithDetails(err.Error()))
		return
	}

	tagUUIDs := parseCommaSeparatedTagIDs(c.Query("tag_ids"))
	var legacyTagSeqID int64
	tagIDStr := c.Query("tag_id")
	if tagIDStr != "" {
		var err error
		legacyTagSeqID, err = strconv.ParseInt(tagIDStr, 10, 64)
		if err != nil {
			_ = c.Error(errors.NewBadRequestError("tag_id must be an integer"))
			return
		}
	}
	keyword := secutils.SanitizeForLog(c.Query("keyword"))
	searchField := secutils.SanitizeForLog(c.Query("search_field"))
	sortOrder := secutils.SanitizeForLog(c.Query("sort_order"))
	isEnabled, err := parseOptionalFAQEnabled(c)
	if err != nil {
		c.Error(err)
		return
	}

	result, err := h.knowledgeService.ListFAQEntries(ctx, kbID, &page, tagUUIDs, legacyTagSeqID, keyword, searchField, sortOrder, isEnabled)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// parseOptionalFAQEnabled preserves the distinction between an omitted filter
// and an explicit false value so the management endpoint remains backward compatible.
func parseOptionalFAQEnabled(c *gin.Context) (*bool, error) {
	raw, exists := c.GetQuery("is_enabled")
	if !exists {
		return nil, nil
	}

	var value bool
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		value = true
	case "false":
		value = false
	default:
		return nil, errors.NewBadRequestError("is_enabled must be true or false")
	}
	return &value, nil
}

// UpsertEntries godoc
// @Summary      Batch upsert FAQ entries
// @Description  Asynchronously updates or inserts FAQ entries in batch. With dry_run=true it validates asynchronously
// @Description  without importing.
// @Description  dry_run is asynchronous: it returns a task_id; poll /faq/import/progress/{task_id} for progress and
// @Description  results.
// @Description  Validation covers: 1) entry format 2) duplicate questions (within the batch and against the knowledge
// @Description  base) 3) content safety checks.
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id       path      string                    true  "Knowledge base ID"
// @Param        request  body      types.FAQBatchUpsertPayload  true  "Batch request"
// @Success      200      {object}  map[string]interface{}    "Task ID"
// @Failure      400      {object}  errors.AppError           "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries [post]
func (h *FAQHandler) UpsertEntries(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req types.FAQBatchUpsertPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind FAQ upsert payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	taskID, err := h.knowledgeService.UpsertFAQEntries(ctx, kbID, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"task_id": taskID,
		},
	})
}

// CreateEntry godoc
// @Summary      Create an FAQ entry
// @Description  Creates a single FAQ entry synchronously
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id       path      string                true  "Knowledge base ID"
// @Param        request  body      types.FAQEntryPayload true  "FAQ entry"
// @Success      200      {object}  map[string]interface{}  "Created FAQ entry"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entry [post]
func (h *FAQHandler) CreateEntry(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req types.FAQEntryPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind FAQ entry payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	entry, err := h.knowledgeService.CreateFAQEntry(ctx, kbID, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entry,
	})
}

// UpdateEntry godoc
// @Summary      Update an FAQ entry
// @Description  Updates the specified FAQ entry
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id        path      string                true  "Knowledge base ID"
// @Param        entry_id  path      int                   true  "FAQ entry ID (seq_id)"
// @Param        request   body      types.FAQEntryPayload true  "FAQ entry"
// @Success      200       {object}  map[string]interface{}  "Updated"
// @Failure      400       {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries/{entry_id} [put]
func (h *FAQHandler) UpdateEntry(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req types.FAQEntryPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind FAQ entry payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	entrySeqID, err := strconv.ParseInt(c.Param("entry_id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.NewBadRequestError("entry_id must be an integer"))
		return
	}

	entry, err := h.knowledgeService.UpdateFAQEntry(ctx, kbID, entrySeqID, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entry,
	})
}

// UpdateEntryTagBatch godoc
// @Summary      Batch update FAQ tags
// @Description  Updates the tags of FAQ entries in batch
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "Knowledge base ID"
// @Param        request  body      object  true  "Tag update request"
// @Success      200      {object}  map[string]interface{}  "Updated"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries/tags [put]
func (h *FAQHandler) UpdateEntryTagBatch(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req faqEntryTagBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind FAQ entry tag batch payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	if err := h.knowledgeService.UpdateFAQEntryTagBatch(ctx, kbID, req.Updates); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// UpdateEntryFieldsBatch godoc
// @Summary      Batch update FAQ fields
// @Description  Updates several fields of FAQ entries in batch (is_enabled, is_recommended, tag_id)
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id       path      string                        true  "Knowledge base ID"
// @Param        request  body      types.FAQEntryFieldsBatchUpdate  true  "Field update request"
// @Success      200      {object}  map[string]interface{}        "Updated"
// @Failure      400      {object}  errors.AppError               "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries/fields [put]
func (h *FAQHandler) UpdateEntryFieldsBatch(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req types.FAQEntryFieldsBatchUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind FAQ entry fields batch payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	if err := h.knowledgeService.UpdateFAQEntryFieldsBatch(ctx, kbID, &req); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// DeleteEntries godoc
// @Summary      Batch delete FAQ entries
// @Description  Deletes the specified FAQ entries
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id       path      string  true  "Knowledge base ID"
// @Param        request  body      object{ids=[]int}  true  "FAQ IDs to delete (seq_id)"
// @Success      200      {object}  map[string]interface{}  "Deleted"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries [delete]
func (h *FAQHandler) DeleteEntries(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req faqDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf(ctx, "Failed to bind FAQ delete payload: %s", secutils.SanitizeForLog(err.Error()))
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	if err := h.knowledgeService.DeleteFAQEntries(ctx, kbID, req.IDs); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// SearchFAQ godoc
// @Summary      Search FAQ
// @Description  Searches FAQ entries with hybrid retrieval and two-level tag priority: first_priority_tag_ids ranks
// @Description  highest, then second_priority_tag_ids
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id       path      string                true  "Knowledge base ID"
// @Param        request  body      types.FAQSearchRequest  true  "Search request"
// @Success      200      {object}  map[string]interface{}  "Search results"
// @Failure      400      {object}  errors.AppError         "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/search [post]
func (h *FAQHandler) SearchFAQ(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req types.FAQSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind FAQ search payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	req.QueryText = secutils.SanitizeForLog(req.QueryText)
	if req.MatchCount <= 0 {
		req.MatchCount = 10
	}
	if req.MatchCount > 200 {
		req.MatchCount = 200
	}
	entries, err := h.knowledgeService.SearchFAQEntries(ctx, kbID, &req)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entries,
	})
}

// ExportEntries godoc
// @Summary      Export FAQ entries
// @Description  Exports all FAQ entries as CSV (default) or JSON. ?format=json returns an array compatible with
// @Description  FAQEntryPayload.
// @Tags         FAQ
// @Accept       json
// @Produce      text/csv
// @Produce      application/json
// @Param        id      path      string  true   "Knowledge base ID"
// @Param        format  query     string  false  "Export format: csv (default) or json"
// @Success      200     {file}    file    "Exported file"
// @Failure      400     {object}  errors.AppError  "Invalid request parameters"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries/export [get]
func (h *FAQHandler) ExportEntries(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))
	format := strings.ToLower(strings.TrimSpace(c.Query("format")))

	if format == "json" {
		jsonData, err := h.knowledgeService.ExportFAQEntriesJSON(ctx, kbID)
		if err != nil {
			logger.ErrorWithFields(ctx, err, nil)
			c.Error(err)
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=faq_export.json")
		c.Data(http.StatusOK, "application/json; charset=utf-8", jsonData)
		return
	}

	csvData, err := h.knowledgeService.ExportFAQEntries(ctx, kbID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	// Set response headers for CSV download
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=faq_export.csv")
	// Add BOM for Excel compatibility with UTF-8
	bom := []byte{0xEF, 0xBB, 0xBF}
	c.Data(http.StatusOK, "text/csv; charset=utf-8", append(bom, csvData...))
}

// GetEntry godoc
// @Summary      Get an FAQ entry
// @Description  Returns the details of one FAQ entry by ID
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id        path      string  true  "Knowledge base ID"
// @Param        entry_id  path      int     true  "FAQ entry ID (seq_id)"
// @Success      200       {object}  map[string]interface{}  "FAQ entry details"
// @Failure      400       {object}  errors.AppError         "Invalid request parameters"
// @Failure      404       {object}  errors.AppError         "Entry not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries/{entry_id} [get]
func (h *FAQHandler) GetEntry(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	entrySeqID, err := strconv.ParseInt(c.Param("entry_id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.NewBadRequestError("entry_id must be an integer"))
		return
	}

	entry, err := h.knowledgeService.GetFAQEntry(ctx, kbID, entrySeqID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entry,
	})
}

// GetImportProgress godoc
// @Summary      Get FAQ import progress
// @Description  Returns the progress of an FAQ import task
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        task_id  path      string  true  "Task ID"
// @Success      200      {object}  map[string]interface{}  "Import progress"
// @Failure      404      {object}  errors.AppError         "Task not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /faq/import/progress/{task_id} [get]
func (h *FAQHandler) GetImportProgress(c *gin.Context) {
	ctx := c.Request.Context()
	taskID := secutils.SanitizeForLog(c.Param("task_id"))
	if err := requireTaskProgressTenant(ctx, taskID); err != nil {
		c.Error(err)
		return
	}

	progress, err := h.knowledgeService.GetFAQImportProgress(ctx, taskID)
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

// UpdateLastImportResultDisplayStatus godoc
// @Summary      Update visibility of the last FAQ import result
// @Description  Shows or hides the import result summary card of an FAQ knowledge base
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id      path      string                                         true  "Knowledge base ID"
// @Param        request body      updateLastFAQImportResultDisplayStatusRequest  true  "Status update request"
// @Success      200     {object}  map[string]interface{}                         "Updated"
// @Failure      400     {object}  errors.AppError                                "Invalid request parameters"
// @Failure      404     {object}  errors.AppError                                "KB or import record not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/import/last-result/display [put]
func (h *FAQHandler) UpdateLastImportResultDisplayStatus(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	var req updateLastFAQImportResultDisplayStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind display status update payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	if err := h.knowledgeService.UpdateLastFAQImportResultDisplayStatus(ctx, kbID, req.DisplayStatus); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// AddSimilarQuestions godoc
// @Summary      Add similar questions
// @Description  Adds similar questions to the specified FAQ entry
// @Tags         FAQ
// @Accept       json
// @Produce      json
// @Param        id        path      string                      true  "Knowledge base ID"
// @Param        entry_id  path      int                         true  "FAQ entry ID (seq_id)"
// @Param        request   body      addSimilarQuestionsRequest  true  "Similar questions"
// @Success      200       {object}  map[string]interface{}      "Updated FAQ entry"
// @Failure      400       {object}  errors.AppError             "Invalid request parameters"
// @Failure      404       {object}  errors.AppError             "Entry not found"
// @Security     Bearer
// @Security     ApiKeyAuth
// @Router       /knowledge-bases/{id}/faq/entries/{entry_id}/similar-questions [post]
func (h *FAQHandler) AddSimilarQuestions(c *gin.Context) {
	ctx := c.Request.Context()
	kbID := secutils.SanitizeForLog(c.Param("id"))

	entrySeqID, err := strconv.ParseInt(c.Param("entry_id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.NewBadRequestError("entry_id must be an integer"))
		return
	}

	var req addSimilarQuestionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to bind add similar questions payload", err)
		_ = c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	entry, err := h.knowledgeService.AddSimilarQuestions(ctx, kbID, entrySeqID, req.SimilarQuestions)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    entry,
	})
}
