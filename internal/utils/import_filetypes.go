package utils

import (
	"sort"
	"strings"
)

// supportedImportFileExtensions is the single source of truth for extensions
// accepted by every knowledge import path: direct upload, file-URL download,
// the worker's post-download re-check and data source connectors. Keeping one
// set avoids the drift that let direct upload accept xlsx while URL import
// rejected it (#2447).
var supportedImportFileExtensions = map[string]struct{}{
	"pdf": {}, "txt": {}, "docx": {}, "doc": {}, "epub": {},
	"html": {}, "htm": {}, "mhtml": {}, "md": {}, "markdown": {}, "mdx": {},
	"xmind": {},
	"png":   {}, "jpg": {}, "jpeg": {}, "gif": {},
	"csv": {}, "xlsx": {}, "xls": {}, "xlsb": {}, "pptx": {}, "ppt": {}, "json": {},
	"mp3": {}, "wav": {}, "m4a": {}, "flac": {}, "ogg": {},
}

// NormalizeImportExtension lowercases an extension and strips surrounding
// whitespace and one leading dot, so callers can pass "xlsx", ".XLSX" or a
// raw user-supplied file_type.
func NormalizeImportExtension(ext string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}

// IsSupportedImportExtension reports whether a bare extension can be imported.
func IsSupportedImportExtension(ext string) bool {
	ext = NormalizeImportExtension(ext)
	if ext == "" {
		return false
	}
	_, ok := supportedImportFileExtensions[ext]
	return ok
}

// SupportedImportExtensions returns a sorted copy of the supported extensions.
func SupportedImportExtensions() []string {
	out := make([]string, 0, len(supportedImportFileExtensions))
	for ext := range supportedImportFileExtensions {
		out = append(out, ext)
	}
	sort.Strings(out)
	return out
}

// The image table is wider than the import set on purpose: these classify a stored file_type;
// IsImportable adds the support check.
var (
	imageExtensions = map[string]struct{}{
		"jpg": {}, "jpeg": {}, "png": {}, "gif": {}, "webp": {}, "bmp": {}, "svg": {}, "tiff": {},
	}
	audioExtensions = map[string]struct{}{"mp3": {}, "wav": {}, "m4a": {}, "flac": {}, "ogg": {}}
)

// ImportCapabilities records which model-dependent file kinds a knowledge base can ingest.
type ImportCapabilities struct {
	Images bool `json:"images"`
	Audio  bool `json:"audio"`
}

// AllImportCapabilities is a knowledge base with both a vision and a speech model.
var AllImportCapabilities = ImportCapabilities{Images: true, Audio: true}

// Gained reports whether c can ingest a kind that prev could not.
func (c ImportCapabilities) Gained(prev ImportCapabilities) bool {
	return c.Images && !prev.Images || c.Audio && !prev.Audio
}

// IsImageExtension reports whether ext is an image format.
func IsImageExtension(ext string) bool {
	_, ok := imageExtensions[NormalizeImportExtension(ext)]
	return ok
}

// IsAudioExtension reports whether ext is an audio format.
func IsAudioExtension(ext string) bool {
	_, ok := audioExtensions[NormalizeImportExtension(ext)]
	return ok
}

// MissingModel names the model kind ("vision" or "speech") that ext needs and caps lacks, or "".
func MissingModel(ext string, caps ImportCapabilities) string {
	switch {
	case IsImageExtension(ext) && !caps.Images:
		return "vision"
	case IsAudioExtension(ext) && !caps.Audio:
		return "speech"
	}
	return ""
}

// IsImportable reports whether a file with ext is supported and has the model it needs.
func IsImportable(ext string, caps ImportCapabilities) bool {
	return IsSupportedImportExtension(ext) && MissingModel(ext, caps) == ""
}
