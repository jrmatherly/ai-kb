package doris

import (
	"database/sql"
	"net/http"
	"sync"
)

// dorisRepository is the retrieval engine repository for Apache Doris 4.1.
//
// Channels:
//   - main read/write path: MySQL protocol (database/sql + go-sql-driver/mysql), FE port 9030 by default.
//   - Stream Load: HTTP (FE port 8030 by default), used for partial update in legacy mode.
//
// Tables are split by dimension: <tableBaseName>_<dim>.
// DORIS_COMPAT_MODE selects the compat mode:
//   - legacy: UNIQUE KEY(id) + cosine_distance ANN + Stream Load partial update
//   - inner_product_duplicate: DUPLICATE KEY(id) + normalized inner product + delete/insert rewrite
//
// The mode cannot be switched once the embedding tables exist; rebuild those tables before switching.
//
// As in Qdrant/Milvus/Weaviate, initializedTables caches the dimensions already ensured to exist,
// so writes do not run SHOW TABLES every time.
type dorisRepository struct {
	db *sql.DB

	httpClient *http.Client
	// FE HTTP base, such as "http://doris-fe:8030". streamLoadURL(table) builds the Stream Load path:
	// <feHTTPBase>/api/<database>/<table>/_stream_load.
	feHTTPBase string

	username string
	password string
	database string

	tableBaseName       string
	bucketsNum          int // 0 -> default 10
	replicationNum      int // 0 -> default 1
	compatModeRequested dorisCompatMode
	compatModeResolved  dorisCompatMode
	compatResolveOnce   sync.Once
	compatResolveErr    error

	// Dimensions that already passed ensureTable: dim -> true.
	initializedTables sync.Map
}

// DorisVectorEmbedding is the domain model of one row in a Doris table.
//
// Field order matches the column order in columns (query.go);
// change insertRows and columns together.
type DorisVectorEmbedding struct {
	ID              string
	Content         string
	SourceID        string
	SourceType      int
	ChunkID         string
	KnowledgeID     string
	KnowledgeBaseID string
	TagID           string
	IsEnabled       bool
	Embedding       []float32
}

// DorisVectorEmbeddingWithScore is the domain model of a retrieval result.
// Score follows the current compat mode in vector retrieval and is 1.0 in keyword retrieval.
type DorisVectorEmbeddingWithScore struct {
	DorisVectorEmbedding
	Score float64
}
