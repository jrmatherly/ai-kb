package types

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// WebSearchConfig represents the web search configuration for a tenant
type WebSearchConfig struct {
	// Deprecated: Use WebSearchProviderEntity.Parameters.APIKey instead.
	Provider string `json:"provider,omitempty"`
	// Deprecated: Use WebSearchProviderEntity.Parameters.APIKey instead.
	APIKey string `json:"api_key,omitempty"`

	// Per-call Agent filters; never persisted in tenant configuration.
	Filters WebSearchFilters `json:"-"`

	MaxResults        int      `json:"max_results"`        // maximum number of results
	IncludeDate       bool     `json:"include_date"`       // include dates
	CompressionMethod string   `json:"compression_method"` // compression method: none, summary, extract, rag
	Blacklist         []string `json:"blacklist"`          // blacklist rules
	// RAG compression settings
	EmbeddingModelID   string `json:"embedding_model_id,omitempty"`  // embedding model ID (RAG compression)
	EmbeddingDimension int    `json:"embedding_dimension,omitempty"` // embedding dimension (RAG compression)
	RerankModelID      string `json:"rerank_model_id,omitempty"`     // rerank model ID (RAG compression)
	DocumentFragments  int    `json:"document_fragments,omitempty"`  // number of document fragments (RAG compression)
	ProxyURL           string `json:"proxy_url,omitempty"`           // Optional per-request proxy override; normally empty — use WebSearchProviderEntity.Parameters.proxy_url. Merged at call time when set.
}

const (
	DefaultWebSearchMaxResults        = 10
	DefaultWebSearchCompressionMethod = "none"
)

// DefaultWebSearchConfig returns the shared default tenant-level web search configuration.
func DefaultWebSearchConfig() *WebSearchConfig {
	return &WebSearchConfig{
		MaxResults:        DefaultWebSearchMaxResults,
		IncludeDate:       false,
		CompressionMethod: DefaultWebSearchCompressionMethod,
		Blacklist:         []string{},
	}
}

// EffectiveWebSearchConfig normalizes a possibly empty config to the effective runtime config.
func EffectiveWebSearchConfig(cfg *WebSearchConfig) *WebSearchConfig {
	if cfg == nil {
		return DefaultWebSearchConfig()
	}

	normalized := *cfg
	if normalized.MaxResults <= 0 {
		normalized.MaxResults = DefaultWebSearchMaxResults
	}
	if normalized.CompressionMethod == "" {
		normalized.CompressionMethod = DefaultWebSearchCompressionMethod
	}
	if normalized.Blacklist == nil {
		normalized.Blacklist = []string{}
	}

	return &normalized
}

// Value implements driver.Valuer interface for WebSearchConfig
func (c WebSearchConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements sql.Scanner interface for WebSearchConfig
func (c *WebSearchConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// WebSearchResult represents a single web search result
type WebSearchResult struct {
	Title   string `json:"title"`   // result title
	URL     string `json:"url"`     // result URL
	Snippet string `json:"snippet"` // snippet
	Content string `json:"content"` // full content (optional, needs an extra fetch)
	Source  string `json:"source"`  // source (for example duckduckgo)
	// Provider-reported age, without inventing an exact publication date.
	Age string `json:"age,omitempty"`

	PublishedAt *time.Time `json:"published_at,omitempty"` // publish time, if any
}

// WebSearchProviderInfo represents information about a web search provider
type WebSearchProviderInfo struct {
	ID             string `json:"id"`                // provider ID
	Name           string `json:"name"`              // provider name
	Free           bool   `json:"free"`              // free to use
	RequiresAPIKey bool   `json:"requires_api_key"`  // needs an API key
	Description    string `json:"description"`       // description
	APIURL         string `json:"api_url,omitempty"` // API URL (optional)
}
