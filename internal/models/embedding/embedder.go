package embedding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/models/api"
	"github.com/jrmatherly/ai-kb/internal/models/imageprep"
	"github.com/jrmatherly/ai-kb/internal/models/utils/ollama"
	"github.com/jrmatherly/ai-kb/internal/tracing/langfuse"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// Embedder defines the interface for text vectorization
type Embedder interface {
	// Embed converts text to vector
	Embed(ctx context.Context, text string) ([]float32, error)

	// BatchEmbed converts multiple texts to vectors in batch
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)

	// GetModelName returns the model name
	GetModelName() string

	// GetDimensions returns the vector dimensions
	GetDimensions() int

	// GetModelID returns the model ID
	GetModelID() string

	EmbedderPooler
}

// Image is one image to embed: its bytes and MIME type.
type Image = api.EmbedImage

// ErrImagesUnsupported is returned when an embedder is asked for an image
// vector its model or endpoint cannot produce.
var ErrImagesUnsupported = errors.New("embedding model does not accept images")

// ImageEmbedder is the image side of an Embedder whose model maps images into
// the same space as its text vectors, so that a text query can find an image.
// It is a separate interface because most embedders have no image side; use
// AsImageEmbedder rather than a type assertion, since every decorator
// implements it whether or not the model underneath does.
type ImageEmbedder interface {
	// AcceptsImages reports whether both the model and its endpoint take
	// images.
	AcceptsImages() bool
	// ImageLimits are the vendor's documented per-image limits. The caller
	// shrinks or converts an image to fit; an image outside them is refused.
	ImageLimits() ImageLimits
	// BatchEmbedImages converts images to vectors comparable with Embed's.
	BatchEmbedImages(ctx context.Context, images []Image) ([][]float32, error)
}

// ImageLimits describes what one image may be; see imageprep.Limits.
type ImageLimits = imageprep.Limits

// AsImageEmbedder returns e's image side when its model accepts images.
func AsImageEmbedder(e Embedder) (ImageEmbedder, bool) {
	ie, ok := e.(ImageEmbedder)
	if !ok || !ie.AcceptsImages() {
		return nil, false
	}
	return ie, true
}

// imageSide is AsImageEmbedder for decorators, which must fail a call their
// inner embedder cannot serve.
func imageSide(e Embedder) (ImageEmbedder, error) {
	if ie, ok := AsImageEmbedder(e); ok {
		return ie, nil
	}
	return nil, fmt.Errorf("%s: %w", e.GetModelName(), ErrImagesUnsupported)
}

func imageLimitsOf(e Embedder) ImageLimits {
	if ie, ok := AsImageEmbedder(e); ok {
		return ie.ImageLimits()
	}
	return ImageLimits{}
}

type EmbedderPooler interface {
	BatchEmbedWithPool(ctx context.Context, model Embedder, texts []string) ([][]float32, error)
}

// EmbedderType represents the embedder type
type EmbedderType string

// Config represents the embedder configuration
type Config struct {
	Source                    types.ModelSource `json:"source"`
	BaseURL                   string            `json:"base_url"`
	ModelName                 string            `json:"model_name"`
	APIKey                    string            `json:"api_key"`
	TruncatePromptTokens      int               `json:"truncate_prompt_tokens"`
	Dimensions                int               `json:"dimensions"`
	SupportsDimensionOverride bool              `json:"supports_dimension_override"`
	ModelID                   string            `json:"model_id"`
	Provider                  string            `json:"provider"`
	// MaxConcurrency caps concurrent background calls to this model; 0 falls
	// back to the process-wide default (see limiter.GateN).
	MaxConcurrency int                      `json:"max_concurrency"`
	Spec           *types.ModelSpecOverride `json:"spec,omitempty"`
	ExtraConfig    map[string]string        `json:"extra_config"`
	// CustomHeaders adds custom HTTP request headers to remote API calls (like extra_headers in the OpenAI Python SDK).
	CustomHeaders map[string]string `json:"custom_headers"`
	AppID         string
	AppSecret     string // encrypted value, passed in by the factory's caller and decrypted before use
}

// ConfigFromModel builds an embedding.Config from a types.Model.
// The production path (loaded from the DB) and the test-connection path (a temporary form) share this mapping.
// appID / appSecret are the decrypted WeKnoraCloud credentials, passed in by the caller.
func ConfigFromModel(m *types.Model, appID, appSecret string) Config {
	if m == nil {
		return Config{}
	}
	return Config{
		Source:                    m.Source,
		BaseURL:                   m.Parameters.BaseURL,
		APIKey:                    m.Parameters.APIKey,
		ModelID:                   m.ID,
		ModelName:                 m.Name,
		Dimensions:                m.Parameters.EmbeddingParameters.Dimension,
		SupportsDimensionOverride: m.Parameters.EmbeddingParameters.SupportsDimensionOverride,
		TruncatePromptTokens:      m.Parameters.EmbeddingParameters.TruncatePromptTokens,
		Provider:                  m.Parameters.Provider,
		MaxConcurrency:            m.Parameters.MaxConcurrency,
		Spec:                      m.Parameters.Spec,
		ExtraConfig:               m.Parameters.ExtraConfig,
		CustomHeaders:             m.Parameters.CustomHeaders,
		AppID:                     appID,
		AppSecret:                 appSecret,
	}
}

// NewEmbedder creates an embedder based on the configuration
func NewEmbedder(config Config, pooler EmbedderPooler, ollamaService *ollama.OllamaService) (Embedder, error) {
	e, err := newEmbedder(config, pooler, ollamaService)
	if err != nil {
		return e, err
	}
	if setter, ok := e.(interface{ SetSupportsDimensionOverride(bool) }); ok {
		setter.SetSupportsDimensionOverride(config.SupportsDimensionOverride)
	}
	// Innermost: gate the real provider round-trips (including the per-sub-batch
	// pool callbacks) before debug/langfuse wrap for logging/tracing. See
	// concurrencyEmbedder for why this sits below the observability decorators.
	e = wrapEmbeddingConcurrency(e, config.MaxConcurrency)
	if logger.LLMDebugEnabled() {
		e = &debugEmbedder{inner: e}
	}
	if langfuse.GetManager().Enabled() {
		e = &langfuseEmbedder{inner: e}
	}
	return e, nil
}

func newEmbedder(config Config, pooler EmbedderPooler, ollamaService *ollama.OllamaService) (Embedder, error) {
	switch strings.ToLower(string(config.Source)) {
	case string(types.ModelSourceLocal):
		return NewOllamaEmbedder(config.BaseURL,
			config.ModelName, config.TruncatePromptTokens, config.Dimensions, config.ModelID, pooler, ollamaService)
	case string(types.ModelSourceRemote):
		return newRemoteEmbedder(config, pooler)
	default:
		return nil, fmt.Errorf("unsupported embedder source: %s", config.Source)
	}
}
