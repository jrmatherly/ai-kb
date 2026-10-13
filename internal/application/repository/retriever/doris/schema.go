package doris

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jrmatherly/ai-kb/internal/logger"
)

// Default bucket and replica counts. Doris uses the cluster default when PROPERTIES omits them;
// these are conservative values that suit a single node or small cluster.
const (
	defaultBucketsNum     = 10
	defaultReplicationNum = 1

	// Maximum wait for the ANN index to become ready. An unready index does not block writes,
	// only ensureTable itself (on first table creation), so 30s is acceptable.
	annReadyTimeout = 30 * time.Second
	annReadyPoll    = 1 * time.Second
)

// getTableName returns the physical table name for a dimension: <base>_<dim>.
//
// Matches the Qdrant/Milvus/Weaviate collection naming, so data from
// different embedding models (different dimensions) never collides.
func (r *dorisRepository) getTableName(dimension int) string {
	return fmt.Sprintf("%s_%d", r.tableBaseName, dimension)
}

// ensureTable makes sure the table for the target dimension exists;
// if not, it runs CREATE TABLE IF NOT EXISTS and then polls for the ANN index.
//
// Called before every Save / BatchSave. The result is cached in initializedTables,
// so a process runs SHOW TABLES + DDL only once per dimension.
func (r *dorisRepository) ensureTable(ctx context.Context, dimension int) error {
	if _, ok := r.initializedTables.Load(dimension); ok {
		return nil
	}
	compatMode, err := r.resolveCompatMode(ctx)
	if err != nil {
		return err
	}

	log := logger.GetLogger(ctx)
	tableName := r.getTableName(dimension)

	exists, err := r.tableExists(ctx, tableName)
	if err != nil {
		log.Errorf("[Doris] Failed to check table existence: %v", err)
		return fmt.Errorf("check table existence: %w", err)
	}

	if !exists {
		log.Infof("[Doris] Creating table %s with dimension %d in compat mode %s", tableName, dimension, compatMode)
		if err := r.createTable(ctx, tableName, dimension, compatMode); err != nil {
			log.Errorf("[Doris] Failed to create table: %v", err)
			return fmt.Errorf("create table: %w", err)
		}

		// Doris builds the ANN index asynchronously. Poll for readiness in a background goroutine
		// so writes do not block: until the index is ready, retrieval falls back to brute force (correct but slow),
		// which beats stalling the first write for 30s.
		go func(tn string) {
			// Separate context (with timeout) so a canceled request ctx does not stop the background poll.
			bgCtx, cancel := context.WithTimeout(context.Background(), annReadyTimeout)
			defer cancel()
			if err := r.waitANNReady(bgCtx, tn); err != nil {
				logger.GetLogger(bgCtx).Warnf(
					"[Doris] ANN index for %s not ready within %s: %v "+
						"(queries may fall back to brute force temporarily)",
					tn, annReadyTimeout, err)
				return
			}
			logger.GetLogger(bgCtx).Infof("[Doris] ANN index for %s ready", tn)
		}(tableName)
	}

	r.initializedTables.Store(dimension, true)
	return nil
}

// tableExists checks information_schema for the table.
//
// Not SHOW TABLES because Doris 4.1 treats SHOW TABLES LIKE as case sensitive,
// and information_schema is more MySQL compatible.
func (r *dorisRepository) tableExists(ctx context.Context, tableName string) (bool, error) {
	const q = `SELECT COUNT(1) FROM information_schema.tables
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`
	var n int
	if err := r.db.QueryRowContext(ctx, q, r.database, tableName).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// createTable issues the CREATE TABLE DDL. Doris DDL is synchronous (except ANN index builds),
// so success means the table is writable.
func (r *dorisRepository) createTable(ctx context.Context, tableName string, dimension int, compatMode dorisCompatMode) error {
	buckets := r.bucketsNum
	if buckets <= 0 {
		buckets = defaultBucketsNum
	}
	replication := r.replicationNum
	if replication <= 0 {
		replication = defaultReplicationNum
	}

	ddl := buildCreateTableDDL(tableName, dimension, buckets, replication, compatMode)
	_, err := r.db.ExecContext(ctx, ddl)
	if err != nil && compatMode == dorisCompatModeLegacy {
		return fmt.Errorf(
			"legacy Doris table creation failed: %w. If your Doris build rejects ANN indexes on UNIQUE KEY tables, set %s=%s before creating embedding tables. %s is not interchangeable after %s_* tables are created",
			err,
			envDorisCompatMode,
			dorisCompatModeInnerProductDuplicate,
			envDorisCompatMode,
			r.tableBaseName,
		)
	}
	return err
}

// buildCreateTableDDL generates the CREATE TABLE DDL for a dimension.
//
// Key points:
//   - DUPLICATE KEY(id): the table model current Doris/SelectDB requires for ANN indexes.
//     AI KB keeps replace-by-id write semantics with delete + insert in Go.
//   - INVERTED indexes cover every filter column, plus a full-text index on content with Chinese segmentation.
//   - The ANN index uses HNSW + inner_product; vectors are normalized before writes and queries,
//     so the overall cosine similarity semantics match the other vector stores.
//
// The dimension / buckets / replication numbers in the DDL are formatted in Go;
// there is no SQL injection risk (they all come from controlled IndexConfig ints).
func buildCreateTableDDL(tableName string, dimension, buckets, replication int, compatMode dorisCompatMode) string {
	metricType := "inner_product"
	keyMode := "DUPLICATE KEY(id)"
	properties := fmt.Sprintf("\t\"replication_num\"=\"%d\"", replication)
	if compatMode == dorisCompatModeLegacy {
		metricType = "cosine_distance"
		keyMode = "UNIQUE KEY(id)"
		properties = fmt.Sprintf("\t\"replication_num\"=\"%d\",\n\t\"enable_unique_key_merge_on_write\"=\"true\"", replication)
	}

	const tpl = `CREATE TABLE IF NOT EXISTS ` + "`%s`" + ` (
    id                VARCHAR(64)  NOT NULL,
    chunk_id          VARCHAR(64),
    knowledge_id      VARCHAR(64),
    knowledge_base_id VARCHAR(64),
    source_id         VARCHAR(255),
    source_type       INT,
    tag_id            VARCHAR(64),
    is_enabled        BOOLEAN,
    content           TEXT,
    embedding         ARRAY<FLOAT> NOT NULL,
    INDEX idx_chunk    (chunk_id)          USING INVERTED,
    INDEX idx_kb       (knowledge_base_id) USING INVERTED,
    INDEX idx_kid      (knowledge_id)      USING INVERTED,
    INDEX idx_src      (source_id)         USING INVERTED,
    INDEX idx_tag      (tag_id)            USING INVERTED,
    INDEX idx_enabled  (is_enabled)        USING INVERTED,
    INDEX idx_content  (content)           USING INVERTED PROPERTIES("parser"="chinese","support_phrase"="true"),
    INDEX idx_emb      (embedding)         USING ANN PROPERTIES(
        "index_type"="hnsw",
		"metric_type"="%s",
        "dim"="%d",
        "max_degree"="32",
        "ef_construction"="200"
    )
) ENGINE=OLAP
%s
DISTRIBUTED BY HASH(id) BUCKETS %d
PROPERTIES(
	%s
);`
	return fmt.Sprintf(tpl, tableName, metricType, dimension, keyMode, buckets, properties)
}

// waitANNReady polls SHOW INDEX until the ANN index reaches FINISHED.
//
// Doris builds the ANN index asynchronously after table creation; meanwhile
// queries fall back to brute force (correct but slow).
// This is a best-effort wait: if not ready in time it only logs a warning and does not block writes.
func (r *dorisRepository) waitANNReady(ctx context.Context, tableName string) error {
	deadline := time.Now().Add(annReadyTimeout)
	for {
		ready, err := r.annIndexReady(ctx, tableName)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("ann index not ready within %s", annReadyTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(annReadyPoll):
		}
	}
}

// annIndexReady checks whether the ANN index State is FINISHED.
//
// SHOW INDEX FROM <table> returns many columns on Doris, in an order that varies slightly between minor versions,
// so columns are matched by name (information_schema.statistics plus a custom view does not work;
// scanning SHOW INDEX output is enough).
//
// Compatibility: if SHOW INDEX has no idx_emb row (very old versions), treat the index as ready
// so differences in Doris version output cannot hang startup.
func (r *dorisRepository) annIndexReady(ctx context.Context, tableName string) (bool, error) {
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf("SHOW INDEX FROM `%s`", tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	keyNameIdx, stateIdx := -1, -1
	for i, c := range cols {
		switch strings.ToLower(c) {
		case "key_name":
			keyNameIdx = i
		case "state", "index_state":
			stateIdx = i
		}
	}

	for rows.Next() {
		// sql.RawBytes accepts any column type.
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return false, err
		}

		var keyName, state string
		if keyNameIdx >= 0 {
			keyName = bytesToString(raw[keyNameIdx])
		}
		if stateIdx >= 0 {
			state = bytesToString(raw[stateIdx])
		}

		if keyName != "idx_emb" {
			continue
		}
		if stateIdx < 0 {
			// Old versions do not expose a state column; assume ready.
			return true, nil
		}
		if !strings.EqualFold(state, "FINISHED") &&
			!strings.EqualFold(state, "NORMAL") {
			return false, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	// Reaching here means one of:
	//   1. the idx_emb row was found and its state is FINISHED/NORMAL (or stateIdx<0 on old versions);
	//   2. no idx_emb row was found (very old Doris does not expose that index name).
	// Both count as ready. The not-ready branch already returned false inside the loop.
	return true, nil
}

// listEmbeddingTables returns every table named <base>_% in the current database,
// for keyword retrieval and cross-dimension BatchUpdate.
func (r *dorisRepository) listEmbeddingTables(ctx context.Context) ([]string, error) {
	const q = `SELECT TABLE_NAME FROM information_schema.tables
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME LIKE ?`
	rows, err := r.db.QueryContext(ctx, q, r.database, r.tableBaseName+"\\_%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// bytesToString converts the raw any from SHOW INDEX (usually []byte or string)
// to a string safely.
func bytesToString(v any) string {
	switch s := v.(type) {
	case []byte:
		return string(s)
	case string:
		return s
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", s)
	}
}
