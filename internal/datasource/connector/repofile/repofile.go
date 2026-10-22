// Package repofile holds the repository path rules shared by the git connectors.
package repofile

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
)

// NormalizePath trims a configured repository path and rejects traversal and
// backslashes. An empty result means the repository root.
func NormalizePath(value string) (string, error) {
	v := strings.Trim(strings.TrimSpace(value), "/")
	if v == "" {
		return "", nil
	}
	if strings.Contains(v, "\\") {
		return "", fmt.Errorf("%w: path must use forward slashes", datasource.ErrInvalidConfig)
	}
	if path.Clean(v) != v || v == "." || strings.HasPrefix(v, "../") || strings.Contains(v, "/../") {
		return "", fmt.Errorf("%w: invalid repository path", datasource.ErrInvalidConfig)
	}
	return v, nil
}

// CollapsePaths sorts paths in place and drops duplicates and paths nested
// under another. Nil means the whole repository.
func CollapsePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	sort.Strings(paths)
	for _, p := range paths {
		if p == "" {
			return nil
		}
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if len(out) > 0 && (p == out[len(out)-1] || strings.HasPrefix(p, out[len(out)-1]+"/")) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// InScope reports whether file is one of roots or below one. No roots means
// every file is in scope.
func InScope(file string, roots []string) bool {
	if len(roots) == 0 {
		return true
	}
	for _, r := range roots {
		if file == r || strings.HasPrefix(file, r+"/") {
			return true
		}
	}
	return false
}

// KnowledgeRelativePath maps a repository file to the KB folder convention:
// <repository name>-<branch>/<repository-relative file path>.
func KnowledgeRelativePath(root, ref, file string) string {
	dir := strings.TrimSpace(root) + "-" + strings.ReplaceAll(strings.TrimSpace(ref), "/", "-")
	return path.Join(dir, file)
}
