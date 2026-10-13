package handler

import (
	"archive/zip"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/filetransport"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
)

const (
	maxBatchDownloadFiles             = 200
	maxBatchDownloadBytes       int64 = 512 * 1024 * 1024
	maxConcurrentBatchDownloads       = 4
)

var batchDownloadSlots = make(chan struct{}, maxConcurrentBatchDownloads)

// BatchDownloadKnowledgeRequest lists the documents to download from one knowledge base.
type BatchDownloadKnowledgeRequest struct {
	IDs []string `json:"ids" binding:"required,min=1,max=200,dive,required,max=128"`
}

type knowledgeDownloadEntry struct {
	ID         string
	FolderPath string
}

// BatchDownloadKnowledge godoc
// @Summary Batch download knowledge files
// @Description Pack up to 200 documents of one knowledge base into a ZIP, with at most 512 MiB of original content in
// @Description total. Entries without an original file are skipped; on missing access, a cross-knowledge-base ID or a
// @Description read failure, no partial archive is returned.
// @Tags Knowledge
// @Accept json
// @Produce application/zip
// @Param id path string true "Knowledge base ID"
// @Param request body BatchDownloadKnowledgeRequest true "Document IDs"
// @Success 200 {file} file "ZIP archive"
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Failure 403 {object} errors.ErrorResponse
// @Failure 404 {object} errors.ErrorResponse
// @Failure 429 {object} errors.ErrorResponse
// @Failure 500 {object} errors.ErrorResponse
// @Security Bearer
// @Security ApiKeyAuth
// @Router /knowledge-bases/{id}/knowledge/batch-download [post]
func (h *KnowledgeHandler) BatchDownloadKnowledge(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	var req BatchDownloadKnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.NewBadRequestError("Select 1 to 200 documents to download"))
		return
	}

	kbID := c.Param("id")
	grant, err := resolveHandlerKBAccessFor(
		c, kbID, h.kbService, h.kbShareService, h.agentShareService, types.OrgRoleEditor,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	tenantID := grant.EffectiveTenantID
	ctx := types.WithExecutionTenant(c.Request.Context(), tenantID)
	ids := uniqueKnowledgeDownloadIDs(req.IDs)
	if len(ids) == 0 {
		_ = c.Error(errors.NewBadRequestError("Document ID cannot be empty"))
		return
	}

	items, err := h.kgService.GetKnowledgeBatch(ctx, tenantID, ids)
	if err != nil {
		_ = c.Error(errors.NewInternalServerError("Could not load the documents to download, please try again later"))
		return
	}
	byID := make(map[string]*types.Knowledge, len(items))
	for _, item := range items {
		if item != nil && item.KnowledgeBaseID == kbID && item.TenantID == tenantID {
			byID[item.ID] = item
		}
	}
	entries := make([]knowledgeDownloadEntry, 0, len(ids))
	// Check the whole batch before reading any file so no ID from another knowledge base or tenant slips in.
	for _, id := range ids {
		item := byID[id]
		if item == nil {
			_ = c.Error(errors.NewNotFoundError("Some documents are missing or outside this knowledge base"))
			return
		}
		if !item.IsManual() && item.FilePath == "" {
			continue
		}
		entries = append(entries, knowledgeDownloadEntry{ID: id, FolderPath: item.FolderPath})
	}
	if len(entries) == 0 {
		_ = c.Error(errors.NewBadRequestError("The selected documents have no original file to download"))
		return
	}

	if !tryAcquireBatchDownloadSlot() {
		_ = c.Error(errors.NewTooManyRequestsError("Too many batch downloads in progress, please try again later"))
		return
	}
	defer releaseBatchDownloadSlot()

	// Build the whole archive in a temp file first so a read failure never sends a partial ZIP.
	archive, err := os.CreateTemp("", "ai-kb-download-*.zip")
	if err != nil {
		_ = c.Error(errors.NewInternalServerError("Could not create the download archive, please try again later"))
		return
	}
	archivePath := archive.Name()
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archivePath)
	}()
	if err := writeKnowledgeDownloadArchive(
		ctx, archive, entries, h.kgService.GetKnowledgeFile, maxBatchDownloadBytes,
	); err != nil {
		mapped := mapKnowledgeDownloadError(err)
		if appErr, ok := errors.IsAppError(mapped); !ok || appErr.HTTPCode >= 500 {
			logger.ErrorWithFields(ctx, err, nil)
		}
		_ = c.Error(mapped)
		return
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		_ = c.Error(errors.NewInternalServerError("Could not read the download archive, please try again later"))
		return
	}
	filename := "knowledge-files-" + time.Now().Format("20060102-150405") + ".zip"
	if err := filetransport.Serve(c.Writer, c.Request, archive, filetransport.Options{
		Filename: filename, Download: true, ContentType: "application/zip", CacheControl: "private, no-store",
	}); err != nil {
		logger.Errorf(ctx, "Failed to send knowledge archive: %v", err)
	}
}

func tryAcquireBatchDownloadSlot() bool {
	select {
	case batchDownloadSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseBatchDownloadSlot() {
	<-batchDownloadSlots
}

func uniqueKnowledgeDownloadIDs(input []string) []string {
	ids := make([]string, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, id := range input {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func isKnowledgeDownloadCanceled(err error) bool {
	return err != nil && (stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded))
}

func mapKnowledgeDownloadError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.IsAppError(err); ok {
		return err
	}
	if isKnowledgeDownloadCanceled(err) {
		return errors.NewBadRequestError("Download cancelled")
	}
	return err
}

type knowledgeDownloadOpener func(context.Context, string) (io.ReadCloser, string, error)

func writeKnowledgeDownloadArchive(
	ctx context.Context,
	output io.Writer,
	entries []knowledgeDownloadEntry,
	open knowledgeDownloadOpener,
	limit int64,
) error {
	writer := zip.NewWriter(output)
	usedNames := make(map[string]bool, len(entries))
	var total int64
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		file, filename, err := open(ctx, entry.ID)
		if err != nil {
			if isKnowledgeDownloadCanceled(err) {
				return err
			}
			return errors.NewInternalServerError("Some files could not be read, no archive created").WithDetails(
				gin.H{"knowledge_id": entry.ID},
			)
		}
		name := uniqueKnowledgeDownloadZipPath(entry.FolderPath, filename, usedNames)
		// Store, not deflate: PDF, Office and similar files are already compressed.
		zipEntry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			_ = file.Close()
			return errors.NewInternalServerError("Could not write the download archive, please try again later")
		}
		n, copyErr := io.Copy(zipEntry, io.LimitReader(
			&knowledgeDownloadReader{ctx: ctx, reader: file}, limit-total+1,
		))
		closeErr := file.Close()
		total += n
		if total > limit {
			return errors.NewBadRequestError("The selected files exceed 512 MiB, download in smaller batches")
		}
		if isKnowledgeDownloadCanceled(copyErr) {
			return copyErr
		}
		if copyErr != nil || closeErr != nil {
			return errors.NewInternalServerError("Some files failed to read, no archive created").WithDetails(
				gin.H{"knowledge_id": entry.ID},
			)
		}
	}
	if err := writer.Close(); err != nil {
		return errors.NewInternalServerError("Could not finish the download archive, please try again later")
	}
	return nil
}

// knowledgeDownloadReader checks for cancellation before each read, so packing stops once the user leaves the page.
type knowledgeDownloadReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *knowledgeDownloadReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func uniqueKnowledgeDownloadName(filename string, used map[string]bool) string {
	return uniqueKnowledgeDownloadZipPath("", filename, used)
}

func uniqueKnowledgeDownloadZipPath(folderPath, filename string, used map[string]bool) string {
	base := sanitizeDownloadFileName(filename)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	dir := sanitizeDownloadFolderPath(folderPath)
	name := base
	if dir != "" {
		name = dir + "/" + base
	}
	for index := 2; used[strings.ToLower(name)]; index++ {
		suffixed := fmt.Sprintf("%s (%d)%s", stem, index, ext)
		if dir != "" {
			name = dir + "/" + suffixed
		} else {
			name = suffixed
		}
	}
	used[strings.ToLower(name)] = true
	return name
}

func sanitizeDownloadFileName(filename string) string {
	filename = path.Base(strings.ReplaceAll(filename, "\\", "/"))
	filename = sanitizeDownloadRunes(filename)
	filename = strings.Trim(filename, " .")
	if filename == "" {
		filename = "document"
	}
	ext := path.Ext(filename)
	if len(ext) > 20 {
		ext = ""
	}
	stem := strings.TrimSuffix(filename, ext)
	stem = shortenDownloadName(stem, 180)
	stem = strings.TrimRight(stem, " .")
	if stem == "" {
		stem = "document"
	}
	return escapeWindowsReservedName(stem) + ext
}

func sanitizeDownloadFolderPath(folderPath string) string {
	folderPath = strings.ReplaceAll(folderPath, "\\", "/")
	var parts []string
	var total int
	for _, part := range strings.Split(folderPath, "/") {
		part = sanitizeDownloadRunes(part)
		part = strings.Trim(part, " .")
		if part == "" || part == "." || part == ".." {
			continue
		}
		part = shortenDownloadName(part, 80)
		part = strings.TrimRight(part, " .")
		if part == "" {
			continue
		}
		part = escapeWindowsReservedName(part)
		if total+len(part)+1 > 160 {
			break
		}
		parts = append(parts, part)
		total += len(part) + 1
	}
	return strings.Join(parts, "/")
}

func sanitizeDownloadRunes(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, value)
}

func shortenDownloadName(value string, limit int) string {
	var shortened strings.Builder
	for _, r := range value {
		if shortened.Len()+len(string(r)) > limit {
			break
		}
		shortened.WriteRune(r)
	}
	return shortened.String()
}

func escapeWindowsReservedName(stem string) string {
	reserved := strings.ToUpper(strings.Split(stem, ".")[0])
	reservedRunes := []rune(reserved)
	if reserved == "CON" || reserved == "PRN" || reserved == "AUX" || reserved == "NUL" ||
		(len(reservedRunes) == 4 && (strings.HasPrefix(reserved, "COM") || strings.HasPrefix(reserved, "LPT")) &&
			strings.ContainsRune("123456789¹²³", reservedRunes[3])) {
		return "_" + stem
	}
	return stem
}
