package doris

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/logger"
	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

// Stream Load constants.
const (
	// JSON body cap for one Stream Load batch (conservative, far below the Doris default streaming_load_max_mb=10240).
	// It bounds the tail latency of one HTTP request; larger payloads are split automatically.
	streamLoadMaxBatchBytes = 1 << 20 // 1 MiB

	// The Authorization header uses Basic auth. Stream Load also supports tokens,
	// but username/password is the common case and matches the MySQL protocol.
	headerAuthorization = "Authorization"
	headerExpect        = "Expect"
	headerContentType   = "Content-Type"
)

// streamLoadResponse is the Stream Load result body returned by Doris FE/BE.
//
// Key field: Status must be "Success" or "Publish Timeout" (data written but the publish transaction timed out,
// still a success). Any other status is a failure.
type streamLoadResponse struct {
	TxnId                  int64  `json:"TxnId"`
	Label                  string `json:"Label"`
	Status                 string `json:"Status"`
	Message                string `json:"Message"`
	NumberTotalRows        int64  `json:"NumberTotalRows"`
	NumberLoadedRows       int64  `json:"NumberLoadedRows"`
	NumberFilteredRows     int64  `json:"NumberFilteredRows"`
	NumberUnselectedRows   int64  `json:"NumberUnselectedRows"`
	LoadBytes              int64  `json:"LoadBytes"`
	LoadTimeMs             int64  `json:"LoadTimeMs"`
	BeginTxnTimeMs         int64  `json:"BeginTxnTimeMs"`
	StreamLoadPutTimeMs    int64  `json:"StreamLoadPutTimeMs"`
	ReadDataTimeMs         int64  `json:"ReadDataTimeMs"`
	WriteDataTimeMs        int64  `json:"WriteDataTimeMs"`
	CommitAndPublishTimeMs int64  `json:"CommitAndPublishTimeMs"`
	ErrorURL               string `json:"ErrorURL"`
}

// streamLoadURL builds the Stream Load HTTP endpoint for a table.
func (r *dorisRepository) streamLoadURL(table string) string {
	return fmt.Sprintf("%s/api/%s/%s/_stream_load",
		r.feHTTPBase, url.PathEscape(r.database), url.PathEscape(table))
}

// newDorisStreamLoadHTTPClient protects both the initial FE request and every
// FE -> BE redirect at connection time. Doris requires Basic auth to survive
// the 307 redirect, so redirect targets on another host must be explicitly
// trusted through the SSRF whitelist before credentials are forwarded.
func newDorisStreamLoadHTTPClient() *http.Client {
	cfg := secutils.DefaultSSRFSafeHTTPClientConfig()
	// Stream Load calls carry their own context deadline. Preserve the previous
	// client behaviour instead of imposing the generic 30-second HTTP timeout.
	cfg.Timeout = 0

	client := secutils.NewSSRFSafeHTTPClient(cfg)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= cfg.MaxRedirects {
			return fmt.Errorf("stopped after %d redirects", cfg.MaxRedirects)
		}
		if len(via) == 0 {
			return nil
		}

		// Stream Load deliberately replays PUT bodies to a BE. Keep this
		// exception local to Doris; generic clients must not replay requests
		// across origins. Every cross-origin destination must be trusted.
		source := via[0].URL
		sameOrigin := strings.EqualFold(source.Scheme, req.URL.Scheme) &&
			strings.EqualFold(source.Host, req.URL.Host)
		if !sameOrigin && !secutils.IsSSRFWhitelisted(req.URL.Hostname()) {
			return fmt.Errorf("%w: stream load target host %q is not trusted to receive credentials",
				secutils.ErrSSRFRedirectBlocked, req.URL.Hostname())
		}
		if strings.EqualFold(source.Scheme, "https") && !strings.EqualFold(req.URL.Scheme, "https") {
			return fmt.Errorf("%w: stream load HTTPS downgrade is forbidden", secutils.ErrSSRFRedirectBlocked)
		}
		if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
			return fmt.Errorf("%w: %w", secutils.ErrSSRFRedirectBlocked, err)
		}

		// The shared transport still validates each URL and connection. Preserve
		// load options and restore Basic auth only after checking the destination.
		req.Header.Set(headerAuthorization, via[0].Header.Get(headerAuthorization))
		return nil
	}
	return client
}

// partialUpdateRows writes rows back to the target table in Stream Load partial update mode.
//
// columns are the columns in this partial update (must include the UNIQUE KEY column, "id").
// Each entry in rows is a field value array the same length as columns.
//
// Implementation notes:
//  1. The body is a JSON array, with the strip_outer_array=true header.
//  2. partial_columns=true and merge_type=APPEND trigger Doris partial update mode
//     (the standard approach for Doris 4.1 UNIQUE KEY MoW tables).
//  3. Batches are split by streamLoadMaxBatchBytes so no single request is too large.
//  4. 307 handling: the Doris FE redirects to a BE and net/http follows by default;
//     GetBody must be replayable (satisfied by building Body with bytes.NewReader).
func (r *dorisRepository) partialUpdateRows(ctx context.Context,
	table string, columns []string, rows []map[string]any,
) error {
	if len(rows) == 0 {
		return nil
	}
	for _, batch := range chunkRows(rows, streamLoadMaxBatchBytes) {
		if err := r.streamLoadOnce(ctx, table, columns, batch); err != nil {
			return err
		}
	}
	return nil
}

// streamLoadOnce sends one Stream Load HTTP request.
func (r *dorisRepository) streamLoadOnce(ctx context.Context,
	table string, columns []string, rows []map[string]any,
) error {
	log := logger.GetLogger(ctx)
	if len(rows) == 0 {
		return nil
	}

	body, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("marshal stream load body: %w", err)
	}

	url := r.streamLoadURL(table)
	// Validate at the final outbound boundary as well as when user-supplied
	// vector-store configuration is created. This covers stored/env configs and
	// keeps the tainted value from reaching http.Client.Do unchecked.
	if err := secutils.ValidateURLForSSRF(url); err != nil {
		return fmt.Errorf("stream load URL blocked by SSRF validation: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build stream load request: %w", err)
	}

	// GetBody lets a redirect re-read the body (the FE -> BE 307 must resend the PUT body).
	bodyCopy := append([]byte(nil), body...)
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyCopy)), nil
	}
	req.ContentLength = int64(len(body))

	auth := base64.StdEncoding.EncodeToString([]byte(r.username + ":" + r.password))
	req.Header.Set(headerAuthorization, "Basic "+auth)
	req.Header.Set(headerExpect, "100-continue")
	req.Header.Set(headerContentType, "application/json")
	req.Header.Set("format", "json")
	req.Header.Set("strip_outer_array", "true")
	req.Header.Set("partial_columns", "true")
	req.Header.Set("columns", strings.Join(columns, ","))
	req.Header.Set("merge_type", "APPEND")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream load HTTP: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read stream load response: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("stream load HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result streamLoadResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("decode stream load response: %w (raw=%s)", err, string(respBody))
	}

	switch result.Status {
	case "Success", "Publish Timeout":
		log.Infof("[Doris] Stream load %s OK: rows=%d, loaded=%d, label=%s",
			table, result.NumberTotalRows, result.NumberLoadedRows, result.Label)
		return nil
	default:
		return fmt.Errorf("stream load failed: status=%s msg=%s err_url=%s",
			result.Status, result.Message, result.ErrorURL)
	}
}

// chunkRows splits rows by cumulative JSON body size, at most maxBytes per segment.
//
// The real JSON cost is about the marshaled byte count, and each row's marshaled size
// plus comma and array brackets approximates this estimate. The rough estimate avoids marshaling each segment.
func chunkRows(rows []map[string]any, maxBytes int) [][]map[string]any {
	if len(rows) == 0 {
		return nil
	}

	var (
		out    [][]map[string]any
		curr   []map[string]any
		size   int
		header = 2 // "[" + "]"
	)

	for _, row := range rows {
		raw, err := json.Marshal(row)
		if err != nil {
			// On marshal failure put this row in its own segment; streamLoadOnce marshals again and reports the error.
			if len(curr) > 0 {
				out = append(out, curr)
			}
			out = append(out, []map[string]any{row})
			curr = nil
			size = 0
			continue
		}
		// Add the comma (for every row after the first).
		need := len(raw)
		if len(curr) > 0 {
			need++
		}
		if size+need+header > maxBytes && len(curr) > 0 {
			out = append(out, curr)
			curr = nil
			size = 0
		}
		curr = append(curr, row)
		size += need
	}
	if len(curr) > 0 {
		out = append(out, curr)
	}
	return out
}

// ---------------------------------------------------------------------------
// Business methods: BatchUpdateChunkEnabledStatus / BatchUpdateChunkTagID
// ---------------------------------------------------------------------------

// BatchUpdateChunkEnabledStatus updates the is_enabled field of chunks in bulk.
// legacy mode uses Stream Load partial update; inner_product_duplicate mode reads
// whole rows and writes them back with replaceRows.
func (r *dorisRepository) BatchUpdateChunkEnabledStatus(ctx context.Context,
	chunkStatusMap map[string]bool,
) error {
	if len(chunkStatusMap) == 0 {
		return nil
	}
	compatMode, err := r.resolveCompatMode(ctx)
	if err != nil {
		return err
	}
	if !compatMode.usesRewriteChunkUpdates() {
		return r.batchUpdateChunkEnabledStatusLegacy(ctx, chunkStatusMap)
	}

	chunkIDs := make([]string, 0, len(chunkStatusMap))
	for id := range chunkStatusMap {
		chunkIDs = append(chunkIDs, id)
	}

	return r.rewriteChunkRows(ctx, chunkIDs, func(row *DorisVectorEmbedding) bool {
		enabled, ok := chunkStatusMap[row.ChunkID]
		if !ok || row.IsEnabled == enabled {
			return false
		}
		row.IsEnabled = enabled
		return true
	}, "rewrite is_enabled")
}

// BatchUpdateChunkTagID updates the tag_id field of chunks in bulk. Same logic as EnabledStatus.
func (r *dorisRepository) BatchUpdateChunkTagID(ctx context.Context,
	chunkTagMap map[string]string,
) error {
	if len(chunkTagMap) == 0 {
		return nil
	}
	compatMode, err := r.resolveCompatMode(ctx)
	if err != nil {
		return err
	}
	if !compatMode.usesRewriteChunkUpdates() {
		return r.batchUpdateChunkTagIDLegacy(ctx, chunkTagMap)
	}

	chunkIDs := make([]string, 0, len(chunkTagMap))
	for id := range chunkTagMap {
		chunkIDs = append(chunkIDs, id)
	}

	return r.rewriteChunkRows(ctx, chunkIDs, func(row *DorisVectorEmbedding) bool {
		tagID, ok := chunkTagMap[row.ChunkID]
		if !ok || row.TagID == tagID {
			return false
		}
		row.TagID = tagID
		return true
	}, "rewrite tag_id")
}

func (r *dorisRepository) rewriteChunkRows(ctx context.Context,
	chunkIDs []string,
	mutate func(*DorisVectorEmbedding) bool,
	action string,
) error {
	if len(chunkIDs) == 0 {
		return nil
	}

	tables, err := r.listEmbeddingTables(ctx)
	if err != nil {
		return fmt.Errorf("list tables: %w", err)
	}

	for _, table := range tables {
		rows, err := r.loadRowsByChunkIDs(ctx, table, chunkIDs)
		if err != nil {
			return fmt.Errorf("load chunk rows from %s: %w", table, err)
		}

		updated := make([]*DorisVectorEmbedding, 0, len(rows))
		for _, row := range rows {
			if !mutate(row) {
				continue
			}
			updated = append(updated, row)
		}
		if len(updated) == 0 {
			continue
		}

		if err := r.replaceRows(ctx, table, updated); err != nil {
			return fmt.Errorf("%s in %s: %w", action, table, err)
		}
	}
	return nil
}

func (r *dorisRepository) batchUpdateChunkEnabledStatusLegacy(ctx context.Context,
	chunkStatusMap map[string]bool,
) error {
	chunkIDs := make([]string, 0, len(chunkStatusMap))
	for id := range chunkStatusMap {
		chunkIDs = append(chunkIDs, id)
	}

	mapping, err := r.lookupChunkRowKeys(ctx, chunkIDs)
	if err != nil {
		return err
	}

	byTable := make(map[string][]map[string]any)
	for chunkID, locations := range mapping {
		enabled, ok := chunkStatusMap[chunkID]
		if !ok {
			continue
		}
		for _, loc := range locations {
			byTable[loc.table] = append(byTable[loc.table], map[string]any{
				fieldID:        loc.id,
				fieldIsEnabled: enabled,
			})
		}
	}
	for table, rows := range byTable {
		if err := r.partialUpdateRows(ctx, table, []string{fieldID, fieldIsEnabled}, rows); err != nil {
			return fmt.Errorf("partial update is_enabled in %s: %w", table, err)
		}
	}
	return nil
}

func (r *dorisRepository) batchUpdateChunkTagIDLegacy(ctx context.Context,
	chunkTagMap map[string]string,
) error {
	chunkIDs := make([]string, 0, len(chunkTagMap))
	for id := range chunkTagMap {
		chunkIDs = append(chunkIDs, id)
	}

	mapping, err := r.lookupChunkRowKeys(ctx, chunkIDs)
	if err != nil {
		return err
	}

	byTable := make(map[string][]map[string]any)
	for chunkID, locations := range mapping {
		tagID, ok := chunkTagMap[chunkID]
		if !ok {
			continue
		}
		for _, loc := range locations {
			byTable[loc.table] = append(byTable[loc.table], map[string]any{
				fieldID:    loc.id,
				fieldTagID: tagID,
			})
		}
	}
	for table, rows := range byTable {
		if err := r.partialUpdateRows(ctx, table, []string{fieldID, fieldTagID}, rows); err != nil {
			return fmt.Errorf("partial update tag_id in %s: %w", table, err)
		}
	}
	return nil
}

func (r *dorisRepository) loadRowsByChunkIDs(ctx context.Context,
	table string, chunkIDs []string,
) ([]*DorisVectorEmbedding, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(chunkIDs))
	args := make([]any, len(chunkIDs))
	for i, v := range chunkIDs {
		placeholders[i] = "?"
		args[i] = v
	}

	stmt := fmt.Sprintf(
		"SELECT %s FROM `%s` WHERE %s IN (%s)",
		strings.Join(columnsForCopy, ", "),
		table,
		fieldChunkID,
		strings.Join(placeholders, ", "),
	)
	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	batch, err := scanCopyRows(rows)
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	return batch, nil
}

// rowLocation is the table holding a row and its primary key id.
type rowLocation struct {
	table string
	id    string
}

// lookupChunkRowKeys finds where the given chunkIDs live across all <base>_<dim> tables:
//   - key: chunk_id
//   - value: [(table, id), ...], because one chunk can have copies in tables of several dimensions.
//
// The lookup covers every matching table from listEmbeddingTables, one query per table:
// SELECT id, chunk_id FROM <table> WHERE chunk_id IN (?, ?, ...)。
func (r *dorisRepository) lookupChunkRowKeys(ctx context.Context,
	chunkIDs []string,
) (map[string][]rowLocation, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}
	tables, err := r.listEmbeddingTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	if len(tables) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(chunkIDs))
	args := make([]any, len(chunkIDs))
	for i, v := range chunkIDs {
		placeholders[i] = "?"
		args[i] = v
	}

	out := make(map[string][]rowLocation)
	for _, table := range tables {
		stmt := fmt.Sprintf(
			"SELECT %s, %s FROM `%s` WHERE %s IN (%s)",
			fieldID, fieldChunkID, table, fieldChunkID, strings.Join(placeholders, ", "),
		)
		rows, err := r.db.QueryContext(ctx, stmt, args...)
		if err != nil {
			return nil, fmt.Errorf("lookup chunk row keys in %s: %w", table, err)
		}
		for rows.Next() {
			var id, chunkID string
			if err := rows.Scan(&id, &chunkID); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan row keys: %w", err)
			}
			out[chunkID] = append(out[chunkID], rowLocation{table: table, id: id})
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return out, nil
}
