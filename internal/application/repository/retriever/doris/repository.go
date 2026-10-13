package doris

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

const (
	defaultTableBaseName = "aikb_embeddings"
	envDorisTablePrefix  = "DORIS_TABLE_PREFIX"
)

// NewDorisRetrieveEngineRepository creates the Doris retrieval engine repository.
//
// Parameters:
//   - db: *sql.DB over the MySQL protocol. The caller sets SetMaxOpenConns and similar.
//   - feHTTPBase: FE HTTP base address for Stream Load (with scheme), such as "http://doris-fe:8030".
//   - username/password: credentials shared by MySQL and Stream Load.
//   - database: target database name (used in the MySQL DSN and the Stream Load URL path).
//   - indexCfg: optional. When nil, falls back to environment variables and defaults (the env path).
func NewDorisRetrieveEngineRepository(
	db *sql.DB,
	feHTTPBase, username, password, database string,
	indexCfg *types.IndexConfig,
) interfaces.RetrieveEngineRepository {
	log := logger.GetLogger(context.Background())
	log.Info("[Doris] Initializing Doris retriever engine repository")

	tableBaseName := types.ResolveCollectionName(indexCfg, envDorisTablePrefix, defaultTableBaseName)
	compatMode, invalidCompatMode := resolveConfiguredDorisCompatMode()
	if invalidCompatMode != "" {
		log.Warnf("[Doris] Invalid %s=%q, defaulting to %s", envDorisCompatMode, invalidCompatMode, dorisCompatModeAuto)
	}

	repo := &dorisRepository{
		db:                  db,
		httpClient:          newDorisStreamLoadHTTPClient(),
		feHTTPBase:          strings.TrimRight(feHTTPBase, "/"),
		username:            username,
		password:            password,
		database:            database,
		tableBaseName:       tableBaseName,
		bucketsNum:          indexCfg.GetBucketsNum(0),
		replicationNum:      indexCfg.GetReplicationNum(0),
		compatModeRequested: compatMode,
	}
	log.Infof("[Doris] Repository initialized: db=%s, base=%s, fe_http=%s, compat_mode=%s",
		database, tableBaseName, repo.feHTTPBase, repo.compatModeRequested)
	if os.Getenv(envDorisCompatMode) == "" {
		log.Infof("[Doris] %s not set, defaulting to %s and probing on first use", envDorisCompatMode, dorisCompatModeAuto)
	}
	return repo
}

func (r *dorisRepository) EngineType() types.RetrieverEngineType {
	return types.DorisRetrieverEngineType
}

func (r *dorisRepository) Support() []types.RetrieverType {
	return []types.RetrieverType{types.KeywordsRetrieverType, types.VectorRetrieverType}
}

// EstimateStorageSize estimates the storage bytes for a list of IndexInfo.
//
// Same algorithm as Qdrant: payload field lengths + vector bytes + HNSW neighbors + metadata.
func (r *dorisRepository) EstimateStorageSize(_ context.Context,
	indexInfoList []*types.IndexInfo, params map[string]any,
) int64 {
	var total int64
	for _, info := range indexInfoList {
		emb := toDorisVectorEmbedding(info, params, dorisCompatModeInnerProductDuplicate)
		total += calculateStorageSize(emb)
	}
	return total
}

// Save writes one record to the table for its dimension. BatchSave rejects empty vectors.
func (r *dorisRepository) Save(ctx context.Context,
	info *types.IndexInfo, additionalParams map[string]any,
) error {
	return r.BatchSave(ctx, []*types.IndexInfo{info}, additionalParams)
}

// BatchSave groups one batch of IndexInfo by dimension. Doris ANN-compatible tables use
// DUPLICATE KEY, so this does an explicit delete + insert by id to keep replace semantics.
func (r *dorisRepository) BatchSave(ctx context.Context,
	indexInfoList []*types.IndexInfo, additionalParams map[string]any,
) error {
	log := logger.GetLogger(ctx)
	if len(indexInfoList) == 0 {
		return nil
	}
	compatMode, err := r.resolveCompatMode(ctx)
	if err != nil {
		return err
	}

	groups := make(map[int][]*DorisVectorEmbedding)
	for _, info := range indexInfoList {
		emb := toDorisVectorEmbedding(info, additionalParams, compatMode)
		if len(emb.Embedding) == 0 {
			log.Warnf("[Doris] Skipping empty embedding for chunk %s", info.ChunkID)
			continue
		}
		if err := validateEmbedding(emb.Embedding); err != nil {
			return fmt.Errorf("invalid embedding for chunk %s: %w", info.ChunkID, err)
		}
		// A stable primary key. SourceID is the most meaningful row identity upstream,
		// and it is already unique when one chunk has several questions, so use it directly.
		if emb.ID == "" {
			emb.ID = emb.SourceID
		}
		if emb.ID == "" {
			emb.ID = uuid.New().String()
		}
		dim := len(emb.Embedding)
		groups[dim] = append(groups[dim], emb)
	}

	for dim, rows := range groups {
		if err := r.ensureTable(ctx, dim); err != nil {
			return err
		}
		if compatMode.usesReplaceWrite() {
			err = r.replaceRows(ctx, r.getTableName(dim), rows)
		} else {
			err = r.insertRows(ctx, r.getTableName(dim), rows)
		}
		if err != nil {
			return fmt.Errorf("batch save dim=%d: %w", dim, err)
		}
		log.Infof("[Doris] Saved %d rows to %s", len(rows), r.getTableName(dim))
	}
	return nil
}

// insertRows builds one multi-VALUES INSERT in column order. The embedding column
// must be inlined as a literal because go-sql-driver/mysql has no ARRAY placeholder.
func (r *dorisRepository) insertRows(ctx context.Context,
	table string, rows []*DorisVectorEmbedding,
) error {
	if len(rows) == 0 {
		return nil
	}

	// 9 ordinary placeholders + 1 embedding literal.
	const perRowPlaceholders = "(?, ?, ?, ?, ?, ?, ?, ?, ?, %s)"

	parts := make([]string, len(rows))
	args := make([]any, 0, len(rows)*9)
	for i, e := range rows {
		parts[i] = fmt.Sprintf(perRowPlaceholders, embeddingLiteral(e.Embedding))
		args = append(args,
			e.ID, e.Content, e.SourceID, e.SourceType,
			e.ChunkID, e.KnowledgeID, e.KnowledgeBaseID, e.TagID,
			e.IsEnabled,
		)
	}

	stmt := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s",
		table,
		strings.Join(columns, ", "),
		strings.Join(parts, ", "),
	)
	_, err := r.db.ExecContext(ctx, stmt, args...)
	return err
}

// replaceRows emulates "overwrite by id" on a DUPLICATE KEY table.
func (r *dorisRepository) replaceRows(ctx context.Context,
	table string, rows []*DorisVectorEmbedding,
) error {
	rows = dedupeRowsByID(rows)
	if len(rows) == 0 {
		return nil
	}

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if err := r.deleteRowsByID(ctx, table, ids); err != nil {
		return err
	}
	return r.insertRows(ctx, table, rows)
}

func (r *dorisRepository) deleteRowsByID(ctx context.Context,
	table string, ids []string,
) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	stmt := fmt.Sprintf("DELETE FROM `%s` WHERE %s IN (%s)",
		table, fieldID, strings.Join(placeholders, ", "))
	_, err := r.db.ExecContext(ctx, stmt, args...)
	return err
}

func dedupeRowsByID(rows []*DorisVectorEmbedding) []*DorisVectorEmbedding {
	if len(rows) < 2 {
		return rows
	}

	out := make([]*DorisVectorEmbedding, 0, len(rows))
	positions := make(map[string]int, len(rows))
	for _, row := range rows {
		if idx, ok := positions[row.ID]; ok {
			out[idx] = row
			continue
		}
		positions[row.ID] = len(out)
		out = append(out, row)
	}
	return out
}

// DeleteByChunkIDList deletes by the chunk_id column. dimension selects the table.
func (r *dorisRepository) DeleteByChunkIDList(ctx context.Context,
	chunkIDList []string, dimension int, _ string,
) error {
	return r.deleteByField(ctx, fieldChunkID, chunkIDList, dimension)
}

// DeleteByKnowledgeIDList deletes by the knowledge_id column.
func (r *dorisRepository) DeleteByKnowledgeIDList(ctx context.Context,
	knowledgeIDList []string, dimension int, _ string,
) error {
	return r.deleteByField(ctx, fieldKnowledgeID, knowledgeIDList, dimension)
}

// DeleteBySourceIDList deletes by the source_id column.
func (r *dorisRepository) DeleteBySourceIDList(ctx context.Context,
	sourceIDList []string, dimension int, _ string,
) error {
	return r.deleteByField(ctx, fieldSourceID, sourceIDList, dimension)
}

// deleteByField is the shared implementation of the three Delete* methods:
// DELETE FROM <table> WHERE <field> IN (?, ?, ...)。
func (r *dorisRepository) deleteByField(ctx context.Context,
	field string, ids []string, dimension int,
) error {
	log := logger.GetLogger(ctx)
	if len(ids) == 0 {
		return nil
	}

	table := r.getTableName(dimension)
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, v := range ids {
		placeholders[i] = "?"
		args[i] = v
	}
	stmt := fmt.Sprintf("DELETE FROM `%s` WHERE %s IN (%s)",
		table, field, strings.Join(placeholders, ", "))

	if _, err := r.db.ExecContext(ctx, stmt, args...); err != nil {
		log.Errorf("[Doris] Delete by %s failed: %v", field, err)
		return fmt.Errorf("delete by %s: %w", field, err)
	}
	log.Infof("[Doris] Deleted %d rows from %s by %s", len(ids), table, field)
	return nil
}

// Retrieve dispatches to vector or keyword retrieval by RetrieverType.
func (r *dorisRepository) Retrieve(ctx context.Context,
	params types.RetrieveParams,
) ([]*types.RetrieveResult, error) {
	switch params.RetrieverType {
	case types.VectorRetrieverType:
		return r.VectorRetrieve(ctx, params)
	case types.KeywordsRetrieverType:
		return r.KeywordsRetrieve(ctx, params)
	}
	return nil, fmt.Errorf("invalid retriever type: %v", params.RetrieverType)
}

// VectorRetrieve normalizes the query vector, then runs an ANN search with
// inner_product_approximate. For unit vectors inner product equals cosine similarity,
// so the score keeps its "higher is more similar" meaning.
func (r *dorisRepository) VectorRetrieve(ctx context.Context,
	params types.RetrieveParams,
) ([]*types.RetrieveResult, error) {
	log := logger.GetLogger(ctx)
	if err := validateEmbedding(params.Embedding); err != nil {
		return nil, fmt.Errorf("invalid query embedding: %w", err)
	}
	compatMode, err := r.resolveCompatMode(ctx)
	if err != nil {
		return nil, err
	}
	queryEmbedding := append([]float32(nil), params.Embedding...)
	if compatMode.normalizeEmbeddings() {
		queryEmbedding = normalizeEmbedding(queryEmbedding)
	}
	dim := len(params.Embedding)
	table := r.getTableName(dim)

	exists, err := r.tableExists(ctx, table)
	if err != nil {
		return nil, fmt.Errorf("check table %s: %w", table, err)
	}
	if !exists {
		log.Warnf("[Doris] Table %s does not exist, returning empty results", table)
		return buildRetrieveResult(nil, types.VectorRetrieverType), nil
	}

	wb := buildBaseFilter(params)
	whereClause, whereArgs := wb.build()

	// embedding must be a literal, and Doris does not accept placeholders for LIMIT/OFFSET, so they are inlined too.
	// HAVING because score is a SELECT alias that WHERE cannot see yet.
	scoreExpr := fmt.Sprintf("inner_product_approximate(`%s`, %s)", fieldEmbedding, embeddingLiteral(queryEmbedding))
	if compatMode == dorisCompatModeLegacy {
		scoreExpr = fmt.Sprintf("(1 - cosine_distance_approximate(`%s`, %s))", fieldEmbedding, embeddingLiteral(queryEmbedding))
	}
	stmt := fmt.Sprintf(
		"SELECT %s, %s AS score "+
			"FROM `%s` WHERE %s "+
			"HAVING score >= ? "+
			"ORDER BY score DESC LIMIT %d",
		strings.Join(columnsForRetrieve, ", "),
		scoreExpr,
		table,
		whereClause,
		params.TopK,
	)
	args := append(whereArgs, params.Threshold)

	rows, err := r.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, r.wrapVectorRetrieveError(table, compatMode, err)
	}
	defer rows.Close()

	results, err := scanRetrieveRows(rows, types.MatchTypeEmbedding)
	if err != nil {
		return nil, err
	}
	log.Infof("[Doris] Vector retrieval found %d results in %s", len(results), table)
	return buildRetrieveResult(results, types.VectorRetrieverType), nil
}

// KeywordsRetrieve matches keywords with the Doris inverted index and MATCH_ANY.
//
// No client-side jieba segmentation: idx_content declares the chinese parser at CREATE TABLE.
// Results from tables of different dimensions are merged to topK, as in Milvus/Weaviate.
func (r *dorisRepository) KeywordsRetrieve(ctx context.Context,
	params types.RetrieveParams,
) ([]*types.RetrieveResult, error) {
	log := logger.GetLogger(ctx)
	query := strings.TrimSpace(params.Query)
	if query == "" {
		return buildRetrieveResult(nil, types.KeywordsRetrieverType), nil
	}

	tables, err := r.listEmbeddingTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	if len(tables) == 0 {
		return buildRetrieveResult(nil, types.KeywordsRetrieverType), nil
	}

	wb := buildBaseFilter(params)
	whereClause, whereArgs := wb.build()

	var all []*types.IndexWithScore
	for _, table := range tables {
		stmt := fmt.Sprintf(
			"SELECT %s FROM `%s` WHERE %s AND %s MATCH_ANY ? LIMIT %d",
			strings.Join(columnsForRetrieve, ", "),
			table, whereClause, fieldContent,
			params.TopK,
		)
		args := append(append([]any{}, whereArgs...), query)

		rows, err := r.db.QueryContext(ctx, stmt, args...)
		if err != nil {
			log.Warnf("[Doris] Keyword retrieve in %s failed: %v", table, err)
			continue
		}
		// score is fixed at 1.0 in KeywordsRetrieve, matching Qdrant.
		batch, scanErr := scanRetrieveRows(rows, types.MatchTypeKeywords)
		_ = rows.Close()
		if scanErr != nil {
			return nil, scanErr
		}
		all = append(all, batch...)
	}
	if len(all) > params.TopK {
		all = all[:params.TopK]
	}
	log.Infof("[Doris] Keywords retrieval found %d results across %d tables", len(all), len(tables))
	return buildRetrieveResult(all, types.KeywordsRetrieverType), nil
}

// CopyIndices copies the source knowledge base's chunks to the target knowledge base without re-embedding.
//
// Mirrors the Qdrant implementation:
//   - page through the source table
//   - map chunk_id through sourceToTargetChunkIDMap
//   - apply the source_id translation rules (plain chunk / generated question / other)
//   - write the target rows back to the same table
func (r *dorisRepository) CopyIndices(ctx context.Context,
	sourceKnowledgeBaseID string,
	sourceToTargetKBIDMap map[string]string,
	sourceToTargetChunkIDMap map[string]string,
	targetKnowledgeBaseID string,
	dimension int,
	_ string,
) error {
	log := logger.GetLogger(ctx)
	if len(sourceToTargetChunkIDMap) == 0 {
		return nil
	}
	if err := r.ensureTable(ctx, dimension); err != nil {
		return err
	}

	table := r.getTableName(dimension)
	const pageSize = 64
	offset := 0
	totalCopied := 0

	for {
		stmt := fmt.Sprintf(
			"SELECT %s FROM `%s` WHERE %s = ? ORDER BY %s LIMIT %d OFFSET %d",
			strings.Join(columnsForCopy, ", "),
			table, fieldKnowledgeBaseID, fieldID,
			pageSize, offset,
		)
		rows, err := r.db.QueryContext(ctx, stmt, sourceKnowledgeBaseID)
		if err != nil {
			return fmt.Errorf("copy indices scan: %w", err)
		}
		batch, err := scanCopyRows(rows)
		_ = rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		var targets []*DorisVectorEmbedding
		for _, src := range batch {
			targetChunkID, ok := sourceToTargetChunkIDMap[src.ChunkID]
			if !ok {
				log.Warnf("[Doris] Source chunk %s not in target mapping", src.ChunkID)
				continue
			}
			targetKnowledgeID, ok := sourceToTargetKBIDMap[src.KnowledgeID]
			if !ok {
				log.Warnf("[Doris] Source knowledge %s not in target mapping", src.KnowledgeID)
				continue
			}

			targetSourceID := translateSourceID(src.SourceID, src.ChunkID, targetChunkID)
			targets = append(targets, &DorisVectorEmbedding{
				ID:              uuid.New().String(),
				Content:         src.Content,
				SourceID:        targetSourceID,
				SourceType:      src.SourceType,
				ChunkID:         targetChunkID,
				KnowledgeID:     targetKnowledgeID,
				KnowledgeBaseID: targetKnowledgeBaseID,
				TagID:           src.TagID,
				IsEnabled:       src.IsEnabled,
				Embedding:       src.Embedding,
			})
		}

		if len(targets) > 0 {
			if err := r.insertRows(ctx, table, targets); err != nil {
				return fmt.Errorf("copy indices insert: %w", err)
			}
			totalCopied += len(targets)
		}

		if len(batch) < pageSize {
			break
		}
		offset += pageSize
	}
	log.Infof("[Doris] CopyIndices done, dim=%d, copied=%d", dimension, totalCopied)
	return nil
}

// BatchUpdateChunkEnabledStatus / BatchUpdateChunkTagID live in streamload.go
// and choose partial update or rewrite rows by compat mode.

// ---------------------------------------------------------------------------
// Private helpers
// ---------------------------------------------------------------------------

// toDorisVectorEmbedding maps an IndexInfo plus the caller's embedding map to the
// Doris row model. The embedding is read from the map[string][]float32 in
// additionalParams[fieldEmbedding] by SourceID, as in Qdrant/Milvus.
// inner_product_duplicate mode normalizes the vector; legacy mode keeps it as is.
func toDorisVectorEmbedding(
	info *types.IndexInfo,
	additionalParams map[string]any,
	compatMode dorisCompatMode,
) *DorisVectorEmbedding {
	emb := &DorisVectorEmbedding{
		ID:              info.ID,
		Content:         info.Content,
		SourceID:        info.SourceID,
		SourceType:      int(info.SourceType),
		ChunkID:         info.ChunkID,
		KnowledgeID:     info.KnowledgeID,
		KnowledgeBaseID: info.KnowledgeBaseID,
		TagID:           info.TagID,
		IsEnabled:       info.IsEnabled,
	}
	if additionalParams != nil {
		if v, ok := additionalParams[fieldEmbedding]; ok {
			if m, ok := v.(map[string][]float32); ok {
				emb.Embedding = append([]float32(nil), m[info.SourceID]...)
				if compatMode.normalizeEmbeddings() {
					emb.Embedding = normalizeEmbedding(emb.Embedding)
				}
			}
		}
	}
	return emb
}

func (r *dorisRepository) wrapVectorRetrieveError(table string, compatMode dorisCompatMode, err error) error {
	if compatMode == dorisCompatModeLegacy {
		return fmt.Errorf(
			"vector retrieve %s in Doris compat mode %s: %w. If your Doris build does not support cosine_distance_approximate or ANN on UNIQUE KEY tables, set %s=%s before creating embedding tables. %s is not interchangeable after %s_* tables are created",
			table,
			compatMode,
			err,
			envDorisCompatMode,
			dorisCompatModeInnerProductDuplicate,
			envDorisCompatMode,
			r.tableBaseName,
		)
	}
	return fmt.Errorf("vector retrieve %s in Doris compat mode %s: %w", table, compatMode, err)
}

// translateSourceID maps a source SourceID to the target SourceID, mirroring Qdrant:
//   - plain chunk: SourceID == ChunkID -> targetChunkID
//   - generated question: SourceID == "<chunkID>-<questionID>" -> "<targetChunkID>-<questionID>"
//   - anything else: a new UUID (keeps it unique)
func translateSourceID(originalSourceID, sourceChunkID, targetChunkID string) string {
	switch {
	case originalSourceID == sourceChunkID:
		return targetChunkID
	case strings.HasPrefix(originalSourceID, sourceChunkID+"-"):
		questionID := strings.TrimPrefix(originalSourceID, sourceChunkID+"-")
		return fmt.Sprintf("%s-%s", targetChunkID, questionID)
	default:
		return uuid.New().String()
	}
}

// scanRetrieveRows decodes Retrieve rows into an IndexWithScore list.
//
// Two shapes:
//   - columns == columnsForRetrieve+1: column N+1 is score (vector retrieval)
//   - columns == columnsForRetrieve: score is 1.0 (keyword retrieval)
func scanRetrieveRows(rows *sql.Rows, matchType types.MatchType) ([]*types.IndexWithScore, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	withScore := len(cols) == len(columnsForRetrieve)+1

	var out []*types.IndexWithScore
	for rows.Next() {
		var (
			id, content, sourceID, chunkID      string
			knowledgeID, knowledgeBaseID, tagID string
			sourceType                          int
			isEnabled                           bool
			score                               float64
			err                                 error
		)
		if withScore {
			err = rows.Scan(&id, &content, &sourceID, &sourceType,
				&chunkID, &knowledgeID, &knowledgeBaseID, &tagID, &isEnabled, &score)
		} else {
			err = rows.Scan(&id, &content, &sourceID, &sourceType,
				&chunkID, &knowledgeID, &knowledgeBaseID, &tagID, &isEnabled)
			score = 1.0
		}
		if err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		out = append(out, &types.IndexWithScore{
			ID:              id,
			Content:         content,
			SourceID:        sourceID,
			SourceType:      types.SourceType(sourceType),
			ChunkID:         chunkID,
			KnowledgeID:     knowledgeID,
			KnowledgeBaseID: knowledgeBaseID,
			TagID:           tagID,
			Score:           score,
			MatchType:       matchType,
		})
	}
	return out, rows.Err()
}

// scanCopyRows decodes the paginated query results of CopyIndices.
//
// Unlike scanRetrieveRows it needs the embedding column (to copy the original vector).
// Doris returns ARRAY<FLOAT> over the mysql protocol as a string literal "[1,2,3]".
func scanCopyRows(rows *sql.Rows) ([]*DorisVectorEmbedding, error) {
	var out []*DorisVectorEmbedding
	for rows.Next() {
		var (
			id, content, sourceID, chunkID      string
			knowledgeID, knowledgeBaseID, tagID string
			sourceType                          int
			isEnabled                           bool
			embeddingRaw                        sql.RawBytes
		)
		if err := rows.Scan(&id, &content, &sourceID, &sourceType,
			&chunkID, &knowledgeID, &knowledgeBaseID, &tagID, &isEnabled, &embeddingRaw); err != nil {
			return nil, fmt.Errorf("scan copy row: %w", err)
		}
		vec, err := parseEmbeddingLiteral(embeddingRaw)
		if err != nil {
			return nil, fmt.Errorf("parse embedding: %w", err)
		}
		out = append(out, &DorisVectorEmbedding{
			ID:              id,
			Content:         content,
			SourceID:        sourceID,
			SourceType:      sourceType,
			ChunkID:         chunkID,
			KnowledgeID:     knowledgeID,
			KnowledgeBaseID: knowledgeBaseID,
			TagID:           tagID,
			IsEnabled:       isEnabled,
			Embedding:       vec,
		})
	}
	return out, rows.Err()
}

// buildRetrieveResult wraps an IndexWithScore list in a RetrieveResult.
func buildRetrieveResult(results []*types.IndexWithScore, retrieverType types.RetrieverType) []*types.RetrieveResult {
	return []*types.RetrieveResult{{
		Results:             results,
		RetrieverEngineType: types.DorisRetrieverEngineType,
		RetrieverType:       retrieverType,
		Error:               nil,
	}}
}

// calculateStorageSize estimates the storage cost of one row.
//
// Same as Qdrant: payload string bytes + vector (dim*4) + HNSW M*2*8 + metadata 24.
func calculateStorageSize(emb *DorisVectorEmbedding) int64 {
	var payload int64
	payload += int64(len(emb.Content))
	payload += int64(len(emb.SourceID))
	payload += int64(len(emb.ChunkID))
	payload += int64(len(emb.KnowledgeID))
	payload += int64(len(emb.KnowledgeBaseID))
	payload += int64(len(emb.TagID))
	payload += 8 // source_type int

	var vec int64
	var hnsw int64
	if len(emb.Embedding) > 0 {
		vec = int64(len(emb.Embedding)) * 4
		const hnswM = 32
		hnsw = hnswM * 2 * 8
	}
	const metaBytes int64 = 24
	return payload + vec + hnsw + metaBytes
}
