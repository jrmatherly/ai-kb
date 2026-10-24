package msgraph

import (
	"path"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
)

// knowledgeFileName joins sanitized path segments into the knowledge base
// folder convention "<drive>/<folders>/<file>"; the extension survives so the
// parser is chosen from it. Depth and length limits are applied by the service.
func knowledgeFileName(segments []string) string {
	if len(segments) == 0 {
		return datasource.SanitizeFileName("")
	}
	out := make([]string, len(segments))
	last := len(segments) - 1
	for i := 0; i < last; i++ {
		out[i] = datasource.SanitizeFileName(segments[i])
	}
	ext := path.Ext(segments[last])
	out[last] = datasource.SanitizeFileName(strings.TrimSuffix(segments[last], ext)) + ext
	return strings.Join(out, "/")
}
