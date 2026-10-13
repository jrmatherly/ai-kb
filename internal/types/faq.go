package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jrmatherly/ai-kb/internal/textconv"
)

// FAQChunkMetadata is the shape of an FAQ entry stored in Chunk.Metadata.
type FAQChunkMetadata struct {
	StandardQuestion  string         `json:"standard_question"`
	SimilarQuestions  []string       `json:"similar_questions,omitempty"`
	NegativeQuestions []string       `json:"negative_questions,omitempty"`
	Answers           []string       `json:"answers,omitempty"`
	AnswerStrategy    AnswerStrategy `json:"answer_strategy,omitempty"`
	Version           int            `json:"version,omitempty"`
	Source            string         `json:"source,omitempty"`
}

// GeneratedQuestion is one AI-generated question.
type GeneratedQuestion struct {
	ID              string `json:"id"`                         // unique ID, used to build source_id
	Question        string `json:"question"`                   // question text
	ContentRevision *int   `json:"content_revision,omitempty"` // chunk content revision the question was generated for
}

const maxGeneratedQuestionSourceIDLength = 64

// GeneratedQuestionSourceID builds the retrieval source identifier for a
// generated question. PostgreSQL stores source_id as varchar(64), while a
// chunk UUID plus a question UUID would be 73 bytes. Preserve the historical
// representation for short IDs and hash only oversized question IDs so
// existing index rows remain addressable by delete/reindex operations.
func GeneratedQuestionSourceID(chunkID, questionID string) string {
	candidate := chunkID + "-" + questionID
	if len(candidate) <= maxGeneratedQuestionSourceIDLength {
		return candidate
	}
	digest := sha256.Sum256([]byte(questionID))
	// UUID chunk IDs use 36 bytes; "-q" plus 24 hex characters keeps the
	// complete identifier at 62 bytes while retaining ample collision space.
	return chunkID + "-q" + hex.EncodeToString(digest[:12])
}

// DocumentChunkMetadata is the metadata of a document chunk, holding
// enrichments such as AI-generated questions.
type DocumentChunkMetadata struct {
	// GeneratedQuestions are AI-generated questions about the chunk, indexed
	// separately to improve recall.
	GeneratedQuestions []GeneratedQuestion `json:"generated_questions,omitempty"`
	// GeneratedQuestionsRevision ties the questions to Chunk.ContentRevision.
	GeneratedQuestionsRevision int `json:"generated_questions_revision,omitempty"`
}

// IsQuestionCurrent reports whether a generated question was authored for the
// current chunk body. This is advisory metadata for the UI: questions remain
// valid retrieval aliases across chunk edits. Legacy rows fall back to the
// metadata-level revision.
func (m *DocumentChunkMetadata) IsQuestionCurrent(question GeneratedQuestion, chunkRevision int) bool {
	if question.ContentRevision != nil {
		return *question.ContentRevision == chunkRevision
	}
	return m != nil && m.GeneratedQuestionsRevision == chunkRevision
}

// GetQuestionStrings returns the question texts (kept for older callers).
func (m *DocumentChunkMetadata) GetQuestionStrings() []string {
	if m == nil || len(m.GeneratedQuestions) == 0 {
		return nil
	}
	result := make([]string, len(m.GeneratedQuestions))
	for i, q := range m.GeneratedQuestions {
		result[i] = q.Question
	}
	return result
}

// DocumentMetadata parses the document metadata of the chunk.
func (c *Chunk) DocumentMetadata() (*DocumentChunkMetadata, error) {
	if c == nil || len(c.Metadata) == 0 {
		return nil, nil
	}
	var meta DocumentChunkMetadata
	if err := json.Unmarshal(c.Metadata, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// SetDocumentMetadata sets the document metadata of the chunk.
func (c *Chunk) SetDocumentMetadata(meta *DocumentChunkMetadata) error {
	if c == nil {
		return nil
	}
	if meta == nil {
		c.Metadata = nil
		return nil
	}
	bytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	c.Metadata = JSON(bytes)
	return nil
}

// Sanitize does basic cleanup (trim whitespace, dedupe) and keeps the original
// text. It is for DB storage and does no semantic normalization.
func (m *FAQChunkMetadata) Sanitize() {
	if m == nil {
		return
	}
	m.StandardQuestion = strings.TrimSpace(m.StandardQuestion)
	m.SimilarQuestions = SanitizeStrings(m.SimilarQuestions)
	m.NegativeQuestions = SanitizeStrings(m.NegativeQuestions)
	m.Answers = SanitizeStrings(m.Answers)
	if m.Version <= 0 {
		m.Version = 1
	}
}

// Normalize returns a normalized copy for hashing and vector indexing; the
// receiver is not modified.
func (m *FAQChunkMetadata) Normalize() *FAQChunkMetadata {
	if m == nil {
		return nil
	}
	return &FAQChunkMetadata{
		StandardQuestion:  NormalizeQuestion(m.StandardQuestion),
		SimilarQuestions:  normalizeQuestionStrings(m.SimilarQuestions),
		NegativeQuestions: normalizeQuestionStrings(m.NegativeQuestions),
		Answers:           SanitizeStrings(m.Answers), // answers get basic cleanup only
		AnswerStrategy:    m.AnswerStrategy,
		Version:           m.Version,
		Source:            m.Source,
	}
}

// SanitizeStrings trims and dedupes a string list.
func SanitizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	dedup := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		dedup = append(dedup, trimmed)
	}
	if len(dedup) == 0 {
		return nil
	}
	return dedup
}

// FAQMetadata parses the FAQ metadata of the chunk and returns the original
// data with basic cleanup only.
func (c *Chunk) FAQMetadata() (*FAQChunkMetadata, error) {
	if c == nil || len(c.Metadata) == 0 {
		return nil, nil
	}
	var meta FAQChunkMetadata
	if err := json.Unmarshal(c.Metadata, &meta); err != nil {
		return nil, err
	}
	meta.Sanitize()
	return &meta, nil
}

// SetFAQMetadata sets the FAQ metadata of the chunk. The DB stores the
// original data; ContentHash is computed from the normalized data.
func (c *Chunk) SetFAQMetadata(meta *FAQChunkMetadata) error {
	if c == nil {
		return nil
	}
	if meta == nil {
		c.Metadata = nil
		c.ContentHash = ""
		return nil
	}
	meta.Sanitize()
	bytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	c.Metadata = JSON(bytes)
	// The hash is over normalized data so it can match duplicates.
	normalized := meta.Normalize()
	c.ContentHash = CalculateFAQContentHash(normalized)
	return nil
}

// CalculateFAQContentHash hashes the FAQ content for fast matching and
// deduplication: standard question + similar questions (sorted) + negative
// questions (sorted) + answers (sorted).
func CalculateFAQContentHash(meta *FAQChunkMetadata) string {
	if meta == nil {
		return ""
	}

	// Normalize() returns a new copy; the old code discarded the return value.
	normalized := meta.Normalize()
	if normalized == nil {
		return ""
	}

	// Sort so that the same content always yields the same hash.
	similarQuestions := make([]string, len(normalized.SimilarQuestions))
	copy(similarQuestions, normalized.SimilarQuestions)
	sort.Strings(similarQuestions)

	negativeQuestions := make([]string, len(normalized.NegativeQuestions))
	copy(negativeQuestions, normalized.NegativeQuestions)
	sort.Strings(negativeQuestions)

	answers := make([]string, len(normalized.Answers))
	copy(answers, normalized.Answers)
	sort.Strings(answers)

	var builder strings.Builder
	builder.WriteString(normalized.StandardQuestion)
	builder.WriteString("|")
	builder.WriteString(strings.Join(similarQuestions, ","))
	builder.WriteString("|")
	builder.WriteString(strings.Join(negativeQuestions, ","))
	builder.WriteString("|")
	builder.WriteString(strings.Join(answers, ","))

	hash := sha256.Sum256([]byte(builder.String()))
	return hex.EncodeToString(hash[:])
}

// AnswerStrategy controls how answers are returned.
type AnswerStrategy string

const (
	// AnswerStrategyAll returns all answers.
	AnswerStrategyAll AnswerStrategy = "all"
	// AnswerStrategyRandom returns one random answer.
	AnswerStrategyRandom AnswerStrategy = "random"
)

// FAQEntry is an FAQ entry as returned to the frontend.
type FAQEntry struct {
	ID                int64          `json:"id"`
	ChunkID           string         `json:"chunk_id"`
	KnowledgeID       string         `json:"knowledge_id"`
	KnowledgeBaseID   string         `json:"knowledge_base_id"`
	TagID             int64          `json:"tag_id"`
	TagName           string         `json:"tag_name"`
	IsEnabled         bool           `json:"is_enabled"`
	IsRecommended     bool           `json:"is_recommended"`
	StandardQuestion  string         `json:"standard_question"`
	SimilarQuestions  []string       `json:"similar_questions"`
	NegativeQuestions []string       `json:"negative_questions"`
	Answers           []string       `json:"answers"`
	AnswerStrategy    AnswerStrategy `json:"answer_strategy"`
	IndexMode         FAQIndexMode   `json:"index_mode"`
	UpdatedAt         time.Time      `json:"updated_at"`
	CreatedAt         time.Time      `json:"created_at"`
	Score             float64        `json:"score,omitempty"`
	MatchType         MatchType      `json:"match_type,omitempty"`
	ChunkType         ChunkType      `json:"chunk_type"`
	// MatchedQuestion is the actual question text that was matched in FAQ search
	// Could be the standard question or one of the similar questions
	MatchedQuestion string `json:"matched_question,omitempty"`
}

// FAQExportEntry is an FAQ entry in a JSON export. It matches the
// FAQEntryPayload import format so an export can be edited and re-imported.
// New fields must keep omitempty so older export files stay compatible.
type FAQExportEntry struct {
	ID                int64          `json:"id"`
	TagName           string         `json:"tag_name,omitempty"`
	StandardQuestion  string         `json:"standard_question"`
	SimilarQuestions  []string       `json:"similar_questions,omitempty"`
	NegativeQuestions []string       `json:"negative_questions,omitempty"`
	Answers           []string       `json:"answers,omitempty"`
	AnswerStrategy    AnswerStrategy `json:"answer_strategy,omitempty"`
	IsEnabled         bool           `json:"is_enabled"`
	IsRecommended     bool           `json:"is_recommended"`
}

// FAQEntryPayload is the payload for creating or updating an FAQ entry.
type FAQEntryPayload struct {
	// ID is optional; data migrations use it to set seq_id (must be below the
	// auto-increment start, 100000000).
	ID                *int64          `json:"id,omitempty"`
	StandardQuestion  string          `json:"standard_question"    binding:"required"`
	SimilarQuestions  []string        `json:"similar_questions"`
	NegativeQuestions []string        `json:"negative_questions"`
	Answers           []string        `json:"answers"`
	AnswerStrategy    *AnswerStrategy `json:"answer_strategy,omitempty"`
	TagID             int64           `json:"tag_id"`
	TagName           string          `json:"tag_name"`
	IsEnabled         *bool           `json:"is_enabled,omitempty"`
	IsRecommended     *bool           `json:"is_recommended,omitempty"`
}

const (
	FAQBatchModeAppend  = "append"
	FAQBatchModeReplace = "replace"
)

// FAQBatchUpsertPayload is a batch import of FAQ entries.
type FAQBatchUpsertPayload struct {
	Entries     []FAQEntryPayload `json:"entries"      binding:"required"`
	Mode        string            `json:"mode"         binding:"oneof=append replace"`
	KnowledgeID string            `json:"knowledge_id"`
	TaskID      string            `json:"task_id"` // optional; a UUID is generated when empty
	DryRun      bool              `json:"dry_run"` // validate only, import nothing
}

// FAQFailedEntry is an entry that failed import or validation.
type FAQFailedEntry struct {
	Index       int    `json:"index"`                  // index of the entry in the batch (0-based)
	Reason      string `json:"reason"`                 // failure reason
	FailureType string `json:"failure_type,omitempty"` // pre_validation / post_validation
	// similar/negative questions were removed but the entry still imports
	IsPartialFailure  bool     `json:"is_partial_failure,omitempty"`
	TagName           string   `json:"tag_name,omitempty"`           // category
	StandardQuestion  string   `json:"standard_question"`            // standard question
	SimilarQuestions  []string `json:"similar_questions,omitempty"`  // similar questions
	NegativeQuestions []string `json:"negative_questions,omitempty"` // negative questions
	Answers           []string `json:"answers,omitempty"`            // answers
	AnswerAll         bool     `json:"answer_all,omitempty"`         // return all answers
	IsDisabled        bool     `json:"is_disabled,omitempty"`        // disabled
	// Partial-failure details (when IsPartialFailure is true)
	// removed similar questions with reasons
	RemovedSimilarQuestions []string `json:"removed_similar_questions,omitempty"`
	// removed negative questions with reasons
	RemovedNegativeQuestions []string `json:"removed_negative_questions,omitempty"`
}

// FAQMergeDetail summarizes how an FAQ entry merged into an existing chunk in append mode.
type FAQMergeDetail struct {
	Index            int    `json:"index"`              // row index in the batch
	StandardQuestion string `json:"standard_question"`  // standard question
	AnswerChanged    bool   `json:"answer_changed"`     // whether the answer changed
	NewSimilarCount  int    `json:"new_similar_count"`  // similar questions added
	NewNegativeCount int    `json:"new_negative_count"` // negative questions added
}

// FAQSuccessEntry is a short record of a successfully imported entry.
type FAQSuccessEntry struct {
	Index            int    `json:"index"`              // index of the entry in the batch (0-based)
	SeqID            int64  `json:"seq_id"`             // entry sequence ID after import
	TagID            int64  `json:"tag_id,omitempty"`   // category ID (seq_id)
	TagName          string `json:"tag_name,omitempty"` // category name
	StandardQuestion string `json:"standard_question"`  // standard question
}

// FAQDryRunResult is the validation result of a dry_run.
type FAQDryRunResult struct {
	TaskID        string           `json:"task_id,omitempty"` // async task ID (async mode only)
	Total         int              `json:"total"`             // total entries
	SuccessCount  int              `json:"success_count"`     // entries that passed validation
	FailedCount   int              `json:"failed_count"`      // entries that failed validation
	FailedEntries []FAQFailedEntry `json:"failed_entries"`    // failed entry details
}

// FAQSearchRequest is the FAQ search request.
type FAQSearchRequest struct {
	QueryText       string  `json:"query_text"             binding:"required"`
	VectorThreshold float64 `json:"vector_threshold"`
	MatchCount      int     `json:"match_count"`
	// first-priority tag IDs; limit the hit range, highest priority
	FirstPriorityTagIDs []int64 `json:"first_priority_tag_ids"`
	// second-priority tag IDs; limit the hit range, below first priority
	SecondPriorityTagIDs []int64 `json:"second_priority_tag_ids"`
	OnlyRecommended      bool    `json:"only_recommended"` // return recommended entries only
}

// UntaggedTagName is the default tag name for entries without a tag
const UntaggedTagName = "Untagged"

// FAQEntryFieldsUpdate is a field update for one FAQ entry.
type FAQEntryFieldsUpdate struct {
	IsEnabled     *bool  `json:"is_enabled,omitempty"`
	IsRecommended *bool  `json:"is_recommended,omitempty"`
	TagID         *int64 `json:"tag_id,omitempty"`
}

// FAQEntryFieldsBatchUpdate is a batch field update of FAQ entries, in one of
// two modes:
// 1. By entry ID: the ByID field.
// 2. By tag: the ByTag field applies the same update to every entry under the tag.
type FAQEntryFieldsBatchUpdate struct {
	// ByID updates by entry, keyed by entry ID (seq_id)
	ByID map[int64]FAQEntryFieldsUpdate `json:"by_id,omitempty"`
	// ByTag updates by tag, keyed by TagID (seq_id)
	ByTag map[int64]FAQEntryFieldsUpdate `json:"by_tag,omitempty"`
	// ExcludeIDs are entry IDs (seq_id) the ByTag update skips
	ExcludeIDs []int64 `json:"exclude_ids,omitempty"`
}

// FAQImportTaskStatus is the status of an import task.
type FAQImportTaskStatus string

const (
	// FAQImportStatusPending represents the pending status of the FAQ import task
	FAQImportStatusPending FAQImportTaskStatus = "pending"
	// FAQImportStatusProcessing represents the processing status of the FAQ import task
	FAQImportStatusProcessing FAQImportTaskStatus = "processing"
	// FAQImportStatusCompleted represents the completed status of the FAQ import task
	FAQImportStatusCompleted FAQImportTaskStatus = "completed"
	// FAQImportStatusFailed represents the failed status of the FAQ import task
	FAQImportStatusFailed FAQImportTaskStatus = "failed"
)

// FAQImportProgress represents the progress of an FAQ import task stored in Redis
// When Status is "completed", the result fields (SkippedCount, ImportMode, ImportedAt, DisplayStatus, ProcessingTime) are populated.
type FAQImportProgress struct {
	TaskID      string              `json:"task_id"`      // UUID for the import task
	KBID        string              `json:"kb_id"`        // Knowledge Base ID
	KnowledgeID string              `json:"knowledge_id"` // FAQ Knowledge ID
	Status      FAQImportTaskStatus `json:"status"`       // Task status
	Progress    int                 `json:"progress"`     // 0-100 percentage
	Total       int                 `json:"total"`        // Total entries to import
	Processed   int                 `json:"processed"`    // Entries processed so far
	// fully successful entries (partial failures excluded)
	SuccessCount int `json:"success_count"`
	FailedCount  int `json:"failed_count"` // failed entries
	// partially failed entries (similar/negative questions removed)
	PartialFailedCount int `json:"partial_failed_count,omitempty"`
	// skipped entries (duplicates and the like)
	SkippedCount int `json:"skipped_count,omitempty"`
	// failed entry details (returned inline when few)
	FailedEntries []FAQFailedEntry `json:"failed_entries,omitempty"`
	// failed entries CSV download URL (when many)
	FailedEntriesURL string `json:"failed_entries_url,omitempty"`
	// successful entry summaries (returned inline when few)
	SuccessEntries []FAQSuccessEntry `json:"success_entries,omitempty"`
	// indices that passed validation (a retry skips validating them)
	ValidEntryIndices []int `json:"valid_entry_indices,omitempty"`
	// indices to merge (internal; a retry skips detecting them)
	MergeEntryIndices []int `json:"merge_entry_indices,omitempty"`
	// chunks of the batch being written (internal; a retry or the final failure deletes those still unindexed)
	PendingChunkIDs []string         `json:"pending_chunk_ids,omitempty"`
	MergedCount     int              `json:"merged_count,omitempty"`  // entries merged into existing ones
	AddedCount      int              `json:"added_count,omitempty"`   // entries added
	MergeDetails    []FAQMergeDetail `json:"merge_details,omitempty"` // merge details
	Message         string           `json:"message"`                 // Status message
	Error           string           `json:"error"`                   // Error message if failed
	CreatedAt       int64            `json:"created_at"`              // Task creation timestamp
	UpdatedAt       int64            `json:"updated_at"`              // Last update timestamp
	DryRun          bool             `json:"dry_run,omitempty"`       // dry run mode

	// Result fields (populated when Status == "completed")
	ImportMode     string    `json:"import_mode,omitempty"`     // append or replace
	ImportedAt     time.Time `json:"imported_at,omitempty"`     // import completion time
	DisplayStatus  string    `json:"display_status,omitempty"`  // open or close
	ProcessingTime int64     `json:"processing_time,omitempty"` // processing time (ms)
}

// FAQImportMetadata is FAQ import task info stored in Knowledge.Metadata.
// Deprecated: Use FAQImportProgress with Redis storage instead
type FAQImportMetadata struct {
	ImportProgress  int `json:"import_progress"` // 0-100
	ImportTotal     int `json:"import_total"`
	ImportProcessed int `json:"import_processed"`
}

// FAQImportResult holds the statistics of a finished FAQ import. It is
// persisted independently of the progress state until the next import
// replaces it.
type FAQImportResult struct {
	TotalEntries int `json:"total_entries"` // total entries
	SuccessCount int `json:"success_count"` // fully successful entries (partial failures excluded)
	FailedCount  int `json:"failed_count"`  // fully failed entries
	// partially failed entries (similar/negative questions removed, entry imported)
	PartialFailedCount int `json:"partial_failed_count"`
	SkippedCount       int `json:"skipped_count"` // skipped entries (duplicates and the like)
	MergedCount        int `json:"merged_count"`  // entries merged into existing ones
	AddedCount         int `json:"added_count"`   // entries added

	ImportMode string    `json:"import_mode"` // append or replace
	ImportedAt time.Time `json:"imported_at"` // import completion time
	TaskID     string    `json:"task_id"`     // import task ID

	// Download link offered when many entries failed
	FailedEntriesURL string `json:"failed_entries_url,omitempty"` // failed entries CSV download URL

	DisplayStatus string `json:"display_status"` // open or close

	ProcessingTime int64 `json:"processing_time"` // processing time (ms)
}

// ToJSON converts the metadata to JSON type.
func (m *FAQImportMetadata) ToJSON() (JSON, error) {
	if m == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return JSON(bytes), nil
}

// ToJSON converts the import result to JSON type.
func (r *FAQImportResult) ToJSON() (JSON, error) {
	if r == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return JSON(bytes), nil
}

// ParseFAQImportMetadata parses FAQ import metadata from Knowledge.
func ParseFAQImportMetadata(k *Knowledge) (*FAQImportMetadata, error) {
	if k == nil || len(k.Metadata) == 0 {
		return nil, nil
	}
	var metadata FAQImportMetadata
	if err := json.Unmarshal(k.Metadata, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

// normalizeQuestionStrings normalizes and dedupes a question list (fullwidth
// to halfwidth, trailing punctuation removed, spaces collapsed, and so on).
func normalizeQuestionStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	dedup := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		normalized := NormalizeQuestion(v)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		dedup = append(dedup, normalized)
	}
	if len(dedup) == 0 {
		return nil
	}
	return dedup
}

var multiSpaceRegex = regexp.MustCompile(`\s+`)

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)

// URLNormMode selects how URLs are normalized.
type URLNormMode int

const (
	// URLRemove removes the URL entirely
	URLRemove URLNormMode = iota
	// URLPlaceholder replaces it with the placeholder <URL>
	URLPlaceholder
	// URLKeepDomain keeps only the domain
	URLKeepDomain
	// URLKeepDomainAndPath keeps the domain and path
	URLKeepDomainAndPath
)

// NormalizeQuestion normalizes question text to raise the vector match rate.
// The order follows the reference
// query = convert_st(trim_url(query.lower().strip().strip(<punctuation>)), 1):
// 1. Trim surrounding whitespace
// 2. Remove URLs
// 3. Lowercase
// 4. Trim leading and trailing punctuation
// 5. Traditional to Simplified Chinese
// 6. Fullwidth to halfwidth
// 7. Space handling (drop spaces next to CJK, keep them between ASCII letters/digits)
func NormalizeQuestion(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}

	q = trimURL(q)

	q = strings.ToLower(q)

	// Fullwidth ? . , ; enumeration comma, fullwidth : and ! plus their ASCII forms.
	q = strings.Trim(q, "\uff1f\u3002\uff0c\uff1b\u3001\uff1a\"\uff01?.,;!:'")

	q = toSimplified(q)

	q = toHalfWidth(q)

	q = normalizeSpaces(q)

	return strings.TrimSpace(q)
}

// normalizeSpaces collapses whitespace runs and keeps a space only between two
// ASCII letters or digits, so spaces between CJK words disappear:
// "iphone 15 <CJK> <CJK>" becomes "iphone 15<CJK><CJK>".
func normalizeSpaces(s string) string {
	s = multiSpaceRegex.ReplaceAllString(s, " ")

	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(s))

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if r != ' ' {
			builder.WriteRune(r)
			continue
		}

		var prevRune rune
		if i > 0 {
			prevRune = runes[i-1]
		}

		var nextRune rune
		for j := i + 1; j < len(runes); j++ {
			if runes[j] != ' ' {
				nextRune = runes[j]
				break
			}
		}

		if isASCIIAlphaNum(prevRune) && isASCIIAlphaNum(nextRune) {
			builder.WriteRune(' ')
		}
	}

	return builder.String()
}

// isASCIIAlphaNum reports whether r is an ASCII letter or digit.
func isASCIIAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// trimURL removes URLs from s (URLRemove mode).
func trimURL(s string) string {
	return NormalizeURL(s, URLRemove)
}

// NormalizeURL rewrites the URLs in text according to mode.
func NormalizeURL(text string, mode URLNormMode) string {
	return urlRegex.ReplaceAllStringFunc(text, func(raw string) string {
		switch mode {
		case URLRemove:
			return ""
		case URLPlaceholder:
			return "<URL>"
		case URLKeepDomain:
			domain, _ := parseURL(raw)
			if domain != "" {
				return domain
			}
			return "<URL>"
		case URLKeepDomainAndPath:
			domain, path := parseURL(raw)
			if domain != "" {
				return domain + path
			}
			return "<URL>"
		default:
			return "<URL>"
		}
	})
}

// parseURL returns the domain and path of a URL.
func parseURL(raw string) (domain, path string) {
	u := raw
	if strings.HasPrefix(u, "https://") {
		u = u[8:]
	} else if strings.HasPrefix(u, "http://") {
		u = u[7:]
	}

	slashIdx := strings.Index(u, "/")
	if slashIdx == -1 {
		// No path: the whole thing is the domain, possibly with a query string.
		queryIdx := strings.Index(u, "?")
		if queryIdx != -1 {
			domain = u[:queryIdx]
		} else {
			domain = u
		}
		return domain, ""
	}

	domain = u[:slashIdx]
	path = u[slashIdx:]

	// Drop the query string and fragment.
	if queryIdx := strings.Index(path, "?"); queryIdx != -1 {
		path = path[:queryIdx]
	}
	if fragIdx := strings.Index(path, "#"); fragIdx != -1 {
		path = path[:fragIdx]
	}

	return domain, path
}

// toSimplified converts Traditional Chinese to Simplified Chinese.
func toSimplified(s string) string {
	return textconv.ToSimplified(s)
}

// toHalfWidth converts the ideographic space and fullwidth ASCII forms
// (punctuation included) to their halfwidth equivalents.
func toHalfWidth(s string) string {
	var builder strings.Builder
	builder.Grow(len(s))

	for _, r := range s {
		switch {
		// Ideographic space -> ASCII space
		case r == '\u3000':
			builder.WriteRune(' ')
		// Fullwidth ASCII (U+FF01 to U+FF5E) -> U+0021 to U+007E
		case r >= 0xFF01 && r <= 0xFF5E:
			builder.WriteRune(r - 0xFF01 + 0x21)
		default:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

// NormalizeQueryText normalizes search query text with the same steps as
// NormalizeQuestion.
func NormalizeQueryText(q string) string {
	return NormalizeQuestion(q)
}

// IsChineseChar reports whether r is a Han character.
func IsChineseChar(r rune) bool {
	return unicode.Is(unicode.Han, r)
}
