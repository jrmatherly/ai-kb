package asr

import (
	"context"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// Segment represents a transcribed segment with timestamps.
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscriptionResult holds the full text and its segments.
type TranscriptionResult struct {
	Text     string    `json:"text"`
	Segments []Segment `json:"segments,omitempty"`
	// Duration is the audio length in seconds when the vendor reports it,
	// which several do without segments; 0 when unknown.
	Duration float64 `json:"duration,omitempty"`
}

type languageKey struct{}

// WithLanguage attaches the operator's language hint (for example "zh") to
// a transcription. Empty and "auto" mean auto-detection, which is every
// vendor's default, and send nothing.
func WithLanguage(ctx context.Context, language string) context.Context {
	return context.WithValue(ctx, languageKey{}, language)
}

func languageFrom(ctx context.Context) string {
	language, _ := ctx.Value(languageKey{}).(string)
	language = strings.TrimSpace(language)
	if strings.EqualFold(language, "auto") {
		return ""
	}
	return language
}

// ASR defines the interface for Automatic Speech Recognition model operations.
type ASR interface {
	// Transcribe sends audio bytes to the ASR model and returns the transcribed text and segments.
	Transcribe(ctx context.Context, audioBytes []byte, fileName string) (*TranscriptionResult, error)

	GetModelName() string
	GetModelID() string
}

// Config holds the configuration needed to create an ASR instance.
type Config struct {
	Source    types.ModelSource
	BaseURL   string
	ModelName string
	APIKey    string
	ModelID   string
	// Provider is the vendor id stored on the row; empty detects it from
	// BaseURL.
	Provider    string
	Spec        *types.ModelSpecOverride `json:"spec,omitempty"`
	ExtraConfig map[string]string
	// CustomHeaders adds custom HTTP request headers to remote API calls (like extra_headers in the OpenAI Python SDK).
	CustomHeaders map[string]string
}

// ConfigFromModel builds an asr.Config from a types.Model.
// The production path (loaded from the DB) and the test-connection path (a temporary form) share this mapping.
// ASR uses no WeKnoraCloud credentials today, so the signature has no appID/appSecret.
func ConfigFromModel(m *types.Model) *Config {
	if m == nil {
		return nil
	}
	return &Config{
		ModelID:       m.ID,
		APIKey:        m.Parameters.APIKey,
		BaseURL:       m.Parameters.BaseURL,
		ModelName:     m.Name,
		Source:        m.Source,
		Provider:      m.Parameters.Provider,
		Spec:          m.Parameters.Spec,
		ExtraConfig:   m.Parameters.ExtraConfig,
		CustomHeaders: m.Parameters.CustomHeaders,
	}
}

// NewASR creates an ASR instance based on the provided configuration.
func NewASR(config *Config) (ASR, error) {
	a, err := newASR(config)
	return wrapASRLangfuse(a, err)
}
