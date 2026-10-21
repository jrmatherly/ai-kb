package storageallowlist

import (
	"os"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/envlist"
)

const AllowListEnv = "STORAGE_ALLOW_LIST"

var supported = []string{"local", "minio", "cos", "tos", "s3", "oss", "ks3", "obs"}

// Supported returns the canonical storage provider names in display order.
func Supported() []string {
	providers := make([]string, len(supported))
	copy(providers, supported)
	return providers
}

// AllowedMap returns which providers are permitted by STORAGE_ALLOW_LIST.
// Unknown names are ignored without a warning.
func AllowedMap() map[string]bool {
	enabled, _ := envlist.Parse(os.Getenv(AllowListEnv), supported)
	allowed := make(map[string]bool, len(enabled))
	for provider := range enabled {
		allowed[provider] = true
	}
	return allowed
}

// IsAllowed reports whether provider is permitted. Empty provider is treated as allowed.
func IsAllowed(provider string) bool {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return true
	}
	return AllowedMap()[provider]
}

// FirstAllowed returns the first supported provider allowed by STORAGE_ALLOW_LIST.
func FirstAllowed() string {
	allowed := AllowedMap()
	for _, provider := range supported {
		if allowed[provider] {
			return provider
		}
	}
	return ""
}

// AllowedList returns allowed providers in canonical order.
func AllowedList() []string {
	allowed := AllowedMap()
	out := make([]string, 0, len(supported))
	for _, provider := range supported {
		if allowed[provider] {
			out = append(out, provider)
		}
	}
	return out
}
