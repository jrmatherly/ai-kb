package doris

import (
	"math"
	"strconv"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// Column name constants. Doris is a SQL store and column names repeat across
// SELECT/WHERE/INSERT, so constants guard against typos.
const (
	fieldID              = "id"
	fieldContent         = "content"
	fieldSourceID        = "source_id"
	fieldSourceType      = "source_type"
	fieldChunkID         = "chunk_id"
	fieldKnowledgeID     = "knowledge_id"
	fieldKnowledgeBaseID = "knowledge_base_id"
	fieldTagID           = "tag_id"
	fieldIsEnabled       = "is_enabled"
	fieldEmbedding       = "embedding"
)

// columns is the standard column order for INSERT and SELECT.
var columns = []string{
	fieldID, fieldContent, fieldSourceID, fieldSourceType,
	fieldChunkID, fieldKnowledgeID, fieldKnowledgeBaseID, fieldTagID,
	fieldIsEnabled, fieldEmbedding,
}

// columnsForRetrieve is the SELECT column order for Retrieve. It omits
// embedding: the vector is not needed in results and costs bandwidth.
var columnsForRetrieve = []string{
	fieldID, fieldContent, fieldSourceID, fieldSourceType,
	fieldChunkID, fieldKnowledgeID, fieldKnowledgeBaseID, fieldTagID,
	fieldIsEnabled,
}

// columnsForCopy is the column order for the paginated SELECT in CopyIndices.
// It adds embedding to columnsForRetrieve because copying moves the vector itself.
var columnsForCopy = []string{
	fieldID, fieldContent, fieldSourceID, fieldSourceType,
	fieldChunkID, fieldKnowledgeID, fieldKnowledgeBaseID, fieldTagID,
	fieldIsEnabled, fieldEmbedding,
}

// whereCond is one WHERE sub-condition: clause is a parameterized SQL fragment (with ? placeholders)
// and args holds the values in order. Every user-supplied field (IDs) must go through args,
// never into the clause string.
type whereCond struct {
	clause string
	args   []any
}

// whereBuilder translates the filters in RetrieveParams into a SQL WHERE clause.
//
// Each add* method maps to one IN / NOT IN / = operator; build() joins them with AND.
type whereBuilder struct {
	conds []whereCond
}

// addEqual appends a field = ? condition.
func (w *whereBuilder) addEqual(field string, value any) {
	w.conds = append(w.conds, whereCond{
		clause: field + " = ?",
		args:   []any{value},
	})
}

// addIn appends a field IN (?, ?, ...) condition. Empty values append nothing.
func (w *whereBuilder) addIn(field string, values []string) {
	if len(values) == 0 {
		return
	}
	placeholders := make([]string, len(values))
	args := make([]any, len(values))
	for i, v := range values {
		placeholders[i] = "?"
		args[i] = v
	}
	w.conds = append(w.conds, whereCond{
		clause: field + " IN (" + strings.Join(placeholders, ", ") + ")",
		args:   args,
	})
}

// addNotIn appends a field NOT IN (?, ?, ...) condition.
func (w *whereBuilder) addNotIn(field string, values []string) {
	if len(values) == 0 {
		return
	}
	placeholders := make([]string, len(values))
	args := make([]any, len(values))
	for i, v := range values {
		placeholders[i] = "?"
		args[i] = v
	}
	w.conds = append(w.conds, whereCond{
		clause: field + " NOT IN (" + strings.Join(placeholders, ", ") + ")",
		args:   args,
	})
}

// build returns the WHERE clause (without the "WHERE " prefix) and its args.
// With no conditions it returns ("1 = 1", nil) so callers can concatenate unconditionally.
func (w *whereBuilder) build() (string, []any) {
	if len(w.conds) == 0 {
		return "1 = 1", nil
	}
	parts := make([]string, len(w.conds))
	var args []any
	for i, c := range w.conds {
		parts[i] = c.clause
		args = append(args, c.args...)
	}
	return strings.Join(parts, " AND "), args
}

// buildBaseFilter translates the filters in RetrieveParams into a whereBuilder.
// It always appends is_enabled = TRUE, matching Qdrant/Milvus/Weaviate:
// disabled chunks are excluded from retrieval.
func buildBaseFilter(params types.RetrieveParams) *whereBuilder {
	w := &whereBuilder{}
	w.addEqual(fieldIsEnabled, true)

	if len(params.KnowledgeBaseIDs) > 0 {
		w.addIn(fieldKnowledgeBaseID, params.KnowledgeBaseIDs)
	}
	if len(params.KnowledgeIDs) > 0 {
		w.addIn(fieldKnowledgeID, params.KnowledgeIDs)
	}
	if len(params.TagIDs) > 0 {
		w.addIn(fieldTagID, params.TagIDs)
	}
	if len(params.ExcludeKnowledgeIDs) > 0 {
		w.addNotIn(fieldKnowledgeID, params.ExcludeKnowledgeIDs)
	}
	if len(params.ExcludeChunkIDs) > 0 {
		w.addNotIn(fieldChunkID, params.ExcludeChunkIDs)
	}
	return w
}

// parseEmbeddingLiteral parses the literal string Doris returns for ARRAY<FLOAT>
// over the MySQL protocol (such as "[1,2,3]") into []float32.
//
// CopyIndices reads the vector from the source row and writes it to the target row, so parsing is lenient:
// input without [] is accepted and an empty array returns nil.
func parseEmbeddingLiteral(raw []byte) ([]float32, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil, nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		f, err := strconv.ParseFloat(p, 32)
		if err != nil {
			return nil, err
		}
		out = append(out, float32(f))
	}
	return out, nil
}

// validateEmbedding checks that every vector element is finite.
//
// strconv.FormatFloat renders NaN/+-Inf as "NaN"/"+Inf"/"-Inf",
// which make the generated Doris SQL a syntax error (or undefined behavior on some versions).
// The embedding model should never emit non-finite values, but GPU OOM, upstream bugs
// and test stubs can; failing fast is safer than silently writing bad data.
func validateEmbedding(vec []float32) error {
	for i, v := range vec {
		if f := float64(v); math.IsNaN(f) || math.IsInf(f, 0) {
			return errInvalidEmbedding{index: i, value: v}
		}
	}
	return nil
}

// normalizeEmbedding returns a unit-length copy of vec so Doris inner-product
// ANN search can preserve cosine-style similarity semantics.
func normalizeEmbedding(vec []float32) []float32 {
	if len(vec) == 0 {
		return nil
	}
	var sumSquares float64
	for _, value := range vec {
		f := float64(value)
		sumSquares += f * f
	}
	if sumSquares == 0 {
		return append([]float32(nil), vec...)
	}
	norm := float32(math.Sqrt(sumSquares))
	normalized := make([]float32, len(vec))
	for i, value := range vec {
		normalized[i] = value / norm
	}
	return normalized
}

// errInvalidEmbedding names the index holding a non-finite value; a struct rather than fmt.Errorf
// lets callers log the index for diagnosis.
type errInvalidEmbedding struct {
	index int
	value float32
}

func (e errInvalidEmbedding) Error() string {
	return "doris: embedding[" + strconv.Itoa(e.index) +
		"] is not finite: " + strconv.FormatFloat(float64(e.value), 'g', -1, 32)
}

// embeddingLiteral converts []float32 to a Doris ARRAY<FLOAT> literal string:
// "[1.23,4.56,...]"。
//
// Why no placeholder: go-sql-driver/mysql cannot bind ARRAY parameters,
// and Doris accepts only the literal form. strconv.FormatFloat ('g', bitSize=32)
// is used instead of fmt.Sprintf("%f", v) for two reasons:
//  1. fmt can use thousands separators under some locales, breaking SQL syntax;
//  2. 'g' is shorter than 'f' and loses no precision.
//
// Injection risk: the elements are finite floats from the embedding model; serialized they can only
// contain [0-9eE+-.\s] characters, so they cannot escape the literal context.
func embeddingLiteral(vec []float32) string {
	if len(vec) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.Grow(len(vec) * 12)
	sb.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(strconv.FormatFloat(float64(v), 'g', -1, 32))
	}
	sb.WriteByte(']')
	return sb.String()
}
